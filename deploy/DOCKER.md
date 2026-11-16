# Running Workout Explorer with Docker Compose

This is one of the three install paths (RUNBOOK.md, "Which way should I run it?"). It runs our published app image and the official Caddy image with Docker Compose. Everything in RUNBOOK.md applies as well: the operator file and Impressum, configuration keys, notifications, the reverse proxy, data and privacy. This file covers what is specific to Docker.

## 1. Prerequisites

| Need | Detail |
| --- | --- |
| CPU | **linux/amd64 (x86-64).** Our image is published for amd64 only |
| RAM | 2 GB. The `app` container is capped at 1200 MB (`compose.yaml`); leave room for the OS, Docker and Caddy |
| Disk | About 1 GB for the images, plus the storage cap you set (default 1000 MB), plus Docker logs (at most 5 x 10 MB per container). Local disk, one file system |
| Docker | **Docker Engine 28 or newer** (ADR-039 relies on it to block direct access to unpublished container ports) with the Compose plugin, **Compose 2.24 or newer** (`docker compose version`). Default daemon settings. Pulls from `ghcr.io` and Docker Hub, no login |
| Tools | `curl`, `tar`, `sha256sum`, `flock`, `minisign` (checks every release's signature, RUNBOOK.md, "Updates"). `sqlite3` only to read reports from visitors (RUNBOOK.md, "Reports from visitors"). Debian: `apt install curl minisign sqlite3` |
| Domain and proxy | A domain name pointing at your host, and a reverse proxy **on the same host** that terminates TLS (RUNBOOK.md, "Reverse proxy") |
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

The user that runs these commands becomes the **deploy user**: it owns `/srv/workout-explorer`, must be in the `docker` group, and runs every later update (`update.sh`, section 8). Never root.

First pin the release signing key as in RUNBOOK.md, "Updates", step 1 (compare GitHub with the DNS record), and keep it in `key`. Then download, check the signature, then the checksum:

```sh
key=RW...                            # the line you pinned
v=v1.4.0                             # the release you chose
base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
sudo install -d -o "$USER" -m 0755 /srv/workout-explorer /srv/workout-explorer/conf
mkdir -p /tmp/wx-$v && cd /tmp/wx-$v
curl -fLO $base/workout-explorer-$v-deploy.tar.gz -O $base/SHA256SUMS -O $base/SHA256SUMS.minisig
minisign -V -P "$key" -m SHA256SUMS                 # must print: Signature and comment signature verified
grep -c "  workout-explorer-$v-" SHA256SUMS         # must print 2: both names carry the tag you chose
sha256sum -c --ignore-missing SHA256SUMS            # must print: ...deploy.tar.gz: OK
mkdir d && tar -xzf workout-explorer-$v-deploy.tar.gz -C d
cd /srv/workout-explorer
cp /tmp/wx-$v/d/compose.yaml /tmp/wx-$v/d/compose.notify.yaml /tmp/wx-$v/d/example.env /tmp/wx-$v/d/*.example.txt .
install -m 0755 /tmp/wx-$v/d/update.sh update.sh
cp -r /tmp/wx-$v/d/updater .                        # the timer units (section 8); every update refreshes them
cp /tmp/wx-$v/d/Caddyfile conf/Caddyfile
```

If `minisign` says anything else, stop: the files are not the operator's. All following commands run in `/srv/workout-explorer` unless stated otherwise. The image tag `latest` exists for a quick `docker pull` look only; never put it into `compose.yaml` (a newer app with older deploy files can break pages).

## 3. Create the data directory

Compose never creates host folders. The server runs as user ID 10001, Caddy as 10002 (and group 10001, so it can read published files). The data directory must belong to 10001:

```sh
sudo install -d -o 10001 -g 10001 -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

You should see: `ls -ld /srv/workouts /srv/workouts/public /srv/workouts/site` shows `drwxr-x---` and owner `10001 10001` for all three. `site/` holds the static pages (upload, expired workout, edit, Impressum) and the web files (`site/assets/`) that the server writes at every start; Caddy serves them from there. `bundles/` (next to it, created by the server) lists which web files belong to which release.

Keep it on the local disk. To use another path, set `WX_DATA` (section 4) to it.

## 4. Configure

Keys and their meaning: RUNBOOK.md, "Configure". The operator file and the Impressum go to `/srv/workout-explorer/operator/` (RUNBOOK.md, "The About page: operator file"); restart `app` after every edit with `docker compose restart app`.

```sh
cp example.env .env && chmod 0600 .env
```

Set `WX_UPDATE_PUBLIC_KEY` to the key you pinned in section 2.

## 5. Notifications (optional)

What they send and what they cost in privacy: RUNBOOK.md, "Notifications" (step 1, the two keys in `.env`, is there). On Docker the second step is the override file:

Give the server a route out by adding `compose.notify.yaml` to `COMPOSE_FILE` in `.env`:

```
COMPOSE_FILE=compose.yaml:compose.notify.yaml                      # the published image
COMPOSE_FILE=compose.yaml:compose.build.yaml:compose.notify.yaml   # self-builders (section 9)
```

Then `docker compose up -d`. Check: `docker compose logs app | grep 'server started'` shows `"notifications":true`. Report one of your own published workouts from its page; a message arrives within seconds.

To turn it off: comment out both settings, remove `:compose.notify.yaml` from `COMPOSE_FILE`, `docker compose up -d`.

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

Your reverse proxy: RUNBOOK.md, "Reverse proxy". Open `https://workouts.example.org/` in a browser: the upload page appears. Upload a FIT file, publish it and open the link.

If it does not start: `docker compose logs app` names the bad setting.

## 7. Check the result

- `ss -ltn`: the only listener of this project is `WX_LISTEN`'s port. Nothing on 80 or 443 comes from it.
- `curl -sI https://workouts.example.org/ | grep -i strict-transport-security` prints exactly one line.
- `docker compose logs caddy` and `docker compose logs app` contain no visitor addresses (section 10).
- Hardening: `grep -E 'CapPrm|CapEff|CapBnd' /proc/$(docker inspect -f '{{.State.Pid}}' workout-explorer-caddy-1)/status` shows only zeros; the same for `workout-explorer-app-1`.
- Test with your own file first. The About page (`/about`) is the privacy page; your part of it comes from the operator file (see RUNBOOK.md, "The About page: operator file").

## 8. Updating

Read the release notes first. Then, as the deploy user (section 2), with the new version:

```sh
/srv/workout-explorer/update.sh v1.5.0
```

What it checks and does, its exit codes, the log line and the stopped state: RUNBOOK.md, "Updates". In short: it downloads `SHA256SUMS` and its signature and checks them with your pinned key, then the deploy archive against the checked sums; then the release's `deploy.sh` pulls both images and checks that the pulled Caddy starts under our hardening and accepts the new Caddyfile (if not, the previous Caddy keeps its tag and nothing changes, exit 1); installs `compose.yaml`, `compose.notify.yaml`, `conf/Caddyfile` and `update.sh`; runs `docker compose up -d` (a few seconds without API; a moment without pages when Caddy is recreated); waits up to 60 s for `/api/health` to show the new version; and rebuilds every stored workout with `regenerate`, which also removes the web files no page needs. It ends with `rebuilt=`, `deleted=`, `kept=` and `skipped=` counts: a workout whose file the new version cannot read at all is deleted (as if it had expired); one that failed for another reason (load, disk) is kept unchanged with its old web files, and rebuilt by the next run.

After a failure, fix the cause and run the same command again; it is idempotent.

### Automatic updates (optional)

Off by default. To install every newer signed release at night, install the timer once, as root (it runs `update.sh` as the deploy user; root runs no file of ours). The units come from the last release you installed: section 2 and every `update.sh` put them into `/srv/workout-explorer/updater/`:

```sh
u=/srv/workout-explorer/updater/system
sudo install -m 0644 $u/workout-explorer-update.service $u/workout-explorer-update.timer /etc/systemd/system/
```

(Installed before these units were in the deploy directory? Run `update.sh` once with the running version, section 8.)

If your deploy user is not called `deploy`, say so in a drop-in (`sudo systemctl edit workout-explorer-update.service`):

```ini
[Service]
User=alice
```

Then turn it on. First `cat /srv/workout-explorer/update.stopped`: if the file exists, an earlier deploy failed and the nightly run would do nothing; fix that and run `update.sh vX.Y.Z` by hand until it succeeds. Then set `WORKOUT_EXPLORER_AUTO_UPDATE=on` in `.env`, `docker compose up -d` (the About page now says that updates come from GitHub), and `sudo systemctl daemon-reload && sudo systemctl enable --now workout-explorer-update.timer`.

- **When:** every night between 03:00 and 04:00 (host time), always at the same minute. Another hour: `sudo systemctl edit workout-explorer-update.timer` with `[Timer]`, `OnCalendar=` (empty, clears the default) and `OnCalendar=*-*-* 05:00:00`. A night missed while the host was off is not caught up.
- **Check:** `systemctl list-timers workout-explorer-update.timer` shows the next run; `journalctl -u workout-explorer-update.service -n 20` shows the last runs and their `update:` lines. Run one now: `sudo systemctl start workout-explorer-update.service`.
- **Confinement:** the service may write only to `/srv/workout-explorer` and a private `/tmp`, and gets no new privileges. The deploy user is in the `docker` group, which is as powerful as root; that is accepted (the same rights a manual deploy has).
- **Stopped:** after a refused or failed update, `update.sh` alerts you once and stops (RUNBOOK.md, "Updates"): `cat /srv/workout-explorer/update.stopped`. A successful `update.sh vX.Y.Z` by hand clears it.
- **Turn off:** remove `WORKOUT_EXPLORER_AUTO_UPDATE` from `.env` and `docker compose up -d`. The timer then still runs, but makes no request. To remove it: `sudo systemctl disable --now workout-explorer-update.timer && sudo rm /etc/systemd/system/workout-explorer-update.service /etc/systemd/system/workout-explorer-update.timer && sudo systemctl daemon-reload`. `update.sh` stays: it is how you deploy by hand.

### Caddy between releases

Between releases, Caddy's newest patch of its minor comes with the same guard (never a plain `docker compose pull`: a refused patch would stay the local tag and the next `up -d` would start it):

```sh
cd /srv/workout-explorer
c=docker.io/library/caddy:2.11                                    # the Caddy tag of compose.yaml
prev=$(docker image inspect -f '{{.Id}}' $c)
docker compose pull caddy
if docker run --rm --network none --read-only --user 10002:10002 --cap-drop ALL --security-opt no-new-privileges:true \
  --tmpfs /config --tmpfs /data -v "$PWD/conf:/c:ro" $c caddy validate --config /c/Caddyfile --adapter caddyfile
then docker compose up -d
else docker tag $prev $c; echo 'REFUSED: Caddy stays on its previous patch'
fi
```

Remove old images when you like: `docker image prune`.

## 9. Building the image yourself (not a separate path)

For changes of your own, build the app image from a clone instead of pulling ours. Everything else stays as above (Caddy still comes from the official image); `update.sh` and automatic updates are not for this setup (they would install our image instead of yours).

```sh
git clone <this repository's URL> /opt/workout-explorer-src && cd /opt/workout-explorer-src && git checkout v1.4.0
cd deploy && cp example.env .env                   # as in section 4, plus the line below
echo 'COMPOSE_FILE=compose.yaml:compose.build.yaml' >> .env
docker compose build --build-arg VERSION=v1.4.0    # several minutes; without the argument the version is "unknown"
docker compose up -d
```

Here `deploy/` of the clone is the deploy directory, and the Caddyfile is mounted from `deploy/conf/`: create it with `mkdir -p conf && cp Caddyfile conf/`. To update: `git fetch && git checkout <new tag>`, copy the Caddyfile again, `docker compose build ... && docker compose up -d`, then `regenerate` (section 12, step 6). Small hosts (2 GB): stop `app` while building (`docker compose stop app`).

## 10. Logs and day to day

```sh
docker compose logs --tail 100 app
docker compose logs -f app
docker compose logs --tail 100 caddy
```

- Docker keeps them in rotating files on the host (at most 5 files of 10 MB per container). `docker compose down` removes them.

Delete one workout (what it does and its exit codes: RUNBOOK.md, "Delete one workout"):

```sh
docker compose exec app /srv/workout-explorer/bin/workout-explorer delete -data /srv/workouts <public_id>
```

The `-y` flag skips the question; it is needed when there is no terminal, for example `docker compose exec -T` or a script. While `app` is stopped use `docker compose run --rm app delete -data /srv/workouts <public_id>` instead. Use your `WX_DATA` path if you changed it.

## 11. Stop, remove, delete all data

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

With automatic updates, remove the timer first (section 8, "Turn off").

## 12. Moving an existing install to the image (from the tarball layout)

Installs from before the image (the operator's server, and "use a public release" of earlier runbooks) have `bin/`, `web/`, `bundles/` and `clean-assets.sh` in `/srv/workout-explorer` and `WX_WEB` in `.env`. The move changes program files only: the data directory stays in place with its owner, so every workout, draft, report, edit link, preview image and published page keeps its links and expiry. Pages and API are down for a minute. The Podman setup has its own steps (PODMAN.md section 9).

1. Get the new release's deploy files to `/tmp/wx-$v/d` and check them (section 2, without copying yet); add `WX_UPDATE_PUBLIC_KEY` to `.env`.
2. Stop: `docker compose down` in `/srv/workout-explorer`.
3. Copy the old web files into the data directory as the server's user, so pages published under the old release keep their scripts and styles until `regenerate` has rebuilt them:
   ```sh
   sudo setpriv --reuid=10001 --regid=10001 --clear-groups sh -c 'mkdir -p -m 0750 /srv/workouts/site/assets && cp -rn /srv/workout-explorer/web/assets/. /srv/workouts/site/assets/'
   ```
4. Install the new files: `compose.yaml`, `compose.notify.yaml`, `update.sh`, `conf/Caddyfile` (section 2). In `.env`: delete the `WX_WEB` line; keep `COMPOSE_FILE` only as `compose.yaml` or `compose.yaml:compose.notify.yaml` (no `compose.build.yaml`).
5. `docker compose pull`, then the Caddy check of section 8, "Caddy between releases" (a fresh pull: there is no previous Caddy to give the tag back to; if it is refused, stop and report it), then `docker compose up -d`; wait for `/api/health` to show the new tag. The first start turns the copied files into one list in `bundles/`. Database changes of the new release run at this start, as on any update (there is no way back to the old release afterwards).
6. `docker compose exec -T app /srv/workout-explorer/bin/workout-explorer regenerate -data /srv/workouts -web /srv/workout-explorer/web` (your `WX_DATA` if you changed it) must exit 0. It rebuilds every page with the new version and prints `rebuilt=`, `deleted=`, `kept=`, `skipped=`, `files_removed=` and `bundles_removed=`; the old web files go once no served page names them. Exit 1: it stopped early, nothing removed, run it again; 4: the clean-up failed, run it again later.
7. Only then remove the old program files: `rm -rf /srv/workout-explorer/bin /srv/workout-explorer/web /srv/workout-explorer/bundles /srv/workout-explorer/clean-assets.sh`.

Rehearse it on a copy first (copy `/srv/workouts` and `/srv/workout-explorer` to a scratch host with the same owners): afterwards every workout, link, expiry and report must be unchanged, and `regenerate` reports `deleted=0`.

## 13. Troubleshooting

| Symptom | Likely cause and fix |
| --- | --- |
| Proxy shows 502 / "bad gateway" | The containers are not running (`docker compose ps`), or the proxy points to the wrong address or port. Test `curl http://127.0.0.1:8088/api/health` on the host. A proxy in a container cannot use `127.0.0.1` (RUNBOOK.md, "Reverse proxy") |
| `docker compose up` says port is already allocated | Something else uses 8088. Find it with `ss -ltnp 'sport = :8088'`. Set another port in `.env`, for example `WX_LISTEN=127.0.0.1:8090`, and point the proxy there |
| `app` exits at start, `docker compose logs app` shows "must be ..." | A setting is invalid or `WORKOUT_EXPLORER_BASE_URL` is missing. Fix `.env` |
| Upload fails with permission errors, or `app` cannot create files | The data directory is not owned by 10001: `sudo chown -R 10001:10001 /srv/workouts && sudo chmod 0750 /srv/workouts /srv/workouts/public` |
| Published pages give 404 from the container but exist | Caddy cannot read `public/`: mode must be 0750 with group 10001 (section 3) |
| `caddy` does not start: a bind mount source `.../site` or `conf` does not exist | Create `site/` as in section 3 and `conf/Caddyfile` as in section 2, then `docker compose up -d` |
| `/`, `/edit/...` or the expired-workout page give 404 | `app` has not started once since `site/` was created; it writes those pages at start (`docker compose logs app`) |
| "network ... overlaps" or no route to the other 10.250.x hosts | The project fixes two networks, `10.250.0.0/29` (edge) and `10.250.1.0/29` (backend). If your host, VPN or other containers already use them, change them in `compose.yaml` in all places: the two `subnet` lines, `WORKOUT_EXPLORER_TRUSTED_PROXY`, both `ipv4_address` lines of Caddy, the `app` address, and the `WX_TRUSTED_PROXIES` default (the edge gateway). Then `docker compose up -d` |
| `app` fails to pull: `manifest unknown` for `workout-explorer:unreleased` | You use the repository's `compose.yaml`. Use the one from a release's deploy archive (section 2), or build the image yourself (section 9) |
| `caddy` exits with `operation not permitted` | A Caddy patch pulled between releases no longer runs without capabilities. Report it; a release of ours handles it. The check of every update (section 8) catches this before anything changes |
| `update:` line says `reason=config` | `.env` lacks `WX_UPDATE_PUBLIC_KEY` or it is not one `RW...` line, `WORKOUT_EXPLORER_AUTO_UPDATE` is neither empty nor `on`, `minisign`, `curl` or `flock` is missing, it ran as root, or `/srv/workout-explorer` is not writable by the deploy user. The line before it says which |
| `update:` line says `reason=signature` | The release's signature is missing or not by the key you pinned. Do not install it another way. Compare your key with GitHub and DNS again (RUNBOOK.md, "Updates", step 1); if the operator announced a new key, pin it, then `update.sh vX.Y.Z` |
| `update:` line says `reason=health` every night | The server does not answer on `WX_LISTEN`, or runs a self-built version. Check `curl http://127.0.0.1:8088/api/health` |
| Styles missing on old workouts after an update | `app` has not started once with the new version, or `site/assets` was emptied by hand. Restart `app`, then run `regenerate` (section 8) |
| No notification; `docker compose logs app` shows `"notification failed","class":"network"` | `compose.notify.yaml` is missing from `COMPOSE_FILE`, or the host has no internet route (section 5). `"status":400` means a wrong token or user key, `"status":429` Pushover's monthly quota |
| `delete` says "database schema N is not this binary's" | `app` has not yet run with this version. Start it once (`docker compose up -d`), then delete |
