**thebe = network plane** Global services, network-wide services, public services

|                                 |                                                                                                        |
| ------------------------------- | ------------------------------------------------------------------------------------------------------ |
| DONE `adguardhome`              | primary resolver                                                                                       |
| DONE `unbound`                  | recursive resolver behind AGH, so no upstream sees your query log                                      |
| DONE `tailscale`                | subnet router advertising `192.168.250.0/24`, full LAN from anywhere                                   |
| DONE `caddy`                    | reverse proxy for everything on both pis, real names instead of `:3000`                                |
| DONE `ntfy`                     | push target for your scripts; same shape as your `notify-send "✦ topic"` convention, but on your phone |
| DONE `prometheus-node-exporter` | metrics scrape target for this box, ships to `victoria-metrics` on io                                  |

**leda = service plane** Personal local services

|                                  |                                                                          |
| -------------------------------- | ------------------------------------------------------------------------ |
| DONE `adguardhome`               | secondary resolver, independently forwards over Quad9 DoH                |
| DONE `soju`                      | persistent IRC bouncer (saw the irc commit, this is the obvious fit)     |
| DONE `prometheus-node-exporter`  | metrics scrape target for this box, ships to `victoria-metrics` on io    |
| DONE `tailscale`                 | exit node, kept off thebe so bulk traffic doesn't compete with DNS       |
| DONE `spotifyd` + `pyatv-bridge` | spotify connect device that relays audio via airplay to the homepod mini |

**io = storage plane** Storage heavy services

|                                  |                                                                                                               |
| -------------------------------- | ------------------------------------------------------------------------------------------------------------- |
| DONE `tailscale`                 | joined the tailnet                                                                                            |
| DONE `yt-dlp` + custom ui        | `yt.elara.boo`, mp4/mp3 with a web ui, 20 GB cache + `/srv/media` library, tailnet-only                       |
| DONE `share` custom quickshare   | `share.elara.boo`, upload via web ui or the `share` script, keeps the last 10 files, tailnet-only             |
| DONE `caddy`                     | reverse proxy + DNS-01 cert for `yt.elara.boo`, so bulk downloads never transit thebe                         |
| DONE `jellyfin`                  | media server at `jellyfin.elara.boo`, video side of the yt-dlp output, no hw transcode                        |
| DONE `transmission-daemon`       | torrent client at `torrent.elara.boo`, `script-torrent-done` hardlinks finished video into jellyfin's library |
| DONE `prometheus-node-exporter`  | metrics scrape target for io itself, scraped over loopback                                                    |
| DONE `victoria-metrics` + `dash` | custom go+astro fleet dashboard at `dash.elara.boo`                                                           |
| DONE `vaultwarden`               | bitwarden-compatible vault at `vault.elara.boo`, tailnet-only, apk package                                    |
| DONE `watch` custom watchlist    | `watch.elara.boo`, tier list + queue with anilist/tmdb lookup, tailnet-only                                   |

## layout

each app is one directory at the root: a Go api in `server/`, an Astro ui in
`web/` that the api embeds, and its openrc unit next to them. `apps.toml` is
the only place that knows an app exists, and everything else reads it.

```
ytdl/ share/ dash/ torrent/ watch/   the apps
apps.toml                            ports, domains, which host runs what
dns.toml                             every name the fleet resolves
hosts/<name>/                        one pi: host.toml, Caddyfile, install
internal/                            Go helpers the apis share
shared/ui/                           palette, fonts, html shell, fetch glue
shell/ bin/                          what gets pushed to the pis
cli/                                 the fleet cli, run as `pi`
tools/                               dev, build and check, run on this machine
pi                                   the entrypoint for cli/
```

## development

[mise](https://mise.jdx.dev/) pins the Go, Node, pnpm, ruff and golangci-lint
versions and is the single command surface:

```sh
mise install
mise run apps              # ports and domains for everything
mise run dev watch         # one app's ui and api together
mise run check             # everything; or one stage: check go
mise run fmt               # write the formatting
mise run build watch       # cross-compile just that app for the pis
```

## the fleet

`pi` drives the three pis over tailscale. the mise tasks are thin wrappers, so
either spelling works.

```sh
pi records                 # every name, resolved, without touching a box
pi dns --check             # what would change in adguard; -n writes nothing
pi dns                     # reconcile both resolvers against dns.toml
pi diff io                 # which staged files differ from the box
pi sync io --services      # build the apps, push, install
pi status                  # service pulse for every pi
pi logs io watch -f        # follow a service's log
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
