# Running Workout Explorer

Workout Explorer turns an uploaded FIT file (from a bike computer or sports watch) into a shareable page with map, charts and laps. There are no accounts. Every workout is deleted after 7 days by default (at most 30). Nothing is backed up, on purpose: a lost server loses its workouts.

Design background: `architect/F-31-server-deploy-and-monitoring.md`, `architect/F-59-container-image-and-install-paths.md`, `architect/F-52-auto-update.md`, ADR-034, ADR-035, ADR-050 to ADR-054.

## Which way should I run it?

Every release publishes our app image (`ghcr.io/b0wter/workout-explorer:vX.Y.Z`: the server and the web files, nothing else), a **deploy archive** with the files each path installs, a bare-metal tarball, `SHA256SUMS` and its signature `SHA256SUMS.minisig` ("Updates"). Caddy, the small web server in front of the app, is not ours: the official Caddy image, or Caddy's own Debian package. The privacy and security guarantees are the same on every path. Each path has its own guide (prerequisites, install, check, update, remove); this file holds what they share.

| | Docker Compose ([DOCKER.md](DOCKER.md)) | Rootless Podman ([PODMAN.md](PODMAN.md)) | Bare metal ([BAREMETAL.md](BAREMETAL.md)) |
| --- | --- | --- | --- |
| For | Anyone with Docker. **Pick this if unsure** | Hosts with Podman and systemd, no Docker daemon | Hosts without containers |
| Runs | Our image and the official Caddy image | The same two images, as systemd user units (Quadlet) | The binary from the tarball and Caddy's apt package, as two systemd services |
| Updates | `update.sh vX.Y.Z`, or automatically every night (DOCKER.md section 8) | The same (PODMAN.md section 9) | By hand, BAREMETAL.md section 6 |

Also in DOCKER.md: building the image yourself from a clone, moving an existing tarball install to the image. The project operator's own deploy and releasing are at the end of this file.

## How it fits together

```
browser --HTTPS--> your reverse proxy --HTTP--> 127.0.0.1:8088 (Caddy)
                                                   |-- /w/..., /assets/..., pages: files from the data directory
                                                   '-- /api/*  --> Go server (no internet unless notifications are on, see Notifications)
```

- Your reverse proxy (Caddy, nginx, Traefik, ...) handles the domain and TLS. This project does not get certificates and opens no public port.
- Our Caddy listens on plain HTTP, by default on `127.0.0.1:8088` only, and reads only `public/` and `site/` of the data directory (default `/srv/workouts`). The server writes the static pages and the web files there at every start; files of older releases stay until no published page needs them.

## The About page: operator file

The About page has a fixed part (what the app does with data) and a part only you can write: who runs this site, how to reach you, your hosting, what your web server logs, backups, legal basis, complaints. It lives in a plain text file that is **public**: every visitor reads it. `app` reads the folder `/srv/workout-explorer/operator` read-only (`deploy.sh` never writes there). Create it once (`deploy.sh` also creates the empty folder on the operator server, so an upgrade does not stop `app`), then write the file:

```sh
sudo install -d -o "$USER" -m 0755 /srv/workout-explorer/operator
cp operator.example.txt /srv/workout-explorer/operator/about.txt   # fill every [bracketed prompt], including your proxy's logging
```

Set `WORKOUT_EXPLORER_OPERATOR_FILE=/srv/workout-explorer/operator/about.txt` in `.env` (Configure, below) and restart `app` after every edit (Docker: `docker compose restart app`; Podman and bare metal: their restart command). Format: `## Heading`, `- list item`, other lines are paragraphs, a blank line ends a block; no links or markup (everything is shown as plain text); at most 16 KiB. Without the file the page shows `[OPERATOR DETAILS: ...]` and `app` logs a warning: replace it before you send links to anyone. Never commit this file.

## Impressum (optional)

If you need an Impressum (legal notice), write it as a plain text file in the operator folder, in the same format as the operator file above (`## Heading`, `- list item`, paragraphs; at most 16 KiB; no links or markup):

```sh
cp impressum.example.txt /srv/workout-explorer/operator/impressum.txt   # replace every [bracketed prompt]
```

Then:

1. Set `WORKOUT_EXPLORER_IMPRESSUM_FILE=/srv/workout-explorer/operator/impressum.txt` in `.env` (Configure, below).
2. Restart `app` (Docker: `docker compose restart app`). `/impressum` and the "Impressum" footer link on every page appear; a missing, unreadable or invalid file stops `app` at start (exit code 2, the log names the setting).
3. Run `regenerate` (the Updating section of your path's guide), so already published workout pages get the footer link too.

Do the same after every edit, and to remove it (unset the key, restart, `regenerate`). The page shows the text only after a click on "Show Impressum", fetched from `/api/impressum`, which answers only that page's request and at most 30 per hour per address; `robots.txt` asks search engines to skip both. This makes the text **hard to crawl, not secret**: anyone can read and copy it, and a determined scraper can get it. Never commit the file.

## Configure

Each path's guide says where `.env` lives and how to create it. Edit `.env`. Unset or empty keys use the default. An invalid value stops the server at start (exit code 2) with a message naming the key.

| Key | Default | Meaning and when to change it |
| --- | --- | --- |
| `WORKOUT_EXPLORER_BASE_URL` | none, **required** | The public address of your site, as visitors type it: `https://workouts.example.org`. Scheme and host (and port if not standard) only: no path, no trailing slash. Used for the absolute links in preview cards (Open Graph). A wrong value breaks previews in chat apps (Troubleshooting) |
| `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` | `1000` | Total size of the data directory in megabytes (1 to 1000000). When full, uploads are refused with 503. Keep it below free disk minus Docker logs and one over-limit upload |
| `WORKOUT_EXPLORER_MAX_CONCURRENT_UPLOADS` | `4` | Uploads processed at once (1 to 16). More means more memory; the 1200 MB cap in `compose.yaml` assumes 4. Extra uploads see "try again in a minute" |
| `WORKOUT_EXPLORER_PARSE_TIMEOUT` | `10` | Seconds allowed to parse one file (5 to 60). Raise it only if big files fail on a slow host |
| `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4` | `60` | Uploads per hour per IPv4 address (1 to 240) |
| `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV6` | `60` | Uploads per hour per IPv6 /64 prefix (1 to 240) |
| `WX_DATA` | `/srv/workouts` | Host folder for all workout data. Same path inside the containers |
| `WX_LISTEN` | `127.0.0.1:8088` | Where Caddy's HTTP port is published on the host. Your proxy forwards here. **Never a public address**: Docker publishes ports around the host firewall, and this port is plain HTTP. Change it only for a proxy in a container ("Reverse proxy") |
| `WX_TRUSTED_PROXIES` | `10.250.0.1` (when unset) | Source address(es) of your reverse proxy as Caddy sees them. Separate several by spaces, not commas. See "Reverse proxy" |
| `WORKOUT_EXPLORER_PUSHOVER_TOKEN` | unset (off) | Pushover application token for report notifications, "Notifications". **A secret** |
| `WORKOUT_EXPLORER_PUSHOVER_USER` | unset (off) | Pushover user key, "Notifications". **A secret** |
| `WORKOUT_EXPLORER_OPERATOR_FILE` | unset (placeholder) | Absolute path of your About page text, see "The About page: operator file" above. `WX_OPERATOR` (default `/srv/workout-explorer/operator`) is the host folder mounted for it |
| `WORKOUT_EXPLORER_IMPRESSUM_FILE` | unset (no Impressum) | Absolute path of your Impressum text, see "Impressum (optional)" above. Set but missing or invalid stops the server at start |
| `WORKOUT_EXPLORER_IOS_SHORTCUT_URL` | unset (no section) | `https://` link to your iPhone upload Shortcut, shown on the About page, "iPhone upload Shortcut". Anything that is not an absolute `https://` link stops the server at start |
| `COMPOSE_FILE` | unset | With notifications: `compose.yaml:compose.notify.yaml` (DOCKER.md section 5). Self-builders: DOCKER.md section 9 |
| `WX_UPDATE_PUBLIC_KEY` | none, **required by `update.sh`** | The release signing key you pinned ("Updates", step 1). Docker Compose and rootless Podman: every release is installed by `update.sh`, which refuses to run without it. Read by `update.sh` only; no deploy ever writes it |
| `WORKOUT_EXPLORER_AUTO_UPDATE` | unset (off) | `on`: the nightly timer installs every newer signed release ("Updates"). Anything else stops the server at start (exit code 2) and `update.sh` refuses to run. When on, the About page says that this server downloads updates from GitHub |

`WX_*` and `COMPOSE_FILE` are read by Docker Compose only (the other paths ignore them), except `WX_UPDATE_PUBLIC_KEY`, which `update.sh` reads on both container paths. `WORKOUT_EXPLORER_TRUSTED_PROXY` (singular) is set by `compose.yaml` itself. Do not put it in `.env`. There is no setting for the images: the release's `compose.yaml` fixes ours by digest and Caddy's by minor version.

With notifications on, the file holds two secrets. Keep it mode `0600` and never commit it.

## Notifications (optional)

When a visitor reports a workout, the server can send you a push message through [Pushover](https://pushover.net). Off by default. The message is `New report #<number>: <reason>` (title "Workout Explorer") and a **Delete this workout** link; tapping the notification opens the reported workout. Never the workout's title or the visitor's text. Pushover is a third party outside the EU, and its messages pass through Apple's or Google's push services. At most 10 messages per hour are sent; the rest are counted into the next one ("(+3 more)") and get no delete link. A message can be lost (Pushover down, restart); the reports table is the record.

The delete link opens a confirmation page (`/report-delete`) that shows the workout and the report; only its **Delete workout** button deletes, exactly like the delete command ("Delete one workout"). The link works once, only for that report's workout, and only while the workout lives (at most 30 days after the report). It is a credential: anyone with your Pushover account or your unlocked phone can use it. The lock screen shows only the words "Delete this workout", not the link. **To revoke every delete link** (lost phone, leaked Pushover account): turn notifications off as below and restart `app`; a start without the two settings removes every link, and turning notifications on again does not bring them back.

Turning it on needs **both** steps; one without the other does not work:

1. In Pushover, create an application and note its API token; your user key is on your Pushover dashboard. Both are 30 letters and digits. In `.env`, remove the `#` from the two lines and fill them in:

   ```
   WORKOUT_EXPLORER_PUSHOVER_TOKEN=<application token>
   WORKOUT_EXPLORER_PUSHOVER_USER=<user key>
   ```

2. Give the server a route out: Docker, `compose.notify.yaml` (DOCKER.md section 5); rootless Podman, a drop-in (PODMAN.md section 8); bare metal, `notify.conf` (BAREMETAL.md section 4).

Without that route the server has no internet access at all. With it, the server can reach any host on the internet, not only Pushover (Docker cannot filter by name); that is the price of turning notifications on. Only one of the two settings, or a value that is not 30 letters and digits, stops the server at start (exit code 2, the message names the setting but never prints its value).

## iPhone upload Shortcut (optional)

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

Then restart `app` (Docker: `docker compose up -d app`) and check that `/about` shows the section. Test once on an iPhone: share a FIT file to the Shortcut; Safari opens its edit page. Share a non-FIT file; the alert shows the server's message. To turn it off, comment out the setting and restart `app`.

## Reverse proxy

Do this before or after starting, but it must be done. All examples use `workouts.example.org` and forward to `127.0.0.1:8088`.

Rules for any proxy:

- **Pass `X-Forwarded-For` with the client address as its last entry.** Without it, the server sees every visitor as your proxy and the upload limit is shared by everybody (Troubleshooting).
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
3. Check the setting: after starting (DOCKER.md section 6), upload with two different browsers and watch for the rate-limit symptoms in Troubleshooting. If the proxy's source does not match, set the value from what the proxy actually shows.
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

## Updates (Docker Compose and rootless Podman)

Every release is installed by `update.sh` in `/srv/workout-explorer`, by hand or, if you turn it on, every night. It downloads the release's `SHA256SUMS` and checks its signature with `minisign` against the key you pinned, before anything is unpacked or run; then it runs the release's own `deploy.sh` (pull both images, check the new Caddy, install, restart, wait for `/api/health`, `regenerate`). The checksums alone are never enough: a release without a valid signature by the pinned key is refused, whatever its checksums say. Bare metal is updated by hand (BAREMETAL.md section 6). Self-built images (DOCKER.md section 9) are not updated by it.

Each path's guide has the commands (who runs them, the timer): DOCKER.md section 8, PODMAN.md section 9. What they share:

**1. Pin the release signing key (once).** The operator signs every release with a key kept offline. Its public half is published in two places that one attacker is unlikely to control both of: the file `deploy/minisign.pub` in the repository on GitHub (its second line), and a DNS TXT record on the operator's domain:

```sh
curl -fsS https://raw.githubusercontent.com/b0wter/WorkoutExplorer/main/deploy/minisign.pub | sed -n 2p
dig +short TXT _wx-release-key.workouts.plugman.de          # the same line, in quotes
```

Both must show the same line, starting with `RW`. If they differ, stop and ask the operator. Put the line into `.env` as `WX_UPDATE_PUBLIC_KEY=RW...`. It is read from there only, never from a download. If the operator announces a new key (the old one was lost or stolen), repeat this step with the new line; until then every update is refused and alerts you.

**2. Install `minisign`** from your distribution (`apt install minisign`; Debian 12 and 13 have it) next to `curl`, `tar`, `sha256sum` and `flock` (util-linux), which every Debian has.

**3. The first release is checked by hand** with the same key (the path's guide shows it), because `update.sh` itself comes with it. From then on every deploy goes through the installed `update.sh`, which replaces itself with the copy of each release it installs (the new copy acts from the next run).

**By hand:** `update.sh vX.Y.Z` installs exactly that release: also an older one, the running one again (for example after `regenerate` failed), and whether or not automatic updates are on. Exit codes: `0` installed and regenerated; `1` refused (signature, checksum, archive), not healthy, another deploy running, or the network failed; `2` a configuration error (`.env`, missing tool, wrong user); `3` `regenerate` did not finish (the new version is live; run the same command again); `4` the clean-up of old web files failed (run it again later). Never run the `deploy.sh` of a downloaded archive yourself; it refuses unless `update.sh` started it.

**Automatically:** with `WORKOUT_EXPLORER_AUTO_UPDATE=on` and the timer installed, once a night (03:00 plus a random but fixed minute within the hour; the guide shows how to change the hour) `update.sh` asks the running server for its version, asks GitHub which release is the latest, and installs it if it is newer: patch, minor or major. Pre-releases, drafts, tags not in `vX.Y.Z` form, older versions and the running version are never installed. A stopped stack, or a server that does not answer `/api/health`, is left alone. No port is opened and nothing is sent anywhere except ordinary download requests to GitHub (and your Pushover message on a failure). The app itself still has no internet access. With the switch off, an installed timer runs but makes no request at all.

**One at a time.** A manual and an automatic run never overlap: the second one sees the lock (`deploy.lock`); the nightly run skips (`reason=busy`), a manual run says so and exits 1.

**The log line.** Every run ends with one line, in the journal for the timer and in the terminal by hand:

```
update: running=v1.4.0 latest=v1.5.0 action=applied reason=- exit=0
```

`action` is `none` (nothing to do), `applied`, `refused` (the release failed a check; nothing was unpacked or run) or `failed` (its `deploy.sh` ended with the exit code shown). `reason`: `off` (automatic updates off), `stopped`, `busy`, `health` (server not answering or not a release version), `current` (nothing newer), `network` (GitHub not reachable, an HTTP error, a missing file; tried again the next night, nothing changed), `config`, `signature`, `manifest`, `checksum`, `archive`, `deploy`. Before it, on an install, are `deploy.sh`'s own lines.

**When an update is refused or fails**, `update.sh` sends one Pushover message (when both Pushover keys are set in `.env`, "Notifications"; this one comes from the host, not from the app), for example `v1.5.0 failed (exit 3). Automatic updates are stopped until a manual deploy succeeds.`, and writes the stopped state `/srv/workout-explorer/update.stopped`. While that file exists, the nightly run does nothing and sends nothing more (`reason=stopped`). Without Pushover the log line is the only alert: check the journal now and then.

- Read the state: `cat /srv/workout-explorer/update.stopped` (time, version, `refused` or `failed`, exit code, reason).
- Clear it: fix the cause (or wait for a newer release), then deploy by hand with `update.sh vX.Y.Z`. Its success removes the file and the nightly runs resume. A failed manual deploy sets the state too, and with automatic updates on says so in the terminal and alerts you.
- A failed `regenerate` may have deleted workouts whose stored file the new version cannot read, exactly as on a manual deploy (exit 3 or the `deleted=` count).

There is no rollback (ADR-019): a broken release is fixed by a newer one.

## Data, backups and privacy

- All workout data is in `WX_DATA` (default `/srv/workouts`): the database (`workouts.db`), the private originals (`private/`), what is served to visitors (`public/`), and the static pages and web assets of the releases still in use (`site/`, `bundles/`; a few MB per release, counted towards the storage limit).
- A workout whose sharer's browser sent a map for the link preview also keeps that map in `private/` (`preview-map.*.png`, up to about 2.5 MB each). It counts towards the storage limit, is never served as it is, and is deleted with the workout. The server itself never contacts the map tile host.
- **There are no backups, by design.** Workouts are short-lived, and a copy would outlive the deletion that the app promises to its users. Do not back up this folder. Turn off provider snapshots for the host if you can. Losing the server loses its workouts; that is accepted.
- Expiry deletes everything stored for a workout, in the database and on disk. Drafts are deleted after 48 hours.
- Only unlisted, published workouts have files under `public/`. Original FIT files are never served.

## Reports and deleting a workout

### Reports from visitors

Docker Compose: this query. Podman: `wx reports` (PODMAN.md section 5). Bare metal: BAREMETAL.md section 5.

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

Run it in your path's way: Docker, DOCKER.md section 10; Podman, `wx delete <public_id>` (PODMAN.md section 5); bare metal, `sudo workout-explorer-run delete [-y] <public_id>` (BAREMETAL.md section 5). It prints the workout's ID, state, created and expiry times, title and number of reports, and asks `Delete? [y/N]`; `-y` skips the question.

The page is gone as soon as the command finishes (nothing is cached). Exit status: `0` deleted; `1` not deleted or not completely (the message says which; a leftover public folder goes within about a minute, a private folder at the next start of `app`), also when you answered no; `2` wrong usage or a malformed ID; `3` no workout with this ID (also when it was already deleted). The storage total shown to uploads frees the space only at the next restart of `app`; restart it if uploads are refused as full.

## Logs

- The server logs JSON lines (start, expiry, errors). Caddy writes no access log, and the server writes none. Visitor addresses are not logged by either.
- `preview map refused` (with a `reason`: `size`, `format`, `dimensions`, `decode`, `stale`) and `preview map not used` (`size`, `read`, `render`) are informational: the link preview then uses the plain card. Many `size` lines for real routes mean the map card is too large for messengers (ADR-041, "Would choose differently if").
- Your reverse proxy has its own logs. Those are yours to configure ("Reverse proxy").

## Troubleshooting

Problems of one install path are in its guide (DOCKER.md section 13, PODMAN.md "Troubleshooting"). These apply to all:

| Symptom | Likely cause and fix |
| --- | --- |
| Everyone gets "too many uploads" / limit hits quickly | The server sees your proxy as the client. `X-Forwarded-For` is missing, or `WX_TRUSTED_PROXIES` does not match the proxy's source address, so Caddy ignores the header. Fix the proxy header and the setting, then restart the stack |
| People can bypass the limit | `WX_TRUSTED_PROXIES` is too wide. List only the proxy's own address or its network |
| Preview cards (WhatsApp, Telegram, ...) show no image or a wrong link | `WORKOUT_EXPLORER_BASE_URL` differs from the public address (http vs https, `www`, port). Fix it, restart `app`, and republish or wait: pages published before keep the old links until `regenerate` runs (the Updating section of your path's guide) |
| Uploads return 503 | The storage limit is reached. Raise `WORKOUT_EXPLORER_STORAGE_LIMIT_MB` (and check free disk), or wait for expiry |
| "exec format error", or no image for your platform | Only linux/amd64 is supported |

## Operator deploy (for the project operator only)

Everything below is the operator's own VPS setup. It is the Docker Compose path (DOCKER.md), deployed with `update.sh` on the server like every hoster's ("Updates").

### The VPS (DP-13)

- EU data centre, **linux/amd64**, at least 2 GB RAM, Debian stable, local disk. About 600 MB of RAM stays free for the OS, Docker and Caddy next to the 1200 MB cap of the app.
- **Provider backups and snapshots off.** No backup job, no off-server copy of `/srv/workouts`.
- SSH by key only; unattended security upgrades on (`unattended-upgrades`). Packages: `curl`, `tar`, `minisign` ("Updates"), `sqlite3` (reports, "Reports from visitors").
- Firewall: this setup opens nothing. Docker's own rules bypass a host firewall, which is why `WX_LISTEN` stays on loopback.
- Docker Engine and the Compose plugin from Docker's Debian repository, default daemon settings. Docker starts at boot; there is no unit of ours.

### Users and directories (DP-3 to DP-5)

`deploy` is the account that runs `update.sh` (over SSH and from the timer): it owns the deploy directory and is in the `docker` group (root-equivalent on this box).

```sh
adduser --disabled-password deploy && usermod -aG docker deploy      # add your SSH key to ~deploy/.ssh/authorized_keys
install -d -o deploy -g deploy -m 0755 /srv/workout-explorer /srv/workout-explorer/conf
install -d -o 10001 -g 10001 -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
install -d -o deploy -g deploy -m 0755 /srv/workout-explorer/operator   # About page text (public), from operator.example.txt
install -m 0600 -o deploy -g deploy /dev/null /srv/workout-explorer/.env
```

Fill `/srv/workout-explorer/.env` from `example.env` as in "Configure", including `WX_UPDATE_PUBLIC_KEY` ("Updates", step 1); with notifications set `COMPOSE_FILE=compose.yaml:compose.notify.yaml` (DOCKER.md section 5). It must stay readable by `deploy` (`0600`). Your reverse proxy: "Reverse proxy". A server still on the tarball layout (`bin/`, `web/` in `/srv/workout-explorer`) is moved once with DOCKER.md section 12 before the first deploy of a release with the image. A server without `update.sh` gets it once by hand: DOCKER.md section 2 with the new release (verify, then copy only `update.sh` into `/srv/workout-explorer`), then deploy as below.

### Releasing and deploying

**One-time, after the first release with an image:** in GitHub, Packages, `workout-explorer`, Package settings: visibility **public**, linked to this repository, and under "Manage Actions access" only this repository with the **Write** role. No other repository or person gets write access; no personal token is ever created for it.

**Before every release:** check for a newer Caddy minor version (CLAUDE.md, Releases). A minor bump changes the tag in `deploy/compose.yaml`, `deploy/podman/workout-explorer-caddy.container` and `deploy/systemd/caddy.pref` together (`make ci` fails if they differ) and must pass `make caddy-check`.

**CSP hash of the inline theme script** (ADR-032 item 3, F-30 RG-14): a release that changes the script adds its new hash to `deploy/Caddyfile` and keeps the old one, because Caddy reloads before `regenerate` has rebuilt the old pages. Drop the old hash only in a later release, and only if the previous deploy reported `kept=0` and finished with exit 0.

**Release signing key (once, and after a loss).** On your own machine, never on the VPS or in CI, with the distribution's `minisign`:

```sh
rm deploy/minisign.pub                     # the placeholder
mkdir -p ~/.minisign && minisign -G -p deploy/minisign.pub -s ~/.minisign/workout-explorer.key   # asks for a password
```

The secret key `~/.minisign/workout-explorer.key` and its password never enter the repository, CI, a release or the VPS. Keep an offline copy (for example on a USB stick). Commit `deploy/minisign.pub` (it replaces the placeholder) and publish its second line as a DNS TXT record on your domain, `_wx-release-key.workouts.plugman.de` (the name in "Updates", step 1), with the same value. If the key is lost or stolen: make a new one, replace both places, and tell hosters to pin the new line (until they do, every update is refused and alerts them); announce a stolen key at once, because releases it signs install until hosters re-pin.

**Cut a release.** On the commit to ship (pushed; CI runs only on `v*` tags and by hand via "Run workflow", never on branch pushes, so run `make ci` locally first):

```sh
git tag v1.4.0 && git push origin v1.4.0
```

CI builds and tests the image once, and only if everything passes the `release` job pushes exactly that image to `ghcr.io/b0wter/workout-explorer:v1.4.0` and creates the release as a **draft** (bare-metal tarball, deploy archive with the image's digest in `compose.yaml`, `SHA256SUMS`). A draft is invisible to hosters and to their updaters. Only plain `vX.Y.Z` tags release. A version is never replaced: a re-run of the whole workflow builds another digest and stops before pushing; if a release is broken, ship the next version. If the job failed after pushing the image, "Re-run failed jobs" on the same run completes it (within a day, while the hand-off artefacts last).

**Sign and publish it.** When CI is green, on your machine, in a checkout that has the tag (`gh auth login` once; `minisign` installed):

```sh
deploy/sign-release.sh v1.4.0
```

It downloads the draft's assets and refuses unless: the release is still a draft, no higher version is already published (releases go out in version order, no back-ports), every asset is listed in `SHA256SUMS` and matches, every name carries the tag, `COMMIT` in both archives is the tag's commit, and the deploy archive names our image by digest. Then it signs `SHA256SUMS` (minisign asks for the key's password), checks the signature against `deploy/minisign.pub`, uploads `SHA256SUMS.minisig`, and publishes the release as "latest". Publishing moves the image's `latest` tag (the `latest` workflow, registry rights only, and only to a higher version). From that moment hosters with automatic updates install it in their next night. Never sign a release you have not reviewed: the signature is what every hoster trusts.

**Deploy it.** Over SSH, as the deploy user:

```sh
ssh deploy@your.vps /srv/workout-explorer/update.sh v1.4.0
```

It verifies the signature with the pinned key, then runs the release's `deploy.sh` ("Updates"). It refuses a missing or wrong signature, a bad checksum, or a server still on the tarball layout. It pulls both images with the new `compose.yaml` and checks the new Caddyfile with the pulled Caddy; **nothing on the server changes until both succeed**. Then it installs `compose.yaml`, `compose.notify.yaml`, `conf/Caddyfile` and `update.sh`, runs `docker compose up -d` (Caddyfile-only change: a graceful reload), and waits (60 s) for `/api/health` through Caddy to report exactly the tag. It has no rollback; a failed wait prints `docker compose ps` and the last log lines of `app` and `caddy` and exits 1 (fix forward with a newer version).

Then it runs `regenerate` in the `app` container (same `.env`, mounts and memory cap as `serve`), which rebuilds every live workout and removes old web files. It ends with one line:

```
deploy: v1.4.0 done: rebuilt=12 deleted=0 kept=0 skipped=0 files_removed=17 bundles_removed=1
```

- `deleted`: workouts whose stored file the new version rejects; they are gone, as on expiry. Each has a `fit rejected` log line with the device model.
- `kept`: workouts whose rebuild failed for a server-side reason (load, time limit, disk); they keep serving their previous files and web files. `skipped`: expired or deleted during the run.
- Old web files: the server keeps one list per release in `/srv/workouts/bundles/`. After a complete run it keeps the current release and every release whose files a served page still names, and removes the rest from `site/assets/`. A viewer who had a page open across the clean-up may need to reload it.
- Exit codes: 0 healthy and regenerated (whatever was deleted or kept); 1 refused or not healthy; 2 a configuration error; 3 `regenerate` did not finish (the new version is live, nothing was removed); 4 the clean-up failed. A failure also sets the stopped state ("Updates").
- After a deploy: `docker inspect -f '{{.State.OOMKilled}}' workout-explorer-app-1` must print `false` (F-30 RG-13, ADR-017 rev. 8). `OOMKilled` resets on a container restart and may not show for an exec'd process, so also compare `docker inspect -f '{{.RestartCount}}' workout-explorer-app-1` before and after the deploy (it must not grow), and treat `regenerate` exit 3 as a possible OOM kill.
- **Retry** after exit 3 or 4, or to rebuild kept workouts: run `update.sh` again with the same tag. It is idempotent.

API downtime is the recreate of `app`. A newer Caddy patch, a changed Caddy definition in `compose.yaml`, or a changed `.env` value that Caddy itself uses (`WX_LISTEN`, `WX_TRUSTED_PROXIES`, `WX_DATA`) recreates the Caddy container: published pages are unreachable for a moment.

### Day to day

- Stop: `docker compose stop app` stays stopped across reboots. Start: `docker compose start app`. Run `docker compose` in `/srv/workout-explorer`.
- Delete a workout: `docker compose exec app /srv/workout-explorer/bin/workout-explorer delete -data /srv/workouts <public_id>` (DOCKER.md section 10).
- Reports: the read-only `sqlite3` query of "Reports from visitors", as user ID 10001 (INF-2, INF-3). With notifications on ("Notifications") each new report also arrives as a Pushover message with its number, a tap to open the workout and a single-use delete link.
- Uptime monitor (yours, later): check `/api/health` and `/`, never a workout page.
- Links go to friends once the operator file is filled in and `/about` shows your details (F-29).

### Measurements to record (F-31 AC 8, 9, 10)

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
