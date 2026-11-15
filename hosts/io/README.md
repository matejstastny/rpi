# io

the storage and application pi. everything here is reachable only from the
house lan and the tailnet; caddy terminates tls with a DNS-01 cert and gates
every name on the source address.

the apps themselves live at the repo root (`ytdl/`, `share/`, `dash/`,
`torrent/`, `watch/`) and are listed in `apps.toml`. this directory holds only
what belongs to the host: the Caddyfile, the configs for the packaged services
(`jellyfin`, `transmission`, `victoria-metrics`, `vaultwarden`), the metrics
scrape config, and `install`.

## notes worth keeping

`vaultwarden.confd` overrides the apk-shipped default to bind loopback only and
point at `DOMAIN=https://vault.elara.boo`. the package's own initd is used as-is.

transmission's rpc answers on loopback only. `transmission-done` hardlinks a
finished video into jellyfin's library, which is why transmission is in the
`media` group.

the watchlist looks up metadata through a chain rather than one provider,
because AniList has gone down for days at a time and TMDB needs a key that may
not be set: anime is AniList then Kitsu, series is TMDB then TVmaze, movies is
TMDB then IMDb's title suggester. only TMDB needs a key
(`WATCH_TMDB_KEY` in `/etc/watch/watch.env`). posters are fetched once and
cached under `/var/lib/watch/art`, and the browser only ever loads art through
the service, never from a provider cdn.

## re-importing the watchlist

the watchlist keeps all its state in `/var/lib/watch/state.json`, written
through a temp file and renamed, so that one file is the whole backup. the seed
is only read when it does not exist:

```sh
watch/import ~/notes/watchlist.md
mise run deploy io
pi ssh io 'sudo rc-service watch stop && sudo rm /var/lib/watch/state.json && sudo rc-service watch start'
```
