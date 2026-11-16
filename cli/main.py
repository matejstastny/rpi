"""the command surface."""

from __future__ import annotations

import argparse
import os
import sys

from . import Fault, commands, manifest, ui

EPILOG = """examples:
  pi dns                     reconcile every resolver against dns.toml
  pi dns --check             show what would change, write nothing
  pi records                 the names dns.toml and apps.toml add up to
  pi diff io                 which staged files differ from the box
  pi sync io --services      build the apps, push, install
  pi status                  service pulse for every pi
  pi logs io ytdl -f         follow a service's log
  pi ssh io 'rc-service caddy status'
"""


def build_parser() -> argparse.ArgumentParser:
    fleet = argparse.ArgumentParser(add_help=False)
    fleet.add_argument("hosts", nargs="*", metavar="HOST", help="host names (default: all)")
    fleet.add_argument(
        "--host",
        action="append",
        dest="host_flags",
        metavar="HOST",
        help="select a host, may be repeated",
    )

    p = argparse.ArgumentParser(
        prog="pi",
        description="manage the raspberry pis over tailscale",
        epilog=EPILOG,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument("-V", "--version", action="version", version="pi 3.0")
    p.add_argument("-v", "--verbose", action="store_true", help="show every remote step")
    p.add_argument("-q", "--quiet", action="store_true", help="only the summary line")

    sub = p.add_subparsers(dest="command", metavar="COMMAND")

    sync = sub.add_parser("sync", parents=[fleet], help="push config to hosts")
    sync.add_argument("-n", "--dry-run", action="store_true", help="stage but write nothing")
    sync.add_argument("-b", "--bootstrap", action="store_true", help="also run bin/setup-pi")
    sync.add_argument(
        "-s", "--services", action="store_true", help="build apps and install services"
    )
    sync.set_defaults(fn=commands.sync)

    diff = sub.add_parser("diff", parents=[fleet], help="compare the staged payload with the hosts")
    diff.set_defaults(fn=commands.diff)

    dns = sub.add_parser("dns", parents=[fleet], help="reconcile adguard against dns.toml")
    dns.add_argument("-n", "--check", action="store_true", help="show the plan, write nothing")
    dns.set_defaults(fn=commands.dns)

    sub.add_parser(
        "records", help="the fleet's names, resolved, without touching a box"
    ).set_defaults(fn=commands.records)

    sub.add_parser("ping", parents=[fleet], help="ping each host").set_defaults(fn=commands.ping)
    sub.add_parser("ip", parents=[fleet], help="each host's lan addresses").set_defaults(
        fn=commands.ip
    )
    sub.add_parser("status", parents=[fleet], help="service pulse for each host").set_defaults(
        fn=commands.status
    )
    sub.add_parser("hosts", help="the pis and how they are reached").set_defaults(
        fn=commands.hosts_cmd
    )
    sub.add_parser("apps", help="which apps run where").set_defaults(fn=commands.apps_cmd)

    reboot = sub.add_parser("reboot", parents=[fleet], help="reboot hosts")
    reboot.add_argument("--all", action="store_true", help="reboot every host")
    reboot.set_defaults(fn=commands.reboot)

    logs = sub.add_parser("logs", help="tail a service's log files")
    logs.add_argument("host", metavar="HOST")
    logs.add_argument("service", metavar="SERVICE")
    logs.add_argument("-n", "--lines", type=int, default=50, help="lines to show, default 50")
    logs.add_argument("-f", "--follow", action="store_true", help="keep streaming")
    logs.set_defaults(fn=commands.logs)

    ssh = sub.add_parser("ssh", help="run a command over ssh, or open a shell")
    ssh.add_argument("host", metavar="HOST")
    ssh.add_argument("ssh_command", nargs=argparse.REMAINDER, metavar="COMMAND")
    ssh.set_defaults(fn=commands.ssh)

    magnet = sub.add_parser("magnet", help="queue a magnet link on transmission")
    magnet.add_argument("link", metavar="MAGNET")
    magnet.set_defaults(fn=commands.magnet)

    return p


def main(argv: list[str] | None = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    ui.setup(0 if args.quiet else 2 if args.verbose else 1)

    if args.command is None:
        parser.print_help()
        return 0

    # the only command where "every host" is not a sensible default
    if args.command == "reboot" and not args.hosts and not args.host_flags and not args.all:
        parser.error("reboot needs at least one HOST, or --all")

    for name in ("host",):
        if (value := getattr(args, name, None)) and value not in manifest.host_names():
            parser.error(f"unknown host: {value} (have: {', '.join(manifest.host_names())})")

    try:
        return args.fn(args)
    except KeyboardInterrupt:
        ui.line()
        return 130
    except BrokenPipeError:
        # `pi records | head` closes the pipe on us; exit the way every other
        # unix tool does instead of complaining about it
        os.dup2(os.open(os.devnull, os.O_WRONLY), sys.stdout.fileno())
        return 141
    except Fault as exc:
        ui.error(str(exc))
        return 1
    except (LookupError, ValueError, OSError) as exc:
        ui.error(str(exc))
        return 1


if __name__ == "__main__":
    sys.exit(main())
