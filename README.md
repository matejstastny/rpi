<h1 style="centered">rpi</h1>

config and a fleet cli for three raspberry pis, connected over tailscale

|             |                                          |
| ----------- | ---------------------------------------- |
| ◑ **thebe** | global, network-wide and public services |
| ◐ **leda**  | personal local services                  |
| ◒ **io**    | storage heavy services                   |

### ◑ [thebe](<https://en.wikipedia.org/wiki/Thebe_(moon)>)

|                            |                                                                             |
| -------------------------- | --------------------------------------------------------------------------- |
| `adguardhome`              | primary resolver                                                            |
| `unbound`                  | recursive resolver behind AGH, keeps the query log off the upstream         |
| `tailscale`                | subnet router advertising `192.168.250.0/24`, full LAN from anywhere        |
| `caddy`                    | reverse proxy for everything on thebe and io, real names instead of `:3000` |
| `ntfy`                     | push target for scripts, same idea as `notify-send` but on your phone       |
| `cloudflared`              | tunnels `elara.boo` out, no public port open anywhere                       |
| `prometheus-node-exporter` | metrics scrape target for this box, ships to `victoria-metrics` on io       |

### ◐ [leda](<https://en.wikipedia.org/wiki/Leda_(moon)>)

|                            |                                                                       |
| -------------------------- | --------------------------------------------------------------------- |
| `adguardhome`              | secondary resolver, independently forwards over Quad9 DoH             |
| `soju`                     | persistent IRC bouncer                                                |
| `tailscale`                | exit node, kept off thebe so bulk traffic doesn't compete with dns    |
| `prometheus-node-exporter` | metrics scrape target for this box, ships to `victoria-metrics` on io |

### ◒ [io](<https://en.wikipedia.org/wiki/Io_(moon)>)

|                             |                                                                                                             |
| --------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `tailscale`                 | joined the tailnet                                                                                          |
| `ytdl`                      | `yt.elara.boo`, mp4/mp3 with a web ui, 20 GB cache + `/srv/media` library, tailnet-only                     |
| `share`                     | `share.elara.boo`, upload via web ui or the `share` script, keeps the last 10 files, tailnet-only           |
| `caddy`                     | reverse proxy + DNS-01 cert for everything here, so bulk downloads never transit thebe                      |
| `jellyfin`                  | media server at `jellyfin.elara.boo`, video side of the yt-dlp output, no hw transcode                      |
| `transmission-daemon`       | torrent client at `torrent.elara.boo`; `transmission-done` hardlinks finished video into jellyfin's library |
| `prometheus-node-exporter`  | metrics scrape target for io itself, scraped over loopback                                                  |
| `victoria-metrics` + `dash` | custom Go + Astro fleet dashboard at `dash.elara.boo`                                                       |
| `tasks`                     | weekly planner at `tasks.elara.boo`, repeats, carry-over and subscribed ics calendars, tailnet-only         |

## layout

each app is one directory at the root: a Go api in `server/`, a ui in `web/`
that the api embeds, and its openrc unit next to them. the ui is Astro for the
read-mostly services and React for `tasks`; the tooling runs each app's own
`dev`, `build` and `typecheck` scripts rather than knowing which. `apps.toml`
is the only place that knows an app exists, and everything else reads it.

```
ytdl/ share/ dash/           the apps
torrent/ tasks/
apps.toml                    ports, domains, which host runs what
dns.toml                     every name the fleet resolves
hosts/<name>/                one pi: host.toml, Caddyfile, install
internal/                    Go helpers the apis share
shared/ui/                   palette, fonts, html shell, fetch glue
shell/ bin/                  what gets pushed to the pis
cli/                         the fleet cli, run as `pi`
tools/                       dev, build and check, run on this machine
pi                           the entrypoint for cli/
```

## development

[mise](https://mise.jdx.dev/) pins the Go, Node, pnpm, ruff and golangci-lint
versions and is the single command surface:

```sh
mise install
mise run apps              # ports and domains for everything
mise run dev ytdl          # one app's ui and api together
mise run check             # everything; or one stage: check go
mise run fmt               # write the formatting
mise run build ytdl        # cross-compile just that app for the pis
```

## the fleet

`pi` drives the three pis over tailscale. the mise tasks are thin wrappers, so
either spelling works.

```sh
pi records              # every name, resolved, without touching a box
pi dns --check          # what would change in adguard; -n writes nothing
pi dns                  # reconcile both resolvers against dns.toml
pi diff io              # which staged files differ from the box
pi sync io --services   # build the apps, push, install
pi status               # service pulse for every pi
pi logs io ytdl -f      # follow a service's log
pi ssh io 'rc-service caddy status'
```

### dns

`dns.toml` is the only place a name is written down. the app names come from
`apps.toml` pointed at whichever host runs them, and every address comes from
`tailscale status`, so there is no ip in git to go stale. `pi dns` reconciles
both adguards through adguard's own api rather than editing the yaml it owns,
and only ever touches names inside the declared zone.

it needs adguard's web password, which is not in git. either export
`PI_ADGUARD_PASS`, or add `# adguardpsw: ...` above a host in your
remote-hosts file, the same convention as `# rootpsw:`.
