#!/usr/bin/env python3
# Test harness for deploy/update.sh (F-52, critical lane): runs the real script against a local
# fake release host (HTTPS, self-signed certificate trusted through CURL_CA_BUNDLE), a fake health
# endpoint and a fake Pushover endpoint, with a throwaway minisign key made at test time. The
# release's deploy.sh is a test double that records its run. Uses the distribution's minisign when
# installed, else deploy/test/fake-minisign (Python `cryptography`); fails, never skips, without
# either. `make update-check` runs it; WX_TEST_SHELL picks the shell (default sh).
#   python3 -I deploy/test/update_test.py [-v] [Test.test_name]
import base64, hashlib, http.server, io, json, os, shutil, ssl, subprocess, sys, tarfile, tempfile, threading, time, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
UPDATE = os.environ.get("WX_TEST_UPDATE") or os.path.join(HERE, "..", "update.sh")
SHELL = os.environ.get("WX_TEST_SHELL", "sh").split()
REAL_MINISIGN = shutil.which("minisign")
CURL = shutil.which("curl")


class Host:
    """The fake release host (HTTPS) plus Pushover, and the health endpoint (HTTP)."""

    def __init__(self, tmp):
        self.files, self.latest, self.requests, self.pushover = {}, None, [], []
        self.health_version, self.http_redirect, self.latest_status = None, False, 302
        host = self

        class TLS(http.server.BaseHTTPRequestHandler):
            def log_message(self, *a):
                pass

            def send(self, code, body=b"", headers=()):
                self.send_response(code)
                for k, v in headers:
                    self.send_header(k, v)
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def do_GET(self):
                host.requests.append(self.path)
                p = self.path
                if p == "/releases/latest":
                    if host.latest is None or host.latest_status != 302:
                        return self.send(host.latest_status if host.latest is not None else 404)
                    return self.send(302, headers=[("Location", "https://127.0.0.1:%d/releases/tag/%s" % (host.port, host.latest))])
                if p.startswith("/releases/download/"):
                    if host.http_redirect:
                        return self.send(302, headers=[("Location", "http://127.0.0.1:%d%s" % (host.hport, p.replace("/releases/download/", "/objects/")))])
                    return self.send(302, headers=[("Location", "/objects/" + p[len("/releases/download/"):])])
                if p.startswith("/objects/"):
                    b = host.files.get(p[len("/objects/"):])
                    return self.send(200, b) if b is not None else self.send(404)
                self.send(404)

            def do_POST(self):
                body = self.rfile.read(int(self.headers.get("Content-Length", 0)))
                host.requests.append(self.path)
                if self.path == "/pushover":
                    host.pushover.append(body.decode())
                    return self.send(200, b'{"status":1}')
                self.send(404)

        class Health(TLS): # plain HTTP: the health endpoint, and the same releases (so only --proto refuses them)
            def do_GET(self):
                if self.path != "/api/health":
                    return super().do_GET()
                if host.health_version:
                    return self.send(200, json.dumps({"status": "ok", "version": host.health_version}, separators=(",", ":")).encode())
                self.send(503)

        cert, key = os.path.join(tmp, "cert.pem"), os.path.join(tmp, "key.pem")
        subprocess.run(["openssl", "req", "-x509", "-newkey", "ec", "-pkeyopt", "ec_paramgen_curve:prime256v1", "-nodes",
                        "-keyout", key, "-out", cert, "-days", "1", "-subj", "/CN=localhost",
                        "-addext", "subjectAltName=IP:127.0.0.1"], check=True, capture_output=True)
        self.cert = cert
        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        ctx.load_cert_chain(cert, key)
        self.tls = http.server.ThreadingHTTPServer(("127.0.0.1", 0), TLS)
        self.tls.socket = ctx.wrap_socket(self.tls.socket, server_side=True)
        self.port = self.tls.server_address[1]
        self.health = http.server.ThreadingHTTPServer(("127.0.0.1", 0), Health)
        self.hport = self.health.server_address[1]
        for s in (self.tls, self.health):
            threading.Thread(target=s.serve_forever, daemon=True).start()

    def reset(self):
        self.files, self.latest, self.requests, self.pushover = {}, None, [], []
        self.health_version, self.http_redirect, self.latest_status = None, False, 302

    def release_requests(self):
        return [r for r in self.requests if r.startswith(("/releases", "/objects"))]


def minisign(*args, check=True):
    exe = REAL_MINISIGN or os.path.join(HERE, "fake-minisign")
    return subprocess.run([exe, *args], check=check, capture_output=True, text=True)


def keypair(d, name):
    pub, sec = os.path.join(d, name + ".pub"), os.path.join(d, name + ".key")
    minisign("-G", "-W", "-p", pub, "-s", sec)
    return pub, sec, open(pub).read().split("\n")[1]


def tgz(members):
    """members: list of (name, kind, data); kind file|dir|symlink|hardlink."""
    buf = io.BytesIO()
    with tarfile.open(fileobj=buf, mode="w:gz", format=tarfile.GNU_FORMAT) as t:
        for name, kind, data in members:
            ti = tarfile.TarInfo(name)
            ti.mtime = 0
            if kind == "dir":
                ti.type, ti.mode = tarfile.DIRTYPE, 0o755
                t.addfile(ti)
            elif kind == "symlink":
                ti.type, ti.linkname = tarfile.SYMTYPE, data
                t.addfile(ti)
            elif kind == "hardlink":
                ti.type, ti.linkname = tarfile.LNKTYPE, data
                t.addfile(ti)
            else:
                ti.size, ti.mode = len(data), 0o644
                t.addfile(ti, io.BytesIO(data))
    return buf.getvalue()


class Test(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.base = tempfile.mkdtemp(prefix="wx-update-test-")
        cls.host = Host(cls.base)
        cls.pub, cls.sec, cls.key = keypair(cls.base, "release")
        cls.opub, cls.osec, _ = keypair(cls.base, "other")
        shim = os.path.join(cls.base, "bin")
        os.mkdir(shim)
        with open(os.path.join(shim, "curl"), "w") as f: # records every curl command line (secrets must not appear)
            f.write('#!/bin/sh\nprintf "%%s\\n" "$*" >>"$WX_TEST_ARGV"\nexec %s "$@"\n' % CURL)
        os.chmod(os.path.join(shim, "curl"), 0o755)
        if not REAL_MINISIGN:
            os.symlink(os.path.join(HERE, "fake-minisign"), os.path.join(shim, "minisign"))
        cls.shim = shim

    @classmethod
    def tearDownClass(cls):
        cls.host.tls.shutdown()
        cls.host.health.shutdown()
        shutil.rmtree(cls.base, ignore_errors=True)

    def setUp(self):
        self.host.reset()
        self.d = tempfile.mkdtemp(dir=self.base)
        self.srv = os.path.join(self.d, "srv")
        self.home = os.path.join(self.d, "home")
        os.makedirs(self.srv)
        os.makedirs(self.home)
        open(os.path.join(self.srv, "compose.yaml"), "w").close() # the Compose path
        self.env = {"WORKOUT_EXPLORER_AUTO_UPDATE": "on", "WX_UPDATE_PUBLIC_KEY": self.key, "WX_LISTEN": "127.0.0.1:%d" % self.host.hport}
        self.write_env()
        self.host.health_version = "v1.4.0"

    def write_env(self):
        with open(os.path.join(self.srv, ".env"), "w") as f:
            f.write("# test\nWORKOUT_EXPLORER_BASE_URL=https://workouts.example.org\n")
            for k, v in self.env.items():
                f.write("%s=%s\n" % (k, v))

    # --- releases -------------------------------------------------------------------------
    def release(self, tag, rc=0, sec=None, sign=True, sleep=0, members=None, files=None):
        """Publishes a release (signed unless sign=False); its deploy.sh records the run, sleeps, exits rc."""
        deploy = ("#!/bin/sh\necho \"deploy: fake $1\"\n[ \"$WX_DEPLOY_FROM_UPDATE\" = 1 ] || exit 99\n"
                  "printf '%%s %%s\\n' \"$1\" \"$DEPLOY_DIR\" >>%s/deploys\nsleep %d\nexit %d\n" % (self.d, sleep, rc)).encode()
        if members is None:
            members = [("./", "dir", b""), ("./deploy.sh", "file", deploy), ("./update.sh", "file", open(UPDATE, "rb").read()),
                       ("./COMMIT", "file", b"0" * 40 + b"\n"), ("./podman/", "dir", b""), ("./podman/workout-explorer-ctl", "file", b"#!/bin/sh\n")]
        files = files or {
            "workout-explorer-%s-deploy.tar.gz" % tag: tgz(members),
            "workout-explorer-%s-linux-amd64.tar.gz" % tag: tgz([("workout-explorer", "file", b"bin")]),
        }
        sums = "".join("%s  %s\n" % (hashlib.sha256(b).hexdigest(), n) for n, b in sorted(files.items())).encode()
        out = {n: b for n, b in files.items()}
        out["SHA256SUMS"] = sums
        if sign:
            p = os.path.join(self.d, "SHA256SUMS")
            open(p, "wb").write(sums)
            minisign("-S", "-s", sec or self.sec, "-m", p, "-x", p + ".minisig")
            out["SHA256SUMS.minisig"] = open(p + ".minisig", "rb").read()
        for n, b in out.items():
            self.host.files["%s/%s" % (tag, n)] = b
        return out

    def deploys(self):
        p = os.path.join(self.d, "deploys")
        return [l.split()[0] for l in open(p)] if os.path.exists(p) else []

    # --- running update.sh ----------------------------------------------------------------
    def run_update(self, *args, env=None, wait=True):
        e = {"PATH": self.shim + ":" + os.environ["PATH"], "HOME": self.home, "DEPLOY_DIR": self.srv, "LANG": "C",
             "CURL_CA_BUNDLE": self.host.cert, "WX_TEST_ARGV": os.path.join(self.d, "argv"),
             "WX_UPDATE_RELEASES": "https://127.0.0.1:%d/releases" % self.host.port,
             "WX_UPDATE_PUSHOVER": "https://127.0.0.1:%d/pushover" % self.host.port}
        e.update(env or {})
        p = subprocess.Popen([*SHELL, UPDATE, *args], env=e, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        if not wait:
            return p
        out, err = p.communicate(timeout=60)
        self.last = (p.returncode, out, err)
        return p.returncode, out, err

    def logline(self, out):
        lines = [l for l in out.splitlines() if l.startswith("update: running=")]
        self.assertEqual(len(lines), 1, out)
        self.assertEqual(out.splitlines()[-1], lines[0], "the log line comes last")
        return dict(kv.split("=", 1) for kv in lines[0][len("update: "):].split())

    def expect(self, args=(), action=None, reason=None, rc=0, **kw):
        code, out, err = self.run_update(*args, **kw)
        line = self.logline(out)
        msg = "stdout=%r stderr=%r" % (out, err)
        self.assertEqual((line["action"], line["reason"], line["exit"]), (action, reason, str(rc)), msg)
        self.assertEqual(code, rc, msg)
        return line, out, err

    def state(self):
        p = os.path.join(self.srv, "update.stopped")
        return open(p).read() if os.path.exists(p) else None

    def assert_stopped(self, action, tag, exit_):
        s = self.state()
        self.assertIsNotNone(s, "stopped state not set")
        self.assertRegex(s, r"^time=\d{4}-\d\d-\d\dT\d\d:\d\d:\d\dZ tag=%s action=%s exit=%s reason=\w+\n$" % (tag.replace(".", r"\."), action, exit_))

    def assert_refused(self, reason, tag="v1.5.0"):
        self.expect(action="refused", reason=reason, rc=1)
        self.assertEqual(self.deploys(), [], "deploy.sh ran")
        self.assert_stopped("refused", tag, 1)
        self.assertEqual(len(self.host.pushover), 1)
        self.assertIn("refused%%20%%28%s%%29" % reason, self.host.pushover[0])

    def pushover_on(self):
        self.env["WORKOUT_EXPLORER_PUSHOVER_TOKEN"] = "a" * 30
        self.env["WORKOUT_EXPLORER_PUSHOVER_USER"] = "u" * 30
        self.write_env()

    # --- AC: opt-in, switch, configuration ---------------------------------------------------
    def test_switch_off_makes_no_request(self):
        for v in ("", None):
            self.host.reset()
            self.host.health_version = "v1.4.0"
            self.env.pop("WORKOUT_EXPLORER_AUTO_UPDATE", None)
            if v is not None:
                self.env["WORKOUT_EXPLORER_AUTO_UPDATE"] = v
            self.write_env()
            self.release("v1.5.0")
            self.host.latest = "v1.5.0"
            self.expect(action="none", reason="off")
            self.assertEqual(self.host.requests, [])
            self.assertEqual(self.deploys(), [])

    def test_config_errors_make_no_request(self):
        self.release("v1.5.0")
        self.host.latest = "v1.5.0"
        cases = [("WORKOUT_EXPLORER_AUTO_UPDATE", "yes"), ("WORKOUT_EXPLORER_AUTO_UPDATE", "ON"), ("WX_UPDATE_PUBLIC_KEY", ""),
                 ("WX_UPDATE_PUBLIC_KEY", self.key[:-1]), ("WX_UPDATE_PUBLIC_KEY", "RW" + "!" * 54), ("WX_UPDATE_PUBLIC_KEY", self.key + "A")]
        for k, v in cases:
            with self.subTest(k=k, v=v):
                saved = dict(self.env)
                self.env[k] = v
                self.write_env()
                self.expect(action="none", reason="config", rc=2)
                self.env = saved
                self.write_env()
        os.makedirs(os.path.join(self.home, ".config/containers/systemd"))
        open(os.path.join(self.home, ".config/containers/systemd/workout-explorer-app.container"), "w").close()
        self.expect(action="none", reason="config", rc=2) # both paths
        os.remove(os.path.join(self.srv, "compose.yaml"))
        os.remove(os.path.join(self.home, ".config/containers/systemd/workout-explorer-app.container"))
        self.expect(action="none", reason="config", rc=2) # neither
        self.expect(args=("v1.5",), action="none", reason="config", rc=2)
        self.expect(args=("v1.5.0", "x"), action="none", reason="config", rc=2)
        self.assertEqual(self.host.requests, [])

    def test_podman_path_uses_its_health_address(self):
        os.remove(os.path.join(self.srv, "compose.yaml"))
        os.makedirs(os.path.join(self.home, ".config/containers/systemd"))
        open(os.path.join(self.home, ".config/containers/systemd/workout-explorer-app.container"), "w").close()
        self.release("v1.5.0")
        self.host.latest = "v1.5.0"
        # 127.0.0.1:8080 is not the fake health server: health unreachable, no release request.
        self.expect(action="none", reason="health")
        self.assertEqual(self.host.requests, [])

    # --- AC: which version ------------------------------------------------------------------
    def test_applies_patch_minor_major(self):
        for new in ("v1.4.2", "v1.5.0", "v2.0.0"):
            with self.subTest(new=new):
                self.host.reset()
                self.host.health_version = "v1.4.0"
                self.release(new)
                self.host.latest = new
                line, out, _ = self.expect(action="applied", reason="-")
                self.assertEqual((line["running"], line["latest"]), ("v1.4.0", new))
                self.assertEqual(self.deploys()[-1], new)
                self.assertIn("deploy: fake " + new, out)
                self.assertLess(out.index("deploy: fake"), out.index("update: running="))
                self.assertEqual(self.host.pushover, [])
                self.assertIsNone(self.state())

    def test_several_newer_installs_latest_directly(self):
        for t in ("v1.4.1", "v1.5.0", "v2.0.0"):
            self.release(t)
        self.host.latest = "v2.0.0"
        self.expect(action="applied", reason="-")
        self.assertEqual(self.deploys(), ["v2.0.0"])

    def test_numeric_comparison(self):
        self.host.health_version = "v1.9.0"
        self.release("v1.10.0")
        self.host.latest = "v1.10.0"
        self.expect(action="applied", reason="-")
        self.host.health_version = "v1.10.0"
        self.host.latest = "v1.9.9"
        self.expect(action="none", reason="current")

    def test_never_applies_older_same_prerelease_or_other_tags(self):
        for t in ("v1.3.9", "v1.4.0", "v0.9.99", "v1.5.0-rc1", "v1.5", "1.5.0", "v1.5.0.1", "V1.5.0", "v1.5.0%0a", "v1234567890.0.0"):
            with self.subTest(latest=t):
                self.host.reset()
                self.host.health_version = "v1.4.0"
                self.release(t) if "%" not in t else None
                self.host.latest = t
                self.expect(action="none", reason="current")
                self.assertEqual(self.host.release_requests(), ["/releases/latest"])
                self.assertEqual(self.deploys(), [])

    def test_health_not_a_release_version(self):
        for v in ("unknown", "v1.4", None):
            with self.subTest(running=v):
                self.host.reset()
                self.host.health_version = v
                self.release("v1.5.0")
                self.host.latest = "v1.5.0"
                self.expect(action="none", reason="health")
                self.assertEqual(self.host.requests, [])

    # --- AC: refusals (signature, names, checksum, archive) ----------------------------------
    def test_refuses_without_signature(self):
        self.pushover_on()
        self.release("v1.5.0", sign=False)
        self.host.latest = "v1.5.0"
        self.assert_refused("signature")

    def test_refuses_checksums_without_valid_signature(self):
        self.pushover_on()
        r = self.release("v1.5.0")
        self.host.files["v1.5.0/SHA256SUMS.minisig"] = b"untrusted comment: x\nnot a signature\n"
        self.host.latest = "v1.5.0"
        self.assert_refused("signature")

    def test_refuses_signature_by_another_key(self):
        self.pushover_on()
        self.release("v1.5.0", sec=self.osec)
        self.host.latest = "v1.5.0"
        self.assert_refused("signature")

    def test_refuses_manifest_changed_after_signing(self):
        self.pushover_on()
        r = self.release("v1.5.0")
        self.host.files["v1.5.0/SHA256SUMS"] = r["SHA256SUMS"] + b"0" * 64 + b"  workout-explorer-v1.5.0-extra\n"
        self.host.latest = "v1.5.0"
        self.assert_refused("signature")

    def test_refuses_file_not_matching_the_signed_manifest(self):
        self.pushover_on()
        self.release("v1.5.0")
        evil = tgz([("./", "dir", b""), ("./deploy.sh", "file", b"#!/bin/sh\ntouch %s/pwned\n" % self.d.encode())])
        self.host.files["v1.5.0/workout-explorer-v1.5.0-deploy.tar.gz"] = evil
        self.host.latest = "v1.5.0"
        self.assert_refused("checksum")
        self.assertFalse(os.path.exists(os.path.join(self.d, "pwned")))

    def test_refuses_replayed_old_signature(self):
        self.pushover_on()
        old = self.release("v1.4.2")
        new = self.release("v1.5.0")
        # The old valid manifest and signature served under the new tag: names carry the old tag.
        for n in ("SHA256SUMS", "SHA256SUMS.minisig"):
            self.host.files["v1.5.0/" + n] = old[n]
        self.host.files["v1.5.0/workout-explorer-v1.4.2-deploy.tar.gz"] = old["workout-explorer-v1.4.2-deploy.tar.gz"]
        self.host.latest = "v1.5.0"
        self.assert_refused("manifest")
        # The new manifest with the old signature: the signature does not match.
        self.host.reset()
        self.host.health_version = "v1.4.0"
        os.remove(os.path.join(self.srv, "update.stopped"))
        new["SHA256SUMS.minisig"] = old["SHA256SUMS.minisig"]
        for n, b in new.items():
            self.host.files["v1.5.0/" + n] = b
        self.host.latest = "v1.5.0"
        self.assert_refused("signature")

    def test_refuses_manifest_naming_another_tag(self):
        self.pushover_on()
        files = {"workout-explorer-v1.5.0-deploy.tar.gz": tgz([("./", "dir", b""), ("./deploy.sh", "file", b"#!/bin/sh\nexit 0\n")]),
                 "workout-explorer-v1.5.0-linux-amd64.tar.gz": b"bin", "workout-explorer-v1.4.2-linux-amd64.tar.gz": b"old"}
        self.release("v1.5.0", files=files)
        self.host.latest = "v1.5.0"
        self.assert_refused("manifest")

    def test_refuses_manifest_without_both_archives(self):
        self.pushover_on()
        members = [("./", "dir", b""), ("./deploy.sh", "file", b"#!/bin/sh\nexit 0\n")]
        self.release("v1.5.0", files={"workout-explorer-v1.5.0-deploy.tar.gz": tgz(members)})
        self.host.latest = "v1.5.0"
        self.assert_refused("manifest")

    def test_refuses_unsafe_archive_members(self):
        good = [("./", "dir", b""), ("./deploy.sh", "file", b"#!/bin/sh\ntouch %s/pwned\n" % self.d.encode())]
        for name, extra in {
            "symlink": ("./compose.yaml", "symlink", "/etc/passwd"),
            "hardlink": ("./h", "hardlink", "./deploy.sh"),
            "dotdot": ("./../evil", "file", b"x"),
            "absolute": ("/tmp/evil-wx-test", "file", b"x"),
            "odd name": ("./a b", "file", b"x"),
        }.items():
            with self.subTest(member=name):
                self.host.reset()
                self.host.health_version = "v1.4.0"
                s = os.path.join(self.srv, "update.stopped")
                os.path.exists(s) and os.remove(s)
                self.release("v1.5.0", members=good + [extra])
                self.host.latest = "v1.5.0"
                self.expect(action="refused", reason="archive", rc=1)
                self.assert_stopped("refused", "v1.5.0", 1)
                self.assertFalse(os.path.exists(os.path.join(self.d, "pwned")))
                self.assertFalse(os.path.exists("/tmp/evil-wx-test"))

    def test_refuses_archive_without_deploy_sh(self):
        self.release("v1.5.0", members=[("./", "dir", b""), ("./x", "file", b"x")])
        self.host.latest = "v1.5.0"
        self.expect(action="refused", reason="archive", rc=1)

    # --- AC: failures, stopped state, manual runs ----------------------------------------------
    def test_failed_deploy_alerts_once_and_stops(self):
        self.pushover_on()
        for rc in (1, 3, 4):
            with self.subTest(rc=rc):
                self.host.reset()
                self.host.health_version = "v1.4.0"
                s = os.path.join(self.srv, "update.stopped")
                os.path.exists(s) and os.remove(s)
                self.release("v1.5.0", rc=rc)
                self.host.latest = "v1.5.0"
                self.expect(action="failed", reason="deploy", rc=rc)
                self.assert_stopped("failed", "v1.5.0", rc)
                self.assertEqual(len(self.host.pushover), 1)
                body = self.host.pushover[0]
                self.assertIn("title=Workout%20Explorer", body)
                self.assertIn("message=v1.5.0%20failed%20%28exit%20" + str(rc) + "%29.%20Automatic%20updates%20are%20stopped", body)
                self.assertIn("token=" + "a" * 30, body)
                # While stopped: no request, no alert, not even for a newer valid patch.
                self.release("v1.5.1")
                self.host.latest = "v1.5.1"
                n = len(self.host.requests)
                self.expect(action="none", reason="stopped")
                self.assertEqual(len(self.host.requests), n)
                self.assertEqual(len(self.host.pushover), 1)
        argv = open(os.path.join(self.d, "argv")).read()
        self.assertNotIn("a" * 30, argv, "a Pushover secret on a curl command line")
        self.assertNotIn("u" * 30, argv)

    def test_failure_without_pushover_logs_only(self):
        self.release("v1.5.0", rc=3)
        self.host.latest = "v1.5.0"
        self.expect(action="failed", reason="deploy", rc=3)
        self.assertEqual(self.host.pushover, [])
        self.assertEqual([r for r in self.host.requests if r == "/pushover"], [])

    def test_manual_success_clears_stopped_state(self):
        self.release("v1.5.0", rc=1)
        self.host.latest = "v1.5.0"
        self.expect(action="failed", reason="deploy", rc=1)
        self.assertIsNotNone(self.state())
        self.release("v1.5.1")
        self.expect(args=("v1.5.1",), action="applied", reason="-")
        self.assertIsNone(self.state())
        self.host.health_version = "v1.5.1"
        self.release("v1.5.2")
        self.host.latest = "v1.5.2"
        self.expect(action="applied", reason="-")
        self.assertEqual(self.deploys(), ["v1.5.0", "v1.5.1", "v1.5.2"])

    def test_manual_runs_ignore_switch_and_version(self):
        self.env["WORKOUT_EXPLORER_AUTO_UPDATE"] = ""
        self.write_env()
        self.release("v1.3.0")
        self.expect(args=("v1.3.0",), action="applied", reason="-") # older allowed by hand
        self.host.health_version = None # health is not asked by manual runs
        self.release("v1.4.0")
        self.expect(args=("v1.4.0",), action="applied", reason="-")
        self.assertNotIn("/releases/latest", self.host.requests)

    def test_manual_failure_with_switch_off_sets_state_quietly(self):
        self.pushover_on()
        self.env["WORKOUT_EXPLORER_AUTO_UPDATE"] = ""
        self.write_env()
        self.release("v1.5.0", rc=3)
        _, _, err = self.expect(args=("v1.5.0",), action="failed", reason="deploy", rc=3)
        self.assert_stopped("failed", "v1.5.0", 3)
        self.assertEqual(self.host.pushover, [])
        self.assertNotIn("automatic updates are now stopped", err)

    def test_manual_failure_with_switch_on_says_so_and_alerts(self):
        self.pushover_on()
        self.release("v1.5.0", sign=False)
        _, _, err = self.expect(args=("v1.5.0",), action="refused", reason="signature", rc=1)
        self.assertIn("automatic updates are now stopped", err)
        self.assertEqual(len(self.host.pushover), 1)

    def test_manual_refusal_runs_nothing(self):
        self.release("v1.5.0", sec=self.osec)
        self.expect(args=("v1.5.0",), action="refused", reason="signature", rc=1)
        self.assertEqual(self.deploys(), [])

    # --- AC: network ---------------------------------------------------------------------------
    def test_network_errors_change_nothing(self):
        self.pushover_on()
        self.release("v1.5.0")
        self.host.latest = "v1.5.0"
        self.expect(action="none", reason="network", env={"WX_UPDATE_RELEASES": "https://127.0.0.1:1/releases"})
        self.expect(action="none", reason="network", env={"WX_UPDATE_RELEASES": "http://127.0.0.1:%d/releases" % self.host.hport})
        self.expect(action="none", reason="network", env={"CURL_CA_BUNDLE": "/dev/null"}) # certificate not trusted
        self.host.latest_status = 500
        self.expect(action="none", reason="network")
        self.host.latest_status = 302
        self.host.http_redirect = True # download redirected to plain HTTP
        self.expect(action="none", reason="network")
        self.host.http_redirect = False
        del self.host.files["v1.5.0/SHA256SUMS"] # manifest missing: not a signature refusal
        self.expect(action="none", reason="network")
        self.expect(args=("v1.5.0",), action="none", reason="network", rc=1)
        self.release("v1.5.0")
        del self.host.files["v1.5.0/workout-explorer-v1.5.0-deploy.tar.gz"]
        self.expect(action="none", reason="network")
        self.assertIsNone(self.state())
        self.assertEqual(self.host.pushover, [])
        self.assertEqual(self.deploys(), [])

    # --- AC: one at a time -------------------------------------------------------------------
    def test_lock_busy(self):
        lock = os.path.join(self.srv, "deploy.lock")
        holder = subprocess.Popen(["flock", lock, "sleep", "5"], start_new_session=True)
        try:
            time.sleep(0.3)
            self.release("v1.5.0")
            self.host.latest = "v1.5.0"
            self.expect(action="none", reason="busy")
            self.expect(args=("v1.5.0",), action="none", reason="busy", rc=1)
            self.assertEqual(self.host.requests, [])
        finally:
            os.killpg(holder.pid, 9)
            holder.wait()
        self.expect(action="applied", reason="-")

    def test_manual_and_scheduled_never_overlap(self):
        self.release("v1.5.0", sleep=2)
        self.host.latest = "v1.5.0"
        manual = self.run_update("v1.5.0", wait=False)
        time.sleep(0.8)
        self.expect(action="none", reason="busy")
        out, _ = manual.communicate(timeout=30)
        self.assertEqual(self.logline(out)["action"], "applied")
        self.assertEqual(self.deploys(), ["v1.5.0"])
        # The lock went with the process: the next run proceeds.
        self.host.health_version = "v1.4.0"
        self.expect(action="applied", reason="-")


if __name__ == "__main__":
    if os.getuid() == 0:
        sys.exit("update_test: run as a normal user (update.sh refuses root)")
    for tool in ("openssl", "flock", "curl"):
        if not shutil.which(tool):
            sys.exit("update_test: %s is not installed" % tool)
    if not REAL_MINISIGN:
        try:
            import cryptography # noqa: F401
        except ImportError:
            sys.exit("update_test: neither minisign nor the Python cryptography package is installed")
    print("update_test: shell %s, %s" % (SHELL, "minisign " + REAL_MINISIGN if REAL_MINISIGN else "fake-minisign (no minisign installed)"), file=sys.stderr)
    unittest.main()
