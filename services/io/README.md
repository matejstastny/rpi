# io services

the storage and application pi. its services are only reachable on the house
lan and tailnet.

- `server/` + `web/` is the ytdl service.
- `share-server/` + `share-web/` is quickshare.
- `dash-server/` + `dash-web/` is the fleet dashboard.
- `torrent-server/` + `torrent-web/` is a web ui in front of transmission's
  rpc; transmission itself only answers on loopback.
- `watch-server/` + `watch-web/` is the watchlist and tier list. anime lookup
  is AniList, which needs no key; series and movies are TMDB, which needs
  `WATCH_TMDB_KEY` in `/etc/watch/watch.env`. posters are fetched once and
  cached under `/var/lib/watch/art`, and the browser only ever loads art
  through the service, never from a provider cdn.
- `vaultwarden.confd` overrides the `apk`-shipped default to bind loopback
  only and point at `DOMAIN=https://vault.elara.boo`; the package's own
  `vaultwarden.initd` is used as-is.
- `Caddyfile`, `*.initd`, `*.confd`, and `scrape.yml` are the host services.
- `prepare` cross-builds the five go binaries for linux arm64.
- `watch-import` converts `~/notes/watchlist.md` into `watch-seed.json`. the
  service only reads that seed when `/var/lib/watch/state.json` does not exist
  yet, so it is a first-boot import and never overwrites a live list.
- `install` installs the staged service payload on io.

after `mise install`, develop a web app with `pnpm dev` in its `*-web`
directory. Build it once before starting the matching api, since the Go binary
embeds the generated `dist/` directory:

```sh
cd services/io/web && pnpm build
cd ../server && go run .
```

Run every repository check from the root with `mise run check`. Deploy with:

```sh
bin/pi sync io --services
```

the watchlist keeps its whole state in `/var/lib/watch/state.json`, written
through a temp file and renamed, so nothing but that one file has to be backed
up. re-importing the markdown means deleting it first:

```sh
services/io/watch-import ~/notes/watchlist.md
bin/pi sync io --services
ssh io 'sudo rc-service watch stop && sudo rm /var/lib/watch/state.json && sudo rc-service watch start'
```
