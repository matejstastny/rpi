# io

the storage and application pi. everything here is reachable only from the
house lan and the tailnet; caddy terminates tls with a DNS-01 cert and gates
every name on the source address.

the apps themselves live at the repo root (`ytdl/`, `share/`, `dash/`,
`torrent/`) and are listed in `apps.toml`. this directory holds only what
belongs to the host: the Caddyfile, the configs for the packaged services
(`jellyfin`, `transmission`, `victoria-metrics`), the metrics scrape config,
and `install`.

## notes worth keeping

transmission's rpc answers on loopback only. `transmission-done` hardlinks a
finished video into jellyfin's library, which is why transmission is in the
`media` group.
