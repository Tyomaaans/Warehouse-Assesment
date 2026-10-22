#!/usr/bin/env python3
"""Exercise deployment activation and rollback without Docker or SSH access."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest


ROOT = Path(__file__).resolve().parents[1]


class DeploymentTest(unittest.TestCase):
    def setUp(self):
        work = ROOT / ".work"
        work.mkdir(exist_ok=True)
        self.fixture = tempfile.TemporaryDirectory(prefix="deploy-test-", dir=work)
        self.addCleanup(self.fixture.cleanup)
        self.root = Path(self.fixture.name)
        self.old = self.make_release("a")
        self.new = self.make_release("b")
        self.stale = self.make_release("c")
        (self.root / "current").symlink_to(self.old)
        (self.root / ".env").write_text("APP_ENV=production\n")
        self.bin = self.root / "bin"
        self.bin.mkdir()
        self.log = self.root / "commands.jsonl"
        fake = self.bin / "docker"
        fake.write_text("""#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
a = sys.argv[1:]
with open(os.environ['DEPLOY_TEST_LOG'], 'a') as stream:
    stream.write(json.dumps(a) + '\\n')
mode = os.environ.get('DEPLOY_TEST_FAILURE', '')
if a[0] == 'info': print('x86_64')
if 'pull' in a and mode == 'pull': sys.exit(1)
if a[:2] == ['buildx', 'create'] and mode == 'builder': sys.exit(1)
if a[0] == 'compose' and '-f' in a:
    config = a[a.index('-f') + 1]
    is_new = '/bbbbbbbb' in config
    if 'build' in a and mode == 'build': sys.exit(1)
    if 'run' in a and a[-1] == 'migrate' and mode == 'migration': sys.exit(1)
    if 'up' in a and 'api' in a and is_new and mode == 'health': sys.exit(1)
    if 'pg_dump' in a: print('fixture-database-dump')
    if 'port' in a: print('127.0.0.1:18080')
""")
        fake.chmod(0o755)
        for name, body in {
            "curl": "#!/bin/sh\nexit 0\n",
            "df": "#!/bin/sh\nprintf 'Avail\\n8589934592\\n'\n",
        }.items():
            path = self.bin / name
            path.write_text(body)
            path.chmod(0o755)

    def make_release(self, letter):
        path = self.root / "releases" / (letter * 40 + "-1-1")
        path.mkdir(parents=True)
        (path / "compose.production.yml").write_text("services: {}\n")
        (path / "images.env").write_text(
            "".join(f"{key}=warehouse-{name}:{path.name}\n"
                    for key, name in [("API_IMAGE", "api"), ("FRONTEND_IMAGE", "frontend")])
        )
        return path

    def deploy(self, failure=""):
        original_environment = (self.root / ".env").read_bytes()
        environment = os.environ.copy()
        environment.update({
            "PATH": str(self.bin) + os.pathsep + environment["PATH"],
            "DEPLOY_TEST_LOG": str(self.log),
            "DEPLOY_TEST_FAILURE": failure,
        })
        result = subprocess.run(
            ["bash", str(ROOT / "scripts/deploy.sh"), str(self.root), self.new.name],
            text=True, capture_output=True,
            env=environment, timeout=15,
        )
        self.assertFalse(list(self.root.glob(".buildx.*")))
        self.assertEqual((self.root / ".env").read_bytes(), original_environment)
        self.assertFalse((self.new / ".env").exists())
        self.assertFalse((self.new / "backend/.env").exists())
        commands = [json.loads(line) for line in self.log.read_text().splitlines()]
        for command in commands:
            if command[0] == "compose" and "-f" in command:
                self.assertIn(str(self.root / ".env"), command)
        self.assertFalse(any("login" in command or "down" in command or "prune" in command for command in commands))
        return result, commands

    def test_success_activates_after_checks_and_keeps_previous_release(self):
        result, commands = self.deploy()
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertEqual((self.root / "current").resolve(), self.new)
        self.assertTrue(self.old.exists())
        self.assertTrue(self.stale.exists())
        self.assertTrue(list((self.root / "backups").glob("*.dump")))
        builds = [command for command in commands if command[0] == "compose" and "build" in command]
        self.assertEqual([command[-1] for command in builds], ["api", "frontend"])
        first_stop = next(i for i, command in enumerate(commands) if "stop" in command)
        self.assertTrue(all(commands.index(command) < first_stop for command in builds))
        create = next(command for command in commands if command[:2] == ["buildx", "create"])
        self.assertIn("memory=384m,memory-swap=1g,cpu-period=100000,cpu-quota=100000", create)
        self.assertTrue(any(command[:2] == ["buildx", "rm"] for command in commands))

    def test_pull_failure_keeps_running_release_untouched(self):
        result, commands = self.deploy("pull")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root / "current").resolve(), self.old)
        self.assertFalse(any("stop" in command for command in commands))

    def test_build_failure_keeps_running_release_and_removes_builder(self):
        result, commands = self.deploy("build")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root / "current").resolve(), self.old)
        self.assertFalse(any("stop" in command for command in commands))
        self.assertTrue(any(command[:2] == ["buildx", "rm"] for command in commands))

    def test_builder_failure_keeps_running_release(self):
        result, commands = self.deploy("builder")
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root / "current").resolve(), self.old)
        self.assertFalse(any("stop" in command for command in commands))

    def test_migration_failure_restarts_previous_application(self):
        self.assert_rollback("migration")

    def test_health_failure_restarts_previous_application(self):
        self.assert_rollback("health")

    def assert_rollback(self, failure):
        result, commands = self.deploy(failure)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual((self.root / "current").resolve(), self.old)
        self.assertTrue(any("up" in command and str(self.old / "compose.production.yml") in command
                            and "api" in command for command in commands))

    def test_first_deployment_failure_keeps_database_and_stops_application(self):
        (self.root / "current").unlink()
        result, commands = self.deploy("health")
        self.assertNotEqual(result.returncode, 0)
        self.assertFalse((self.root / "current").exists())
        stopped = [command for command in commands if "stop" in command]
        self.assertTrue(any("notification-consumer" in command and "api" in command for command in stopped))
        self.assertFalse(any("postgres" in command or "redis" in command for command in stopped))


if __name__ == "__main__":
    unittest.main(verbosity=2)
