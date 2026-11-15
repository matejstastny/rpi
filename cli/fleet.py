"""who the pis are and how to reach them.

credentials come from the remote-hosts file rmt already uses, so there is one
place holding ssh passwords on this machine and it is not this repository.
addresses come from `tailscale status`, so no ip is ever written down.
"""

from __future__ import annotations

import json
import os
import re
import shutil
import subprocess
from dataclasses import dataclass
from functools import cache
from pathlib import Path

HOSTS_FILE = Path(os.environ.get("REMOTE_HOSTS_CONFIG", Path.home() / ".config/remote/hosts.toml"))

# "# rootpsw:" and "# adguardpsw:" above a host carry secrets without making
# rmt treat the host as password-auth (that is what the "password" key means)
SECRET_COMMENT = re.compile(r"^#\s*(rootpsw|adguardpsw):\s*(.+)$")
KEY = re.compile(r'^(\w+)\s*=\s*"(.*)"$')


@dataclass(frozen=True)
class Credentials:
    name: str
    user: str
    address: str
    password: str = ""
    rootpsw: str = ""
    adguardpsw: str = ""

    @property
    def target(self) -> str:
        return f"{self.user}@{self.address}"

    @property
    def sudo(self) -> str:
        return self.rootpsw or self.password


@cache
def credentials() -> dict[str, Credentials]:
    if not HOSTS_FILE.exists():
        return {}

    out: dict[str, Credentials] = {}
    current: dict[str, str] = {}
    pending: dict[str, str] = {}

    def flush():
        if name := current.get("name"):
            out[name] = Credentials(
                name=name,
                user=current.get("user", ""),
                address=current.get("host", name),
                password=current.get("password", ""),
                rootpsw=current.get("rootpsw", ""),
                adguardpsw=current.get("adguardpsw", ""),
            )

    for raw in HOSTS_FILE.read_text().splitlines():
        text = raw.strip()
        if m := SECRET_COMMENT.match(text):
            pending[m.group(1)] = m.group(2).strip()
            continue
        if text == "[[hosts]]":
            flush()
            current = dict(pending)
            pending = {}
            continue
        if m := KEY.match(text):
            current[m.group(1)] = m.group(2)
    flush()
    return out


def credentials_for(name: str) -> Credentials:
    creds = credentials().get(name)
    if creds is None:
        raise LookupError(f"no entry for {name} in {HOSTS_FILE}")
    return creds


@cache
def tailnet() -> dict[str, str]:
    """hostname -> tailnet ipv4, so dns.toml never has to name an address"""
    if shutil.which("tailscale") is None:
        return {}
    try:
        raw = subprocess.run(
            ["tailscale", "status", "--json"],
            capture_output=True,
            timeout=10,
            check=True,
        ).stdout
    except (subprocess.SubprocessError, OSError):
        return {}

    status = json.loads(raw)
    peers = {"self": status.get("Self") or {}, **(status.get("Peer") or {})}
    out: dict[str, str] = {}
    for peer in peers.values():
        name = peer.get("HostName")
        v4 = next((ip for ip in peer.get("TailscaleIPs") or [] if ":" not in ip), None)
        if name and v4:
            out[name] = v4
    return out


def address_of(name: str) -> str:
    """the tailnet address a dns rewrite should answer with"""
    addr = tailnet().get(name)
    if not addr:
        raise LookupError(
            f"tailscale does not know a host called {name!r}; "
            "run `tailscale status` and check the machine name"
        )
    return addr


def sshpass_available() -> bool:
    return shutil.which("sshpass") is not None
