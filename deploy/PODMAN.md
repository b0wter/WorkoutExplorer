# Running Workout Explorer with rootless Podman and systemd (Quadlet)

This is an alternative to the Docker Compose setup in [RUNBOOK.md](RUNBOOK.md). It runs the same two containers (`app` and `caddy`) with the same settings, but as **rootless Podman containers managed by systemd** through [Quadlet](https://docs.podman.io/en/latest/markdown/podman-systemd.unit.5.html) units. It uses a public release (RUNBOOK section 6a), not a local build.

Tested with Podman 5.4.2 (netavark, pasta) on Debian 13 and Workout Explorer v0.4.0. Everything in RUNBOOK.md that is not about Docker still applies: configuration keys (section 4), the operator file and Impressum (section 3), the reverse proxy (section 5), data and privacy (section 9). This file only covers what is different.

## Why not podman-compose?

`compose.yaml` and `compose.build.yaml` use features podman-compose (1.3) does not understand (the `!override` tag, the `local` log driver). Quadlet units are also a better fit for Podman: systemd starts, stops and restarts the containers, at boot too, without a daemon.

## How Compose maps to Quadlet

| `compose.yaml` | Here |
| --- | --- |
| `services.app`, `services.caddy` | `workout-explorer-app.container`, `workout-explorer-caddy.container` |
| `networks.edge`, `networks.backend` | `workout-explorer-edge.network`, `workout-explorer-backend.network` (`Internal=true`) |
| `compose.notify.yaml` (`networks.egress`) | `workout-explorer-egress.network`, optional |
| `mem_limit` / `memswap_limit` | `PodmanArgs=--memory=1200m --memory-swap=1200m` (Quadlet 5.4 has no `Memory=` key) |
| `tmpfs` with `uid=`/`gid=` | `Tmpfs=...,U` (Podman's tmpfs has no `uid`/`gid` options; `U` gives it to the container user) |
| `read_only`, `cap_drop: [ALL]`, `no-new-privileges` | `ReadOnly=true`, `DropCapability=all`, `NoNewPrivileges=true` |
| `restart: unless-stopped` | `Restart=always` in `[Service]`; `systemctl stop` keeps it stopped |
| `ports: 127.0.0.1:8088:8088` | `PublishPort=127.0.0.1:8080:8088` (pick any free local port) |
| log driver with rotating files | journald (Podman's default here); see "Logs" |
| `docker compose ...` | `systemctl --user ...` as the service user, wrapped by `workout-explorer-ctl` below |

### Two differences that matter

1. **`WX_TRUSTED_PROXIES` is Caddy's own edge address, not the gateway.** With rootless port forwarding (pasta/rootlessport), requests from the host proxy reach Caddy with the source address of Caddy's *own* interface on `edge`, not `10.250.0.1` as with Docker. So Caddy's edge address is pinned to `10.250.0.2` and `WX_TRUSTED_PROXIES=10.250.0.2`. With the Docker value, Caddy ignores `X-Forwarded-For` and every visitor shares one upload limit (RUNBOOK section 12). Check it after setup (see "Check the result").
2. **Container UIDs map to other host UIDs.** Rootless Podman maps container UID `N` to host UID `subuid_start + N - 1` of the service user. Container user `10001` is therefore host UID `subuid_start + 10000` (for example `175536` when `/etc/subuid` says `workouts:165536:65536`). The data directory must belong to *that* UID, not to `10001`.

## 1. Prerequisites

- Podman 5 with netavark and pasta (`podman info` shows `networkBackend: netavark`), `curl`, `sqlite3` (for `reports`).
- cgroup v2 with the memory controller delegated to the service user, or the memory limits in the units are silently not applied (Debian 13 default). Check after step 2: `cat /sys/fs/cgroup/user.slice/user-$(id -u workouts).slice/user@$(id -u workouts).service/cgroup.controllers` must list `memory`. Without it the OOM check in section 9 means nothing.
- A reverse proxy on the host for TLS (RUNBOOK section 5). The example below uses Caddy.

## 2. Service user

A dedicated user without login runs the containers. It needs a subordinate UID/GID range (Debian's `useradd` adds one for normal users) and *linger*, so its systemd instance runs without a login session and starts at boot.

```sh
sudo useradd --create-home --home-dir /var/lib/workouts --shell /usr/sbin/nologin \
  --comment "Workout Explorer (rootless podman)" workouts
grep workouts /etc/subuid /etc/subgid          # must show a range of 65536, e.g. workouts:165536:65536
sudo loginctl enable-linger workouts
id -u workouts                                 # the UID; used for the Quadlet folder in step 5
```

Host UID/GID of container user `10001`, used below:

```sh
huid=$(( $(awk -F: '$1=="workouts"{print $2}' /etc/subuid) + 10000 ))
hgid=$(( $(awk -F: '$1=="workouts"{print $2}' /etc/subgid) + 10000 ))
```

## 3. Folders and release

Same layout as `deploy.sh` produces (RUNBOOK sections 6a and 13). Program files belong to root and are mounted read-only; only the data directory is writable, by the mapped `10001`.

```sh
sudo install -d -m 0755 /srv/workout-explorer /srv/workout-explorer/bin /srv/workout-explorer/conf \
  /srv/workout-explorer/web /srv/workout-explorer/operator
sudo install -d -o $huid -g $hgid -m 0750 /srv/workouts /srv/workouts/public /srv/workouts/site
```

Get a release (in a clone of this repository, for `deploy/`):

```sh
v=v0.4.0
base=https://github.com/b0wter/WorkoutExplorer/releases/download/$v
mkdir -p /tmp/wx && cd /tmp/wx
curl -fLO $base/workout-explorer-$v-linux-amd64.tar.gz -O $base/SHA256SUMS
sha256sum -c SHA256SUMS                         # must print: ...tar.gz: OK
tar -xzf workout-explorer-$v-linux-amd64.tar.gz workout-explorer web
sh <clone>/deploy/fetch-caddy.sh ./caddy        # the pinned Caddy, checksum-verified

sudo install -m 0755 workout-explorer caddy /srv/workout-explorer/bin/
sudo install -m 0644 <clone>/deploy/Caddyfile /srv/workout-explorer/conf/Caddyfile
sudo cp -r --no-preserve=ownership,mode web/. /srv/workout-explorer/web/
sudo chmod -R u=rwX,go=rX /srv/workout-explorer/web
# bundle lists for later clean-ups (RUNBOOK section 8)
sudo install -d -m 0755 /srv/workout-explorer/bundles
(cd web/assets && ls -A) | sudo tee /srv/workout-explorer/bundles/$v >/dev/null
sudo install -m 0644 <clone>/deploy/clean-assets.sh /srv/workout-explorer/clean-assets.sh
```

## 4. Configuration

Write `/srv/workout-explorer/.env` from `deploy/example.env` as in RUNBOOK section 4, with these differences:

- Only the `WORKOUT_EXPLORER_*` keys are read (the app unit loads the file with `EnvironmentFile=`). `WX_*` and `COMPOSE_FILE` have no effect; the paths are fixed in the units.
- The file is read by the service user's Podman, so it must be readable by its group, not `0600`:

```sh
sudo chown root:workouts /srv/workout-explorer/.env
sudo chmod 0640 /srv/workout-explorer/.env
```

Do not put `WORKOUT_EXPLORER_TRUSTED_PROXY` in it; the app unit sets it.

## 5. The units

Quadlet units for one user go to `/etc/containers/systemd/users/<uid>/` (root-owned, so the service user cannot change them). Replace `<uid>` with `id -u workouts`. Alternatively they can live in `~workouts/.config/containers/systemd/`.

`/etc/containers/systemd/users/<uid>/workout-explorer-edge.network`:

```ini
# Translated from deploy/compose.yaml (networks.edge): fixed subnet so the addresses are known.
[Unit]
Description=Workout Explorer edge network

[Network]
NetworkName=workout-explorer_edge
Subnet=10.250.0.0/29
Gateway=10.250.0.1
```

`/etc/containers/systemd/users/<uid>/workout-explorer-backend.network`:

```ini
# Translated from deploy/compose.yaml (networks.backend): no route out.
[Unit]
Description=Workout Explorer backend network

[Network]
NetworkName=workout-explorer_backend
Subnet=10.250.1.0/29
Internal=true
```

`/etc/containers/systemd/users/<uid>/workout-explorer-app.container`:

```ini
# Translated from deploy/compose.yaml (services.app).
[Unit]
Description=Workout Explorer API server

[Container]
ContainerName=workout-explorer-app
Image=gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
Pull=missing
User=10001
Group=10001
Entrypoint=/srv/workout-explorer/bin/workout-explorer
Exec=serve -addr :8080 -data /srv/workouts -web /srv/workout-explorer/web
EnvironmentFile=/srv/workout-explorer/.env
# Caddy's address on backend: the one proxy the server believes
Environment=WORKOUT_EXPLORER_TRUSTED_PROXY=10.250.1.2
ReadOnly=true
DropCapability=all
NoNewPrivileges=true
# mem_limit = memswap_limit = 1200m
PodmanArgs=--memory=1200m --memory-swap=1200m
StopTimeout=30
Volume=/srv/workout-explorer/bin:/srv/workout-explorer/bin:ro
Volume=/srv/workout-explorer/web:/srv/workout-explorer/web:ro
Volume=/srv/workouts:/srv/workouts
# About page operator text and Impressum (public), read-only
Volume=/srv/workout-explorer/operator:/srv/workout-explorer/operator:ro
Network=workout-explorer-backend.network:ip=10.250.1.3
NetworkAlias=app

[Service]
Restart=always
TimeoutStartSec=300

[Install]
WantedBy=default.target
```

`/etc/containers/systemd/users/<uid>/workout-explorer-caddy.container`:

```ini
# Translated from deploy/compose.yaml (services.caddy). Plain HTTP on 127.0.0.1:8080 only;
# the host's reverse proxy terminates TLS and forwards here.
[Unit]
Description=Workout Explorer Caddy (static files + API proxy)
Wants=workout-explorer-app.service
After=workout-explorer-app.service

[Container]
ContainerName=workout-explorer-caddy
Image=gcr.io/distroless/static-debian13:nonroot@sha256:25ac8f0c3b2334a9eb05452266bd4697864a8c8ef0ef71e5e6480d7f92868329
Pull=missing
User=10002
Group=10002
GroupAdd=10001
Entrypoint=/srv/workout-explorer/bin/caddy
Exec=run --config /srv/workout-explorer/conf/Caddyfile --adapter caddyfile
Environment=WX_DATA=/srv/workouts
Environment=WX_WEB=/srv/workout-explorer/web
Environment=WX_API=app:8080
Environment=WX_SITE=http://:8088
# Rootless port forwarding shows host clients as Caddy's own edge address (not the gateway)
Environment=WX_TRUSTED_PROXIES=10.250.0.2
Environment=WX_ADMIN=localhost:2019
Environment=XDG_CONFIG_HOME=/caddy-state/config
Environment=XDG_DATA_HOME=/caddy-state/data
Environment=HOME=/caddy-state
# U = owned by the container user (Podman form of uid=10002,gid=10002)
Tmpfs=/caddy-state:rw,size=16m,mode=0700,U
ReadOnly=true
DropCapability=all
NoNewPrivileges=true
StopTimeout=30
Volume=/srv/workout-explorer/bin:/srv/workout-explorer/bin:ro
Volume=/srv/workout-explorer/conf:/srv/workout-explorer/conf:ro
Volume=/srv/workout-explorer/web:/srv/workout-explorer/web:ro
Volume=/srv/workouts/public:/srv/workouts/public:ro
# Page shells the server fills at start (v0.4.0)
Volume=/srv/workouts/site:/srv/workouts/site:ro
PublishPort=127.0.0.1:8080:8088
# Fixed, because WX_TRUSTED_PROXIES names it
Network=workout-explorer-edge.network:ip=10.250.0.2
Network=workout-explorer-backend.network:ip=10.250.1.2

[Service]
Restart=always
TimeoutStartSec=300

[Install]
WantedBy=default.target
```

Install the control script (next section), then start:

```sh
sudo workout-explorer-ctl daemon-reload
sudo workout-explorer-ctl start
curl -s http://127.0.0.1:8080/api/health        # {"status":"ok","version":"v0.4.0"}
```

The first start pulls the base image. `WantedBy=default.target` starts both units at every boot (linger is on).

## 6. Control script

`systemctl --user` must run as the service user with its `XDG_RUNTIME_DIR`. This wrapper does that and covers the RUNBOOK's `docker compose` commands. Save it as `/usr/local/sbin/workout-explorer-ctl` (root, `0755`):

```bash
#!/bin/bash
# Manage Workout Explorer (rootless Podman user units of user "workouts").
#   sudo workout-explorer-ctl status|start|stop|restart
#   sudo workout-explorer-ctl restart-app             (only the app; published pages stay online)
#   sudo workout-explorer-ctl reload-caddy            (graceful Caddyfile reload)
#   sudo workout-explorer-ctl daemon-reload           (after editing a unit)
#   sudo workout-explorer-ctl logs [-f] [-n 100]
#   sudo workout-explorer-ctl podman ps
#   sudo workout-explorer-ctl reports                 (visitor reports, read-only)
#   sudo workout-explorer-ctl delete [-y] <public_id> (delete one workout; app must be running)
#   sudo workout-explorer-ctl regenerate              (rebuild all workouts with the running version)
set -eu
u=workouts; uid=$(id -u "$u"); units=(workout-explorer-app.service workout-explorer-caddy.service)
[ "$(id -u)" = 0 ] || { echo "run with sudo" >&2; exit 1; }
cd /
data=/srv/workouts
# container UID/GID 10001 as seen on the host (rootless: subuid/subgid start + 10000)
huid=$(( $(awk -F: -v u="$u" '$1==u{print $2}' /etc/subuid) + 10000 ))
hgid=$(( $(awk -F: -v u="$u" '$1==u{print $2}' /etc/subgid) + 10000 ))
as_user() { runuser -u "$u" -- env XDG_RUNTIME_DIR=/run/user/$uid "$@"; }
cmd=${1:-status}; [ $# -gt 0 ] && shift
case "$cmd" in
  status)                 as_user systemctl --user --no-pager status "${units[@]}" "$@" ;;
  start|stop|restart)     as_user systemctl --user "$cmd" "${units[@]}" "$@" ;;
  restart-app)            as_user systemctl --user restart "${units[0]}" ;;
  reload-caddy)           as_user podman exec workout-explorer-caddy /srv/workout-explorer/bin/caddy reload --config /srv/workout-explorer/conf/Caddyfile --adapter caddyfile ;;
  daemon-reload)          as_user systemctl --user daemon-reload ;;
  logs)                   exec journalctl --no-pager -o cat _SYSTEMD_USER_UNIT=${units[0]} + _SYSTEMD_USER_UNIT=${units[1]} "$@" ;;
  podman)                 as_user podman "$@" ;;
  reports)
    exec setpriv --reuid=$huid --regid=$hgid --clear-groups sqlite3 -readonly -line "$data/workouts.db" "
      SELECT r.id AS report, datetime(r.created_at, 'unixepoch') AS filed_utc, r.reason,
             w.public_id, w.visibility AS state, datetime(w.expires_at, 'unixepoch') AS expires_utc, w.title,
             quote(replace(r.message, char(10), '\\n')) AS message
      FROM reports r JOIN workouts w ON w.id = r.workout_id
      ORDER BY r.id;" ;;
  regenerate)
    as_user podman exec workout-explorer-app /srv/workout-explorer/bin/workout-explorer regenerate -data "$data" -web /srv/workout-explorer/web "$@" ;;
  delete)
    tty=(); [ -t 0 ] && tty=(-t)
    as_user podman exec -i "${tty[@]}" workout-explorer-app /srv/workout-explorer/bin/workout-explorer delete -data "$data" "$@" ;;
  *) echo "usage: $0 status|start|stop|restart|restart-app|reload-caddy|daemon-reload|logs|podman|reports|delete|regenerate ..." >&2; exit 2 ;;
esac
```

`reports` reads the database as the mapped UID (`setpriv`), read-only, like RUNBOOK section 10 does as `10001`.

| RUNBOOK (Docker) | Here |
| --- | --- |
| `docker compose up -d` / `stop` / `restart app` | `sudo workout-explorer-ctl start` / `stop` / `restart` |
| `docker compose restart app` (published pages stay online) | `sudo workout-explorer-ctl restart-app` |
| `docker compose exec caddy ... reload ...` | `sudo workout-explorer-ctl reload-caddy` |
| `docker compose logs app` | `sudo workout-explorer-ctl logs -n 100` |
| `docker compose exec app ... delete ...` | `sudo workout-explorer-ctl delete <public_id>` |
| `docker compose exec app ... regenerate ...` | `sudo workout-explorer-ctl regenerate` |
| `docker inspect ...` | `sudo workout-explorer-ctl podman inspect workout-explorer-app` |

## 7. Host reverse proxy

Forward to the published port as in RUNBOOK section 5. With Caddy on the host (`/etc/caddy/Caddyfile`):

```caddyfile
workouts.example.org {
	# No header/encode directives: the project's Caddy (the inner Caddyfile) sets CSP, HSTS etc., not the app.
	# No log directive: no access log with visitor addresses.
	reverse_proxy 127.0.0.1:8080 {
		transport http {
			response_header_timeout 8m
		}
	}
}
```

Caddy's `reverse_proxy` sets `X-Forwarded-For` with the client as the last entry, as the RUNBOOK requires.

## 8. Check the result

- `curl -s https://workouts.example.org/api/health` shows the version.
- `/`, `/about` and `/robots.txt` return 200; `sudo ls -l /srv/workouts/site` lists the page shells the server wrote at start.
- `ss -ltn`: the project listens only on `127.0.0.1:8080`.
- **Trusted proxy:** upload once from two different networks (e.g. phone on mobile data and your home connection) and check that the per-address limit is counted separately, or temporarily set `WORKOUT_EXPLORER_UPLOADS_PER_HOUR_IPV4=1` and check that a second address can still upload. If everyone shares one limit, the source address Caddy sees is not `10.250.0.2`: check with `sudo workout-explorer-ctl podman inspect workout-explorer-caddy` and adjust the unit.

## 9. Updating

Follow RUNBOOK section 8; `deploy.sh` cannot be used (it drives Docker over SSH), so do its steps by hand, as root:

1. Read the new RUNBOOK and `deploy/` diff for the release. Unit changes are needed when `compose.yaml` changes in more than comments (example: v0.4.0 added the `site/` folder and its Caddy mount).
2. Verify and unpack the release as in step 3.
3. Bundle lists: `bundles/pre-f30` once (the files already in `web/assets`, if `bundles/` did not exist yet), then `bundles/<tag>` with the new release's `web/assets` list. Install the new `clean-assets.sh`.
4. Replace the binary with **install + rename**, not `cp` over it: the running file is busy (`Text file busy`).
   ```sh
   install -m 755 workout-explorer /srv/workout-explorer/bin/.workout-explorer.new
   mv /srv/workout-explorer/bin/.workout-explorer.new /srv/workout-explorer/bin/workout-explorer
   ```
   Same for `caddy` if `caddy.env` changed.

   Install the release's `conf/Caddyfile` whenever it differs from the installed one (`cmp <clone>/deploy/Caddyfile /srv/workout-explorer/conf/Caddyfile`), after validating it with the new caddy binary, as `deploy.sh` does: `./caddy validate --config <clone>/deploy/Caddyfile --adapter caddyfile`. v0.5.0 changed it (new CSP script hash); a stale Caddyfile blocks the new pages' head script.
5. Copy `web/` **additively** (`cp -r web/. /srv/workout-explorer/web/`), never delete it first: published pages still use the old assets until `regenerate`.
6. `sudo workout-explorer-ctl daemon-reload` (if units changed), then `sudo workout-explorer-ctl restart-app`. If only the Caddyfile changed, then `sudo workout-explorer-ctl reload-caddy`; if the caddy binary or a unit changed, `sudo workout-explorer-ctl restart` instead (brief page outage, as with `deploy.sh`). Wait for `/api/health` to show the new tag.
7. `sudo workout-explorer-ctl regenerate > /tmp/regen.txt`. **Only if it exits 0**: remove old assets with
   ```sh
   cd /srv/workout-explorer && sed -n 's/^asset=//p' /tmp/regen.txt | sh clean-assets.sh <tag>
   ```
   A clean-up after a failed `regenerate` removes assets that published pages still need.
8. OOM check: `sudo workout-explorer-ctl podman inspect -f '{{.State.OOMKilled}}' workout-explorer-app` is `false`, and `NRestarts` of the app unit did not grow.

## 10. Notifications (optional)

The Quadlet form of `compose.notify.yaml`. Set both Pushover keys in `.env` (RUNBOOK section 4a), then add an egress network that only `app` joins:

`/etc/containers/systemd/users/<uid>/workout-explorer-egress.network`:

```ini
# Translated from deploy/compose.notify.yaml (networks.egress): not internal, app is its only member.
[Unit]
Description=Workout Explorer egress network (notifications)

[Network]
NetworkName=workout-explorer_egress
Subnet=10.250.2.0/29
```

and in `workout-explorer-app.container`, below the backend `Network=` line:

```ini
Network=workout-explorer-egress.network
```

Then `sudo workout-explorer-ctl daemon-reload && sudo workout-explorer-ctl restart`. The startup line in `sudo workout-explorer-ctl logs | grep 'server started'` shows `"notifications":true`. To turn it off, remove the line and the keys and restart; as with Docker, the app can then reach no host at all.

## 11. Logs

Podman writes container output to the journal (`podman info` shows `logDriver: journald`), not to rotating files as in RUNBOOK section 11. The app's and Caddy's lines contain no visitor addresses either way. How long they are kept is the journal's setting, for example 14 days:

```ini
# /etc/systemd/journald.conf.d/50-retention.conf
[Journal]
MaxRetentionSec=14day
MaxFileSec=1day
```

`sudo systemctl restart systemd-journald` applies it. This also covers the host reverse proxy's error messages, which can contain visitor addresses.

## 12. Stop, remove, delete all data

```sh
sudo workout-explorer-ctl stop
sudo rm /etc/containers/systemd/users/<uid>/workout-explorer-*
sudo workout-explorer-ctl daemon-reload
sudo workout-explorer-ctl podman network rm workout-explorer_edge workout-explorer_backend workout-explorer_egress
sudo rm -rf /srv/workouts                       # all workout data, irreversibly
```

## Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| Units do not exist after `daemon-reload` | Quadlet rejected a unit. See `sudo -u workouts /usr/lib/systemd/user-generators/podman-user-generator --dryrun 2>&1` (path may differ by distribution) |
| Quadlet rejects a `Memory=` key | Quadlet 5.4 has no `Memory=`; use `PodmanArgs=--memory=... --memory-swap=...` |
| tmpfs: `invalid mount option uid=...` | Use the `U` option instead of `uid=`/`gid=` |
| `app` cannot write `/srv/workouts`, or Caddy cannot read `public/`/`site/` | Owned by `10001` instead of the mapped UID. `sudo chown -R $huid:$hgid /srv/workouts` and mode `0750` on the folders |
| `app` exits: cannot read the env file | `.env` must be group-readable by the service user: `root:workouts 0640` |
| Every visitor shares one upload limit | `WX_TRUSTED_PROXIES` does not match the source address Caddy sees; see section 8 |
| `cp: Text file busy` during an update | Install the binary under a new name and rename it (section 9, step 4) |
| Units are gone after a reboot / logout | Linger is off: `sudo loginctl enable-linger workouts` |
