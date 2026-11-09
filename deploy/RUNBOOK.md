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
                                                   '-- /api/*  --> Go server (in Docker, no internet unless notifications are on, section 4a)
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
| Docker | **Docker Engine 28 or newer** (ADR-039 relies on it to block direct access to unpublished container ports) with the Compose plugin, **Compose 2.24 or newer** (`docker compose version`). Default daemon settings |
| Git | To clone the repository |
| `sqlite3` | Only to read reports from visitors (section 10). Debian: `apt install sqlite3` |
| Domain and proxy | A domain name pointing at your host, and a reverse proxy **on the same host** that terminates TLS (section 5) |
| Port | `8088/tcp` free on the host loopback (or another port, see `WX_LISTEN`) |
| Location | For GDPR purposes, host in the EU if your visitors are there. No analytics or third-party trackers are used. The map tiles come from `tiles.openfreemap.org` (the visitor's browser fetches them) |

Check:

```sh
docker version --format '{{.Server.Version}}'   # 28.0 or newer
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

### The About page: operator file

The About page has a fixed part (what the app does with data) and a part only you can write: who runs this site, how to reach you, your hosting, what your web server logs, backups, legal basis, complaints. It lives in a plain text file that is **public**: every visitor reads it. `app` mounts the folder `/srv/workout-explorer/operator` read-only (`deploy.sh` never writes there). Create it once (`deploy.sh` also creates the empty folder on the operator server, so an upgrade does not stop `app`), then write the file:

```sh
sudo install -d -o "$USER" -m 0755 /srv/workout-explorer/operator
cp deploy/operator.example.txt /srv/workout-explorer/operator/about.txt   # fill every [bracketed prompt], including your proxy's logging
```

Set `WORKOUT_EXPLORER_OPERATOR_FILE=/srv/workout-explorer/operator/about.txt` in `.env` (section 4) and restart `app` after every edit (`docker compose restart app`). Format: `## Heading`, `- list item`, other lines are paragraphs, a blank line ends a block; no links or markup (everything is shown as plain text); at most 16 KiB. Without the file the page shows `[OPERATOR DETAILS: ...]` and `app` logs a warning: replace it before you send links to anyone. Never commit this file.

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
| `WORKOUT_EXPLORER_PUSHOVER_TOKEN` | unset (off) | Pushover application token for report notifications, section 4a. **A secret** |
| `WORKOUT_EXPLORER_PUSHOVER_USER` | unset (off) | Pushover user key, section 4a. **A secret** |
| `WORKOUT_EXPLORER_OPERATOR_FILE` | unset (placeholder) | Absolute path of your About page text, see "The About page: operator file" above. `WX_OPERATOR` (default `/srv/workout-explorer/operator`) is the host folder mounted for it |
| `COMPOSE_FILE` | unset | Self-hosters: `compose.yaml:compose.build.yaml`; with notifications add `:compose.notify.yaml` (section 4a) |

`WORKOUT_EXPLORER_TRUSTED_PROXY` (singular) is set by `compose.yaml` itself. Do not put it in `.env`.

With notifications on, the file holds two secrets. Keep it mode `0600` and never commit it.

## 4a. Notifications (optional)

When a visitor reports a workout, the server can send you a push message through [Pushover](https://pushover.net). Off by default. The message is only `New report #<number>: <reason>` (title "Workout Explorer"), never the link, the workout's ID or title, or the visitor's text. Pushover is a third party outside the EU, and its messages pass through Apple's or Google's push services. At most 10 messages per hour are sent; the rest are counted into the next one ("(+3 more)"). A message can be lost (Pushover down, restart); the reports table is the record.

Turning it on needs **both** steps; one without the other does not work:

1. In Pushover, create an application and note its API token; your user key is on your Pushover dashboard. Both are 30 letters and digits. In `.env`, remove the `#` from the two lines and fill them in:

   ```
   WORKOUT_EXPLORER_PUSHOVER_TOKEN=<application token>
   WORKOUT_EXPLORER_PUSHOVER_USER=<user key>
   ```

2. Give the server a route out by adding `compose.notify.yaml` to `COMPOSE_FILE` in `.env`:

   ```
   COMPOSE_FILE=compose.yaml:compose.build.yaml:compose.notify.yaml   # self-hosters
   COMPOSE_FILE=compose.yaml:compose.notify.yaml                      # operator (section 13); release users (section 6a) copy compose.notify.yaml next to compose.yaml
   ```

Then `docker compose up -d`. Check: `docker compose logs app | grep 'server started'` shows `"notifications":true`. Report one of your own published workouts from its page; a message arrives within seconds.

Without the override the server has no internet access at all. With it, the server can reach any host on the internet, not only Pushover (Docker cannot filter by name); that is the price of turning notifications on. Only one of the two settings, or a value that is not 30 letters and digits, stops the server at start (exit code 2, the message names the setting but never prints its value).

To turn it off: comment out both settings, remove `:compose.notify.yaml` from `COMPOSE_FILE`, `docker compose up -d`.

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
- Test with your own file first. The About page (`/about`) is the privacy page; your part of it comes from the operator file (see "The About page: operator file").

## 8. Updating

```sh
cd workout-explorer
git pull
cd deploy
docker compose build
docker compose up -d
```

Both containers are recreated, so pages are unreachable for a moment. Then rebuild every stored workout with the new version (published pages, drafts, preview images):

```sh
docker compose exec -T app /srv/workout-explorer/bin/workout-explorer regenerate -data /srv/workouts -web /srv/workout-explorer/web
```

(use your `WX_DATA` and `WX_WEB` if you changed them). It prints `rebuilt=`, `deleted=`, `kept=` and `skipped=` counts and the asset names pages still use; its log lines go to stderr. Exit 0 means every workout was processed. A workout whose file the new version cannot read at all is deleted (as if it had expired); one that failed for another reason (load, disk) is kept unchanged and rebuilt by the next run, so simply run it again. Exit 1: it stopped early or could not start; run it again. With your own image build, old web files are replaced, so pages of kept workouts may lose their styles until a later run rebuilds them.

Small hosts (2 GB): stop the app while building (`docker compose stop app`, then build, then `docker compose up -d`), or build on another amd64 machine and move the image: `docker save workout-explorer:local | ssh host docker load`.

Remove old build leftovers when you like: `docker image prune`.

## 9. Data, backups and privacy

- All workout data is in `WX_DATA` (default `/srv/workouts`): the database (`workouts.db`), the private originals (`private/`), and what is served to visitors (`public/`).
- A workout whose sharer's browser sent a map for the link preview also keeps that map in `private/` (`preview-map.*.png`, up to about 2.5 MB each). It counts towards the storage limit, is never served as it is, and is deleted with the workout. The server itself never contacts the map tile host.
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

### Reports from visitors

Visitors can report a published workout (reason and an optional message, no contact details, no address). A report is deleted with its workout, and at the latest 30 days after it was filed. There is no admin page: read them with `sqlite3` on the host, **read-only and as the server's user ID 10001**:

```sh
sudo setpriv --reuid=10001 --regid=10001 --clear-groups sqlite3 -readonly -line /srv/workouts/workouts.db "
  SELECT r.id AS report, datetime(r.created_at, 'unixepoch') AS filed_utc, r.reason,
         w.public_id, w.visibility AS state, datetime(w.expires_at, 'unixepoch') AS expires_utc, w.title,
         quote(replace(r.message, char(10), '\n')) AS message
  FROM reports r JOIN workouts w ON w.id = r.workout_id
  ORDER BY r.id;"
```

- `setpriv` (util-linux, part of every Debian install) runs it as user ID 10001, which needs no account on the host; `sudo -u` does not work, because that ID has no account. Plain `sudo sqlite3` would not break the server either (SQLite gives side files it creates as root the database's owner), but reading needs no root. The message comes last, in quotes and with line breaks shown as `\n`, so a reporter's text cannot pose as another line of the output.
- **Never change anything with `sqlite3`** (no `DELETE`, no `UPDATE`). It knows nothing of the workout's files. Delete a workout only with the command below.
- The page of a reported workout is `https://<your site>/w/<public_id>`.

### Delete one workout

Any workout (published, draft or past its deadline), with its reports and files, exactly like the owner's delete:

```sh
docker compose exec app /srv/workout-explorer/bin/workout-explorer delete -data /srv/workouts <public_id>
```

It prints the workout's ID, state, created and expiry times, title and number of reports, and asks `Delete? [y/N]`. Add `-y` to skip the question (needed when there is no terminal, for example `docker compose exec -T` or a script). While `app` is stopped use `docker compose run --rm app delete -data /srv/workouts <public_id>` instead. Use your `WX_DATA` path if you changed it.

The page is gone as soon as the command finishes (nothing is cached). Exit status: `0` deleted; `1` not deleted or not completely (the message says which; a leftover public folder goes within about a minute, a private folder at the next start of `app`), also when you answered no; `2` wrong usage or a malformed ID; `3` no workout with this ID (also when it was already deleted). The storage total shown to uploads frees the space only at the next restart of `app`; restart it if uploads are refused as full.

## 11. Logs

```sh
docker compose logs --tail 100 app
docker compose logs -f app
docker compose logs --tail 100 caddy
```

- The server logs JSON lines (start, expiry, errors). Caddy writes no access log, and the server writes none. Visitor addresses are not logged by either.
- `preview map refused` (with a `reason`: `size`, `format`, `dimensions`, `decode`, `stale`) and `preview map not used` (`size`, `read`, `render`) are informational: the link preview then uses the plain card. Many `size` lines for real routes mean the map card is too large for messengers (ADR-041, "Would choose differently if").
- Docker keeps them in rotating files on the host (at most 5 files of 10 MB per container). `docker compose down` removes them.
- Your reverse proxy has its own logs. Those are yours to configure (section 5).

## 12. Troubleshooting

| Symptom | Likely cause and fix |
| --- | --- |
| Proxy shows 502 / "bad gateway" | The containers are not running (`docker compose ps`), or the proxy points to the wrong address or port. Test `curl http://127.0.0.1:8088/api/health` on the host. A proxy in a container cannot use `127.0.0.1` (section 5) |
| `docker compose up` says port is already allocated | Something else uses 8088. Find it with `ss -ltnp 'sport = :8088'`. Set another port in `.env`, for example `WX_LISTEN=127.0.0.1:8090`, and point the proxy there |
| Everyone gets "too many uploads" / limit hits quickly | The server sees your proxy as the client. `X-Forwarded-For` is missing, or `WX_TRUSTED_PROXIES` does not match the proxy's source address, so Caddy ignores the header. Fix the proxy header and the setting, then `docker compose up -d` |
| People can bypass the limit | `WX_TRUSTED_PROXIES` is too wide. List only the proxy's own address or its network |
| Preview cards (WhatsApp, Telegram, ...) show no image or a wrong link | `WORKOUT_EXPLORER_BASE_URL` differs from the public address (http vs https, `www`, port). Fix it, `docker compose up -d`, and republish or wait: pages published before keep the old links until `regenerate` runs (section 8) |
| `app` exits at start, `docker compose logs app` shows "must be ..." | A setting is invalid or `WORKOUT_EXPLORER_BASE_URL` is missing. Fix `.env` |
| Upload fails with permission errors, or `app` cannot create files | The data directory is not owned by 10001: `sudo chown -R 10001:10001 /srv/workouts && sudo chmod 0750 /srv/workouts /srv/workouts/public` |
| Published pages give 404 from the container but exist | Caddy cannot read `public/`: mode must be 0750 with group 10001 (section 3) |
| Uploads return 503 | The storage limit is reached. Raise `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` (and check free disk), or wait for expiry |
| "network ... overlaps" or no route to the other 10.250.x hosts | The project fixes two networks, `10.250.0.0/29` (edge) and `10.250.1.0/29` (backend). If your host, VPN or other containers already use them, change them in `compose.yaml` in all places: the two `subnet` lines, `WORKOUT_EXPLORER_TRUSTED_PROXY`, both `ipv4_address` lines of Caddy, the `app` address, and the `WX_TRUSTED_PROXIES` default (the edge gateway). Then `docker compose up -d` |
| `docker compose` complains about `!override` or unknown tag | Compose is older than 2.24. Upgrade the plugin |
| Build fails with "exec format error" or on arm64 | Only linux/amd64 is supported |
| Build is killed (out of memory) | Stop `app` while building, or build elsewhere (section 8) |
| Styles missing on old workouts after an update | Run `regenerate` (section 8); a workout it reports as kept is rebuilt by a later run |
| No notification; `docker compose logs app` shows `"notification failed","class":"network"` | `compose.notify.yaml` is missing from `COMPOSE_FILE`, or the host has no internet route (section 4a). `"status":400` means a wrong token or user key, `"status":429` Pushover's monthly quota |
| `delete` says "database schema N is not this binary's" | `app` has not yet run with this version. Start it once (`docker compose up -d`), then delete |

## 13. Operator deploy (for the project operator only)

Everything below is the operator's own VPS setup. Self-hosters can skip it.

Shape: `deploy.sh` downloads the GitHub Release of a version tag (binary and web files), copies it to the server, and runs the same `compose.yaml` without the build override. Caddy and Go binaries are bind-mounted read-only into a distroless base image (DP-2).

### 13.1 The VPS (DP-13)

- EU data centre, **linux/amd64**, at least 2 GB RAM, Debian stable, local disk. About 600 MB of RAM stays free for the OS, Docker and Caddy next to the 1200 MB cap of the app.
- **Provider backups and snapshots off.** No backup job, no off-server copy of `/srv/workouts`.
- SSH by key only; unattended security upgrades on (`unattended-upgrades`). Packages: `curl`, `tar`, `sqlite3` (reports, section 10).
- Firewall: this setup opens nothing. Docker's own rules bypass a host firewall, which is why `WX_LISTEN` stays on loopback.
- Docker Engine and the Compose plugin from Docker's Debian repository, default daemon settings. Docker starts at boot; there is no unit of ours.

### 13.2 Users and directories (DP-3 to DP-5)

`deploy` is the account `deploy.sh` logs in as: it owns the deploy directory and is in the `docker` group (root-equivalent on this box).

```sh
adduser --disabled-password deploy && usermod -aG docker deploy      # add your SSH key to ~deploy/.ssh/authorized_keys
install -d -o deploy -g deploy -m 0755 /srv/workout-explorer /srv/workout-explorer/bin /srv/workout-explorer/conf /srv/workout-explorer/web /srv/workout-explorer/web/assets
install -d -o 10001 -g 10001 -m 0750 /srv/workouts /srv/workouts/public
install -d -o deploy -g deploy -m 0755 /srv/workout-explorer/operator   # About page text (public), copy deploy/operator.example.txt there
install -m 0600 -o deploy -g deploy /dev/null /srv/workout-explorer/.env
```

Copy `deploy/example.env` to `/srv/workout-explorer/.env` and edit it as in section 4, but **do not** enable the build override (the operator path has none); with notifications set `COMPOSE_FILE=compose.yaml:compose.notify.yaml` (section 4a; `deploy.sh` copies the file). The file must stay readable by `deploy` (`0600`). Your reverse proxy: section 5.

### 13.3 First and later deploys

**Cut a release.** On the commit to ship (pushed; CI runs only on `v*` tags and by hand via "Run workflow", never on branch pushes, so run `make ci` locally first):

```sh
git tag v1.4.0 && git push origin v1.4.0
```

**CSP hash of the inline theme script** (ADR-032 item 3, F-30 RG-14): a release that changes the script adds its new hash to `deploy/Caddyfile` and keeps the old one, because Caddy reloads before `regenerate` has rebuilt the old pages. Drop the old hash only in a later release, and only if the previous deploy reported `kept=0` and finished with exit 0.

CI builds with that version, and the `release` job publishes `workout-explorer-v1.4.0-linux-amd64.tar.gz` and `SHA256SUMS` (only for plain `vX.Y.Z` tags, and only if the tests pass). A release is never replaced: if it is broken or the job failed, ship the next version (or delete the release by hand and use "Re-run all jobs" on the tag run in GitHub; re-running only `release` fails once the 1-day hand-off artefact has expired).

**Deploy it.** On your machine, from a clean checkout of that tag (`git checkout v1.4.0`; `gh auth login` once):

```sh
deploy/deploy.sh v1.4.0 deploy@your.vps
```

Or, when you are logged in on the server itself (clone and `gh` there, running user owns the deploy directory and is in the `docker` group), use `local` instead of `user@host`: `deploy/deploy.sh v1.4.0 local`. Same steps and checks, no `ssh`/`scp`.

Run it from a linux/amd64 machine with `git`, `gh`, `ssh`, `scp`, `tar`, `curl`, `sha256sum` and `sha512sum` (it runs the downloaded Caddy locally to validate the Caddyfile; macOS and arm64 do not work).

It refuses a dirty tree, HEAD not at the tag, a bad checksum, or a release built from another commit. It downloads the release, copies it, starts the services, restarts `app` and waits (60 s) for `/api/health` through Caddy to report exactly the tag as its version. It never connects from CI and has no rollback; a failed wait prints `docker compose ps` and the last log lines of `app` and `caddy` and exits 1 (fix forward with a newer version).

Then it runs `regenerate` in the `app` container (same `.env`, mounts and memory cap as `serve`), which rebuilds every live workout with the new version, and removes old bundles. It ends with one line:

```
deploy: v1.4.0 done: rebuilt=12 deleted=0 kept=0 skipped=0 files_removed=17 bundles_removed=1
```

- `deleted`: workouts whose stored file the new version rejects; they are gone, as on expiry. Each has a `fit rejected` log line with the device model.
- `kept`: workouts whose rebuild failed for a server-side reason (load, time limit, disk); they keep serving their previous files and bundle. `skipped`: expired or deleted during the run.
- Old bundles: `deploy.sh` keeps one list per release in `bundles/{tag}` (the first deploy with F-30 also writes `bundles/pre-f30` for the files already there). After a successful run it keeps the current release and every release whose files a served page still names, and removes every other file in `web/assets/` and every other list. A viewer who had a page open across the clean-up may need to reload it.
- Exit codes: 0 healthy and regenerated (whatever was deleted or kept); 1 refused or not healthy; 3 `regenerate` did not finish (the new version is live, nothing was removed); 4 the bundle clean-up failed.
- After a deploy: `docker inspect -f '{{.State.OOMKilled}}' workout-explorer-app-1` must print `false` (F-30 RG-13, ADR-017 rev. 8). `OOMKilled` resets on a container restart and may not show for an exec'd process, so also compare `docker inspect -f '{{.RestartCount}}' workout-explorer-app-1` before and after the deploy (it must not grow), and treat `regenerate` exit 3 as a possible OOM kill.
- **Retry** after exit 3 or 4, or to rebuild kept workouts: run `deploy/deploy.sh` again with the same tag. It is idempotent and costs one `app` restart.

API downtime is the `app` restart. A new Caddy binary, a changed `compose.yaml`, or a changed `.env` value that Caddy itself uses (`WX_LISTEN`, `WX_TRUSTED_PROXIES`, `WX_DATA`, `WX_WEB`) recreates the Caddy container (other `.env` changes recreate only `app`): published pages are unreachable for a moment. A Caddyfile-only change is a graceful reload.

### 13.4 Day to day

- Stop: `docker compose stop app` stays stopped across reboots. Start: `docker compose start app`. Run `docker compose` in `/srv/workout-explorer`.
- Delete a workout: `docker compose exec app /srv/workout-explorer/bin/workout-explorer delete -data /srv/workouts <public_id>` (section 10, "Delete one workout").
- Reports: the read-only `sqlite3` query of section 10, as user ID 10001 (INF-2, INF-3). With notifications on (section 4a) each new report also arrives as a Pushover message with its number.
- Uptime monitor (yours, later): check `/api/health` and `/`, never a workout page.
- Links go to friends once the operator file is filled in and `/about` shows your details (F-29).

### 13.5 Measurements to record (AC 8, 9, 10)

Fill these in on the real VPS; the Compose cap must stay at or above the first value (+ headroom, DP-8).

| What | How | Value |
| --- | --- | --- |
| `memory.peak` of `app`, 4 uploads at the parse limits at once + one publish with its preview + one preview-map upload (F-36, ADR-017: about 55 MB total allocation plus one frame GET; use a 16-bit interlaced upload as the worst case) | `cat /sys/fs/cgroup/system.slice/docker-$(docker inspect -f '{{.Id}}' workout-explorer-app-1).scope/memory.peak` after the run | not yet measured |
| Cap in `compose.yaml` | `mem_limit` = `memswap_limit`: (peak + 112 MB rate table) x 1.2, rounded up to 100 MB | 1200m (from the F-09 range 620 to 861 MB) |
| `parse` time of the all-caps synthetic file (ADR-017 item 7) | generate the file on a dev machine (`cd src/server && go run ./tools/corpusgen -cap-file /tmp/cap.fit`), copy it into `/srv/workouts`, then `time docker compose exec app /srv/workout-explorer/bin/workout-explorer parse /srv/workouts/cap.fit`; delete it afterwards | not yet measured |
| Parse time limit `WORKOUT_EXPLORER_PARSE_TIMEOUT` | env file | 10 s (default) |
| `regenerate` alone, dev machine, published all-caps file | `/usr/bin/time -v` | max RSS 365 MB (QA-30-r1-6); not added to the cap, overlap accepted (F-30 RG-13, ADR-017 rev. 8) |
| Time of `regenerate` over 100 synthetic workouts (F-30 AC 10) | `time docker compose exec -T app ... regenerate ...` on a scratch data directory; more than 30 s: run two at a time (RG-13) | at the first F-30 deploy. Dev machine: 10.0 to 10.7 s, max RSS 46 MB (25 each of ride-4h, ride-30min, triathlon, run-30min, all published) |

A fifth simultaneous upload gets "try again in a minute"; `docker inspect -f '{{.State.OOMKilled}}' workout-explorer-app-1` stays `false`.
