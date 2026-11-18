# Running Workout Explorer with rootless Podman and systemd (Quadlet)

This is one of the three install paths (RUNBOOK.md, "Which way should I run it?"). It runs the same two containers as the Docker Compose path, our published app image and the official Caddy image, with the same settings, but as **rootless Podman containers managed by systemd** through [Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) units. The units and a control script are files of the release's deploy archive (`podman/`); this guide explains them.

Everything in RUNBOOK.md that is not about Docker still applies: the operator file and Impressum, configuration keys, notifications, the reverse proxy, data and privacy (the sections of that name). This file covers what is different. Written for Podman 5.4 (netavark, pasta) on Debian 13.

## How Compose maps to Quadlet

| `compose.yaml` | Here (`podman/` in the deploy archive) |
| --- | --- |
| `services.app`, `services.caddy` | `workout-explorer-app.container`, `workout-explorer-caddy.container` (same images: ours by digest, Caddy by minor tag) |
| `networks.edge`, `networks.backend` | `workout-explorer-edge.network`, `workout-explorer-backend.network` (`Internal=true`) |
| `compose.notify.yaml` | `workout-explorer-egress.network` plus a drop-in (section 8) |
| `mem_limit` / `memswap_limit` | `PodmanArgs=--memory=1200m --memory-swap=1200m` (Quadlet 5.4 has no `Memory=` key) |
| `tmpfs` with `uid=` | `Tmpfs=...,U` (`U` gives it to the container user) |
| `read_only`, `cap_drop`, `no-new-privileges` | `ReadOnly=true`, `DropCapability=all`, `NoNewPrivileges=true` |
| `restart: unless-stopped` | `Restart=always`; `stop` keeps it stopped until the next boot |
| `ports: 127.0.0.1:8088:8088` | `PublishPort=127.0.0.1:8080:8088` |
| `local` log driver | journald (section 10) |
| `docker compose ...` | `wx ...`, the control script as the service user (section 5) |

Two differences that matter:

1. **`WX_TRUSTED_PROXIES` is Caddy's own edge address, not the gateway.** With rootless port forwarding, requests from the host proxy reach Caddy from the address of Caddy's *own* interface on `edge`. So the Caddy unit pins that address to `10.250.0.2` and trusts it. Check it after setup (section 6).
2. **Container UIDs map to other host UIDs.** Rootless Podman maps container UID `N` to host UID `subuid_start + N - 1` of the service user. The app's `10001` is host UID `subuid_start + 10000` (for example `175536` when `/etc/subuid` says `workouts:165536:65536`). The data directory belongs to *that* UID.

## 1. Prerequisites

- linux/amd64, Podman 5 with netavark and pasta (`podman info` shows `networkBackend: netavark`), `curl`, `tar`, `sha256sum`, `flock`, `minisign` (checks every release's signature, RUNBOOK.md, "Updates"), `sqlite3` (reports). Debian: `apt install podman curl minisign sqlite3`.
- cgroup v2 with the memory controller delegated to the service user (Debian 13 default), or the memory cap is silently not applied. Check after step 2: `cat /sys/fs/cgroup/user.slice/user-$(id -u workouts).slice/user@$(id -u workouts).service/cgroup.controllers` lists `memory`.
- A reverse proxy on the host for TLS (RUNBOOK.md, "Reverse proxy"; Caddy example in section 7 below).

## 2. Service user

A user without login runs the containers. It needs a subordinate UID/GID range and *linger*, so its systemd instance starts at boot.

```sh
sudo useradd --create-home --home-dir /var/lib/workouts --shell /usr/sbin/nologin \
  --comment "Workout Explorer (rootless podman)" workouts
grep workouts /etc/subuid /etc/subgid          # a range of 65536, e.g. workouts:165536:65536
sudo loginctl enable-linger workouts
huid=$(( $(awk -F: '$1=="workouts"{print $2}' /etc/subuid) + 10000 ))   # host UID of container 10001
hgid=$(( $(awk -F: '$1=="workouts"{print $2}' /etc/subgid) + 10000 ))
```

Every command of ours runs **as this user, never as root** (ADR-054): the deploy files, the units and the control script belong to it, and root never runs a file it can write. In your shell, define once:

```sh
wx() { sudo -u workouts /srv/workout-explorer/workout-explorer-ctl "$@"; }
```

## 3. Folders, deploy files, configuration

The deploy directory belongs to the service user (ADR-054); the data directory to the mapped `10001`. There is no `bin/` or `web/` any more: both are in the image.

```sh
sudo install -d -o workouts -g workouts -m 0755 /srv/workout-explorer /srv/workout-explorer/conf /srv/workout-explorer/operator
sudo install -d -o $huid -g $hgid -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

Pin the release signing key as in RUNBOOK.md, "Updates", step 1 (compare GitHub with the DNS record) and keep it in `key`. Download the release's deploy archive and check the signature, then the checksum (take the newest `vX.Y.Z` from the Releases page; never use the repository's `deploy/` files, they name no image):

```sh
key=RW...                                       # the line you pinned
v=v1.4.0
base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
w=$(mktemp -d) && chmod 0755 "$w" && cd "$w"    # a new folder only you can write (workouts may read it); keep this shell open
curl -fLO $base/workout-explorer-$v-deploy.tar.gz -O $base/SHA256SUMS -O $base/SHA256SUMS.minisig
minisign -V -P "$key" -m SHA256SUMS             # must print: Signature and comment signature verified
grep -c "  workout-explorer-$v-" SHA256SUMS     # must print 2: both names carry the tag you chose
sha256sum -c --ignore-missing SHA256SUMS        # must print: ...deploy.tar.gz: OK
mkdir d && tar -xzf workout-explorer-$v-deploy.tar.gz -C d
sudo -u workouts install -m 0755 d/update.sh /srv/workout-explorer/update.sh
sudo -u workouts cp -r d/updater /srv/workout-explorer/   # the timer units (section 9); every update refreshes them
```

If `minisign` says anything else, stop: the files are not the operator's.

Write `/srv/workout-explorer/.env` from `d/example.env` as in RUNBOOK.md, "Configure", with `WX_UPDATE_PUBLIC_KEY` set to the key you pinned. Of the other keys only `WORKOUT_EXPLORER_*` are read (the app unit loads them); `WX_*` and `COMPOSE_FILE` have no effect. Do not set `WORKOUT_EXPLORER_TRUSTED_PROXY`; the app unit sets it. The service user reads the file, so:

```sh
sudo chown root:workouts /srv/workout-explorer/.env && sudo chmod 0640 /srv/workout-explorer/.env
```

## 4. Install and start

```sh
sudo -u workouts sh "$w"/d/podman/workout-explorer-ctl install "$w"/d   # pulls both images, validates the Caddyfile, installs units, Caddyfile and itself; $w: section 3's shell, else download and check again
wx start
curl -s http://127.0.0.1:8080/api/health        # {"status":"ok","version":"v1.4.0"}
```

`install` puts the units into the service user's Quadlet directory `/var/lib/workouts/.config/containers/systemd/`, the Caddyfile into `/srv/workout-explorer/conf/` and the control script into `/srv/workout-explorer/`. Nothing changes if a pull fails or the new Caddyfile is invalid. `WantedBy=default.target` starts both units at every boot. Later releases are installed with `update.sh` (section 9).

## 5. Control script

`/srv/workout-explorer/workout-explorer-ctl` runs `systemctl --user` and `podman` as the service user and refuses to run as root. `wx` (section 2) calls it that way. Its header lists every command.

| DOCKER.md (Docker) | Here |
| --- | --- |
| `docker compose up -d` / `stop` / `restart` | `wx start` / `stop` / `restart` |
| `docker compose restart app` | `wx restart-app` (published pages stay online) |
| `docker compose exec caddy caddy reload ...` | `wx reload-caddy` |
| `docker compose pull` | `wx pull` |
| `docker compose logs app` | `wx logs -n 100` (or, as root, `sudo journalctl -o cat _SYSTEMD_USER_UNIT=workout-explorer-app.service -n 100`) |
| `docker compose exec app ... delete ...` | `wx delete <public_id>` |
| `docker compose exec app ... regenerate ...` | `wx regenerate` |
| the `sqlite3` report query | `wx reports` (read-only, as the container's user ID inside the service user's namespace) |
| `docker inspect ...` | `wx podman inspect workout-explorer-app` |

`stop` keeps both stopped until the next boot. Quadlet units cannot be disabled; to keep the site off across reboots, remove the units (section 10).

## 6. Check the result

- `curl -s https://workouts.example.org/api/health` shows the version; `/`, `/about` and `/robots.txt` return 200.
- `ss -ltn`: the project listens only on `127.0.0.1:8080`.
- Hardening: `wx podman top workout-explorer-app capprm capeff` and the same for `workout-explorer-caddy` show `none` in both columns (the app image has no shell or `grep`).
- **Trusted proxy:** set `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4=1` for a moment, upload from two networks (for example phone on mobile data and home): both must work once. If everyone shares one limit, the source address Caddy sees is not `10.250.0.2` (`wx podman inspect workout-explorer-caddy`).

## 7. Host reverse proxy

As RUNBOOK.md, "Reverse proxy", forwarding to `127.0.0.1:8080`. With Caddy on the host (`/etc/caddy/Caddyfile`):

```caddyfile
workouts.example.org {
	reverse_proxy 127.0.0.1:8080 {
		transport http {
			response_header_timeout 8m
		}
	}
}
```

## 8. Notifications (optional)

Set both Pushover keys in `.env` (RUNBOOK.md, "Notifications"), then give the app alone the egress network with a drop-in, which `install` never overwrites:

```sh
d=/var/lib/workouts/.config/containers/systemd/workout-explorer-app.container.d
sudo -u workouts mkdir -p $d
printf '[Container]\nNetwork=workout-explorer-egress.network\n' | sudo -u workouts tee $d/notify.conf
wx daemon-reload && wx restart
```

`wx logs | grep 'server started'` shows `"notifications":true`. To turn it off, remove `notify.conf` and the keys, `daemon-reload`, `restart`; the app then reaches no host at all.

## 9. Updating

Read the release notes first. Then, as the service user, with the new version:

```sh
sudo -u workouts /srv/workout-explorer/update.sh v1.5.0
```

What it checks and does, its exit codes, the log line and the stopped state: RUNBOOK.md, "Updates". After the signature check it runs the release's `deploy.sh`, which uses the release's control script: pull both images and check the new Caddy (nothing changes if that fails), install units, Caddyfile, control script and `update.sh`, `daemon-reload`, restart both (the app on its new image, Caddy on the newest patch of its minor; brief page outage), wait for `/api/health` on `127.0.0.1:8080` to show the new tag, then `regenerate` (rebuilds every workout and removes old web assets no page needs). After a failure, fix the cause and run it again.

OOM check afterwards: `wx podman inspect -f '{{.State.OOMKilled}}' workout-explorer-app` is `false`, and `NRestarts` of the app unit did not grow.

### Automatic updates (optional)

Off by default. A user timer of the service user runs `update.sh` every night; no root step and no sudo are involved in the update itself. Install it once, as that user. The units come from the last release you installed: section 3 and every `update.sh` put them into `/srv/workout-explorer/updater/` (installed before that? run `update.sh` once with the running version):

```sh
u=/var/lib/workouts/.config/systemd/user s=/srv/workout-explorer/updater/user
sudo -u workouts mkdir -p $u
sudo -u workouts install -m 0644 $s/workout-explorer-update.service $s/workout-explorer-update.timer $u/
```

Then turn it on. First `cat /srv/workout-explorer/update.stopped`: if the file exists, an earlier deploy failed and the nightly run would do nothing; fix that and run `update.sh vX.Y.Z` by hand until it succeeds. Then set `WORKOUT_EXPLORER_AUTO_UPDATE=on` in `.env` and `wx restart-app` (the About page now says that updates come from GitHub), then:

```sh
sudo systemctl --user -M workouts@ daemon-reload
sudo systemctl --user -M workouts@ enable --now workout-explorer-update.timer
```

- **When:** every night between 03:00 and 04:00 (host time), always at the same minute; no catch-up after the host was off. Another hour: a drop-in `$u/workout-explorer-update.timer.d/hour.conf` (as the service user) with `[Timer]`, `OnCalendar=` and `OnCalendar=*-*-* 05:00:00`, then `daemon-reload` as above.
- **Check:** `sudo systemctl --user -M workouts@ list-timers` shows the next run; `sudo journalctl _SYSTEMD_USER_UNIT=workout-explorer-update.service -n 20 -o cat` the last runs and their `update:` lines. Run one now: `sudo systemctl --user -M workouts@ start workout-explorer-update.service`.
- **Rights:** the run is the unprivileged service user; it writes only the deploy directory, its unit directory and its image storage. Linger (section 2) keeps the timer running without a login.
- **Stopped:** after a refused or failed update, `update.sh` alerts you once and stops: `cat /srv/workout-explorer/update.stopped`. A successful `update.sh vX.Y.Z` by hand clears it.
- **Turn off:** remove `WORKOUT_EXPLORER_AUTO_UPDATE` from `.env` and `wx restart-app`; the timer then runs but makes no request. To remove it: `sudo systemctl --user -M workouts@ disable --now workout-explorer-update.timer` and `sudo -u workouts rm $u/workout-explorer-update.service $u/workout-explorer-update.timer`.

### Caddy between releases

`wx pull && wx restart` brings Caddy's newest patch. `install`, `pull`, `start` and `restart` first check that the Caddy image starts under our hardening and accepts the Caddyfile. A pulled patch that fails gives its tag back to the previous image, so neither a restart nor a reboot (Quadlet starts the local tag) runs it; the command then exits 1 and nothing changed. Use `pull`, never `podman pull` directly.

### Moving to the service-user layout (installs before automatic updates)

Before F-52 the units lived in `/etc/containers/systemd/users/<uid>/`, owned by root, and the control script in `/usr/local/sbin`, run with `sudo`. Now both belong to the service user (ADR-054). Data, `.env` and the containers' state stay as they are:

```sh
sudo workout-explorer-ctl stop                                   # the old script, a last time
old=/etc/containers/systemd/users/$(id -u workouts)
q=/var/lib/workouts/.config/containers/systemd
sudo -u workouts mkdir -p $q
[ ! -d $old/workout-explorer-app.container.d ] || sudo cp -r $old/workout-explorer-app.container.d $q/   # the notify drop-in
sudo rm -r $old/workout-explorer-*
sudo rm /usr/local/sbin/workout-explorer-ctl
sudo chown -R workouts:workouts /srv/workout-explorer/conf /srv/workout-explorer/operator $q
sudo chown workouts:workouts /srv/workout-explorer
```

Then sections 3 (pin the key, add `WX_UPDATE_PUBLIC_KEY` to `.env`, download and check the release, install `update.sh`) and 4 (`install`, `wx start`), and `wx regenerate`.

### Migrating from the tarball setup (bind-mounted binaries)

Before F-59 this guide ran both containers on the distroless base with `bin/` and `web/` mounted from the host. The data directory stays in place with its owner, so every workout, link, expiry and report survives:

1. `sudo workout-explorer-ctl stop` (the old, root-run script), then copy the old web assets into the data directory as the mapped UID, so pages published under the old release keep their scripts and styles until `regenerate` rebuilds them:
   ```sh
   sudo install -d -o $huid -g $hgid -m 0750 /srv/workouts/site/assets
   sudo setpriv --reuid=$huid --regid=$hgid --clear-groups cp -rn /srv/workout-explorer/web/assets/. /srv/workouts/site/assets/
   ```
2. Move to the service-user layout as above (from `old=` on), then sections 3 and 4 with the new release: pin the key, download and check, `install`, `wx start`. The first start turns the copied files into one bundle list.
3. `wx regenerate` must exit 0.
4. Only then remove the old program files: `sudo rm -rf /srv/workout-explorer/bin /srv/workout-explorer/web /srv/workout-explorer/bundles /srv/workout-explorer/clean-assets.sh`. A notification setup moves to the drop-in of section 8 (the old `Network=` line in the unit is gone with the new unit).

## 10. Logs, stop, remove

Container output goes to the journal; neither the app nor Caddy logs visitor addresses. How long the journal keeps it is its own setting, for example 14 days in `/etc/systemd/journald.conf.d/50-retention.conf`:

```ini
[Journal]
MaxRetentionSec=14day
MaxFileSec=1day
```

then `sudo systemctl restart systemd-journald`. This also covers the host proxy's error messages, which can hold visitor addresses.

Remove everything:

```sh
sudo systemctl --user -M workouts@ disable --now workout-explorer-update.timer   # if automatic updates were installed
wx stop
sudo -u workouts sh -c 'rm -rf ~/.config/containers/systemd/workout-explorer-* ~/.config/systemd/user/workout-explorer-update.*'
wx daemon-reload
wx podman network rm workout-explorer_edge workout-explorer_backend workout-explorer_egress
sudo rm -rf /srv/workout-explorer /srv/workouts  # deploy files and all workout data, irreversibly
```

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| Units do not exist after `daemon-reload` | Quadlet rejected a unit: `sudo -u workouts /usr/lib/systemd/user-generators/podman-user-generator --dryrun 2>&1` (path may differ by distribution) |
| `wx` says "never as root" | You ran the script with plain `sudo`; use `wx` (section 2), which runs it as `workouts` |
| `update:` line says `reason=config` or `reason=signature` | As DOCKER.md section 13; also: the deploy directory must belong to `workouts` (section 3) |
| `app` cannot write `/srv/workouts`, or Caddy cannot read `public/`/`site/` | Owned by `10001` instead of the mapped UID: `sudo chown -R $huid:$hgid /srv/workouts`, mode `0750` on the folders |
| `app` exits: cannot read the env file | `.env` must be group-readable by the service user: `root:workouts 0640` |
| `install` says the repository's copy | Use a release's deploy archive; the repository's units name no image |
| Every visitor shares one upload limit | `WX_TRUSTED_PROXIES` does not match the source Caddy sees; section 6 |
| Units are gone after a reboot or logout | Linger is off: `sudo loginctl enable-linger workouts` |
