# Running Workout Explorer on bare metal (no containers)

This is one of the three install paths (RUNBOOK.md, "Which way should I run it?"). Everything in RUNBOOK.md applies as well: the operator file and Impressum, configuration keys, notifications, the reverse proxy, data and privacy. This file covers what is specific to this path.

The server binary and Caddy as two systemd services, with the same guarantees as the containers: both run as their own non-root users without capabilities, the app has no outbound connection unless notifications are on, Caddy sees only `public/` and `site/` of the data directory, the app's memory is capped. Needs Debian 13 (or another distribution with systemd 254+ and cgroup v2), linux/amd64. The unit files are in the release's deploy archive (`systemd/`).

## 1. Caddy from Caddy's apt repository

```sh
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl sqlite3
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
```

Then, after unpacking the deploy archive (section 2): `sudo install -m 0644 /tmp/wx-$v/systemd/caddy.pref /etc/apt/preferences.d/workout-explorer-caddy.pref`, `sudo apt update && sudo apt install caddy`. The pin holds the package at our release's minor version (for example `2.11.*`): patches arrive with your normal package updates, a new minor only with a release of ours. It applies to every use of the package on this host: if your TLS proxy is the same Caddy package, it is held there too.

The package starts its own `caddy.service` on port 80. Unless that is your TLS proxy, turn it off: `sudo systemctl disable --now caddy`. Ours is a separate unit with our Caddyfile.

## 2. Users, directories, files

```sh
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin workout-explorer
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin -G workout-explorer workout-explorer-caddy
sudo install -d -m 0755 /srv/workout-explorer /srv/workout-explorer/bin /srv/workout-explorer/conf /srv/workout-explorer/operator
sudo install -d -o workout-explorer -g workout-explorer -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

Download both archives of the release, the deploy archive and the tarball, and check the signature first, then the checksums. The checksums alone prove nothing: whoever can change the files can change `SHA256SUMS` too. Pin the release signing key once as in RUNBOOK.md, "Updates", step 1 (compare GitHub with the DNS record), and install `minisign` (`apt install minisign`). If `minisign` says anything else than shown below, stop: the files are not the operator's.

```sh
key=RW...                                       # the line you pinned
v=v1.4.0; base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
mkdir -p /tmp/wx-$v && cd /tmp/wx-$v
curl -fLO $base/workout-explorer-$v-deploy.tar.gz -O $base/workout-explorer-$v-linux-amd64.tar.gz -O $base/SHA256SUMS -O $base/SHA256SUMS.minisig
minisign -V -P "$key" -m SHA256SUMS             # must print: Signature and comment signature verified
grep -c "  workout-explorer-$v-" SHA256SUMS     # must print 2: both names carry the tag you chose
sha256sum -c SHA256SUMS                         # both: OK
tar -xzf workout-explorer-$v-deploy.tar.gz && mkdir app && tar -xzf workout-explorer-$v-linux-amd64.tar.gz -C app
sudo install -m 0755 app/workout-explorer /srv/workout-explorer/bin/workout-explorer
sudo cp -r app/web /srv/workout-explorer/web && sudo chmod -R u=rwX,go=rX /srv/workout-explorer/web
sudo install -m 0644 Caddyfile /srv/workout-explorer/conf/Caddyfile
sudo install -m 0644 systemd/*.service systemd/*.slice systemd/*.path /etc/systemd/system/
sudo install -m 0755 systemd/workout-explorer-run /usr/local/sbin/workout-explorer-run
```

`.env`: from `example.env` as in RUNBOOK.md, "Configure", **root-owned, `0600`** (`sudo install -m 0600 /dev/null /srv/workout-explorer/.env`, then edit). systemd reads it for the app only; Caddy never sees it. Only the `WORKOUT_EXPLORER_*` keys matter; do not set `WORKOUT_EXPLORER_TRUSTED_PROXY` (the app trusts loopback, where Caddy runs). The operator file and Impressum go to `/srv/workout-explorer/operator/` as in RUNBOOK.md, "The About page: operator file".

## 3. Start

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now workout-explorer.service workout-explorer-caddy.service workout-explorer-caddy-upgrade.path
curl -fsS http://127.0.0.1:8088/api/health     # {"status":"ok","version":"v1.4.0"}
```

Your reverse proxy forwards to `127.0.0.1:8088` (RUNBOOK.md, "Reverse proxy", "Proxy on the host"); our Caddy trusts `X-Forwarded-For` from `127.0.0.1` and `::1` only. To change a value of the Caddy unit (port in `WX_SITE`, `WX_TRUSTED_PROXIES`, `WX_BIND`) use a drop-in: `sudo systemctl edit workout-explorer-caddy`. `workout-explorer-caddy-upgrade.path` restarts our Caddy when apt replaces `/usr/bin/caddy` (a moment without pages).

Both services restart after a crash, an out-of-memory kill or a reboot. Stop: `sudo systemctl stop workout-explorer workout-explorer-caddy` (until the next boot); to keep them off, `sudo systemctl disable --now workout-explorer workout-explorer-caddy`.

## 4. Notifications (optional)

Set both Pushover keys in `.env` (RUNBOOK.md, "Notifications"), then lift the app's outbound block (Caddy's stays):

```sh
sudo install -d /etc/systemd/system/workout-explorer.service.d
sudo install -m 0644 /tmp/wx-$v/systemd/notify.conf /etc/systemd/system/workout-explorer.service.d/notify.conf
sudo systemctl daemon-reload && sudo systemctl restart workout-explorer
```

## 5. Day to day

- `sudo workout-explorer-run regenerate`, `sudo workout-explorer-run delete [-y] <public_id>`, `sudo workout-explorer-run parse /srv/workouts/<file>.fit`: the subcommands as the app's user, with its `.env`, memory cap and sandbox (exit codes: section 6, step 6, and RUNBOOK.md, "Delete one workout").
- Reports: the query of RUNBOOK.md, "Reports from visitors", as the app's user: `sudo -u workout-explorer sqlite3 -readonly -line /srv/workouts/workouts.db "SELECT ..."`.
- Logs: `journalctl -u workout-explorer -u workout-explorer-caddy`; journal retention as in PODMAN.md section 10.
- Checks: `ss -ltnp` shows Caddy on `127.0.0.1:8088` and the app on `127.0.0.1:8080` only. `sudo systemd-run --pipe --wait -p User=workout-explorer -p IPAddressDeny=any -p IPAddressAllow=localhost curl -sS https://api.pushover.net` fails (that is the app's view without notifications). `grep Cap /proc/$(systemctl show -p MainPID --value workout-explorer-caddy)/status` shows only zeros.

## 6. Updating

By hand until automatic updates exist for this path. As root, with the new `v`:

1. Download and check both archives as in section 2, unpack them in `/tmp/wx-$v`.
2. `caddy validate --config /tmp/wx-$v/Caddyfile --adapter caddyfile` (with `WX_DATA=/srv/workouts`); when `systemd/caddy.pref` changed, install it first and `apt update && apt install caddy`.
3. Binary by temporary name and rename (the running file is busy): `install -m 0755 app/workout-explorer /srv/workout-explorer/bin/.workout-explorer.new && mv /srv/workout-explorer/bin/.workout-explorer.new /srv/workout-explorer/bin/workout-explorer`. Replace `web/` whole: `rm -rf /srv/workout-explorer/web.new && cp -r app/web /srv/workout-explorer/web.new && chmod -R u=rwX,go=rX /srv/workout-explorer/web.new && rm -rf /srv/workout-explorer/web && mv /srv/workout-explorer/web.new /srv/workout-explorer/web` (old pages keep their files: those live in `/srv/workouts/site/assets/`).
4. Unit files and `workout-explorer-run` when they changed (`diff -r systemd/ ...`), then `systemctl daemon-reload`.
5. `systemctl restart workout-explorer`; wait for `/api/health` to show the new tag. If the Caddyfile changed: install it and `systemctl reload workout-explorer-caddy`.
6. `workout-explorer-run regenerate`: rebuilds every workout and removes the web files no page needs. It prints `rebuilt=`, `deleted=`, `kept=` and `skipped=`, then `files_removed=` and `bundles_removed=`. Exit 0: done; 1: it stopped early, nothing removed, run it again; 4: rebuilt, but the clean-up failed, run it again later.
