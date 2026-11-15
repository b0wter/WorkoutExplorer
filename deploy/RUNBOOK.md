# Running Workout Explorer

Workout Explorer turns an uploaded FIT file (from a bike computer or sports watch) into a shareable page with map, charts and laps. There are no accounts. Every workout is deleted after 7 days by default (at most 30). Nothing is backed up, on purpose: a lost server loses its workouts.

Design background: `architect/F-31-server-deploy-and-monitoring.md`, `architect/F-59-container-image-and-install-paths.md`, ADR-034, ADR-035, ADR-052, ADR-053.

## Which way should I run it?

Every release publishes our app image (`ghcr.io/b0wter/workout-explorer:vX.Y.Z`: the server and the web files, nothing else), a **deploy archive** with the files each path installs, a bare-metal tarball and `SHA256SUMS`. Caddy, the small web server in front of the app, is not ours: the official Caddy image, or Caddy's own Debian package. The privacy and security guarantees are the same on every path.

| | Docker Compose (sections 1 to 12) | Rootless Podman ([PODMAN.md](PODMAN.md)) | Bare metal (section 14) |
| --- | --- | --- | --- |
| For | Anyone with Docker. **Pick this if unsure** | Hosts with Podman and systemd, no Docker daemon | Hosts without containers |
| Runs | Our image and the official Caddy image | The same two images, as systemd user units (Quadlet) | The binary from the tarball and Caddy's apt package, as two systemd services |
| Updates | By hand, section 8 | By hand, PODMAN.md section 9 | By hand, section 14.6 |

Also here: building the image yourself from a clone (section 6a), the project operator's own deploy (section 13), moving an existing tarball install to the image (section 15), releasing (section 13.3).

## How it fits together

```
browser --HTTPS--> your reverse proxy --HTTP--> 127.0.0.1:8088 (Caddy)
                                                   |-- /w/..., /assets/..., pages: files from the data directory
                                                   '-- /api/*  --> Go server (no internet unless notifications are on, section 4a)
```

- Your reverse proxy (Caddy, nginx, Traefik, ...) handles the domain and TLS. This project does not get certificates and opens no public port.
- Our Caddy listens on plain HTTP, by default on `127.0.0.1:8088` only, and reads only `public/` and `site/` of the data directory (default `/srv/workouts`). The server writes the static pages and the web files there at every start; files of older releases stay until no published page needs them.

## 1. Prerequisites

| Need | Detail |
| --- | --- |
| CPU | **linux/amd64 (x86-64).** Our image is published for amd64 only |
| RAM | 2 GB. The `app` container is capped at 1200 MB (`compose.yaml`); leave room for the OS, Docker and Caddy |
| Disk | About 1 GB for the images, plus the storage cap you set (default 1000 MB), plus Docker logs (at most 5 x 10 MB per container). Local disk, one file system |
| Docker | **Docker Engine 28 or newer** (ADR-039 relies on it to block direct access to unpublished container ports) with the Compose plugin, **Compose 2.24 or newer** (`docker compose version`). Default daemon settings. Pulls from `ghcr.io` and Docker Hub, no login |
| Tools | `curl`, `tar`, `sha256sum`. `sqlite3` only to read reports from visitors (section 10). Debian: `apt install curl sqlite3` |
| Domain and proxy | A domain name pointing at your host, and a reverse proxy **on the same host** that terminates TLS (section 5) |
| Port | `8088/tcp` free on the host loopback (or another port, see `WX_LISTEN`) |
| Location | For GDPR purposes, host in the EU if your visitors are there. No analytics or third-party trackers are used. The map tiles come from `tiles.openfreemap.org` (the visitor's browser fetches them) |

Check:

```sh
docker version --format '{{.Server.Version}}'   # 28.0 or newer
docker compose version          # Docker Compose version v2.24.0 or newer
uname -m                        # x86_64
```

## 2. Get the deploy files

Releases are public; no GitHub login is needed. Take the newest `vX.Y.Z` from the repository's Releases page. Use the release's files, not the repository's `deploy/` folder: the release's `compose.yaml` names our image by its digest, so it runs exactly the tested image with the matching Caddyfile.

```sh
v=v1.4.0                             # the release you chose
base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
sudo install -d -o "$USER" -m 0755 /srv/workout-explorer /srv/workout-explorer/conf
cd /srv/workout-explorer
curl -fLO $base/workout-explorer-$v-deploy.tar.gz -O $base/SHA256SUMS
sha256sum -c --ignore-missing SHA256SUMS            # must print: ...deploy.tar.gz: OK
mkdir -p /tmp/wx-$v && tar -xzf workout-explorer-$v-deploy.tar.gz -C /tmp/wx-$v
cp /tmp/wx-$v/compose.yaml /tmp/wx-$v/compose.notify.yaml /tmp/wx-$v/example.env /tmp/wx-$v/*.example.txt .
cp /tmp/wx-$v/Caddyfile conf/Caddyfile
rm workout-explorer-$v-deploy.tar.gz SHA256SUMS
```

All following commands run in `/srv/workout-explorer` unless stated otherwise. The image tag `latest` exists for a quick `docker pull` look only; never put it into `compose.yaml` (a newer app with older deploy files can break pages).

## 3. Create the data directory

Compose never creates host folders. The server runs as user ID 10001, Caddy as 10002 (and group 10001, so it can read published files). The data directory must belong to 10001:

```sh
sudo install -d -o 10001 -g 10001 -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

You should see: `ls -ld /srv/workouts /srv/workouts/public /srv/workouts/site` shows `drwxr-x---` and owner `10001 10001` for all three. `site/` holds the static pages (upload, expired workout, edit, Impressum) and the web files (`site/assets/`) that the server writes at every start; Caddy serves them from there. `bundles/` (next to it, created by the server) lists which web files belong to which release.

Keep it on the local disk. To use another path, set `WX_DATA` (section 4) to it.

### The About page: operator file

The About page has a fixed part (what the app does with data) and a part only you can write: who runs this site, how to reach you, your hosting, what your web server logs, backups, legal basis, complaints. It lives in a plain text file that is **public**: every visitor reads it. `app` reads the folder `/srv/workout-explorer/operator` read-only (`deploy.sh` never writes there). Create it once (`deploy.sh` also creates the empty folder on the operator server, so an upgrade does not stop `app`), then write the file:

```sh
sudo install -d -o "$USER" -m 0755 /srv/workout-explorer/operator
cp operator.example.txt /srv/workout-explorer/operator/about.txt   # fill every [bracketed prompt], including your proxy's logging
```

Set `WORKOUT_EXPLORER_OPERATOR_FILE=/srv/workout-explorer/operator/about.txt` in `.env` (section 4) and restart `app` after every edit (`docker compose restart app`; Podman and bare metal: their restart command). Format: `## Heading`, `- list item`, other lines are paragraphs, a blank line ends a block; no links or markup (everything is shown as plain text); at most 16 KiB. Without the file the page shows `[OPERATOR DETAILS: ...]` and `app` logs a warning: replace it before you send links to anyone. Never commit this file.

### Impressum (optional)

If you need an Impressum (legal notice), write it as a plain text file in the operator folder, in the same format as the operator file above (`## Heading`, `- list item`, paragraphs; at most 16 KiB; no links or markup):

```sh
cp impressum.example.txt /srv/workout-explorer/operator/impressum.txt   # replace every [bracketed prompt]
```

Then:

1. Set `WORKOUT_EXPLORER_IMPRESSUM_FILE=/srv/workout-explorer/operator/impressum.txt` in `.env` (section 4).
2. Restart `app`: `docker compose restart app`. `/impressum` and the "Impressum" footer link on every page appear; a missing, unreadable or invalid file stops `app` at start (exit code 2, the log names the setting).
3. Run `regenerate` (section 8), so already published workout pages get the footer link too.

Do the same after every edit, and to remove it (unset the key, restart, `regenerate`). The page shows the text only after a click on "Show Impressum", fetched from `/api/impressum`, which answers only that page's request and at most 30 per hour per address; `robots.txt` asks search engines to skip both. This makes the text **hard to crawl, not secret**: anyone can read and copy it, and a determined scraper can get it. Never commit the file.

## 4. Configure

```sh
cp example.env .env && chmod 0600 .env
```

Edit `.env`. Unset or empty keys use the default. An invalid value stops the server at start (exit code 2) with a message naming the key.

| Key | Default | Meaning and when to change it |
| --- | --- | --- |
| `WORKOUT_EXPLORER_BASE_URL` | none, **required** | The public address of your site, as visitors type it: `https://workouts.example.org`. Scheme and host (and port if not standard) only: no path, no trailing slash. Used for the absolute links in preview cards (Open Graph). A wrong value breaks previews in chat apps (section 12) |
| `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` | `1000` | Total size of the data directory in megabytes (1 to 1000000). When full, uploads are refused with 503. Keep it below free disk minus Docker logs and one over-limit upload |
| `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` | `4` | Uploads processed at once (1 to 16). More means more memory; the 1200 MB cap in `compose.yaml` assumes 4. Extra uploads see "try again in a minute" |
| `WORKOUT_EXPLORER_PARSE_TIMEOUT` | `10` | Seconds allowed to parse one file (5 to 60). Raise it only if big files fail on a slow host |
| `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4` | `60` | Uploads per hour per IPv4 address (1 to 240) |
| `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV6` | `60` | Uploads per hour per IPv6 /64 prefix (1 to 240) |
| `WX_DATA` | `/srv/workouts` | Host folder for all workout data. Same path inside the containers |
| `WX_LISTEN` | `127.0.0.1:8088` | Where Caddy's HTTP port is published on the host. Your proxy forwards here. **Never a public address**: Docker publishes ports around the host firewall, and this port is plain HTTP. Change it only for a proxy in a container (section 5) |
| `WX_TRUSTED_PROXIES` | `10.250.0.1` (when unset) | Source address(es) of your reverse proxy as Caddy sees them. Separate several by spaces, not commas. See section 5 |
| `WORKOUT_EXPLORER_PUSHOVER_TOKEN` | unset (off) | Pushover application token for report notifications, section 4a. **A secret** |
| `WORKOUT_EXPLORER_PUSHOVER_USER` | unset (off) | Pushover user key, section 4a. **A secret** |
| `WORKOUT_EXPLORER_OPERATOR_FILE` | unset (placeholder) | Absolute path of your About page text, see "The About page: operator file" above. `WX_OPERATOR` (default `/srv/workout-explorer/operator`) is the host folder mounted for it |
| `WORKOUT_EXPLORER_IMPRESSUM_FILE` | unset (no Impressum) | Absolute path of your Impressum text, see "Impressum (optional)" above. Set but missing or invalid stops the server at start |
| `WORKOUT_EXPLORER_IOS_SHORTCUT_URL` | unset (no section) | `https://` link to your iPhone upload Shortcut, shown on the About page, section 4b. Anything that is not an absolute `https://` link stops the server at start |
| `COMPOSE_FILE` | unset | With notifications: `compose.yaml:compose.notify.yaml` (section 4a). Self-builders: section 6a |

`WORKOUT_EXPLORER_TRUSTED_PROXY` (singular) is set by `compose.yaml` itself. Do not put it in `.env`. There is no setting for the images: the release's `compose.yaml` fixes ours by digest and Caddy's by minor version.

With notifications on, the file holds two secrets. Keep it mode `0600` and never commit it.

## 4a. Notifications (optional)

When a visitor reports a workout, the server can send you a push message through [Pushover](https://pushover.net). Off by default. The message is `New report #<number>: <reason>` (title "Workout Explorer") and a **Delete this workout** link; tapping the notification opens the reported workout. Never the workout's title or the visitor's text. Pushover is a third party outside the EU, and its messages pass through Apple's or Google's push services. At most 10 messages per hour are sent; the rest are counted into the next one ("(+3 more)") and get no delete link. A message can be lost (Pushover down, restart); the reports table is the record.

The delete link opens a confirmation page (`/report-delete`) that shows the workout and the report; only its **Delete workout** button deletes, exactly like the delete command in section 10. The link works once, only for that report's workout, and only while the workout lives (at most 30 days after the report). It is a credential: anyone with your Pushover account or your unlocked phone can use it. The lock screen shows only the words "Delete this workout", not the link. **To revoke every delete link** (lost phone, leaked Pushover account): turn notifications off as below and restart `app`; a start without the two settings removes every link, and turning notifications on again does not bring them back.

Turning it on needs **both** steps; one without the other does not work:

1. In Pushover, create an application and note its API token; your user key is on your Pushover dashboard. Both are 30 letters and digits. In `.env`, remove the `#` from the two lines and fill them in:

   ```
   WORKOUT_EXPLORER_PUSHOVER_TOKEN=<application token>
   WORKOUT_EXPLORER_PUSHOVER_USER=<user key>
   ```

2. Give the server a route out by adding `compose.notify.yaml` to `COMPOSE_FILE` in `.env`:

   ```
   COMPOSE_FILE=compose.yaml:compose.notify.yaml                      # the published image
   COMPOSE_FILE=compose.yaml:compose.build.yaml:compose.notify.yaml   # self-builders (section 6a)
   ```

   Rootless Podman: a drop-in instead (PODMAN.md section 8). Bare metal: `notify.conf` (section 14.4).

Then `docker compose up -d`. Check: `docker compose logs app | grep 'server started'` shows `"notifications":true`. Report one of your own published workouts from its page; a message arrives within seconds.

Without the override the server has no internet access at all. With it, the server can reach any host on the internet, not only Pushover (Docker cannot filter by name); that is the price of turning notifications on. Only one of the two settings, or a value that is not 30 letters and digits, stops the server at start (exit code 2, the message names the setting but never prints its value).

To turn it off: comment out both settings, remove `:compose.notify.yaml` from `COMPOSE_FILE`, `docker compose up -d`.

## 4b. iPhone upload Shortcut (optional)

You can offer iPhone users an Apple Shortcut that uploads a FIT file from the share sheet (for example from a watch app or Files) and opens its edit page. The project ships no Shortcut: you build one for your site with the steps below and share it as an iCloud link. With `WORKOUT_EXPLORER_IOS_SHORTCUT_URL` set, the About page shows a section "Upload from your iPhone" with that link; unset, there is no section. The Shortcut uses the normal upload, with the same limits (20 MB, uploads per hour per address); the site stores nothing new.

Build it in the Shortcuts app (replace `https://workouts.example.org` with your `WORKOUT_EXPLORER_BASE_URL`):

1. New Shortcut. In its details, turn on **Show in Share Sheet**; the first action becomes **Receive Files from Share Sheet**. Set "If there's no input" to **Stop and Respond**.
2. **Get Contents of URL**: URL `https://workouts.example.org/api/workouts`, Method **POST**, Request Body **Form**, one field of type **File** with key `file` and value **Shortcut Input**. No other fields and no headers are needed.
3. **Get Dictionary Value**: key `edit_url` from **Contents of URL**.
4. **If** Dictionary Value **has any value**:
   - **Text**: `https://workouts.example.org` followed directly by the variable **Dictionary Value** (`edit_url` is a path such as `/edit/<id>#<token>`).
   - **Open URLs**: the Text. It opens in Safari (the default browser) on the edit page.
5. **Otherwise** (the server refused the file): **Get Dictionary Value** key `message` from **Contents of URL**, then **Show Alert** (or Show Result) with it. Every refusal carries a sentence for people in `message`, for example "This file is not a FIT file." or "Too many uploads. Please try again later."; a `429` (too many uploads) and a `503` "busy" also carry a `Retry-After` header in seconds, which the Shortcut does not need to read. If the response is not JSON at all (site down), the alert is empty: add a fixed fallback text such as "Upload failed. Please try again later."
6. **End If**.

The edit link is a secret: whoever has it can change, download and delete the workout. The Shortcut opens it and does nothing else with it: no Show Result, Quick Look, notification, Copy to Clipboard, Save File, note or message with the Dictionary Value from step 3, and no logging action.

The site accepts FIT files only. A `.zip` from a watch app or a download must be extracted first (tap it in Files); the upload page in the browser can open zip archives, the Shortcut cannot.

Share it: in the Shortcuts app, long-press the Shortcut, **Share**, **Copy iCloud Link**. Put the link in `.env`:

```
WORKOUT_EXPLORER_IOS_SHORTCUT_URL=https://www.icloud.com/shortcuts/<id>
```

Then `docker compose up -d app` and check that `/about` shows the section. Test once on an iPhone: share a FIT file to the Shortcut; Safari opens its edit page. Share a non-FIT file; the alert shows the server's message. To turn it off, comment out the setting and restart `app`.

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

## 6. Start

```sh
docker compose pull
docker compose up -d
```

You should see `Container workout-explorer-app-1 Started` and `Container workout-explorer-caddy-1 Started`.

Check:

```sh
docker compose ps                                  # both services "Up"
curl -fsS http://127.0.0.1:8088/api/health         # {"status":"ok","version":"v1.4.0"}
curl -fsS https://workouts.example.org/api/health  # the same, through your proxy
```

Open `https://workouts.example.org/` in a browser: the upload page appears. Upload a FIT file, publish it and open the link.

If it does not start: `docker compose logs app` names the bad setting.

## 6a. Building the image yourself (not a separate path)

For changes of your own, build the app image from a clone instead of pulling ours. Everything else stays as above (Caddy still comes from the official image); there are no automatic updates for this setup.

```sh
git clone <this repository's URL> /opt/workout-explorer-src && cd /opt/workout-explorer-src && git checkout v1.4.0
cd deploy && cp example.env .env                   # as in section 4, plus the line below
echo 'COMPOSE_FILE=compose.yaml:compose.build.yaml' >> .env
docker compose build --build-arg VERSION=v1.4.0    # several minutes; without the argument the version is "unknown"
docker compose up -d
```

Here `deploy/` of the clone is the deploy directory, and the Caddyfile is mounted from `deploy/conf/`: create it with `mkdir -p conf && cp Caddyfile conf/`. To update: `git fetch && git checkout <new tag>`, copy the Caddyfile again, `docker compose build ... && docker compose up -d`, then `regenerate` (section 8). Small hosts (2 GB): stop `app` while building (`docker compose stop app`).

## 7. Check the result

- `ss -ltn`: the only listener of this project is `WX_LISTEN`'s port. Nothing on 80 or 443 comes from it.
- `curl -sI https://workouts.example.org/ | grep -i strict-transport-security` prints exactly one line.
- `docker compose logs caddy` and `docker compose logs app` contain no visitor addresses (section 11).
- Hardening: `grep -E 'CapPrm|CapEff|CapBnd' /proc/$(docker inspect -f '{{.State.Pid}}' workout-explorer-caddy-1)/status` shows only zeros; the same for `workout-explorer-app-1`.
- Test with your own file first. The About page (`/about`) is the privacy page; your part of it comes from the operator file (see "The About page: operator file").

## 8. Updating

Read the release notes first. Then, with the new `v`:

1. Download and check the new deploy archive and unpack it to `/tmp/wx-$v` as in section 2 (not yet copying anything).
2. Pull the new images before anything changes, and check the new Caddyfile with the pulled Caddy:
   ```sh
   cd /srv/workout-explorer
   docker compose --project-directory . -f /tmp/wx-$v/compose.yaml pull
   docker run --rm --network none --read-only --user 10002:10002 --cap-drop ALL --security-opt no-new-privileges:true \
     --tmpfs /config --tmpfs /data -v /tmp/wx-$v:/c:ro docker.io/library/caddy:2.11 \
     caddy validate --config /c/Caddyfile --adapter caddyfile      # use the Caddy tag of the new compose.yaml
   ```
3. Install: `cp /tmp/wx-$v/compose.yaml /tmp/wx-$v/compose.notify.yaml .` and, if it differs, `cp /tmp/wx-$v/Caddyfile conf/Caddyfile`.
4. `docker compose up -d`. It recreates `app` on the new image (a few seconds without API) and `caddy` only when a newer Caddy patch arrived or its definition changed (a moment without pages). If only the Caddyfile changed: `docker compose exec caddy caddy reload --config /srv/workout-explorer/conf/Caddyfile --adapter caddyfile`.
5. Wait until `curl -fsS http://127.0.0.1:8088/api/health` shows the new version.
6. Rebuild every stored workout with the new version (published pages, drafts, preview images) and remove the web files no page needs any more:
   ```sh
   docker compose exec -T app /srv/workout-explorer/bin/workout-explorer regenerate -data /srv/workouts -web /srv/workout-explorer/web
   ```
   (use your `WX_DATA` if you changed it). It prints `rebuilt=`, `deleted=`, `kept=` and `skipped=` counts, then `files_removed=` and `bundles_removed=`; its log lines go to stderr. Exit 0 means every workout was processed and the old web files cleaned up. A workout whose file the new version cannot read at all is deleted (as if it had expired); one that failed for another reason (load, disk) is kept unchanged with its old web files, and rebuilt by the next run. Exit 1: it stopped early or could not start, nothing was removed; run it again. Exit 4: every workout was processed, but removing old web files failed; run it again later.

`deploy.sh` (section 13) does exactly these steps. Between releases, `docker compose pull && docker compose up -d` brings Caddy's newest patch of its minor.

Remove old images when you like: `docker image prune`.

## 9. Data, backups and privacy

- All workout data is in `WX_DATA` (default `/srv/workouts`): the database (`workouts.db`), the private originals (`private/`), what is served to visitors (`public/`), and the static pages and web assets of the releases still in use (`site/`, `bundles/`; a few MB per release, counted towards the storage limit).
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
docker compose down --rmi all
sudo rm -rf /srv/workouts
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
| `caddy` does not start: a bind mount source `.../site` or `conf` does not exist | Create `site/` as in section 3 and `conf/Caddyfile` as in section 2, then `docker compose up -d` |
| `/`, `/edit/...` or the expired-workout page give 404 | `app` has not started once since `site/` was created; it writes those pages at start (`docker compose logs app`) |
| Uploads return 503 | The storage limit is reached. Raise `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` (and check free disk), or wait for expiry |
| "network ... overlaps" or no route to the other 10.250.x hosts | The project fixes two networks, `10.250.0.0/29` (edge) and `10.250.1.0/29` (backend). If your host, VPN or other containers already use them, change them in `compose.yaml` in all places: the two `subnet` lines, `WORKOUT_EXPLORER_TRUSTED_PROXY`, both `ipv4_address` lines of Caddy, the `app` address, and the `WX_TRUSTED_PROXIES` default (the edge gateway). Then `docker compose up -d` |
| "exec format error", or no image for your platform | Only linux/amd64 is supported |
| `app` fails to pull: `manifest unknown` for `workout-explorer:unreleased` | You use the repository's `compose.yaml`. Use the one from a release's deploy archive (section 2), or build the image yourself (section 6a) |
| `caddy` exits with `operation not permitted` | A Caddy patch pulled between releases no longer runs without capabilities. Report it; a release of ours handles it. The update's check (section 8, step 2) catches this before anything changes |
| Styles missing on old workouts after an update | `app` has not started once with the new version, or `site/assets` was emptied by hand. Restart `app`, then run `regenerate` (section 8) |
| No notification; `docker compose logs app` shows `"notification failed","class":"network"` | `compose.notify.yaml` is missing from `COMPOSE_FILE`, or the host has no internet route (section 4a). `"status":400` means a wrong token or user key, `"status":429` Pushover's monthly quota |
| `delete` says "database schema N is not this binary's" | `app` has not yet run with this version. Start it once (`docker compose up -d`), then delete |

## 13. Operator deploy (for the project operator only)

Everything below is the operator's own VPS setup. It is the Compose path of sections 1 to 12, driven by `deploy/deploy.sh` from the operator's machine.

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
install -d -o deploy -g deploy -m 0755 /srv/workout-explorer /srv/workout-explorer/conf
install -d -o 10001 -g 10001 -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
install -d -o deploy -g deploy -m 0755 /srv/workout-explorer/operator   # About page text (public), from operator.example.txt
install -m 0600 -o deploy -g deploy /dev/null /srv/workout-explorer/.env
```

Fill `/srv/workout-explorer/.env` from `example.env` as in section 4; with notifications set `COMPOSE_FILE=compose.yaml:compose.notify.yaml` (section 4a). It must stay readable by `deploy` (`0600`). Your reverse proxy: section 5. A server still on the tarball layout (`bin/`, `web/` in `/srv/workout-explorer`) is moved once with section 15 before the first deploy of a release with the image.

### 13.3 Releasing and deploying

**One-time, after the first release with an image:** in GitHub, Packages, `workout-explorer`, Package settings: visibility **public**, linked to this repository, and under "Manage Actions access" only this repository with the **Write** role. No other repository or person gets write access; no personal token is ever created for it.

**Before every release:** check for a newer Caddy minor version (CLAUDE.md, Releases). A minor bump changes the tag in `deploy/compose.yaml`, `deploy/podman/workout-explorer-caddy.container` and `deploy/systemd/caddy.pref` together (`make ci` fails if they differ) and must pass `make caddy-check`.

**CSP hash of the inline theme script** (ADR-032 item 3, F-30 RG-14): a release that changes the script adds its new hash to `deploy/Caddyfile` and keeps the old one, because Caddy reloads before `regenerate` has rebuilt the old pages. Drop the old hash only in a later release, and only if the previous deploy reported `kept=0` and finished with exit 0.

**Cut a release.** On the commit to ship (pushed; CI runs only on `v*` tags and by hand via "Run workflow", never on branch pushes, so run `make ci` locally first):

```sh
git tag v1.4.0 && git push origin v1.4.0
```

CI builds and tests the image once, and only if everything passes the `release` job pushes exactly that image to `ghcr.io/b0wter/workout-explorer:v1.4.0`, creates the release (bare-metal tarball, deploy archive with the image's digest in `compose.yaml`, `SHA256SUMS`) and moves `latest` to it if it is the highest version. Only plain `vX.Y.Z` tags release. A version is never replaced: a re-run of the whole workflow builds another digest and stops before pushing; if a release is broken, ship the next version. If the job failed after pushing the image, "Re-run failed jobs" on the same run completes it (within a day, while the hand-off artefacts last).

**Deploy it.** On your machine, from a clean checkout of that tag (`git checkout v1.4.0`; `gh auth login` once):

```sh
deploy/deploy.sh v1.4.0 deploy@your.vps
```

Or, when logged in on the server itself (clone and `gh` there, running user owns the deploy directory and is in the `docker` group): `deploy/deploy.sh v1.4.0 local`. Needs `git`, `gh`, `ssh`, `scp`, `tar` and `sha256sum`; any platform.

It refuses a dirty tree, HEAD not at the tag, a bad checksum, a release built from another commit, or a server still on the tarball layout. It downloads the deploy archive, then on the server pulls both images with the new `compose.yaml` and checks the new Caddyfile with the pulled Caddy; **nothing on the server changes until both succeed**. Then it installs `compose.yaml`, `compose.notify.yaml` and `conf/Caddyfile`, runs `docker compose up -d` (Caddyfile-only change: a graceful reload), and waits (60 s) for `/api/health` through Caddy to report exactly the tag. It never connects from CI and has no rollback; a failed wait prints `docker compose ps` and the last log lines of `app` and `caddy` and exits 1 (fix forward with a newer version).

Then it runs `regenerate` in the `app` container (same `.env`, mounts and memory cap as `serve`), which rebuilds every live workout and removes old web files. It ends with one line:

```
deploy: v1.4.0 done: rebuilt=12 deleted=0 kept=0 skipped=0 files_removed=17 bundles_removed=1
```

- `deleted`: workouts whose stored file the new version rejects; they are gone, as on expiry. Each has a `fit rejected` log line with the device model.
- `kept`: workouts whose rebuild failed for a server-side reason (load, time limit, disk); they keep serving their previous files and web files. `skipped`: expired or deleted during the run.
- Old web files: the server keeps one list per release in `/srv/workouts/bundles/`. After a complete run it keeps the current release and every release whose files a served page still names, and removes the rest from `site/assets/`. A viewer who had a page open across the clean-up may need to reload it.
- Exit codes: 0 healthy and regenerated (whatever was deleted or kept); 1 refused or not healthy; 3 `regenerate` did not finish (the new version is live, nothing was removed); 4 the clean-up failed.
- After a deploy: `docker inspect -f '{{.State.OOMKilled}}' workout-explorer-app-1` must print `false` (F-30 RG-13, ADR-017 rev. 8). `OOMKilled` resets on a container restart and may not show for an exec'd process, so also compare `docker inspect -f '{{.RestartCount}}' workout-explorer-app-1` before and after the deploy (it must not grow), and treat `regenerate` exit 3 as a possible OOM kill.
- **Retry** after exit 3 or 4, or to rebuild kept workouts: run `deploy/deploy.sh` again with the same tag. It is idempotent.

API downtime is the recreate of `app`. A newer Caddy patch, a changed Caddy definition in `compose.yaml`, or a changed `.env` value that Caddy itself uses (`WX_LISTEN`, `WX_TRUSTED_PROXIES`, `WX_DATA`) recreates the Caddy container: published pages are unreachable for a moment.

### 13.4 Day to day

- Stop: `docker compose stop app` stays stopped across reboots. Start: `docker compose start app`. Run `docker compose` in `/srv/workout-explorer`.
- Delete a workout: `docker compose exec app /srv/workout-explorer/bin/workout-explorer delete -data /srv/workouts <public_id>` (section 10, "Delete one workout").
- Reports: the read-only `sqlite3` query of section 10, as user ID 10001 (INF-2, INF-3). With notifications on (section 4a) each new report also arrives as a Pushover message with its number, a tap to open the workout and a single-use delete link.
- Uptime monitor (yours, later): check `/api/health` and `/`, never a workout page.
- Links go to friends once the operator file is filled in and `/about` shows your details (F-29).

### 13.5 Measurements to record (F-31 AC 8, 9, 10)

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

## 14. Bare metal (no containers)

The server binary and Caddy as two systemd services, with the same guarantees as the containers: both run as their own non-root users without capabilities, the app has no outbound connection unless notifications are on, Caddy sees only `public/` and `site/` of the data directory, the app's memory is capped. Needs Debian 13 (or another distribution with systemd 254+ and cgroup v2), linux/amd64. The unit files are in the release's deploy archive (`systemd/`).

### 14.1 Caddy from Caddy's apt repository

```sh
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl sqlite3
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
```

Then, after unpacking the deploy archive (14.2): `sudo install -m 0644 /tmp/wx-$v/systemd/caddy.pref /etc/apt/preferences.d/workout-explorer-caddy.pref`, `sudo apt update && sudo apt install caddy`. The pin holds the package at our release's minor version (for example `2.11.*`): patches arrive with your normal package updates, a new minor only with a release of ours. It applies to every use of the package on this host: if your TLS proxy is the same Caddy package, it is held there too.

The package starts its own `caddy.service` on port 80. Unless that is your TLS proxy, turn it off: `sudo systemctl disable --now caddy`. Ours is a separate unit with our Caddyfile.

### 14.2 Users, directories, files

```sh
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin workout-explorer
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin -G workout-explorer workout-explorer-caddy
sudo install -d -m 0755 /srv/workout-explorer /srv/workout-explorer/bin /srv/workout-explorer/conf /srv/workout-explorer/operator
sudo install -d -o workout-explorer -g workout-explorer -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

Download and check both archives of the release (section 2, plus the tarball):

```sh
v=v1.4.0; base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
mkdir -p /tmp/wx-$v && cd /tmp/wx-$v
curl -fLO $base/workout-explorer-$v-deploy.tar.gz -O $base/workout-explorer-$v-linux-amd64.tar.gz -O $base/SHA256SUMS
sha256sum -c SHA256SUMS                         # both: OK
tar -xzf workout-explorer-$v-deploy.tar.gz && mkdir app && tar -xzf workout-explorer-$v-linux-amd64.tar.gz -C app
sudo install -m 0755 app/workout-explorer /srv/workout-explorer/bin/workout-explorer
sudo cp -r app/web /srv/workout-explorer/web && sudo chmod -R u=rwX,go=rX /srv/workout-explorer/web
sudo install -m 0644 Caddyfile /srv/workout-explorer/conf/Caddyfile
sudo install -m 0644 systemd/*.service systemd/*.slice systemd/*.path /etc/systemd/system/
sudo install -m 0755 systemd/workout-explorer-run /usr/local/sbin/workout-explorer-run
```

`.env`: from `example.env` as in section 4, **root-owned, `0600`** (`sudo install -m 0600 /dev/null /srv/workout-explorer/.env`, then edit). systemd reads it for the app only; Caddy never sees it. Only the `WORKOUT_EXPLORER_*` keys matter; do not set `WORKOUT_EXPLORER_TRUSTED_PROXY` (the app trusts loopback, where Caddy runs). The operator file and Impressum go to `/srv/workout-explorer/operator/` as in section 3.

### 14.3 Start

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now workout-explorer.service workout-explorer-caddy.service workout-explorer-caddy-upgrade.path
curl -fsS http://127.0.0.1:8088/api/health     # {"status":"ok","version":"v1.4.0"}
```

Your reverse proxy forwards to `127.0.0.1:8088` (section 5, "Proxy on the host"); our Caddy trusts `X-Forwarded-For` from `127.0.0.1` and `::1` only. To change a value of the Caddy unit (port in `WX_SITE`, `WX_TRUSTED_PROXIES`, `WX_BIND`) use a drop-in: `sudo systemctl edit workout-explorer-caddy`. `workout-explorer-caddy-upgrade.path` restarts our Caddy when apt replaces `/usr/bin/caddy` (a moment without pages).

Both services restart after a crash, an out-of-memory kill or a reboot. Stop: `sudo systemctl stop workout-explorer workout-explorer-caddy` (until the next boot); to keep them off, `sudo systemctl disable --now workout-explorer workout-explorer-caddy`.

### 14.4 Notifications (optional)

Set both Pushover keys in `.env` (section 4a), then lift the app's outbound block (Caddy's stays):

```sh
sudo install -d /etc/systemd/system/workout-explorer.service.d
sudo install -m 0644 /tmp/wx-$v/systemd/notify.conf /etc/systemd/system/workout-explorer.service.d/notify.conf
sudo systemctl daemon-reload && sudo systemctl restart workout-explorer
```

### 14.5 Day to day

- `sudo workout-explorer-run regenerate`, `sudo workout-explorer-run delete [-y] <public_id>`, `sudo workout-explorer-run parse /srv/workouts/<file>.fit`: the subcommands as the app's user, with its `.env`, memory cap and sandbox (exit codes as in sections 8 and 10).
- Reports: the query of section 10, as the app's user: `sudo -u workout-explorer sqlite3 -readonly -line /srv/workouts/workouts.db "SELECT ..."`.
- Logs: `journalctl -u workout-explorer -u workout-explorer-caddy`; journal retention as in PODMAN.md section 10.
- Checks: `ss -ltnp` shows Caddy on `127.0.0.1:8088` and the app on `127.0.0.1:8080` only. `sudo systemd-run --pipe --wait -p User=workout-explorer -p IPAddressDeny=any -p IPAddressAllow=localhost curl -sS https://api.pushover.net` fails (that is the app's view without notifications). `grep Cap /proc/$(systemctl show -p MainPID --value workout-explorer-caddy)/status` shows only zeros.

### 14.6 Updating

By hand until automatic updates exist for this path. As root, with the new `v`:

1. Download and check both archives as in 14.2, unpack them in `/tmp/wx-$v`.
2. `caddy validate --config /tmp/wx-$v/Caddyfile --adapter caddyfile` (with `WX_DATA=/srv/workouts`); when `systemd/caddy.pref` changed, install it first and `apt update && apt install caddy`.
3. Binary by temporary name and rename (the running file is busy): `install -m 0755 app/workout-explorer /srv/workout-explorer/bin/.workout-explorer.new && mv /srv/workout-explorer/bin/.workout-explorer.new /srv/workout-explorer/bin/workout-explorer`. Replace `web/` whole: `rm -rf /srv/workout-explorer/web.new && cp -r app/web /srv/workout-explorer/web.new && chmod -R u=rwX,go=rX /srv/workout-explorer/web.new && rm -rf /srv/workout-explorer/web && mv /srv/workout-explorer/web.new /srv/workout-explorer/web` (old pages keep their files: those live in `/srv/workouts/site/assets/`).
4. Unit files and `workout-explorer-run` when they changed (`diff -r systemd/ ...`), then `systemctl daemon-reload`.
5. `systemctl restart workout-explorer`; wait for `/api/health` to show the new tag. If the Caddyfile changed: install it and `systemctl reload workout-explorer-caddy`.
6. `workout-explorer-run regenerate` (as in section 8, step 6).

## 15. Moving an existing install to the image (from the tarball layout)

Installs from before the image (the operator's server, and "use a public release" of earlier runbooks) have `bin/`, `web/`, `bundles/` and `clean-assets.sh` in `/srv/workout-explorer` and `WX_WEB` in `.env`. The move changes program files only: the data directory stays in place with its owner, so every workout, draft, report, edit link, preview image and published page keeps its links and expiry. Pages and API are down for a minute. The Podman setup has its own steps (PODMAN.md section 9).

1. Get the new release's deploy files to `/tmp/wx-$v` (section 2, without copying yet).
2. Stop: `docker compose down` in `/srv/workout-explorer`.
3. Copy the old web files into the data directory as the server's user, so pages published under the old release keep their scripts and styles until `regenerate` has rebuilt them:
   ```sh
   sudo setpriv --reuid=10001 --regid=10001 --clear-groups sh -c 'mkdir -p -m 0750 /srv/workouts/site/assets && cp -rn /srv/workout-explorer/web/assets/. /srv/workouts/site/assets/'
   ```
4. Install the new files: `compose.yaml`, `compose.notify.yaml`, `conf/Caddyfile` (section 2). In `.env`: delete the `WX_WEB` line; keep `COMPOSE_FILE` only as `compose.yaml` or `compose.yaml:compose.notify.yaml` (no `compose.build.yaml`).
5. `docker compose pull && docker compose up -d`, wait for `/api/health` to show the new tag. The first start turns the copied files into one list in `bundles/`. Database changes of the new release run at this start, as on any update (there is no way back to the old release afterwards).
6. `regenerate` (section 8, step 6) must exit 0. It rebuilds every page with the new version; the old web files go once no served page names them.
7. Only then remove the old program files: `rm -rf /srv/workout-explorer/bin /srv/workout-explorer/web /srv/workout-explorer/bundles /srv/workout-explorer/clean-assets.sh`.

Rehearse it on a copy first (copy `/srv/workouts` and `/srv/workout-explorer` to a scratch host with the same owners): afterwards every workout, link, expiry and report must be unchanged, and `regenerate` reports `deleted=0`.
