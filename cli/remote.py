"""ssh plumbing.

two rules hold everywhere in here: a sudo password is handed over on stdin's
first line so it never appears in the remote process list, and one unreachable
box never takes the others down with it.
"""

from __future__ import annotations

import os
import subprocess
from dataclasses import dataclass

from .fleet import Credentials

CONNECT_TIMEOUT = 10

SSH_OPTS = [
    "-o",
    f"ConnectTimeout={CONNECT_TIMEOUT}",
    "-o",
    "StrictHostKeyChecking=accept-new",
    "-o",
    "BatchMode=no",
    "-o",
    "LogLevel=ERROR",
]


class Unreachable(Exception):
    """the box did not answer, which is expected and not an error"""


@dataclass
class Result:
    code: int
    out: str
    err: str

    @property
    def ok(self) -> bool:
        return self.code == 0

    @property
    def message(self) -> str:
        return self.err.strip() or self.out.strip() or f"exit {self.code}"


def _command(creds: Credentials, remote: str | None) -> tuple[list[str], dict[str, str]]:
    env = dict(os.environ)
    cmd = ["ssh", *SSH_OPTS, creds.target]
    if remote is not None:
        cmd.append(remote)
    if creds.password:
        env["SSHPASS"] = creds.password
        cmd = ["sshpass", "-e", *cmd]
    return cmd, env


def run(creds: Credentials, remote: str | None, stdin: bytes | None = None) -> Result:
    cmd, env = _command(creds, remote)
    proc = subprocess.run(cmd, input=stdin, env=env, capture_output=True)
    return Result(
        proc.returncode,
        proc.stdout.decode(errors="replace"),
        proc.stderr.decode(errors="replace"),
    )


def attach(creds: Credentials, remote: str | None, stdin: bytes | None = None) -> int:
    """hand the terminal over: an interactive shell, or `tail -f`"""
    cmd, env = _command(creds, remote)
    return subprocess.run(cmd, input=stdin, env=env).returncode


def sudo(creds: Credentials, remote: str, extra: bytes = b"") -> Result:
    """run remote with $SUDO_PASS exported, read off stdin before anything else"""
    payload = creds.sudo.encode() + b"\n" + extra
    wrapper = "IFS= read -r SUDO_PASS; export SUDO_PASS; " + remote
    return run(creds, wrapper, stdin=payload)


def sudo_attach(creds: Credentials, remote: str) -> int:
    payload = creds.sudo.encode() + b"\n"
    wrapper = "IFS= read -r SUDO_PASS; export SUDO_PASS; " + remote
    return attach(creds, wrapper, stdin=payload)


def reachable(creds: Credentials) -> bool:
    return run(creds, "true").ok


def require_reachable(creds: Credentials) -> None:
    if not reachable(creds):
        raise Unreachable(f"{creds.name} did not answer on {creds.address}")
