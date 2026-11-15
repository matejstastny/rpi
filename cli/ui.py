"""terminal output.

the fleet already has a visual language: lowercase text, `▸` for a step, `●`
for a state, `!` for something you have to go fix by hand, a thin rule, and
each pi tinted its own colour from hosts/<name>/host.toml. this module is that
language and nothing more, so `pi`, `bin/services` and the install scripts all
read like the same program.
"""

from __future__ import annotations

import os
import shutil
import sys
import time
from contextlib import contextmanager

RESET = "\033[0m"
BOLD = "\033[1m"
DIM = "\033[38;2;120;120;140m"
GREEN = "\033[38;2;166;227;161m"
RED = "\033[38;2;243;139;168m"
AMBER = "\033[38;2;242;205;140m"

# 0 quiet, 1 normal, 2 verbose
LEVEL = 1
_colour = True


def setup(level: int, stream=None) -> None:
    global LEVEL, _colour
    LEVEL = level
    stream = stream or sys.stdout
    _colour = stream.isatty() and not os.environ.get("NO_COLOR")


def paint(code: str, text: str) -> str:
    return f"{code}{text}{RESET}" if _colour else text


def accent(hex_colour: str, text: str) -> str:
    """truecolor from a #rrggbb in host.toml, the same way bin/services does"""
    if not _colour:
        return text
    h = hex_colour.lstrip("#")
    r, g, b = (int(h[i : i + 2], 16) for i in (0, 2, 4))
    return f"\033[38;2;{r};{g};{b}m{text}{RESET}"


def width() -> int:
    return max(64, min(96, shutil.get_terminal_size((80, 24)).columns))


def visible(text: str) -> int:
    """length without the escape sequences, for right-aligning"""
    out, skip = 0, False
    for ch in text:
        if ch == "\033":
            skip = True
        elif skip:
            skip = ch != "m"
        else:
            out += 1
    return out


def line(text: str = "") -> None:
    if LEVEL >= 1:
        print(text, flush=True)


def rule(colour: str | None = None) -> None:
    bar = "─" * width()
    line(accent(colour, bar) if colour else paint(DIM, bar))


def heading(host) -> None:
    """a host's section header, tinted that host's own colour"""
    left = f"{host.glyph} {accent(host.colors['accent'], host.name)}"
    right = paint(DIM, f"{host.plane} plane")
    gap = max(1, width() - visible(left) - visible(right))
    line()
    line(f"{left}{' ' * gap}{right}")
    rule(host.colors["accent"])


def step(label: str, detail: str = "", timing: float | None = None) -> None:
    body = f"  {paint(DIM, '▸')} {label:<10} {detail}"
    if timing is None:
        line(body.rstrip())
        return
    clock = paint(DIM, f"{timing:.1f}s")
    gap = max(1, width() - visible(body) - visible(clock))
    line(f"{body}{' ' * gap}{clock}")


def detail(text: str) -> None:
    """an extra line under a step, only when asked for"""
    if LEVEL >= 2:
        for ln in text.rstrip().splitlines():
            line(f"      {paint(DIM, ln)}")


def note(text: str) -> None:
    """something the box needs a human for; install prints these as `!`"""
    for ln in text.rstrip().splitlines():
        line(f"      {paint(AMBER, '!')} {ln.lstrip('! ')}")


def added(text: str) -> None:
    line(f"      {paint(GREEN, '+')} {text}")


def removed(text: str) -> None:
    line(f"      {paint(RED, '-')} {text}")


def changed(text: str) -> None:
    line(f"      {paint(AMBER, '~')} {text}")


def same(text: str) -> None:
    if LEVEL >= 2:
        line(f"        {paint(DIM, text)}")


def ok(text: str, timing: float | None = None) -> None:
    body = f"  {paint(GREEN, '●')} {text}"
    if timing is None:
        line(body)
        return
    clock = paint(DIM, f"{timing:.1f}s")
    gap = max(1, width() - visible(body) - visible(clock))
    line(f"{body}{' ' * gap}{clock}")


def failed(text: str) -> None:
    head, *rest = text.splitlines() or [""]
    line(f"  {paint(RED, '●')} {head}")
    for ln in rest:
        line(f"    {paint(DIM, ln)}")


def skipped(text: str) -> None:
    line(f"  {paint(AMBER, '○')} {text}")


def warn(text: str) -> None:
    print(f"{paint(AMBER, 'warn:')} {text}", file=sys.stderr, flush=True)


def error(text: str) -> None:
    print(f"{paint(RED, 'error:')} {text}", file=sys.stderr, flush=True)


def summary(results: dict[str, str], elapsed: float) -> None:
    """the one line you read when you were not watching"""
    from .manifest import hosts

    glyphs = {h.name: (h.glyph, h.colors["accent"]) for h in hosts()}
    parts = []
    for name, state in results.items():
        glyph, colour = glyphs.get(name, ("·", "#888888"))
        mark = {"ok": GREEN, "failed": RED}.get(state, AMBER)
        parts.append(f"{accent(colour, glyph)} {name} {paint(mark, state)}")

    body = "  " + paint(DIM, " · ").join(parts)
    clock = paint(DIM, f"{elapsed:.1f}s")
    gap = max(1, width() - visible(body) - visible(clock))
    line()
    rule()
    line(f"{body}{' ' * gap}{clock}")
    line()


@contextmanager
def timed():
    start = time.monotonic()
    box = [0.0]
    try:
        yield box
    finally:
        box[0] = time.monotonic() - start
