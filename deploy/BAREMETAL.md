# Running Workout Explorer on bare metal (no containers)

This is one of the three install paths (RUNBOOK.md, "Which way should I run it?"). Everything in RUNBOOK.md applies as well: the operator file and Impressum, configuration keys, notifications, the reverse proxy, updates, data and privacy. This file covers what is specific to this path.

The server binary and Caddy as two systemd services, with the same guarantees as the containers: both run as their own non-root users without capabilities, the app has no outbound connection unless notifications are on, Caddy sees only `public/` and `site/` of the data directory, the app's memory is capped. Needs Debian 13 (or another distribution with systemd 254+, cgroup v2 and polkit with JavaScript rules), linux/amd64. The unit files are in the release's deploy archive (`systemd/`).

Three users: `workout-explorer` runs the app and owns the data directory, `workout-explorer-caddy` runs Caddy, `deploy` owns the program files and runs `update.sh`, which installs every release (section 6). `deploy` is in no group of the other two and in no admin group; one polkit rule lets it restart exactly our two services and start exactly the `regenerate` unit, nothing else. It can choose the program the app runs (so it is as trusted as the app), but it can never become root: unit files, `workout-explorer-run`, the Caddy pin, the rule and `/etc/workout-explorer/` (settings and operator file) are root's, and only you install them. Root never reads, runs or installs anything from a place `deploy` can change.

## 1. Caddy from Caddy's apt repository

First note which of these packages are not installed yet: section 8 removes only those.

```sh
for p in debian-keyring debian-archive-keyring apt-transport-https gnupg curl sqlite3 minisign polkitd; do dpkg -s $p >/dev/null 2>&1 || echo $p; done | tee ~/wx-new-packages.txt   # not installed yet
sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https gnupg curl sqlite3 minisign polkitd
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list
```

Then, after unpacking the deploy archive (section 2, in the same shell): `sudo install -m 0644 "$w"/systemd/caddy.pref /etc/apt/preferences.d/workout-explorer-caddy.pref`, `sudo apt update && sudo apt install caddy`. The pin holds the package at our release's minor version (for example `2.11.*`): patches arrive with your normal package updates, a new minor only with a release of ours. It applies to every use of the package on this host: if your TLS proxy is the same Caddy package, it is held there too. `update.sh` never runs `apt` and never changes Caddy.

The package starts its own `caddy.service` on port 80. Unless that is your TLS proxy, turn it off: `sudo systemctl disable --now caddy`. Ours is a separate unit with our Caddyfile.

## 2. Users, directories, root's files

The name `deploy` must be new: `id deploy` must say "no such user". If it exists (often an admin account), stop and pick another name; use it instead of `deploy` everywhere below, in the rule and in a drop-in of the update timer (section 6).

```sh
id deploy                                       # must fail: no such user
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin workout-explorer
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin -G workout-explorer workout-explorer-caddy
sudo useradd --system --user-group --no-create-home --shell /usr/sbin/nologin deploy
sudo install -d -m 0755 /srv/workout-explorer /srv/workout-explorer/bin /srv/workout-explorer/conf
sudo install -d -m 0755 /etc/workout-explorer /etc/workout-explorer/operator
sudo install -d -o workout-explorer -g workout-explorer -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

Download the release's deploy archive, check the signature first, then the checksums. The checksums alone prove nothing: whoever can change the files can change `SHA256SUMS` too. Pin the release signing key once as in RUNBOOK.md, "Updates", step 1 (compare GitHub with the DNS record). If `minisign` says anything else than shown below, stop: the files are not the operator's. Root installs files only from such a hand-checked copy in a folder of your own, never from `/srv/workout-explorer` (which `deploy` can write). Keep this shell open until the last `sudo install` (here and in sections 1 and 4); in a new shell, download and check again.

```sh
key=RW...                                       # the line you pinned
v=v1.4.0; base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
w=$(mktemp -d) && cd "$w"                       # a new folder only you can write
curl -fLO $base/workout-explorer-$v-deploy.tar.gz -O $base/SHA256SUMS -O $base/SHA256SUMS.minisig
minisign -V -P "$key" -m SHA256SUMS             # must print: Signature and comment signature verified
grep -c "  workout-explorer-$v-" SHA256SUMS     # must print 2: both names carry the tag you chose
sha256sum -c --ignore-missing SHA256SUMS        # the deploy archive: OK
tar -xzf workout-explorer-$v-deploy.tar.gz
grep -qx 'EnvironmentFile=/etc/workout-explorer/.env' "$w"/systemd/workout-explorer.service && {
sudo install -m 0644 "$w"/systemd/*.service "$w"/systemd/*.slice "$w"/systemd/*.path /etc/systemd/system/
sudo install -m 0755 "$w"/systemd/workout-explorer-run /usr/local/sbin/workout-explorer-run
sudo install -m 0644 "$w"/systemd/50-workout-explorer.rules /etc/polkit-1/rules.d/50-workout-explorer.rules
sudo install -m 0755 "$w"/update.sh /srv/workout-explorer/update.sh
sudo chown -R deploy:deploy /srv/workout-explorer   # last: from now on root writes nothing there
} || echo "release too old for this guide: stop"
```

(Then the Caddy pin and `apt install caddy`, section 1.)

Settings: `/etc/workout-explorer/.env` (in this guide, `.env` means this file), from `example.env` as in RUNBOOK.md, "Configure", **owned by root, group `deploy`, `0640`**: `sudo install -o root -g deploy -m 0640 "$w"/example.env /etc/workout-explorer/.env`, then `sudoedit /etc/workout-explorer/.env`. systemd reads it for the app, `update.sh` reads its switch, key and notification settings; Caddy never sees it. Only the `WORKOUT_EXPLORER_*` keys and `WX_UPDATE_PUBLIC_KEY` (the key you pinned) matter; do not set `WORKOUT_EXPLORER_TRUSTED_PROXY` (the app trusts loopback, where Caddy runs). The operator file and Impressum (RUNBOOK.md, "The About page: operator file") go to `/etc/workout-explorer/operator/`, root's, files `0644` (the app reads them): `sudo install -m 0644 "$w"/operator.example.txt /etc/workout-explorer/operator/about.txt`, `sudoedit` it, and set `WORKOUT_EXPLORER_OPERATOR_FILE=/etc/workout-explorer/operator/about.txt` (Impressum likewise). `update.sh` refuses to run (exit 2) while a `.env` or `operator` is in `/srv/workout-explorer`.

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

Set the notification service in `.env` (Pushover keys, or ntfy or callback settings; RUNBOOK.md, "Notifications"). A service on this host (`http://127.0.0.1:...`) needs no drop-in; for one on the LAN write your own `notify-lan.conf` with `IPAddressAllow=<address>` instead of `notify.conf` (`deploy.sh` byte-compares only `notify.conf`). Otherwise lift the app's outbound block (Caddy's stays):

```sh
sudo install -d /etc/systemd/system/workout-explorer.service.d
sudo install -m 0644 "$w"/systemd/notify.conf /etc/systemd/system/workout-explorer.service.d/notify.conf   # $w: a release checked as in section 2, same shell
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

**When an update stops with exit 5** (`reason=root` in the log line and the stopped state): the release changes a file only root may install. Choose the version yourself from the [Releases page](https://github.com/b0wter/WorkoutExplorer/releases) (normally the newest), never from `update.stopped`, the log line or the alert: `deploy` writes those, and a genuine but older release could bring back units that read `.env` from its directory. Optional prompt: if it is older than the running one (`curl -fsS http://127.0.0.1:8088/api/health`), stop unless you mean to roll back; the layout check is the guard. To roll back on purpose across a root-file change, choose the older release the same way; it must still pass the layout check (v0.9.1 and older do not). In your own login, download and check that release as in section 2 (a new `w=$(mktemp -d)`, all checks, `tar`, the layout check), install the files `update.sh` named, then run `update.sh` again, all in that one shell:

```sh
grep -qx 'EnvironmentFile=/etc/workout-explorer/.env' "$w"/systemd/workout-explorer.service && {
sudo install -m 0644 "$w"/systemd/*.service "$w"/systemd/*.slice "$w"/systemd/*.path /etc/systemd/system/
sudo install -m 0755 "$w"/systemd/workout-explorer-run /usr/local/sbin/workout-explorer-run
sudo install -m 0644 "$w"/systemd/notify.conf /etc/systemd/system/workout-explorer.service.d/notify.conf   # only if you use notifications
sudo install -m 0644 "$w"/systemd/caddy.pref /etc/apt/preferences.d/workout-explorer-caddy.pref && sudo apt update && sudo apt install caddy
sudo systemctl daemon-reload
sudo systemctl try-restart workout-explorer-caddy.service workout-explorer-caddy-upgrade.path   # their new definitions take effect (update.sh restarts the app)
sudo -u deploy /srv/workout-explorer/update.sh $v         # applies the release and clears the stopped state
} || echo "release too old for this guide: stop"
```

A hand edit of our unit files stops every update the same way: put your changes in drop-ins (`sudo systemctl edit ...`) and install the release's file again.

### Automatic updates (optional)

Off by default. Install the timer once, as root, from a release you checked by hand (section 2; never from `/srv/workout-explorer/updater/`, which `deploy` can write). It runs `update.sh` as `deploy`, may write only to `/srv/workout-explorer` and a private `/tmp`, gets no new privileges and has no outbound filter (so its downloads and its Pushover alert work while the app's outbound block is on):

```sh
sudo install -m 0644 "$w"/updater/system/workout-explorer-update.service "$w"/updater/system/workout-explorer-update.timer /etc/systemd/system/   # $w as in section 2
```

A deploy user not called `deploy`: `sudo systemctl edit workout-explorer-update.service` with `[Service]` and `User=<name>`.

Then turn it on. First `sudo -u deploy cat /srv/workout-explorer/update.stopped`: if the file exists, an earlier update failed and the nightly run would do nothing; fix that and run `update.sh vX.Y.Z` by hand until it succeeds. Then set `WORKOUT_EXPLORER_AUTO_UPDATE=on` in `.env`, `sudo systemctl restart workout-explorer` (the About page now says that updates come from GitHub), and `sudo systemctl daemon-reload && sudo systemctl enable --now workout-explorer-update.timer`.

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

An install set up before this guide had root-owned program files, `.env` and `operator/` in `/srv/workout-explorer`, and was updated by hand. `update.sh` refuses it cleanly until it is moved (it cannot write its lock; nothing changes). The data directory is not touched, so every workout, draft, report and link stays. Do steps 2 to 4 in one go: until step 3's units are installed, a restart of the app would not find `.env`.

1. Section 1's package note and `apt install` line (for `gnupg`, `minisign` and `polkitd`), and pin the signing key as RUNBOOK.md, "Updates", step 1.
2. While `/srv/workout-explorer` is still root's: the user `deploy` (section 2's `id deploy` check and `useradd ... deploy` line), then move root's files to `/etc/workout-explorer` (BM-2):

   ```sh
   sudo install -d -m 0755 /etc/workout-explorer
   sudo mv /srv/workout-explorer/.env /etc/workout-explorer/.env
   sudo chown root:deploy /etc/workout-explorer/.env && sudo chmod 0640 /etc/workout-explorer/.env
   sudo mv /srv/workout-explorer/operator /etc/workout-explorer/operator    # if you have one; else: sudo install -d -m 0755 /etc/workout-explorer/operator
   sudo chown -R root:root /etc/workout-explorer/operator && sudo chmod -R u=rwX,go=rX /etc/workout-explorer/operator
   sudoedit /etc/workout-explorer/.env   # operator and Impressum paths now /etc/workout-explorer/operator/...; add WX_UPDATE_PUBLIC_KEY=RW...
   ```

3. Take the version from the [Releases page](https://github.com/b0wter/WorkoutExplorer/releases): the first release with the `/etc/workout-explorer` layout (the next release after v0.9.1) or newer, never one named by `deploy`'s files. Download and check it as in section 2, run its layout check (the `grep -qx 'EnvironmentFile=/etc/workout-explorer/.env'` line: it stops on an older release) before any `sudo install`, and in that shell run its `install` lines for the units, `workout-explorer-run`, the rule and `update.sh` (not the `chown` yet); `notify.conf` too if you use notifications; the Caddy pin and `apt install caddy` if the release's pin differs. Then `sudo systemctl daemon-reload`.
4. Hand the program directory to `deploy` (`chown -R` follows no symbolic link): `sudo chown -R deploy:deploy /srv/workout-explorer`.
5. `sudo -u deploy /srv/workout-explorer/update.sh $v`. From then on, every update goes through it (section 6).

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
4. Program and data directories, settings, the operator file and Impressum: `sudo rm -rf /srv/workout-explorer /srv/workouts /etc/workout-explorer`. Your download folders (`$w`, in `/tmp`) go at the next reboot, or with `rm -rf "$w"` in their shell.
5. Users and groups: `sudo userdel workout-explorer-caddy && sudo userdel workout-explorer && sudo userdel deploy` (`deploy`, or the name you chose, only because section 2 created it new), then `sudo groupdel` for any of the three groups `getent group` still shows.
6. Caddy. **If your TLS proxy is the same Caddy package (section 1), do not remove the package, its repository or its key:** remove only our pin (`sudo rm /etc/apt/preferences.d/workout-explorer-caddy.pref && sudo apt update`); your Caddy then follows its repository's newest version with the next `apt upgrade`. Otherwise:

   ```sh
   sudo apt purge caddy
   sudo rm /etc/apt/preferences.d/workout-explorer-caddy.pref /etc/apt/sources.list.d/caddy-stable.list /usr/share/keyrings/caddy-stable-archive-keyring.gpg
   sudo apt update
   ```

   The package's own leftovers, if any: `sudo rm -rf /etc/caddy /var/lib/caddy`, `sudo userdel caddy` and `sudo groupdel caddy`.
7. Packages section 1 installed that were not there before (`~/wx-new-packages.txt` from section 1; then `rm ~/wx-new-packages.txt`), if nothing else on the host uses them: `sudo apt purge` some of `polkitd minisign sqlite3 gnupg debian-keyring apt-transport-https` (never `debian-archive-keyring`); apt lists what would go before it asks. Not sure: keep it.
8. The journal keeps our services' lines until its retention removes them (section 5). To drop them now: `sudo journalctl --rotate && sudo journalctl --vacuum-time=1s`, which empties the journal of the whole host.

Check that nothing is left; each command must print nothing (or only "No such file or directory" / "not installed"):

```sh
systemctl list-unit-files 'workout-explorer*' --no-legend; systemctl list-units --all 'workout-explorer*' --no-legend
ls -d /etc/systemd/system/*workout-explorer* /etc/systemd/system/*/*workout-explorer* /etc/polkit-1/rules.d/*workout-explorer* \
  /usr/local/sbin/workout-explorer-run /srv/workout-explorer /srv/workouts /etc/workout-explorer /etc/apt/preferences.d/workout-explorer-caddy.pref
getent passwd workout-explorer workout-explorer-caddy deploy; getent group workout-explorer workout-explorer-caddy deploy
ls -d /etc/apt/sources.list.d/caddy* /usr/share/keyrings/caddy* /etc/caddy /var/lib/caddy; getent passwd caddy; getent group caddy  # unless your proxy is Caddy
dpkg -l caddy polkitd minisign sqlite3 gnupg debian-keyring apt-transport-https 2>/dev/null | grep '^ii'  # only what you kept on purpose
ls ~/wx-new-packages.txt
sudo find / -xdev \( -nouser -o -nogroup \) -print   # files of a deleted user; repeat with /srv etc. for each other file system
```
