"""the declarative files at the repo root, and nothing else.

apps.toml says which apps exist and where they run, hosts/<name>/host.toml
describes one pi, dns.toml owns the fleet's names. every command reads these
rather than hardcoding a list.
"""

from __future__ import annotations

import tomllib
from dataclasses import dataclass, field
from functools import cache
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def load(path: Path) -> dict:
    with path.open("rb") as f:
        return tomllib.load(f)


@dataclass(frozen=True)
class App:
    name: str
    label: str
    host: str
    domain: str
    api: int
    web: int
    needs: str = ""
    env: dict[str, str] = field(default_factory=dict)
    files: tuple[dict[str, str], ...] = ()

    @property
    def unit(self) -> str:
        """the openrc service name, which is also the directory name"""
        return self.name


@dataclass(frozen=True)
class Host:
    name: str
    glyph: str
    plane: str
    blurb: str
    colors: dict[str, str]
    services: tuple[str, ...]

    @property
    def dir(self) -> Path:
        return ROOT / "hosts" / self.name

    @property
    def installable(self) -> bool:
        return (self.dir / "install").is_file()


@cache
def apps() -> tuple[App, ...]:
    raw = load(ROOT / "apps.toml")["app"]
    out = []
    seen: dict[int, str] = {}
    for a in raw:
        for port, kind in ((a["api"], "api"), (a["web"], "web")):
            if port in seen:
                raise ValueError(
                    f"apps.toml: {a['name']} {kind} port {port} collides with {seen[port]}"
                )
            seen[port] = f"{a['name']} {kind}"
        out.append(
            App(
                name=a["name"],
                label=a["label"],
                host=a["host"],
                domain=a["domain"],
                api=a["api"],
                web=a["web"],
                needs=a.get("needs", ""),
                env=a.get("env", {}),
                files=tuple(a.get("files", [])),
            )
        )
    return tuple(out)


def apps_on(host: str) -> tuple[App, ...]:
    return tuple(a for a in apps() if a.host == host)


@cache
def hosts() -> tuple[Host, ...]:
    out = []
    for path in sorted((ROOT / "hosts").glob("*/host.toml")):
        raw = load(path)
        out.append(
            Host(
                name=raw["name"],
                glyph=raw["glyph"],
                plane=raw["plane"],
                blurb=raw["blurb"],
                colors=raw["colors"],
                services=tuple(raw.get("services", [])),
            )
        )
    return tuple(out)


def host_names() -> list[str]:
    return [h.name for h in hosts()]


def host(name: str) -> Host:
    for h in hosts():
        if h.name == name:
            return h
    raise KeyError(f"unknown host {name!r}, have: {', '.join(host_names())}")


@dataclass(frozen=True)
class Record:
    """one adguard rewrite: a name, the host it points at, and why"""

    domain: str
    host: str
    source: str  # "app" or "record"


@cache
def dns() -> dict:
    return load(ROOT / "dns.toml")


def resolvers() -> list[str]:
    """the hosts that run an adguard, in fleet order"""
    declared = dns()["resolvers"]
    return [name for name in host_names() if name in declared]


def records() -> tuple[Record, ...]:
    """every name the fleet's resolvers should answer for.

    the app names come from apps.toml so a new app is one entry in one file;
    dns.toml only lists what is not an app.
    """
    zone = dns()["zone"]
    out = [Record(a.domain, a.host, "app") for a in apps()]
    out += [Record(f"{name}.{zone}", target, "record") for name, target in dns()["records"].items()]

    clash = {r.domain for r in out}
    if len(clash) != len(out):
        dupes = sorted(d for d in clash if sum(r.domain == d for r in out) > 1)
        raise ValueError(f"dns.toml: {', '.join(dupes)} declared twice")

    known = set(host_names())
    for r in out:
        if r.host not in known:
            raise ValueError(f"dns.toml: {r.domain} points at unknown host {r.host!r}")

    return tuple(sorted(out, key=lambda r: r.domain))
