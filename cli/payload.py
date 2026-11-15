"""build the tree that lands in a pi's home directory.

the staging directory is laid out exactly as it should appear under $HOME, so
what gets pushed is always what you can see in `pi diff` and `pi sync -n`.
"""

from __future__ import annotations

import hashlib
import io
import shutil
import tarfile
from dataclasses import dataclass
from pathlib import Path

from . import render
from .manifest import ROOT, Host, apps_on, dns, resolvers

# service configs are root-owned system files, so they land here inert and are
# only installed when --services explicitly runs the host's install script
SERVICES_HOME = Path(".local/share/pi-services")


@dataclass
class Payload:
    host: Host
    root: Path

    @property
    def files(self) -> list[Path]:
        return sorted(p for p in self.root.rglob("*") if p.is_file())

    @property
    def names(self) -> list[str]:
        return [str(p.relative_to(self.root)) for p in self.files]

    @property
    def size(self) -> int:
        return sum(p.stat().st_size for p in self.files)

    def digests(self) -> dict[str, str]:
        return {
            str(p.relative_to(self.root)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in self.files
        }

    def tar(self) -> bytes:
        buf = io.BytesIO()
        with tarfile.open(fileobj=buf, mode="w") as tar:
            for path in sorted(self.root.rglob("*")):
                tar.add(path, arcname=str(path.relative_to(self.root)))
        return buf.getvalue()


def _copy_into(src: Path, dst: Path, mode: int | None = None) -> None:
    dst.mkdir(parents=True, exist_ok=True)
    for f in sorted(src.iterdir()):
        if f.is_file():
            shutil.copy2(f, dst / f.name)
            if mode is not None:
                (dst / f.name).chmod(mode)


def build(host: Host, staging: Path, with_binaries: bool = False) -> Payload:
    staging.mkdir(parents=True, exist_ok=True)

    # the login shell
    zsh = staging / ".config" / "zsh"
    shutil.copy2(ROOT / "shell" / "zshrc", staging / ".zshrc")
    _copy_into(ROOT / "shell" / "zsh", zsh)
    (zsh / "host.zsh").write_text(render.host_zsh(host))

    prompt = (ROOT / "shell" / "starship.toml").read_text()
    (staging / ".config" / "starship.toml").write_text(prompt + render.starship(host))

    pi_conf = staging / ".config" / "pi"
    pi_conf.mkdir(parents=True, exist_ok=True)
    (pi_conf / "host.env").write_text(render.host_env(host))

    _copy_into(ROOT / "bin", staging / ".local" / "bin", mode=0o755)

    if not host.installable:
        return Payload(host, staging)

    services = staging / SERVICES_HOME / host.name
    services.mkdir(parents=True, exist_ok=True)
    for f in sorted(host.dir.iterdir()):
        if f.is_file() and f.name not in ("host.toml", "README.md"):
            shutil.copy2(f, services / f.name)
            if f.name == "install":
                (services / f.name).chmod(0o755)

    # one adguard seed for every resolver on the fleet, so leda and thebe
    # cannot drift apart the way two copies of it did
    if host.name in resolvers():
        shutil.copy2(ROOT / "hosts" / "adguardhome.yaml", services / "adguardhome.yaml")

    # each app contributes its own openrc unit and conf.d, so adding an app is
    # adding a directory rather than editing this file
    for app in apps_on(host.name):
        for unit in render.app_units(app):
            shutil.copy2(unit, services / unit.name)

    # the cross-compiled binaries are too big to push on an ordinary config
    # sync and useless without the install script anyway
    if with_binaries:
        wanted = {a.name for a in apps_on(host.name)}
        wanted |= {f["as"] for a in apps_on(host.name) for f in a.files}
        build_dir = ROOT / "build"
        if build_dir.is_dir():
            for f in sorted(build_dir.iterdir()):
                if f.is_file() and f.name in wanted:
                    shutil.copy2(f, services / f.name)
                    (services / f.name).chmod(0o755)

    return Payload(host, staging)


def missing_binaries(host: Host) -> list[str]:
    """which of this host's artefacts are not in build/ yet"""
    wanted = [a.name for a in apps_on(host.name)]
    wanted += [f["as"] for a in apps_on(host.name) for f in a.files]
    return [n for n in wanted if not (ROOT / "build" / n).is_file()]


def seeds_adguard(host: Host) -> bool:
    return host.name in dns()["resolvers"]
