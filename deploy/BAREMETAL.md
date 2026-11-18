# Running Workout Explorer on bare metal (no containers)

This is one of the three install paths (RUNBOOK.md, "Which way should I run it?"). Everything in RUNBOOK.md applies as well: the operator file and Impressum, configuration keys, notifications, the reverse proxy, updates, data and privacy. This file covers what is specific to this path.

The server binary and Caddy as two systemd services, with the same guarantees as the containers: both run as their own non-root users without capabilities, the app has no outbound connection unless notifications are on, Caddy sees only `public/` and `site/` of the data directory, the app's memory is capped. Needs Debian 13 (or another distribution with systemd 254+, cgroup v2 and polkit with JavaScript rules), linux/amd64. The unit files are in the release's deploy archive (`systemd/`).

Three users: `workout-explorer` runs the app and owns the data directory, `workout-explorer-caddy` runs Caddy, `deploy` owns the program files and runs `update.sh`, which installs every release (section 6). `deploy` is in no group of the other two and in no admin group; one polkit rule lets it restart exactly our two services and start exactly the `regenerate` unit, nothing else. It can choose the program the app runs (so it is as trusted as the app), but it can never become root: unit files, `workout-explorer-run`, the Caddy pin and the rule are root's, and only you install them.

## 1. Caddy from Caddy's apt repository

```sh
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl sqlite3 minisign polkitd
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
```

Then, after unpacking the deploy archive (section 2): `sudo install -m 0644 /tmp/wx-$v/systemd/caddy.pref /etc/apt/preferences.d/workout-explorer-caddy.pref`, `sudo apt update && sudo apt install caddy`. The pin holds the package at our release's minor version (for example `2.11.*`): patches arrive with your normal package updates, a new minor only with a release of ours. It applies to every use of the package on this host: if your TLS proxy is the same Caddy package, it is held there too. `update.sh` never runs `apt` and never changes Caddy.

The package starts its own `caddy.service` on port 80. Unless that is your TLS proxy, turn it off: `sudo systemctl disable --now caddy`. Ours is a separate unit with our Caddyfile.

## 2. Users, directories, root's files

```sh
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin workout-explorer
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin -G workout-explorer workout-explorer-caddy
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin deploy
sudo install -d -o deploy -g deploy -m 0755 /srv/workout-explorer /srv/workout-explorer/bin /srv/workout-explorer/conf
sudo install -d -m 0755 /srv/workout-explorer/operator
sudo install -d -o workout-explorer -g workout-explorer -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

Download the release's deploy archive, check the signature first, then the checksums. The checksums alone prove nothing: whoever can change the files can change `SHA256SUMS` too. Pin the release signing key once as in RUNBOOK.md, "Updates", step 1 (compare GitHub with the DNS record). If `minisign` says anything else than shown below, stop: the files are not the operator's. Root installs files only from such a hand-checked copy, never from `/srv/workout-explorer` (which `deploy` can write).

```sh
key=RW...                                       # the line you pinned
v=v1.4.0; base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
mkdir -p /tmp/wx-$v && cd /tmp/wx-$v
curl -fLO $base/workout-explorer-$v-deploy.tar.gz -O $base/SHA256SUMS -O $base/SHA256SUMS.minisig
minisign -V -P "$key" -m SHA256SUMS             # must print: Signature and comment signature verified
grep -c "  workout-explorer-$v-" SHA256SUMS     # must print 2: both names carry the tag you chose
sha256sum -c --ignore-missing SHA256SUMS        # the deploy archive: OK
tar -xzf workout-explorer-$v-deploy.tar.gz
sudo install -m 0644 systemd/*.service systemd/*.slice systemd/*.path /etc/systemd/system/
sudo install -m 0755 systemd/workout-explorer-run /usr/local/sbin/workout-explorer-run
sudo install -m 0644 systemd/50-workout-explorer.rules /etc/polkit-1/rules.d/50-workout-explorer.rules
sudo install -o deploy -g deploy -m 0755 update.sh /srv/workout-explorer/update.sh
```

(Then the Caddy pin and `apt install caddy`, section 1.) The rule names the user `deploy`; another name: edit it in the rule and in a drop-in of the update timer (section 6).

`.env`: from `example.env` as in RUNBOOK.md, "Configure", **owned by root, group `deploy`, `0640`** (`sudo install -o root -g deploy -m 0640 /dev/null /srv/workout-explorer/.env`, then edit): systemd reads it for the app, `update.sh` reads its switch, key and Pushover keys; Caddy never sees it. Only the `WORKOUT_EXPLORER_*` keys and `WX_UPDATE_PUBLIC_KEY` (the key you pinned) matter; do not set `WORKOUT_EXPLORER_TRUSTED_PROXY` (the app trusts loopback, where Caddy runs). The operator file and Impressum go to `/srv/workout-explorer/operator/` (root's) as in RUNBOOK.md, "The About page: operator file".

## 3. Start

```sh
sudo systemctl daemon-reload
sudo systemctl enable workout-explorer.service workout-explorer-caddy.service workout-explorer-caddy-upgrade.path
sudo systemctl start workout-explorer-caddy-upgrade.path
sudo -u deploy /srv/workout-explorer/update.sh v1.4.0     # installs binary, web files, Caddyfile; starts both
curl -fsS http://127.0.0.1:8088/api/health                # {"status":"ok","version":"v1.4.0"}
```

`update.sh` checks the release again with your pinned key, also the release tarball with the binary and web files, then installs them and starts both services (section 6).

Your reverse proxy forwards to `127.0.0.1:8088` (RUNBOOK.md, "Reverse proxy", "Proxy on the host"); our Caddy trusts `X-Forwarded-For` from `127.0.0.1` and `::1` only. To change a value of the Caddy unit (port in `WX_SITE`, `WX_TRUSTED_PROXIES`, `WX_BIND`) use a drop-in: `sudo systemctl edit workout-explorer-caddy` (`update.sh` follows a moved port). Never edit our unit files themselves: `update.sh` compares them with each release and stops if they differ. `workout-explorer-caddy-upgrade.path` restarts our Caddy when apt replaces `/usr/bin/caddy` (a moment without pages).

Both services restart after a crash, an out-of-memory kill or a reboot. Stop: `sudo systemctl stop workout-explorer workout-explorer-caddy` (until the next boot); to keep them off, `sudo systemctl disable --now workout-explorer workout-explorer-caddy`.

## 4. Notifications (optional)

Set both Pushover keys in `.env` (RUNBOOK.md, "Notifications"), then lift the app's outbound block (Caddy's stays):

```sh
sudo install -d /etc/systemd/system/workout-explorer.service.d
sudo install -m 0644 /tmp/wx-$v/systemd/notify.conf /etc/systemd/system/workout-explorer.service.d/notify.conf
sudo systemctl daemon-reload && sudo systemctl restart workout-explorer
```

## 5. Day to day

- `sudo workout-explorer-run regenerate`, `sudo workout-explorer-run delete [-y] <public_id>`, `sudo workout-explorer-run parse /srv/workouts/<file>.fit`: the subcommands as the app's user, with its `.env`, memory cap and sandbox. `regenerate` exits 0 when done; 1 when it stopped early (nothing removed, run it again); 4 when it rebuilt but the clean-up failed (run it again later). `delete`: RUNBOOK.md, "Delete one workout".
- Reports: the query of RUNBOOK.md, "Reports from visitors", as the app's user: `sudo -u workout-explorer sqlite3 -readonly -line /srv/workouts/workouts.db "SELECT ..."`.
- Logs: `journalctl -u workout-explorer -u workout-explorer-caddy`; `regenerate` during an update: `journalctl -u workout-explorer-regenerate`; journal retention as in PODMAN.md section 10.
- Checks: `ss -ltnp` shows Caddy on `127.0.0.1:8088` and the app on `127.0.0.1:8080` only. `sudo systemd-run --pipe --wait -p User=workout-explorer -p IPAddressDeny=any -p IPAddressAllow=localhost curl -sS https://api.pushover.net` fails (that is the app's view without notifications). `grep Cap /proc/$(systemctl show -p MainPID --value workout-explorer-caddy)/status` shows only zeros.

## 6. Updating

Read the release notes first. Then, as `deploy`, with the new version:

```sh
sudo -u deploy /srv/workout-explorer/update.sh v1.5.0
```

What it checks, its exit codes, the log line and the stopped state: RUNBOOK.md, "Updates". On this path, after the signature and the deploy archive, it also downloads the release tarball and checks it against the signed `SHA256SUMS` and the same archive rules. Then the release's `deploy.sh`, before changing anything:

- compares our unit files, `notify.conf` (when installed), `workout-explorer-run` and the Caddy pin with the release's copies, and checks that the installed Caddy has the pinned minor. Any difference: **exit 5**, nothing changed, the files are named in the terminal or journal (below);
- checks the new Caddyfile with the installed Caddy (rejected: exit 1, nothing changed).

Then it replaces the binary, `web/` and `update.sh`, and the Caddyfile when it changed; restarts the app (and Caddy only for a new Caddyfile: a moment without pages); waits up to 60 s for `/api/health` to show the new version; and starts `workout-explorer-regenerate.service`, which rebuilds every stored workout and removes the web files no page needs. Its counts are in `journalctl -u workout-explorer-regenerate` (`deploy` cannot read the journal). After a failure, fix the cause and run the same command again.

**When an update stops with exit 5** (`reason=root` in the log line and the stopped state): the release changes a file only root may install. As root, with that release downloaded and checked as in section 2, install the files `update.sh` named, then run `update.sh` again:

```sh
cd /tmp/wx-$v                                             # the release, checked by hand (section 2)
sudo install -m 0644 systemd/*.service systemd/*.slice systemd/*.path /etc/systemd/system/
sudo install -m 0755 systemd/workout-explorer-run /usr/local/sbin/workout-explorer-run
sudo install -m 0644 systemd/notify.conf /etc/systemd/system/workout-explorer.service.d/notify.conf   # only if you use notifications
sudo install -m 0644 systemd/caddy.pref /etc/apt/preferences.d/workout-explorer-caddy.pref && sudo apt update && sudo apt install caddy
sudo systemctl daemon-reload
sudo -u deploy /srv/workout-explorer/update.sh $v         # applies the release and clears the stopped state
```

A hand edit of our unit files stops every update the same way: put your changes in drop-ins (`sudo systemctl edit ...`) and install the release's file again.

### Automatic updates (optional)

Off by default. Install the timer once, as root, from a release you checked by hand (section 2; never from `/srv/workout-explorer/updater/`, which `deploy` can write). It runs `update.sh` as `deploy`, may write only to `/srv/workout-explorer` and a private `/tmp`, gets no new privileges and has no outbound filter (so its downloads and its Pushover alert work while the app's outbound block is on):

```sh
sudo install -m 0644 /tmp/wx-$v/updater/system/workout-explorer-update.service /tmp/wx-$v/updater/system/workout-explorer-update.timer /etc/systemd/system/
```

A deploy user not called `deploy`: `sudo systemctl edit workout-explorer-update.service` with `[Service]` and `User=<name>`.

Then turn it on. First `cat /srv/workout-explorer/update.stopped`: if the file exists, an earlier update failed and the nightly run would do nothing; fix that and run `update.sh vX.Y.Z` by hand until it succeeds. Then set `WORKOUT_EXPLORER_AUTO_UPDATE=on` in `.env`, `sudo systemctl restart workout-explorer` (the About page now says that updates come from GitHub), and `sudo systemctl daemon-reload && sudo systemctl enable --now workout-explorer-update.timer`.

- **When, check, run one now:** as DOCKER.md section 8, "Automatic updates" (every night between 03:00 and 04:00 at a fixed minute, `systemctl list-timers workout-explorer-update.timer`, `journalctl -u workout-explorer-update.service -n 20`, `sudo systemctl start workout-explorer-update.service`).
- **Stopped:** after a refused or failed update, `update.sh` alerts you once and stops (RUNBOOK.md, "Updates"); with exit 5, follow "When an update stops with exit 5" above. A successful `update.sh vX.Y.Z` by hand clears it.
- **Pause:** remove `WORKOUT_EXPLORER_AUTO_UPDATE` from `.env` and `sudo systemctl restart workout-explorer`. The timer still runs, but makes no request.

### Turning automatic updates off for good

```sh
sudo systemctl disable --now workout-explorer-update.timer
sudo rm -rf /etc/systemd/system/workout-explorer-update.service /etc/systemd/system/workout-explorer-update.timer \
  /etc/systemd/system/workout-explorer-update.service.d /etc/systemd/system/workout-explorer-update.timer.d
sudo systemctl daemon-reload
```

and remove `WORKOUT_EXPLORER_AUTO_UPDATE` from `.env`, then `sudo systemctl restart workout-explorer` (the About page drops its update line). The polkit rule, the user `deploy` and `update.sh` stay: `update.sh vX.Y.Z` as `deploy` is how you update by hand. They go only when you remove Workout Explorer (section 8).

## 7. Moving a hand-updated install to `update.sh`

An install set up before this guide had root-owned program files and was updated by hand. `update.sh` refuses it cleanly until it is moved (it cannot write its lock; nothing changes). The data directory is not touched, so every workout, draft, report and link stays.

1. Section 1's `apt install` line (for `minisign` and `polkitd`), and pin the signing key as RUNBOOK.md, "Updates", step 1; put it into `.env` as `WX_UPDATE_PUBLIC_KEY=RW...`.
2. Section 2: the `useradd ... deploy` line only, then download and check the release you want to run (the running one or a newer one) and run the `install` lines for the units, `workout-explorer-run` and the rule; `notify.conf` too if you use notifications; the Caddy pin and `apt install caddy` if the release's pin differs. `sudo systemctl daemon-reload`.
3. Hand the program directory to `deploy`, except `.env` and `operator/`:

   ```sh
   cd /srv/workout-explorer
   sudo chown deploy:deploy . && sudo find . -mindepth 1 -maxdepth 1 ! -name .env ! -name operator -exec chown -R deploy:deploy {} +
   sudo chown root:deploy .env && sudo chmod 0640 .env
   sudo install -o deploy -g deploy -m 0755 /tmp/wx-$v/update.sh /srv/workout-explorer/update.sh
   ```

4. `sudo -u deploy /srv/workout-explorer/update.sh $v`. From then on, every update goes through it (section 6).

## 8. Removing Workout Explorer completely

This removes everything this guide installed. **Removing the data directory deletes every workout, draft and report for good: there is no backup and no way back.** Links you shared stop working.

1. Your reverse proxy: remove its entry for this site (the one forwarding to `127.0.0.1:8088`, RUNBOOK.md, "Reverse proxy"). This guide does not change your proxy; do it in your proxy's own way.
2. Services, timers, units and drop-ins:

   ```sh
   for u in workout-explorer-update.timer workout-explorer-caddy-upgrade.path workout-explorer-caddy.service workout-explorer.service; do
     sudo systemctl disable --now $u                          # "does not exist" for one you never installed: fine
   done
   sudo systemctl stop workout-explorer-update.service workout-explorer-regenerate.service workout-explorer-caddy-upgrade.service workout-explorer.slice
   ls -d /etc/systemd/system/workout-explorer*                # check: only ours (units, drop-in directories)
   sudo rm -rf /etc/systemd/system/workout-explorer*
   sudo systemctl daemon-reload && sudo systemctl reset-failed
   ```

3. The polkit rule and the run wrapper: `sudo rm /etc/polkit-1/rules.d/50-workout-explorer.rules /usr/local/sbin/workout-explorer-run`.
4. Program and data directories, with `.env`, the operator file and Impressum: `sudo rm -rf /srv/workout-explorer /srv/workouts`. The release downloads: `sudo rm -rf /tmp/wx-*`.
5. Users and groups: `sudo userdel workout-explorer-caddy && sudo userdel workout-explorer && sudo userdel deploy`, then `sudo groupdel` for any of the three groups `getent group` still shows.
6. Caddy. **If your TLS proxy is the same Caddy package (section 1), do not remove the package, its repository or its key:** remove only our pin (`sudo rm /etc/apt/preferences.d/workout-explorer-caddy.pref && sudo apt update`); your Caddy then follows its repository's newest version with the next `apt upgrade`. Otherwise:

   ```sh
   sudo apt purge caddy
   sudo rm /etc/apt/preferences.d/workout-explorer-caddy.pref /etc/apt/sources.list.d/caddy-stable.list /usr/share/keyrings/caddy-stable-archive-keyring.gpg
   sudo apt update
   ```

   The package's own leftovers, if any: `sudo rm -rf /etc/caddy /var/lib/caddy` and `sudo userdel caddy`.
7. Packages installed only for this: `sudo apt purge polkitd minisign` (and `sqlite3`) if nothing else on the host uses them; apt lists what would go before it asks.
8. The journal keeps our services' lines until its retention removes them (section 5). To drop them now: `sudo journalctl --rotate && sudo journalctl --vacuum-time=1s`, which empties the journal of the whole host.

Check that nothing is left; each command must print nothing (or only "No such file or directory" / "not installed"):

```sh
systemctl list-unit-files 'workout-explorer*' --no-legend; systemctl list-units --all 'workout-explorer*' --no-legend
ls -d /etc/systemd/system/*workout-explorer* /etc/systemd/system/*/*workout-explorer* /etc/polkit-1/rules.d/*workout-explorer* \
  /usr/local/sbin/workout-explorer-run /srv/workout-explorer /srv/workouts /etc/apt/preferences.d/workout-explorer-caddy.pref /tmp/wx-*
getent passwd workout-explorer workout-explorer-caddy deploy; getent group workout-explorer workout-explorer-caddy deploy
ls -d /etc/apt/sources.list.d/caddy* /usr/share/keyrings/caddy* /etc/caddy /var/lib/caddy; getent passwd caddy  # unless your proxy is Caddy
dpkg -l caddy polkitd minisign 2>/dev/null | grep '^ii'                                                          # only what you kept on purpose
```
