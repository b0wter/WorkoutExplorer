# Running Workout Explorer with rootless Podman and systemd (Quadlet)

This is one of the three install paths (RUNBOOK.md, "Which way should I run it?"). It runs the same two containers as the Docker Compose path, our published app image and the official Caddy image, with the same settings, but as **rootless Podman containers managed by systemd** through [Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) units. The units and a control script are files of the release's deploy archive (`podman/`); this guide explains them.

Everything in RUNBOOK.md that is not about Docker still applies: the data directory and the operator file and Impressum (section 3), configuration keys (section 4), the reverse proxy (section 5), data and privacy (section 9). This file covers what is different. Written for Podman 5.4 (netavark, pasta) on Debian 13.

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
| `local` log driver | journald (section 9) |
| `docker compose ...` | `sudo workout-explorer-ctl ...` (section 5) |

Two differences that matter:

1. **`WX_TRUSTED_PROXIES` is Caddy's own edge address, not the gateway.** With rootless port forwarding, requests from the host proxy reach Caddy from the address of Caddy's *own* interface on `edge`. So the Caddy unit pins that address to `10.250.0.2` and trusts it. Check it after setup (section 6).
2. **Container UIDs map to other host UIDs.** Rootless Podman maps container UID `N` to host UID `subuid_start + N - 1` of the service user. The app's `10001` is host UID `subuid_start + 10000` (for example `175536` when `/etc/subuid` says `workouts:165536:65536`). The data directory belongs to *that* UID.

## 1. Prerequisites

- linux/amd64, Podman 5 with netavark and pasta (`podman info` shows `networkBackend: netavark`), `curl`, `sqlite3` (reports), `tar`, `sha256sum`.
- cgroup v2 with the memory controller delegated to the service user (Debian 13 default), or the memory cap is silently not applied. Check after step 2: `cat /sys/fs/cgroup/user.slice/user-$(id -u workouts).slice/user@$(id -u workouts).service/cgroup.controllers` lists `memory`.
- A reverse proxy on the host for TLS (RUNBOOK section 5; Caddy example in section 7 below).

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

## 3. Folders, deploy files, configuration

Program files belong to root; only the data directory is writable, by the mapped `10001`. There is no `bin/` or `web/` any more: both are in the image.

```sh
sudo install -d -m 0755 /srv/workout-explorer /srv/workout-explorer/conf /srv/workout-explorer/operator
sudo install -d -o $huid -g $hgid -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

Download and check the release's deploy archive (take the newest `vX.Y.Z` from the Releases page; never use the repository's `deploy/` files, they name no image):

```sh
v=v1.4.0
base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
mkdir -p /tmp/wx-$v && cd /tmp/wx-$v
curl -fLO $base/workout-explorer-$v-deploy.tar.gz -O $base/SHA256SUMS
sha256sum -c --ignore-missing SHA256SUMS        # must print: ...deploy.tar.gz: OK
mkdir d && tar -xzf workout-explorer-$v-deploy.tar.gz -C d
sudo install -m 0755 d/podman/workout-explorer-ctl /usr/local/sbin/workout-explorer-ctl
```

Write `/srv/workout-explorer/.env` from `d/example.env` as in RUNBOOK section 4. Only the `WORKOUT_EXPLORER_*` keys are read (the app unit loads it); `WX_*` and `COMPOSE_FILE` have no effect. Do not set `WORKOUT_EXPLORER_TRUSTED_PROXY`; the app unit sets it. The service user's Podman reads the file, so:

```sh
sudo chown root:workouts /srv/workout-explorer/.env && sudo chmod 0640 /srv/workout-explorer/.env
```

## 4. Install and start

```sh
sudo workout-explorer-ctl install /tmp/wx-$v/d   # pulls both images, validates the Caddyfile, installs units, Caddyfile and itself
sudo workout-explorer-ctl start
curl -s http://127.0.0.1:8080/api/health        # {"status":"ok","version":"v1.4.0"}
```

`install` puts the units into `/etc/containers/systemd/users/<uid>/` (root-owned, so the service user cannot change them) and the Caddyfile into `/srv/workout-explorer/conf/`. Nothing changes if a pull fails or the new Caddyfile is invalid. `WantedBy=default.target` starts both units at every boot.

## 5. Control script

`workout-explorer-ctl` runs `systemctl --user` and `podman` as the service user. Its header lists every command.

| RUNBOOK (Docker) | Here |
| --- | --- |
| `docker compose up -d` / `stop` / `restart` | `sudo workout-explorer-ctl start` / `stop` / `restart` |
| `docker compose restart app` | `sudo workout-explorer-ctl restart-app` (published pages stay online) |
| `docker compose exec caddy caddy reload ...` | `sudo workout-explorer-ctl reload-caddy` |
| `docker compose pull` | `sudo workout-explorer-ctl pull` |
| `docker compose logs app` | `sudo workout-explorer-ctl logs -n 100` |
| `docker compose exec app ... delete ...` | `sudo workout-explorer-ctl delete <public_id>` |
| `docker compose exec app ... regenerate ...` | `sudo workout-explorer-ctl regenerate` |
| the `sqlite3` report query | `sudo workout-explorer-ctl reports` (read-only, as the mapped UID) |
| `docker inspect ...` | `sudo workout-explorer-ctl podman inspect workout-explorer-app` |

`stop` keeps both stopped until the next boot. Quadlet units cannot be disabled; to keep the site off across reboots, remove the units (section 10).

## 6. Check the result

- `curl -s https://workouts.example.org/api/health` shows the version; `/`, `/about` and `/robots.txt` return 200.
- `ss -ltn`: the project listens only on `127.0.0.1:8080`.
- Hardening: `sudo workout-explorer-ctl podman exec workout-explorer-app grep Cap /proc/1/status` and the same for `workout-explorer-caddy` show `CapPrm` and `CapEff` all zero.
- **Trusted proxy:** set `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4=1` for a moment, upload from two networks (for example phone on mobile data and home): both must work once. If everyone shares one limit, the source address Caddy sees is not `10.250.0.2` (`sudo workout-explorer-ctl podman inspect workout-explorer-caddy`).

## 7. Host reverse proxy

As RUNBOOK section 5, forwarding to `127.0.0.1:8080`. With Caddy on the host (`/etc/caddy/Caddyfile`):

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

Set both Pushover keys in `.env` (RUNBOOK section 4a), then give the app alone the egress network with a drop-in, which `install` never overwrites:

```sh
d=/etc/containers/systemd/users/$(id -u workouts)/workout-explorer-app.container.d
sudo install -d -m 0755 $d
printf '[Container]\nNetwork=workout-explorer-egress.network\n' | sudo tee $d/notify.conf
sudo workout-explorer-ctl daemon-reload && sudo workout-explorer-ctl restart
```

`sudo workout-explorer-ctl logs | grep 'server started'` shows `"notifications":true`. To turn it off, remove `notify.conf` and the keys, `daemon-reload`, `restart`; the app then reaches no host at all.

## 9. Updating

Read the release notes and RUNBOOK's changes first. Then, as root:

1. Download and check the new deploy archive as in section 3 (new `v`).
2. `sudo workout-explorer-ctl install /tmp/wx-$v/d`. It prints `Caddyfile changed` when it did.
3. `sudo workout-explorer-ctl restart` (both: the app on its new image, Caddy on the newest patch of its minor; brief page outage). Wait until `/api/health` shows the new tag.
4. `sudo workout-explorer-ctl regenerate`: rebuilds every workout, then removes old web assets no page needs. It ends with `rebuilt= deleted= kept= skipped=` and `files_removed= bundles_removed=`. Exit 1: run it again; exit 4: the rebuild finished, the clean-up did not (run it again later). There is no separate clean-up step.
5. OOM check: `sudo workout-explorer-ctl podman inspect -f '{{.State.OOMKilled}}' workout-explorer-app` is `false`, and `NRestarts` of the app unit did not grow.

Between releases, `sudo workout-explorer-ctl pull && sudo workout-explorer-ctl restart` brings Caddy's newest patch.

### Migrating from the tarball setup (bind-mounted binaries)

Before F-59 this guide ran both containers on the distroless base with `bin/` and `web/` mounted from the host. The data directory stays in place with its owner, so every workout, link, expiry and report survives:

1. `sudo workout-explorer-ctl stop`, then copy the old web assets into the data directory as the mapped UID, so pages published under the old release keep their scripts and styles until `regenerate` rebuilds them:
   ```sh
   sudo install -d -o $huid -g $hgid -m 0750 /srv/workouts/site/assets
   sudo setpriv --reuid=$huid --regid=$hgid --clear-groups cp -rn /srv/workout-explorer/web/assets/. /srv/workouts/site/assets/
   ```
2. Sections 3 and 4 with the new release: download, `install` (replaces the old units and this script), `start`. The first start turns the copied files into one bundle list.
3. `sudo workout-explorer-ctl regenerate` must exit 0.
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
sudo workout-explorer-ctl stop
sudo rm -r /etc/containers/systemd/users/$(id -u workouts)/workout-explorer-*
sudo workout-explorer-ctl daemon-reload
sudo workout-explorer-ctl podman network rm workout-explorer_edge workout-explorer_backend workout-explorer_egress
sudo rm -rf /srv/workouts                       # all workout data, irreversibly
```

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| Units do not exist after `daemon-reload` | Quadlet rejected a unit: `sudo -u workouts /usr/lib/systemd/user-generators/podman-user-generator --dryrun 2>&1` (path may differ by distribution) |
| `app` cannot write `/srv/workouts`, or Caddy cannot read `public/`/`site/` | Owned by `10001` instead of the mapped UID: `sudo chown -R $huid:$hgid /srv/workouts`, mode `0750` on the folders |
| `app` exits: cannot read the env file | `.env` must be group-readable by the service user: `root:workouts 0640` |
| `install` says the repository's copy | Use a release's deploy archive; the repository's units name no image |
| Every visitor shares one upload limit | `WX_TRUSTED_PROXIES` does not match the source Caddy sees; section 6 |
| Units are gone after a reboot or logout | Linger is off: `sudo loginctl enable-linger workouts` |
