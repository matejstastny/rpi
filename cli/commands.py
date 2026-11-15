"""what each subcommand does.

read-only commands fan out across the fleet in parallel and their output is
buffered per host, so three boxes answering at once still read top to bottom in
fleet order. anything that writes runs one host at a time on purpose.
"""

from __future__ import annotations

import os
import re
import shlex
import subprocess
import tempfile
import time
from concurrent.futures import ThreadPoolExecutor
from dataclasses import dataclass, field
from pathlib import Path

from . import Fault, adguard, manifest, payload, remote, ui
from .fleet import Credentials, credentials_for, sshpass_available
from .manifest import ROOT, Host


@dataclass
class Outcome:
    host: str
    state: str = "ok"
    lines: list[str] = field(default_factory=list)
    elapsed: float = 0.0
    data: dict = field(default_factory=dict)


def human(size: float) -> str:
    """matches internal/human.Bytes so build output and logs agree"""
    if size <= 0:
        return "0 B"
    for unit in ("B", "kB", "MB", "GB"):
        if size < 1000:
            return f"{size:.0f} {unit}" if unit == "B" else f"{size:.1f} {unit}"
        size /= 1000
    return f"{size:.1f} TB"


# ── fan-out ─────────────────────────────────────────────────────────────────


def fan_out(names: list[str], work, parallel: bool = True) -> dict[str, Outcome]:
    """run work(host, creds) for each host, rendering results in fleet order"""

    def one(name: str) -> Outcome:
        start = time.monotonic()
        try:
            creds = credentials_for(name)
        except LookupError as exc:
            return Outcome(name, "no credentials", [str(exc)])
        try:
            out = work(manifest.host(name), creds)
        except remote.Unreachable as exc:
            out = Outcome(name, "unreachable", [str(exc)])
        except Fault as exc:
            out = Outcome(name, "failed", [str(exc)])
        except Exception as exc:  # one bad box never takes the others down
            out = Outcome(name, "failed", [f"{type(exc).__name__}: {exc}"])
        out.elapsed = time.monotonic() - start
        return out

    if parallel and len(names) > 1:
        with ThreadPoolExecutor(max_workers=len(names)) as pool:
            done = list(pool.map(one, names))
    else:
        done = [one(n) for n in names]
    return {o.host: o for o in done}


def report(results: dict[str, Outcome], started: float) -> int:
    ui.summary({n: o.state for n, o in results.items()}, time.monotonic() - started)
    return 0 if all(o.state == "ok" for o in results.values()) else 1


# ── sync ────────────────────────────────────────────────────────────────────


def build_binaries(host: Host) -> tuple[bool, str]:
    targets = [a.name for a in manifest.apps_on(host.name)]
    if not targets:
        return True, "nothing to build"

    proc = subprocess.run(
        ["node", "tools/build.ts", *targets],
        cwd=ROOT,
        capture_output=ui.LEVEL < 2,
    )
    if proc.returncode != 0:
        if proc.stderr:
            ui.detail(proc.stderr.decode(errors="replace"))
        return False, "build failed"

    size = sum(
        (ROOT / "build" / n).stat().st_size for n in targets if (ROOT / "build" / n).is_file()
    )
    return True, f"{' '.join(targets)}  {human(size)}"


def sync(args) -> int:
    started = time.monotonic()
    names = select(args)
    if not sshpass_available():
        ui.warn("sshpass not found, only key-auth hosts will work")
    if args.dry_run:
        ui.line(ui.paint(ui.DIM, "  dry run, nothing is written"))

    results: dict[str, Outcome] = {}
    for name in names:
        host = manifest.host(name)
        ui.heading(host)
        out = Outcome(name)
        start = time.monotonic()

        try:
            creds = credentials_for(name)
        except LookupError as exc:
            ui.failed(str(exc))
            results[name] = Outcome(name, "no credentials")
            continue

        try:
            out = sync_host(host, creds, args)
        except remote.Unreachable as exc:
            ui.skipped(str(exc))
            out = Outcome(name, "unreachable")
        except Fault as exc:
            ui.failed(str(exc))
            out = Outcome(name, "failed")
        except Exception as exc:
            ui.failed(f"{type(exc).__name__}: {exc}")
            out = Outcome(name, "failed")

        out.elapsed = time.monotonic() - start
        if out.state == "ok":
            ui.ok("done", out.elapsed)
        results[name] = out

    return report(results, started)


def sync_host(host: Host, creds: Credentials, args) -> Outcome:
    out = Outcome(host.name)
    services = args.services and host.installable

    if services and not args.dry_run:
        with ui.timed() as clock:
            built, detail = build_binaries(host)
        ui.step("build", detail, clock[0])
        if not built:
            out.state = "failed"
            return out

    with tempfile.TemporaryDirectory() as tmp:
        bundle = payload.build(host, Path(tmp) / "payload", with_binaries=services)
        ui.step("stage", f"{len(bundle.names)} files, {human(bundle.size)}")
        for name in bundle.names:
            ui.detail(f"~/{name}")

        if args.dry_run:
            if args.bootstrap:
                ui.step("bootstrap", "would run bin/setup-pi")
            if services:
                ui.step("install", f"would run hosts/{host.name}/install")
            return out

        remote.require_reachable(creds)

        # a --services sync carries a complete payload, so the host's service
        # directory is replaced wholesale and a file deleted from hosts/<name>/
        # stops lingering on the box. a plain config sync leaves it alone,
        # since it does not ship the binaries that live there.
        prune = ['rm -rf "$HOME/scripts"']
        if services:
            prune.append(f'rm -rf "$HOME/{payload.SERVICES_HOME}/{host.name}"')

        with ui.timed() as clock:
            res = remote.run(
                creds,
                " && ".join(prune) + ' && tar -C "$HOME" -xf -',
                stdin=bundle.tar(),
            )
        if not res.ok:
            ui.failed(f"push failed: {res.message}")
            out.state = "failed"
            return out
        ui.step("push", "unpacked into ~", clock[0])

    if args.bootstrap:
        script = (ROOT / "bin" / "setup-pi").read_bytes()
        with ui.timed() as clock:
            res = remote.sudo(creds, "exec sh -s", extra=script)
        relay(res)
        if not res.ok:
            ui.failed(f"bootstrap failed: {res.message}")
            out.state = "failed"
            return out
        ui.step("bootstrap", "packages and login shell", clock[0])

    if services:
        target = f"~/.local/share/pi-services/{host.name}/install"
        with ui.timed() as clock:
            res = remote.sudo(creds, f"exec sh {target}")
        steps = sum(1 for ln in res.out.splitlines() if ln.startswith("▸"))
        relay(res)
        if not res.ok:
            ui.failed(f"install failed: {res.message}")
            out.state = "failed"
            return out
        ui.step("install", f"{steps} steps", clock[0])

    return out


def relay(res: remote.Result) -> None:
    """the install scripts' own output: steps when verbose, `!` notes always"""
    for ln in res.out.splitlines():
        text = ln.strip()
        if text.startswith("!"):
            ui.note(text)
        elif text:
            ui.detail(text)


# ── diff ────────────────────────────────────────────────────────────────────


def diff(args) -> int:
    """which staged files actually differ from what is on the box"""
    started = time.monotonic()

    def work(host: Host, creds: Credentials) -> Outcome:
        remote.require_reachable(creds)
        with tempfile.TemporaryDirectory() as tmp:
            bundle = payload.build(host, Path(tmp) / "payload")
            want = bundle.digests()

        listing = " ".join(shlex.quote(n) for n in want)
        script = (
            'cd "$HOME" || exit 1; for f in '
            + listing
            + '; do if [ -f "$f" ]; then sha256sum "$f"; else echo "absent  $f"; fi; done'
        )
        res = remote.run(creds, script)
        if not res.ok:
            return Outcome(host.name, "failed", [res.message])

        live: dict[str, str] = {}
        for ln in res.out.splitlines():
            parts = ln.split(None, 1)
            if len(parts) == 2:
                live[parts[1].strip()] = parts[0]

        # a file the box said nothing about is unknown, not identical; without
        # this an empty or garbled reply would read as a clean "in sync"
        if silent := [n for n in want if n not in live]:
            return Outcome(
                host.name,
                "failed",
                [f"no answer for {len(silent)} of {len(want)} files, first: {silent[0]}"],
            )

        new = [n for n in want if live.get(n) == "absent"]
        changed = [n for n in want if n in live and live[n] not in ("absent", want[n])]
        same = len(want) - len(new) - len(changed)
        return Outcome(host.name, "ok", data={"new": new, "changed": changed, "same": same})

    results = fan_out(select(args), work)
    for name in select(args):
        out = results[name]
        ui.heading(manifest.host(name))
        if out.state != "ok":
            ui.failed("\n".join(out.lines) or out.state)
            continue

        new, changed, same = out.data["new"], out.data["changed"], out.data["same"]
        if not new and not changed:
            ui.ok(f"in sync, {same} files identical", out.elapsed)
            continue

        ui.step("changes", f"{len(new)} new, {len(changed)} differing, {same} identical")
        for n in new:
            ui.added(f"~/{n}")
        for n in changed:
            ui.changed(f"~/{n}")

    return report(results, started)


# ── dns ─────────────────────────────────────────────────────────────────────


def adguard_password(creds: Credentials) -> str:
    if env := os.environ.get("PI_ADGUARD_PASS", "").strip():
        return env
    if creds.adguardpsw:
        return creds.adguardpsw
    raise Fault(
        "no adguard password. either\n"
        "  export PI_ADGUARD_PASS=...\n"
        f"or add a line above [[hosts]] {creds.name} in your remote-hosts file:\n"
        "  # adguardpsw: <the adguard web login password>"
    )


def dns(args) -> int:
    """reconcile every resolver against dns.toml"""
    started = time.monotonic()
    names = [n for n in manifest.resolvers() if not args.hosts or n in args.hosts]
    if unknown := sorted(set(args.hosts or []) - set(manifest.resolvers())):
        ui.error(
            f"{', '.join(unknown)} does not run an adguard; "
            f"dns.toml lists {', '.join(manifest.resolvers())}"
        )
        return 1

    def work(host: Host, creds: Credentials) -> Outcome:
        client = adguard.client_for(host.name, creds, adguard_password(creds))
        result = adguard.plan(client, host.name)
        if not args.check and not result.empty:
            adguard.apply(client, result)
        return Outcome(host.name, "ok", data={"plan": result, "client": client})

    results = fan_out(names, work, parallel=args.check)

    for name in names:
        out = results[name]
        ui.heading(manifest.host(name))
        if out.state != "ok":
            ui.failed("\n".join(out.lines) or out.state)
            continue

        p: adguard.Plan = out.data["plan"]
        ui.step(
            "rewrites",
            f"{len(p.add)} to add, {len(p.remove)} to remove, {len(p.keep)} already right",
        )
        for domain, answer, why in p.add:
            ui.added(f"{domain:<24} {answer:<16} {ui.paint(ui.DIM, why)}")
        for domain, answer in p.remove:
            ui.removed(f"{domain:<24} {answer}")
        for domain, answer, why in p.keep:
            ui.same(f"{domain:<24} {answer:<16} {why}")
        if p.foreign:
            ui.step("untouched", f"{p.foreign} rewrites outside {manifest.dns()['zone']}")

        if p.upstream_changes:
            ui.step("upstream", "")
            for u in p.upstream_from:
                ui.removed(u)
            for u in p.upstream_to:
                ui.added(u)
        else:
            ui.step("upstream", ", ".join(p.upstream_to))

        if p.empty:
            ui.ok("already matches dns.toml", out.elapsed)
        elif args.check:
            ui.skipped(f"{len(p.add) + len(p.remove)} changes pending, nothing written")
        else:
            ui.ok("applied", out.elapsed)

    return report(results, started)


def records(args) -> int:
    """the names dns.toml and apps.toml add up to, without touching a box"""
    from .fleet import address_of

    ui.line()
    ui.line(f"  {ui.paint(ui.BOLD, manifest.dns()['zone'])}")
    ui.rule()
    for record in manifest.records():
        host = manifest.host(record.host)
        try:
            answer = address_of(record.host)
        except LookupError:
            answer = ui.paint(ui.RED, "no tailnet address")
        source = "apps.toml" if record.source == "app" else "dns.toml"
        ui.line(
            f"  {record.domain:<24} {answer:<16} "
            f"{ui.accent(host.colors['accent'], host.glyph)} {record.host:<6} "
            f"{ui.paint(ui.DIM, source)}"
        )
    ui.line()
    ui.line(
        f"  {ui.paint(ui.DIM, 'served by')} "
        + ", ".join(manifest.resolvers())
        + ui.paint(ui.DIM, " · apply with `pi dns`")
    )
    ui.line()
    return 0


# ── read-only fleet commands ────────────────────────────────────────────────


def ping(args) -> int:
    started = time.monotonic()

    def work(host: Host, creds: Credentials) -> Outcome:
        proc = subprocess.run(
            ["ping", "-c", "1", "-W", str(remote.CONNECT_TIMEOUT), creds.address],
            capture_output=True,
            text=True,
        )
        if proc.returncode != 0:
            return Outcome(host.name, "unreachable")
        rtt = ""
        if m := re.search(r"time=([\d.]+)\s*(\w+)", proc.stdout):
            rtt = f"{m.group(1)} {m.group(2)}"
        return Outcome(host.name, "ok", data={"rtt": rtt})

    results = fan_out(select(args), work)
    ui.line()
    for name in select(args):
        out = results[name]
        host = manifest.host(name)
        tag = f"{ui.accent(host.colors['accent'], host.glyph)} {name:<6}"
        if out.state == "ok":
            ui.ok(f"{tag} {out.data.get('rtt', '')}")
        else:
            ui.skipped(f"{tag} {out.state}")
    return report(results, started)


def ip(args) -> int:
    started = time.monotonic()

    def work(host: Host, creds: Credentials) -> Outcome:
        remote.require_reachable(creds)
        # busybox hostname has no -I, so read the interfaces directly; the
        # label matters more than the bare address on a box with a tailnet
        res = remote.run(creds, "ip -4 -o addr show scope global")
        found = []
        for ln in res.out.splitlines():
            parts = ln.split()
            if len(parts) >= 4 and parts[2] == "inet":
                found.append((parts[1], parts[3].split("/")[0]))
        return Outcome(host.name, "ok", data={"addrs": found})

    results = fan_out(select(args), work)
    ui.line()
    for name in select(args):
        out = results[name]
        host = manifest.host(name)
        tag = f"{ui.accent(host.colors['accent'], host.glyph)} {name:<6}"
        if out.state != "ok":
            ui.skipped(f"{tag} {out.state}")
            continue
        addrs = out.data["addrs"]
        if not addrs:
            ui.skipped(f"{tag} no global addresses")
            continue
        ui.ok(f"{tag} {addrs[0][1]:<16} {ui.paint(ui.DIM, addrs[0][0])}")
        for iface, addr in addrs[1:]:
            ui.line(f"      {addr:<16} {ui.paint(ui.DIM, iface)}")
    return report(results, started)


def status(args) -> int:
    """the same service pulse the login shell prints, read from here"""
    started = time.monotonic()

    def work(host: Host, creds: Credentials) -> Outcome:
        remote.require_reachable(creds)
        res = remote.run(creds, "~/.local/bin/services")
        # services exits 1 when something is down, which is a real answer and
        # already visible in its table; only an unreachable box is a failure
        if res.code not in (0, 1):
            return Outcome(host.name, "failed", [res.message])
        down = sum(1 for ln in res.out.splitlines() if "stopped" in ln)
        return Outcome(host.name, "ok", [res.out], data={"down": down})

    results = fan_out(select(args), work)
    for name in select(args):
        out = results[name]
        if out.state == "ok":
            print(out.lines[0], end="")
        else:
            ui.heading(manifest.host(name))
            ui.skipped("\n".join(out.lines) or out.state)
    return report(results, started)


# ── one-host commands ───────────────────────────────────────────────────────


def logs(args) -> int:
    creds = credentials_for(args.host)
    tail = f"tail -n {args.lines}{' -f' if args.follow else ''}"
    script = (
        "found=0; "
        f"for f in /var/log/{args.service}/{args.service}.err "
        f"/var/log/{args.service}/{args.service}.log; do "
        '[ -s "$f" ] || continue; found=1; echo "── $f"; ' + tail + ' "$f"; done; '
        f'[ "$found" = 1 ] || echo "nothing under /var/log/{args.service}/"'
    )
    # the log files are owned by the service, not by the login user
    wrapped = 'printf "%s\\n" "$SUDO_PASS" | sudo -S -p "" sh -c ' + shlex.quote(script)

    if args.follow:
        return remote.sudo_attach(creds, wrapped)

    res = remote.sudo(creds, wrapped)
    print(res.out, end="")
    if not res.ok:
        ui.error(f"{args.host}: {res.message}")
    return res.code


def ssh(args) -> int:
    creds = credentials_for(args.host)
    command = " ".join(args.ssh_command) if args.ssh_command else None
    return remote.attach(creds, command)


def reboot(args) -> int:
    started = time.monotonic()

    def work(host: Host, creds: Credentials) -> Outcome:
        remote.require_reachable(creds)
        res = remote.sudo(creds, 'printf "%s\\n" "$SUDO_PASS" | sudo -S -p "" reboot')
        if not res.ok:
            return Outcome(host.name, "failed", [res.message])
        return Outcome(host.name)

    results = fan_out(select(args), work, parallel=False)
    ui.line()
    for name in select(args):
        out = results[name]
        host = manifest.host(name)
        tag = f"{ui.accent(host.colors['accent'], host.glyph)} {name}"
        if out.state == "ok":
            ui.ok(f"{tag} rebooting")
        else:
            ui.failed(f"{tag} {out.state}")
    return report(results, started)


def magnet(args) -> int:
    if not args.link.startswith("magnet:"):
        ui.error("that does not start with 'magnet:'")
        return 1

    torrent = next((a for a in manifest.apps() if a.name == "torrent"), None)
    if torrent is None:
        ui.error("no torrent app in apps.toml")
        return 1

    creds = credentials_for(torrent.host)
    # transmission's rpc is loopback-only, so this has to run on the box
    res = remote.run(
        creds,
        f"transmission-remote localhost:9091 --add {shlex.quote(args.link)}",
    )
    ui.line()
    if not res.ok:
        ui.failed(f"{torrent.host} {res.message}")
        return 1
    ui.ok(f"{torrent.host} queued")
    return 0


# ── listings ────────────────────────────────────────────────────────────────


def hosts_cmd(args) -> int:
    creds = {}
    for h in manifest.hosts():
        try:
            creds[h.name] = credentials_for(h.name)
        except LookupError:
            creds[h.name] = None

    ui.line()
    for h in manifest.hosts():
        c = creds[h.name]
        target = c.target if c else ui.paint(ui.RED, "no credentials")
        apps = manifest.apps_on(h.name)
        tail = f"{len(apps)} apps" if apps else "no apps"
        ui.line(
            f"  {ui.accent(h.colors['accent'], h.glyph)} {h.name:<6} "
            f"{target:<16} {ui.paint(ui.DIM, f'{h.plane} plane · {tail}')}"
        )
    ui.line()
    return 0


def apps_cmd(args) -> int:
    ui.line()
    for h in manifest.hosts():
        apps = manifest.apps_on(h.name)
        if not apps:
            continue
        ui.line(f"  {ui.accent(h.colors['accent'], h.glyph)} {h.name} {ui.paint(ui.DIM, h.blurb)}")
        for a in apps:
            ui.line(
                f"      {a.name:<9} {ui.paint(ui.DIM, 'api')} {a.api}  "
                f"{a.domain:<20} {ui.paint(ui.DIM, a.label)}"
            )
    ui.line()
    return 0


# ── host selection ──────────────────────────────────────────────────────────


def select(args) -> list[str]:
    chosen = [*(getattr(args, "hosts", None) or []), *(getattr(args, "host_flags", None) or [])]
    names = manifest.host_names()
    if not chosen:
        return names
    if unknown := sorted(set(chosen) - set(names)):
        raise SystemExit(f"unknown host: {', '.join(unknown)} (have: {', '.join(names)})")
    return [n for n in names if n in chosen]
