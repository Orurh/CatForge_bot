import copy
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import shutil
import tempfile
import unittest
from unittest.mock import patch

spec = importlib.util.spec_from_file_location('update', Path(__file__).with_name('update.py'))
update = importlib.util.module_from_spec(spec)
spec.loader.exec_module(update)
REVISION = 'a' * 40
PREVIOUS = 'b' * 40


class FakeHost(update.Updater):
    """Emulate the external Git/Docker boundary; deployment files are real."""
    def __init__(self, root):
        super().__init__(root)
        self.calls = []
        self.changed_checkout = False
        self.dirty = False
        self.fail = None
        self.image_revision = REVISION
        self.before = {'services': {'bot': {'environment': {'DATABASE_URL': 'postgres://test'}},
                                    'game-engine': {}, 'postgres': {'image': 'postgres:16'}}}
        self.after = copy.deepcopy(self.before)

    def run(self, *args, **kwargs):
        self.calls.append(args)
        if self.fail and self.fail(args):
            raise update.DeploymentError('injected failure')
        if args[:3] == ('git', 'status', '--porcelain'):
            return ' M file' if self.dirty else ''
        if args[:2] == ('git', 'rev-parse'):
            return {'HEAD': PREVIOUS, '--abbrev-ref': 'main', 'FETCH_HEAD^{commit}': REVISION}[args[2]]
        if args[:2] == ('git', 'checkout'):
            self.changed_checkout = '--detach' in args
        if args[-3:] == ('config', '--format', 'json'):
            return json.dumps(self.after if self.changed_checkout else self.before)
        if 'ps' in args:
            return 'container-' + args[-1]
        if args[:2] == ('docker', 'info'):
            return 'x86_64'
        if args[:3] == ('docker', 'image', 'inspect'):
            repository = args[-1].rsplit(':', 1)[0]
            return json.dumps({'Os': 'linux', 'Architecture': 'amd64',
                               'RepoDigests': [repository + '@sha256:' + 'c' * 64],
                               'Config': {'Labels': {'org.opencontainers.image.revision': self.image_revision}}})
        if args[:2] == ('docker', 'inspect'):
            return 'sha256:' + 'd' * 64
        return ''

    def backup(self, compose, destination):
        self.calls.append(('backup',))
        if self.fail and self.fail(('backup',)):
            raise update.DeploymentError('backup failed')
        destination.write_bytes(b'checked-backup')


class UpdateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        (root / '.deploy').mkdir()
        (root / '.deploy/host.json').write_text(json.dumps({
            'image_prefix': 'ghcr.io/example/catforge-bot', 'project': 'catforge-bot',
            'compose_files': ['docker-compose.dev.yml', 'docker-compose.monitoring.yml']}))
        self.host = FakeHost(root)
        for stream in ('stdout', 'stderr'):
            output = patch.object(update.sys, stream, io.StringIO())
            output.start()
            self.addCleanup(output.stop)

    def deploy(self):
        self.host.deploy('v0.1.0-beta.1')

    def test_success_stops_before_backup_and_migrations_and_pins_both_images(self):
        self.deploy()
        calls = self.host.calls
        stop = next(i for i, c in enumerate(calls) if 'stop' in c)
        backup = calls.index(('backup',))
        engine = next(i for i, c in enumerate(calls) if 'up' in c and c[-1] == 'game-engine')
        bot = next(i for i, c in enumerate(calls) if 'up' in c and c[-1] == 'bot')
        self.assertLess(stop, backup)
        self.assertLess(backup, engine)
        self.assertLess(engine, bot)
        current = json.loads((self.host.state / 'current.json').read_text())
        self.assertEqual(set(current['services']), {'bot', 'game-engine'})
        self.assertTrue(all('@sha256:' in s['image'] for s in current['services'].values()))
        self.assertFalse((self.host.state / 'pending.json').exists())
        for c in calls:
            if 'up' in c:
                self.assertIn('--no-build', c)
                self.assertIn('--no-deps', c)
                self.assertNotIn('postgres', c)
            if c[:2] == ('docker', 'compose'):
                self.assertIn('catforge-bot', c)
                self.assertTrue(any(s.endswith('docker-compose.monitoring.yml') for s in c))

    def test_missing_second_image_does_not_stop_bot_or_change_checkout(self):
        self.host.fail = lambda c: c[:2] == ('docker', 'pull') and '/engine:' in c[-1]
        with self.assertRaises(update.DeploymentError):
            self.deploy()
        self.assertFalse(self.host.changed_checkout)
        self.assertFalse(any('stop' in c for c in self.host.calls))

    def test_wrong_image_revision_does_not_stop_bot(self):
        self.host.image_revision = PREVIOUS
        with self.assertRaisesRegex(update.DeploymentError, 'revision'):
            self.deploy()
        self.assertFalse(any('stop' in c for c in self.host.calls))

    def test_backup_failure_restores_old_checkout_and_starts_existing_bot(self):
        self.host.fail = lambda c: c == ('backup',)
        with self.assertRaisesRegex(update.DeploymentError, 'backup failed'):
            self.deploy()
        self.assertFalse(self.host.changed_checkout)
        self.assertTrue(any('start' in c for c in self.host.calls))
        self.assertFalse(any('up' in c for c in self.host.calls))
        self.assertFalse((self.host.state / 'pending.json').exists())

    def test_failed_readiness_stops_new_bot_and_blocks_retry_without_schema_rollback(self):
        self.host.fail = lambda c: 'up' in c and c[-1] == 'bot'
        with self.assertRaises(update.DeploymentError):
            self.deploy()
        self.assertTrue(self.host.changed_checkout)
        pending = json.loads((self.host.state / 'pending.json').read_text())
        self.assertEqual(pending['phase'], 'activating')
        self.assertTrue(Path(pending['backup']).is_file())
        self.assertTrue((Path(pending['history']) / 'previous-images.json').is_file())
        self.assertEqual(sum('stop' in c for c in self.host.calls), 2)
        self.assertFalse(any('start' in c for c in self.host.calls))
        self.assertFalse((self.host.state / 'current.json').exists())
        with self.assertRaisesRegex(update.DeploymentError, 'Unfinished'):
            self.deploy()

    def test_dirty_checkout_is_rejected_before_fetch(self):
        self.host.dirty = True
        with self.assertRaisesRegex(update.DeploymentError, 'local changes'):
            self.deploy()
        self.assertFalse(any('fetch' in c for c in self.host.calls))

    def test_database_change_is_rejected_before_downtime(self):
        self.host.after['services']['bot']['environment']['DATABASE_URL'] = 'postgres://other'
        with self.assertRaisesRegex(update.DeploymentError, 'DATABASE_URL'):
            self.deploy()
        self.assertFalse(self.host.changed_checkout)
        self.assertFalse(any('stop' in c for c in self.host.calls))

    def test_postgres_upgrade_requires_separate_host_change(self):
        self.host.after['services']['postgres']['image'] = 'postgres:17'
        with self.assertRaisesRegex(update.DeploymentError, 'host services'):
            self.deploy()
        self.assertFalse(any('stop' in c for c in self.host.calls))

    def test_arbitrary_ref_is_rejected(self):
        for tag in ('main', '--upload-pack=bad', 'v1;echo bad', '../v1'):
            with self.subTest(tag=tag), self.assertRaises(update.DeploymentError):
                self.host.deploy(tag)
        self.assertEqual(self.host.calls, [])

    def test_main_sha_fetches_only_main(self):
        self.host.deploy(REVISION, main_sha=True)
        self.assertIn(('git', 'fetch', '--no-tags', 'origin', 'refs/heads/main'), self.host.calls)

    def test_host_local_overlay_is_preserved_without_tracking_in_git(self):
        self.host.files.append('.deploy/proxy.yml')
        (self.host.state / 'proxy.yml').write_text('services: {}\n')
        self.host.deploy(REVISION, main_sha=True)
        tracked = next(c for c in self.host.calls if c[:2] == ('git', 'ls-files'))
        self.assertNotIn('.deploy/proxy.yml', tracked)
        self.assertTrue((self.host.state / 'proxy.yml').exists())

    def test_missing_host_overlay_fails_before_downtime(self):
        self.host.files.append('.deploy/missing.yml')
        with self.assertRaisesRegex(update.DeploymentError, 'Missing host-local'):
            self.host.deploy(REVISION, main_sha=True)
        self.assertFalse(any('stop' in c for c in self.host.calls))

    def test_main_moving_since_selection_does_not_stop_bot(self):
        with self.assertRaisesRegex(update.DeploymentError, 'main moved'):
            self.host.deploy(PREVIOUS, main_sha=True)
        self.assertFalse(any('stop' in c or 'pull' in c for c in self.host.calls))

    def test_main_sha_rejects_arbitrary_refs(self):
        for ref in ('main', 'v1', '--upload-pack=bad', 'a' * 39):
            with self.subTest(ref=ref), self.assertRaises(update.DeploymentError):
                self.host.deploy(ref, main_sha=True)
        self.assertEqual(self.host.calls, [])

    def test_backup_is_published_only_after_validation_and_never_overwritten(self):
        output = self.host.root / 'backup.dump'
        compose = ['docker', 'compose', '-p', 'test', '-f', 'test.yml']

        def dump_then_validate(args, **kwargs):
            if 'pg_restore' in args:
                self.assertFalse(output.exists())
                self.assertEqual(kwargs['stdin'].read(), b'archive')
            else:
                kwargs['stdout'].write(b'archive')

        with patch.object(update.subprocess, 'run', side_effect=dump_then_validate):
            update.Updater.backup(self.host, compose, output)
        self.assertEqual(output.read_bytes(), b'archive')
        self.assertEqual(output.stat().st_mode & 0o777, 0o600)
        with patch.object(update.subprocess, 'run'), self.assertRaises(FileExistsError):
            update.Updater.backup(self.host, compose, output)
        self.assertEqual(output.read_bytes(), b'archive')
        output.unlink()
        with patch.object(update.subprocess, 'run', side_effect=subprocess.CalledProcessError(1, 'pg_dump')):
            with self.assertRaises(subprocess.CalledProcessError):
                update.Updater.backup(self.host, compose, output)
        self.assertFalse(output.exists())
        self.assertEqual(list(self.host.root.glob('.pre-update-*')), [])

    @unittest.skipUnless(shutil.which('docker'), 'Docker Compose CLI is required')
    def test_real_compose_merges_release_images_without_removing_monitoring(self):
        root = Path(__file__).resolve().parent.parent
        overlay = self.host.state / 'images.json'
        images = {name: {'image': f'ghcr.io/example/catforge/{name}@sha256:' + 'c' * 64,
                         'pull_policy': 'never'} for name in ('bot', 'game-engine')}
        overlay.write_text(json.dumps({'services': images}))
        for base in ('docker-compose.dev.yml', 'docker-compose.yml'):
            with self.subTest(base=base):
                result = subprocess.run(['docker', 'compose', '--env-file', str(root / '.env.example'),
                                         '-p', 'catforge-config-test', '-f', str(root / base),
                                         '-f', str(root / 'docker-compose.monitoring.yml'), '-f', str(overlay),
                                         'config', '--format', 'json'], check=True, capture_output=True, text=True)
                config = json.loads(result.stdout)
                for service in ('bot', 'game-engine'):
                    self.assertEqual(config['services'][service]['image'], images[service]['image'])
                self.assertEqual(config['services']['bot']['environment']['METRICS_ADDR'], '0.0.0.0:9091')
                self.assertEqual(config['services']['bot']['healthcheck']['test'], ['CMD', '/app/bot', 'healthcheck'])
                self.assertEqual(config['services']['postgres']['image'], 'postgres:16')
                self.assertIn('prometheus', config['services'])


if __name__ == '__main__':
    unittest.main()
