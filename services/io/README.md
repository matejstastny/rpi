# io services

the storage and application pi. its services are only reachable on the house
lan and tailnet.

- `server/` + `web/` is the ytdl service.
- `share-server/` + `share-web/` is quickshare.
- `dash-server/` + `dash-web/` is the fleet dashboard.
- `Caddyfile`, `*.initd`, `*.confd`, and `scrape.yml` are the host services.
- `prepare` cross-builds the three go binaries for linux arm64.
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
