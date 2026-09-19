#!/usr/bin/env python3
"""Install a tested Git tag on an existing single-host Compose installation."""
import argparse
import datetime
import fcntl
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import tempfile
import time


class DeploymentError(RuntimeError):
    pass


def write_json(path, value):
    temporary = path.with_suffix(path.suffix + '.tmp')
    temporary.write_text(json.dumps(value, indent=2) + '\n')
    temporary.replace(path)


class Updater:
    def __init__(self, root):
        self.root = Path(root).resolve()
        self.state = self.root / '.deploy'
        self.config = json.loads((self.state / 'host.json').read_text())
        self.prefix = self.config['image_prefix']
        self.project = self.config['project']
        self.files = self.config['compose_files']
        if not re.fullmatch(r'ghcr\.io/[a-z0-9_.-]+/[a-z0-9_./-]+', self.prefix):
            raise DeploymentError('Set a lowercase GHCR image_prefix in .deploy/host.json')
        if not re.fullmatch(r'[a-z0-9][a-z0-9_-]*', self.project):
            raise DeploymentError('Invalid Compose project name')
        if not isinstance(self.files, list) or not self.files:
            raise DeploymentError('compose_files must be a nonempty list')
        for file in self.files:
            if not isinstance(file, str) or Path(file).is_absolute() or '..' in Path(file).parts:
                raise DeploymentError('compose_files must be paths inside the checkout')
        self.wait = int(self.config.get('wait_seconds', 180))
        if not 30 <= self.wait <= 900:
            raise DeploymentError('wait_seconds must be between 30 and 900')

    def run(self, *args, timeout=900):
        result = subprocess.run(args, cwd=self.root, text=True, capture_output=True,
                                timeout=timeout, check=True)
        return result.stdout.strip()

    def compose(self, overlay=None):
        args = ['docker', 'compose', '--project-directory', str(self.root),
                '--env-file', str(self.root / '.env'), '-p', self.project]
        for file in self.files:
            args += ['-f', str(self.root / file)]
        current = self.state / 'current.json'
        if current.exists():
            args += ['-f', str(current)]
        if overlay:
            args += ['-f', str(overlay)]
        return args

    def image(self, component, revision):
        tag = f'{self.prefix}/{component}:sha-{revision}'
        self.run('docker', 'pull', tag)
        info = json.loads(self.run('docker', 'image', 'inspect', '--format', '{{json .}}', tag))
        labels = info.get('Config', {}).get('Labels') or {}
        if labels.get('org.opencontainers.image.revision') != revision:
            raise DeploymentError(f'{component}: image revision does not match the Git tag')
        if info.get('Os') != 'linux' or info.get('Architecture') != 'amd64':
            raise DeploymentError(f'{component}: this release requires Linux amd64')
        repository = f'{self.prefix}/{component}@sha256:'
        digests = [d for d in info.get('RepoDigests', []) if d.startswith(repository)]
        if len(digests) != 1 or not re.fullmatch(re.escape(repository) + r'[0-9a-f]{64}', digests[0]):
            raise DeploymentError(f'{component}: cannot resolve an immutable image digest')
        return digests[0]

    def backup(self, compose, destination):
        # Uses exactly the same project/overlays as the running bot, not a default project.
        fd, temporary = tempfile.mkstemp(prefix='.pre-update-', dir=destination.parent)
        try:
            with os.fdopen(fd, 'wb') as output:
                subprocess.run([*compose, 'exec', '-T', 'postgres', 'sh', '-c',
                                'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc'],
                               cwd=self.root, stdout=output, check=True, timeout=900)
                output.flush()
                os.fsync(output.fileno())
            with open(temporary, 'rb') as source:
                subprocess.run([*compose, 'exec', '-T', 'postgres', 'pg_restore', '--list'],
                               cwd=self.root, stdin=source, stdout=subprocess.DEVNULL,
                               check=True, timeout=120)
            os.link(temporary, destination)  # Never overwrite an earlier backup.
        finally:
            os.unlink(temporary)

    def wait_for_existing_bot(self):
        deadline = time.monotonic() + self.wait
        while True:
            try:
                self.run(*self.compose(), 'exec', '-T', 'bot', '/app/bot', 'healthcheck', timeout=15)
                return
            except subprocess.CalledProcessError:
                if time.monotonic() >= deadline:
                    raise
                time.sleep(2)

    @staticmethod
    def check_infrastructure(before, after):
        for key in ('volumes', 'networks', 'secrets', 'configs'):
            if before.get(key) != after.get(key):
                raise DeploymentError(f'Release changes {key}; review the host configuration separately')
        for config in (before, after):
            if not {'bot', 'game-engine', 'postgres'} <= config['services'].keys():
                raise DeploymentError('Compose must contain bot, game-engine and postgres')
        old = {k: v for k, v in before['services'].items() if k not in ('bot', 'game-engine')}
        new = {k: v for k, v in after['services'].items() if k not in ('bot', 'game-engine')}
        if old != new:
            raise DeploymentError('Release changes host services; review them separately before updating')
        old_db = before['services']['bot'].get('environment', {}).get('DATABASE_URL')
        new_db = after['services']['bot'].get('environment', {}).get('DATABASE_URL')
        if not old_db or old_db != new_db:
            raise DeploymentError('Release changes DATABASE_URL; refusing to migrate another database')

    def deploy(self, tag, main_sha=False):
        if main_sha and not re.fullmatch(r'[0-9a-f]{40}', tag):
            raise DeploymentError('Expected a full main commit SHA')
        if not main_sha and not re.fullmatch(r'v[0-9][A-Za-z0-9._-]*', tag):
            raise DeploymentError('Use a version tag, for example v0.1.0-beta.1')
        pending = self.state / 'pending.json'
        if pending.exists():
            raise DeploymentError('Unfinished deployment: inspect .deploy/pending.json and docs/DEPLOYMENT.md')
        if self.run('git', 'status', '--porcelain'):
            raise DeploymentError('Checkout has local changes; use a clean server checkout')
        # Host-specific overlays survive checkouts in the ignored state directory.
        tracked_files = [file for file in self.files if Path(file).parts[0] != '.deploy']
        if tracked_files:
            self.run('git', 'ls-files', '--error-unmatch', '--', *tracked_files)
        for file in self.files:
            if Path(file).parts[0] == '.deploy' and not (self.root / file).is_file():
                raise DeploymentError('Missing host-local Compose overlay')
        previous_revision = self.run('git', 'rev-parse', 'HEAD')
        previous_ref = self.run('git', 'rev-parse', '--abbrev-ref', 'HEAD')
        if previous_ref == 'HEAD':
            previous_ref = previous_revision
        compose = self.compose()
        before = json.loads(self.run(*compose, 'config', '--format', 'json'))
        containers = {}
        for service in ('bot', 'game-engine', 'postgres'):
            ids = self.run(*compose, 'ps', '--status', 'running', '-q', service).split()
            if len(ids) != 1:
                raise DeploymentError(f'Expected one running {service}; check the project or finish initial setup')
            containers[service] = ids[0]
        self.run(*compose, 'exec', '-T', 'bot', '/app/bot', 'healthcheck', timeout=15)
        architecture = self.run('docker', 'info', '--format', '{{.Architecture}}')
        if architecture not in ('x86_64', 'amd64'):
            raise DeploymentError('Published images currently require an amd64 Docker host')
        print(f'Fetching {tag} and both release images; the bot is still running.', flush=True)
        self.run('git', 'fetch', '--no-tags', 'origin',
                 'refs/heads/main' if main_sha else f'refs/tags/{tag}')
        revision = self.run('git', 'rev-parse', 'FETCH_HEAD^{commit}')
        if not re.fullmatch(r'[0-9a-f]{40}', revision):
            raise DeploymentError('Invalid release commit')
        if main_sha and revision != tag:
            raise DeploymentError('main moved since CI selection; wait for the next poll')
        images = {service: self.image(component, revision)
                  for service, component in (('bot', 'bot'), ('game-engine', 'engine'))}
        stamp = datetime.datetime.now(datetime.timezone.utc).strftime('%Y%m%dT%H%M%S.%fZ')
        history = self.state / 'history' / stamp
        history.mkdir(parents=True)
        candidate = history / 'images.json'
        write_json(candidate, {'services': {s: {'image': i, 'pull_policy': 'never'} for s, i in images.items()}})
        previous_images = {}
        for service in ('bot', 'game-engine'):
            image_id = self.run('docker', 'inspect', '--format', '{{.Image}}', containers[service])
            saved_tag = f'catforge-rollback-{self.project}-{service}:{stamp.lower()}'
            self.run('docker', 'tag', image_id, saved_tag)
            previous_images[service] = {'image': image_id, 'pull_policy': 'never'}
        write_json(history / 'previous-images.json', {'services': previous_images})
        backups = self.root / 'backups'
        backups.mkdir(exist_ok=True)
        backup_path = backups / f'pre-update-{stamp}.dump'
        attempt = dict(tag=tag, revision=revision, previous_revision=previous_revision,
                       previous_ref=previous_ref, backup=str(backup_path), history=str(history), phase='preparing')
        write_json(pending, attempt)
        activating = False
        stopped = False
        try:
            self.run('git', 'checkout', '--detach', revision)
            target = self.compose(candidate)
            after = json.loads(self.run(*target, 'config', '--format', 'json'))
            self.check_infrastructure(before, after)
            print('Stopping the bot and saving a database backup.', flush=True)
            stopped = True  # A failed/interrupting stop may already have stopped the container.
            self.run(*compose, 'stop', '-t', '60', 'bot', timeout=90)
            self.backup(compose, backup_path)
            attempt['phase'] = 'activating'
            write_json(pending, attempt)
            activating = True
            print('Installing the engine and bot; waiting for database/engine readiness.', flush=True)
            self.run(*target, 'up', '-d', '--no-deps', '--no-build', '--pull', 'never', 'game-engine')
            self.run(*target, 'up', '-d', '--no-deps', '--no-build', '--pull', 'never',
                     '--wait', '--wait-timeout', str(self.wait), 'bot', timeout=self.wait + 90)
            self.run(*target, 'exec', '-T', 'bot', '/app/bot', 'healthcheck', timeout=15)
            write_json(self.state / 'current.json', json.loads(candidate.read_text()))
            attempt['phase'] = 'healthy'
            write_json(history / 'result.json', attempt)
            write_json(self.state / 'last-success.json', attempt)
            pending.unlink()
            print(f'Installed {tag} ({revision[:12]}). Backup: {backup_path}', flush=True)
        except (Exception, KeyboardInterrupt):
            if activating:
                # New migrations may have committed. Never automatically run an older binary/schema.
                try:
                    self.run(*self.compose(candidate), 'stop', '-t', '60', 'bot', timeout=90)
                except Exception:
                    print('Could not confirm that the bot stopped; inspect Docker immediately.', file=sys.stderr)
                print(f'Update failed after activation. Check {pending}; backup: {backup_path}. '
                      'Automatic schema rollback is disabled.', file=sys.stderr)
            else:
                try:
                    self.run('git', 'checkout', previous_ref)
                    if stopped:
                        self.run(*self.compose(), 'start', 'bot')
                        self.wait_for_existing_bot()
                    pending.unlink()
                except Exception:
                    print(f'Could not restore the previous running state; inspect {pending}.', file=sys.stderr)
            raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('tag', help='Published Git version tag, e.g. v0.1.0-beta.1')
    parser.add_argument('--main-sha', action='store_true', help='Require the exact current main SHA')
    args = parser.parse_args()
    os.umask(0o077)
    root = Path(__file__).resolve().parent.parent
    state = root / '.deploy'
    state.mkdir(exist_ok=True)

    def interrupted(signum, frame):
        raise DeploymentError('Deployment interrupted')

    signal.signal(signal.SIGTERM, interrupted)
    try:
        with (state / 'update.lock').open('w') as lock:
            try:
                fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
            except BlockingIOError:
                raise DeploymentError('Another update is already running')
            Updater(root).deploy(args.tag, main_sha=args.main_sha)
    except (Exception, KeyboardInterrupt) as error:
        print(f'Update failed: {error}', file=sys.stderr)
        return 1
    return 0


if __name__ == '__main__':
    sys.exit(main())
