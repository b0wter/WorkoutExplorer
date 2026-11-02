# Running Workout Explorer

Workout Explorer turns an uploaded FIT file (from a bike computer or sports watch) into a shareable page with map, charts and laps. There are no accounts. Every workout is deleted after 7 days by default (at most 30). Nothing is backed up, on purpose: a lost server loses its workouts.

Design background: `architect/F-31-server-deploy-and-monitoring.md` (DP-1 to DP-18), ADR-034, ADR-035.

## Which way should I run it?

| | Self-host (sections 1 to 12) | Operator deploy (section 13) |
| --- | --- | --- |
| For | You. Anyone with Docker | The project operator's own VPS |
| How | You build one image from a clone and run it with Docker Compose | `deploy/deploy.sh` copies a GitHub Release (built by CI on a version tag) to the server |
| Needs | Docker, Compose, a clone of this repository | SSH access, a pushed `vX.Y.Z` tag with its release, the GitHub CLI |

**Pick self-hosting.** Section 13 is only for the operator and can be skipped. Instead of building the image you can also use a public release (section 6a).

## How it fits together

```
browser --HTTPS--> your reverse proxy --HTTP--> 127.0.0.1:8088 (Caddy, in Docker)
                                                   |-- /w/..., /assets/...: files from disk
                                                   '-- /api/*  --> Go server (in Docker, no internet)
```

- Your reverse proxy (Caddy, nginx, Traefik, ...) handles the domain and TLS. This project does not get certificates and opens no public port.
- Our Caddy listens on plain HTTP, by default on `127.0.0.1:8088` only.
- Two containers (`app` and `caddy`) run from one image. The only host folder they use is the data directory (default `/srv/workouts`).

## 1. Prerequisites

| Need | Detail |
| --- | --- |
| CPU | **linux/amd64 (x86-64).** arm64 (Raspberry Pi, Apple silicon servers) is not supported: the Caddy pin has only an amd64 checksum |
| RAM | 2 GB. The `app` container is capped at 1200 MB (`compose.yaml`); leave room for the OS, Docker and Caddy. Building the image also needs memory (see "Updating", tip for small hosts) |
| Disk | About 3 GB for the image and build cache, plus the storage cap you set (default 1000 MB), plus Docker logs (at most 5 x 10 MB per container). Local disk, one file system |
| Docker | Docker Engine with the Compose plugin, **Compose 2.24 or newer** (`docker compose version`). Default daemon settings |
| Git | To clone the repository |
| Domain and proxy | A domain name pointing at your host, and a reverse proxy **on the same host** that terminates TLS (section 5) |
| Port | `8088/tcp` free on the host loopback (or another port, see `WX_LISTEN`) |
| Location | For GDPR purposes, host in the EU if your visitors are there. No analytics or third-party trackers are used. The map tiles come from `tiles.openfreemap.org` (the visitor's browser fetches them) |

Check:

```sh
docker version --format '{{.Server.Version}}'
docker compose version          # Docker Compose version v2.24.0 or newer
uname -m                        # x86_64
```

## 2. Get the code

```sh
git clone <this repository's URL> workout-explorer
cd workout-explorer/deploy
```

All following commands run in `deploy/` unless stated otherwise.

## 3. Create the data directory

Compose never creates host folders. The server runs as user ID 10001, Caddy as 10002 (and group 10001, so it can read published files). The data directory must belong to 10001:

```sh
sudo install -d -o 10001 -g 10001 -m 0750 /srv/workouts /srv/workouts/public
```

You should see: `ls -ld /srv/workouts /srv/workouts/public` shows `drwxr-x---` and owner `10001 10001` for both.

Keep it on the local disk. To use another path, set `WX_DATA` (section 4) to it.

## 4. Configure

```sh
cp example.env .env
```

Edit `.env`. Then enable the build override by removing the `#` from this line near the end of the file:

```
COMPOSE_FILE=compose.yaml:compose.build.yaml
```

Without that line, `docker compose` expects the operator's prebuilt files and fails. Leave `WX_WEB` at its default (the web files are inside the image).

Unset or empty keys use the default. An invalid value stops the server at start (exit code 2) with a message naming the key.

| Key | Default | Meaning and when to change it |
| --- | --- | --- |
| `WORKOUT_EXPLORER_BASE_URL` | none, **required** | The public address of your site, as visitors type it: `https://workouts.example.org`. Scheme and host (and port if not standard) only: no path, no trailing slash. Used for the absolute links in preview cards (Open Graph). A wrong value breaks previews in chat apps (section 12) |
| `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` | `1000` | Total size of the data directory in megabytes (1 to 1000000). When full, uploads are refused with 503. Keep it below free disk minus Docker logs and one over-limit upload |
| `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` | `4` | Uploads processed at once (1 to 16). More means more memory; the 1200 MB cap in `compose.yaml` assumes 4. Extra uploads see "try again in a minute" |
| `WORKOUT_EXPLORER_PARSE_TIMEOUT` | `10` | Seconds allowed to parse one file (5 to 60). Raise it only if big files fail on a slow host |
| `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4` | `60` | Uploads per hour per IPv4 address (1 to 240) |
| `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV6` | `60` | Uploads per hour per IPv6 /64 prefix (1 to 240) |
| `WX_DATA` | `/srv/workouts` | Host folder for all workout data. Same path inside the containers |
| `WX_WEB` | `/srv/workout-explorer/web` | Web files. **Leave as is when self-hosting**; they come from the image |
| `WX_LISTEN` | `127.0.0.1:8088` | Where Caddy's HTTP port is published on the host. Your proxy forwards here. **Never a public address**: Docker publishes ports around the host firewall, and this port is plain HTTP. Change it only for a proxy in a container (section 5) |
| `WX_TRUSTED_PROXIES` | `10.250.0.1` (when unset) | Source address(es) of your reverse proxy as Caddy sees them. Separate several by spaces, not commas. See section 5 |
| `COMPOSE_FILE` | unset | Self-hosters: `compose.yaml:compose.build.yaml` |

`WORKOUT_EXPLORER_TRUSTED_PROXY` (singular) is set by `compose.yaml` itself. Do not put it in `.env`.

The file holds no secret today. Still, do not commit it.

## 5. Reverse proxy

Do this before or after starting (section 6), but it must be done. All examples use `workouts.example.org` and forward to `127.0.0.1:8088`.

Rules for any proxy:

- **Pass `X-Forwarded-For` with the client address as its last entry.** Without it, the server sees every visitor as your proxy and the upload limit is shared by everybody (section 12).
- **Do not add, change or remove response headers.** The app sets a strict Content-Security-Policy, `Referrer-Policy`, `X-Content-Type-Options`, `X-Robots-Tag` and `Strict-Transport-Security` (HSTS, once) itself (ADR-032). A second HSTS or a different CSP from the proxy breaks the policy.
- No response caching.
- Allow request bodies of at least 21 MB (uploads are capped at 20 MB; the extra bytes are overhead).
- Allow `/api/*` requests to last at least 480 seconds.
- Do not log access for this site, if you can. Your proxy is outside the project's privacy guarantees; its logs hold visitor addresses.

### Proxy on the host, as a normal process (systemd, apt package)

Leave `WX_LISTEN` and `WX_TRUSTED_PROXIES` unset. A host process connecting to `127.0.0.1:8088` shows up in Caddy as `10.250.0.1`, which is the default.

Caddy (`/etc/caddy/Caddyfile`). `reverse_proxy` appends `X-Forwarded-For` and passes headers through by default:

```
workouts.example.org {
	reverse_proxy 127.0.0.1:8088 {
		transport http {
			response_header_timeout 8m
		}
	}
}
```

nginx (inside a `server { listen 443 ssl; server_name workouts.example.org; ... }` block):

```nginx
client_max_body_size 21m;
location / {
    proxy_pass http://127.0.0.1:8088;
    proxy_set_header Host $host;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_read_timeout 480s;
    proxy_send_timeout 480s;
    proxy_request_buffering off;   # optional: stream uploads
    access_log off;
}
```

Do not use `add_header`, `proxy_hide_header` or `proxy_cache` for this site. If you serve other sites that set HSTS globally (`add_header` at `http` level), make sure it is not applied here.

### Proxy in a container (Traefik, nginx-proxy, Caddy in Docker, ...)

A container cannot reach `127.0.0.1` of the host. Publish Caddy on an address the proxy can reach, still not public, and trust only the proxy's source address.

1. Pick the address: the host's Docker bridge gateway, usually `172.17.0.1` (`ip -4 addr show docker0`). In `.env`: `WX_LISTEN=172.17.0.1:8088`.
2. Find your proxy's network subnet and set `WX_TRUSTED_PROXIES` to it, the narrowest range that holds the proxy: `docker network inspect <proxy network> --format '{{range .IPAM.Config}}{{.Subnet}}{{end}}'`, for example `WX_TRUSTED_PROXIES=172.18.0.0/16`. Several entries are separated by spaces.
3. Check the setting: after step 6 below, upload with two different browsers and watch for the rate-limit symptoms in section 12. If the proxy's source does not match, set the value from what the proxy actually shows.
4. Make sure the proxy container really has a route to that address (on Linux, the gateway address of `docker0` is reachable from containers; a firewall rule may be needed).

A proxy using `network_mode: host` behaves like a host process: use the defaults and `127.0.0.1:8088`.

Traefik, file provider (dynamic configuration):

```yaml
http:
  routers:
    workouts:
      rule: Host(`workouts.example.org`)
      entryPoints: [websecure]
      tls: {certResolver: letsencrypt}
      service: workouts
  services:
    workouts:
      loadBalancer:
        servers:
          - url: http://172.17.0.1:8088
        responseForwarding: {flushInterval: 100ms}
```

Do not attach header middlewares, compression middlewares that rewrite headers, or an access log for this router. Traefik adds `X-Forwarded-For` itself. Raise `respondingTimeouts` of the `websecure` entry point (`readTimeout`, `writeTimeout`, `idleTimeout`) to at least 480s, or set them to 0.

Traefik with Docker labels: the same router, but the backend is a host address, not a container, so the file form above is simpler.

Wrong `WX_TRUSTED_PROXIES`: too narrow shows as every visitor sharing one upload limit. Too wide lets clients choose their own address. Only the proxy's own source may be listed.

## 6. Build and start

```sh
docker compose build
docker compose up -d
```

The first build takes several minutes (it builds the web app and the server, and downloads the pinned Caddy with a checksum). You should see `Image workout-explorer:local Built`, then `Container workout-explorer-app-1 Started` and `Container workout-explorer-caddy-1 Started`.

Check:

```sh
docker compose ps                                  # both services "Up"
curl -fsS http://127.0.0.1:8088/api/health         # {"status":"ok","version":"unknown"}
curl -fsS https://workouts.example.org/api/health  # the same, through your proxy
```

The image reports version `unknown`; that is normal. Open `https://workouts.example.org/` in a browser: the upload page appears. Upload a FIT file, publish it and open the link.

If it does not start: `docker compose logs app` names the bad setting.

## 6a. Alternative: use a public release instead of building

Releases are public; no GitHub login is needed. Take the latest `vX.Y.Z` from the repository's Releases page, then (sections 1, 3 and 4 still apply; `.env` as in section 4, with `COMPOSE_FILE` unset):

```sh
cd workout-explorer                  # the clone from section 2, for compose.yaml, Caddyfile and fetch-caddy.sh
v=v1.4.0                             # the release you chose
base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
mkdir -p ../run && cd ../run
curl -fLO $base/workout-explorer-$v-linux-amd64.tar.gz -O $base/SHA256SUMS
sha256sum -c SHA256SUMS              # must print: ...tar.gz: OK
mkdir -p bin conf web
tar -xzf workout-explorer-$v-linux-amd64.tar.gz workout-explorer web
mv workout-explorer bin/
sh ../workout-explorer/deploy/fetch-caddy.sh bin/caddy
cp ../workout-explorer/deploy/compose.yaml ../workout-explorer/deploy/example.env .
cp ../workout-explorer/deploy/Caddyfile conf/Caddyfile
```

Then, in `run/`, start with `docker compose up -d` (set `WX_WEB` in `.env` to the absolute path of `run/web`). This is the layout `deploy.sh` produces on the operator's server (section 13). `/api/health` reports the tag as its version. To update, repeat with the new tag and run `docker compose restart app`.

## 7. Check the result

- `ss -ltn`: the only listener of this project is `WX_LISTEN`'s port. Nothing on 80 or 443 comes from it.
- `curl -sI https://workouts.example.org/ | grep -i strict-transport-security` prints exactly one line.
- `docker compose logs caddy` and `docker compose logs app` contain no visitor addresses (section 11).
- Test with your own file first. This project ships its privacy page in a later feature (F-29); until then, tell visitors yourself what is stored (section 9).

## 8. Updating

```sh
cd workout-explorer
git pull
cd deploy
docker compose build
docker compose up -d
```

Both containers are recreated, so pages are unreachable for a moment. Published pages may lose their styles and scripts after an update that changes the web files, until the `regenerate` command exists (F-30, not yet built). When it exists, run it afterwards:

```sh
docker compose exec app /srv/workout-explorer/bin/workout-explorer regenerate
```

Until then, such workouts expire on their own (at most 30 days).

Small hosts (2 GB): stop the app while building (`docker compose stop app`, then build, then `docker compose up -d`), or build on another amd64 machine and move the image: `docker save workout-explorer:local | ssh host docker load`.

Remove old build leftovers when you like: `docker image prune`.

## 9. Data, backups and privacy

- All workout data is in `WX_DATA` (default `/srv/workouts`): the database (`workouts.db`), the private originals (`private/`), and what is served to visitors (`public/`).
- **There are no backups, by design.** Workouts are short-lived, and a copy would outlive the deletion that the app promises to its users. Do not back up this folder. Turn off provider snapshots for the host if you can. Losing the server loses its workouts; that is accepted.
- Expiry deletes everything stored for a workout, in the database and on disk. Drafts are deleted after 48 hours.
- Only unlisted, published workouts have files under `public/`. Original FIT files are never served.

## 10. Stop, remove, delete all data

```sh
docker compose stop            # stops both; stays stopped after reboot
docker compose start           # starts them again
docker compose down            # removes the containers and networks, keeps data
```

A crash, an out-of-memory kill or a reboot restarts the containers by themselves, unless you stopped them.

To delete everything, including all workouts:

```sh
docker compose down
sudo rm -rf /srv/workouts
docker rmi workout-explorer:local
```

Delete one workout by hand: the `delete` command is planned (F-27, not yet built). Reports from users (INF-2, INF-3) will be read with `sqlite3` on `workouts.db` in the data directory; that command and the report table do not exist yet either.

## 11. Logs

```sh
docker compose logs --tail 100 app
docker compose logs -f app
docker compose logs --tail 100 caddy
```

- The server logs JSON lines (start, expiry, errors). Caddy writes no access log, and the server writes none. Visitor addresses are not logged by either.
- Docker keeps them in rotating files on the host (at most 5 files of 10 MB per container). `docker compose down` removes them.
- Your reverse proxy has its own logs. Those are yours to configure (section 5).

## 12. Troubleshooting

| Symptom | Likely cause and fix |
| --- | --- |
| Proxy shows 502 / "bad gateway" | The containers are not running (`docker compose ps`), or the proxy points to the wrong address or port. Test `curl http://127.0.0.1:8088/api/health` on the host. A proxy in a container cannot use `127.0.0.1` (section 5) |
| `docker compose up` says port is already allocated | Something else uses 8088. Find it with `ss -ltnp 'sport = :8088'`. Set another port in `.env`, for example `WX_LISTEN=127.0.0.1:8090`, and point the proxy there |
| Everyone gets "too many uploads" / limit hits quickly | The server sees your proxy as the client. `X-Forwarded-For` is missing, or `WX_TRUSTED_PROXIES` does not match the proxy's source address, so Caddy ignores the header. Fix the proxy header and the setting, then `docker compose up -d` |
| People can bypass the limit | `WX_TRUSTED_PROXIES` is too wide. List only the proxy's own address or its network |
| Preview cards (WhatsApp, Telegram, ...) show no image or a wrong link | `WORKOUT_EXPLORER_BASE_URL` differs from the public address (http vs https, `www`, port). Fix it, `docker compose up -d`, and republish or wait: pages published before keep the old links until they are regenerated (F-30 not yet built) |
| `app` exits at start, `docker compose logs app` shows "must be ..." | A setting is invalid or `WORKOUT_EXPLORER_BASE_URL` is missing. Fix `.env` |
| Upload fails with permission errors, or `app` cannot create files | The data directory is not owned by 10001: `sudo chown -R 10001:10001 /srv/workouts && sudo chmod 0750 /srv/workouts /srv/workouts/public` |
| Published pages give 404 from the container but exist | Caddy cannot read `public/`: mode must be 0750 with group 10001 (section 3) |
| Uploads return 503 | The storage limit is reached. Raise `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` (and check free disk), or wait for expiry |
| "network ... overlaps" or no route to the other 10.250.x hosts | The project fixes two networks, `10.250.0.0/29` (edge) and `10.250.1.0/29` (backend). If your host, VPN or other containers already use them, change them in `compose.yaml` in all places: the two `subnet` lines, `WORKOUT_EXPLORER_TRUSTED_PROXY`, both `ipv4_address` lines of Caddy, the `app` address, and the `WX_TRUSTED_PROXIES` default (the edge gateway). Then `docker compose up -d` |
| `docker compose` complains about `!override` or unknown tag | Compose is older than 2.24. Upgrade the plugin |
| Build fails with "exec format error" or on arm64 | Only linux/amd64 is supported |
| Build is killed (out of memory) | Stop `app` while building, or build elsewhere (section 8) |
| Styles missing on old workouts after an update | See section 8: regenerate is not yet available; those workouts expire |

## 13. Operator deploy (for the project operator only)

Everything below is the operator's own VPS setup. Self-hosters can skip it.

Shape: `deploy.sh` downloads the GitHub Release of a version tag (binary and web files), copies it to the server, and runs the same `compose.yaml` without the build override. Caddy and Go binaries are bind-mounted read-only into a distroless base image (DP-2).

### 13.1 The VPS (DP-13)

- EU data centre, **linux/amd64**, at least 2 GB RAM, Debian stable, local disk. About 600 MB of RAM stays free for the OS, Docker and Caddy next to the 1200 MB cap of the app.
- **Provider backups and snapshots off.** No backup job, no off-server copy of `/srv/workouts`.
- SSH by key only; unattended security upgrades on (`unattended-upgrades`). Packages: `curl`, `tar`.
- Firewall: this setup opens nothing. Docker's own rules bypass a host firewall, which is why `WX_LISTEN` stays on loopback.
- Docker Engine and the Compose plugin from Docker's Debian repository, default daemon settings. Docker starts at boot; there is no unit of ours.

### 13.2 Users and directories (DP-3 to DP-5)

`deploy` is the account `deploy.sh` logs in as: it owns the deploy directory and is in the `docker` group (root-equivalent on this box).

```sh
adduser --disabled-password deploy && usermod -aG docker deploy      # add your SSH key to ~deploy/.ssh/authorized_keys
install -d -o deploy -g deploy -m 0755 /srv/workout-explorer /srv/workout-explorer/bin /srv/workout-explorer/conf /srv/workout-explorer/web /srv/workout-explorer/web/assets
install -d -o 10001 -g 10001 -m 0750 /srv/workouts /srv/workouts/public
install -m 0600 -o deploy -g deploy /dev/null /srv/workout-explorer/.env
```

Copy `deploy/example.env` to `/srv/workout-explorer/.env` and edit it as in section 4, but **do not** enable `COMPOSE_FILE` (the operator path has no build override). The file must stay readable by `deploy` (`0600`). Your reverse proxy: section 5.

### 13.3 First and later deploys

**Cut a release.** On the commit to ship (pushed; CI runs only on `v*` tags and by hand via "Run workflow", never on branch pushes, so run `make ci` locally first):

```sh
git tag v1.4.0 && git push origin v1.4.0
```

CI builds with that version, and the `release` job publishes `workout-explorer-v1.4.0-linux-amd64.tar.gz` and `SHA256SUMS` (only for plain `vX.Y.Z` tags, and only if the tests pass). A release is never replaced: if it is broken or the job failed, ship the next version (or delete the release by hand and use "Re-run all jobs" on the tag run in GitHub; re-running only `release` fails once the 1-day hand-off artefact has expired).

**Deploy it.** On your machine, from a clean checkout of that tag (`git checkout v1.4.0`; `gh auth login` once):

```sh
deploy/deploy.sh v1.4.0 deploy@your.vps
```

Run it from a linux/amd64 machine with `git`, `gh`, `ssh`, `scp`, `tar`, `curl`, `sha256sum` and `sha512sum` (it runs the downloaded Caddy locally to validate the Caddyfile; macOS and arm64 do not work).

It refuses a dirty tree, HEAD not at the tag, a bad checksum, or a release built from another commit. It downloads the release, copies it, starts the services, restarts `app` and waits (60 s) for `/api/health` through Caddy to report exactly the tag as its version. It never connects from CI and has no rollback; a failed wait prints `docker compose ps` and the last log lines of `app` and `caddy` and exits 1 (fix forward with a newer version). Old `web/assets/` files stay (additive); remove old bundles by hand when you like.

API downtime is the `app` restart. A new Caddy binary, a changed `compose.yaml`, or a changed `.env` value that Caddy itself uses (`WX_LISTEN`, `WX_TRUSTED_PROXIES`, `WX_DATA`, `WX_WEB`) recreates the Caddy container (other `.env` changes recreate only `app`): published pages are unreachable for a moment. A Caddyfile-only change is a graceful reload.

### 13.4 Day to day

- Stop: `docker compose stop app` stays stopped across reboots. Start: `docker compose start app`. Run `docker compose` in `/srv/workout-explorer`.
- Delete a workout (once F-27 adds the subcommand): `docker compose exec app /srv/workout-explorer/bin/workout-explorer delete <public_id>`.
- Reports: `sqlite3` on the database in `/srv/workouts` (INF-2, INF-3).
- Uptime monitor (yours, later): check `/api/health` and `/`, never a workout page.
- Links go to friends only after F-29 (privacy policy).

### 13.5 Measurements to record (AC 8, 9, 10)

Fill these in on the real VPS; the Compose cap must stay at or above the first value (+ headroom, DP-8).

| What | How | Value |
| --- | --- | --- |
| `memory.peak` of `app`, 4 uploads at the parse limits at once + one publish with its preview | `cat /sys/fs/cgroup/system.slice/docker-$(docker inspect -f '{{.Id}}' workout-explorer-app-1).scope/memory.peak` after the run | not yet measured |
| Cap in `compose.yaml` | `mem_limit` = `memswap_limit`: (peak + 112 MB rate table) x 1.2, rounded up to 100 MB | 1200m (from the F-09 range 620 to 861 MB) |
| `parse` time of the all-caps synthetic file (ADR-017 item 7) | generate the file on a dev machine (`cd src/server && go run ./tools/corpusgen -cap-file /tmp/cap.fit`), copy it into `/srv/workouts`, then `time docker compose exec app /srv/workout-explorer/bin/workout-explorer parse /srv/workouts/cap.fit`; delete it afterwards | not yet measured |
| Parse time limit `WORKOUT_EXPLORER_PARSE_TIMEOUT` | env file | 10 s (default) |

A fifth simultaneous upload gets "try again in a minute"; `docker inspect -f '{{.State.OOMKilled}}' workout-explorer-app-1` stays `false`.
