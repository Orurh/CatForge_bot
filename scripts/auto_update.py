#!/usr/bin/env python3
"""Install current main only after its CI and image publication succeed."""
import fcntl
import json
import os
from pathlib import Path
import re
import subprocess
import urllib.request


def successful_run(runs, revision, repository):
    return any(
        run.get('head_sha') == revision
        and run.get('head_branch') == 'main'
        and run.get('event') == 'push'
        and run.get('status') == 'completed'
        and run.get('conclusion') == 'success'
        and (run.get('head_repository') or {}).get('full_name', '').lower() == repository.lower()
        for run in runs
    )


def main():
    os.umask(0o077)
    root = Path(__file__).resolve().parent.parent
    state = root / '.deploy'
    config = json.loads((state / 'host.json').read_text())
    repository = config['repository']
    if not re.fullmatch(r'[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+', repository):
        raise RuntimeError('Invalid repository in .deploy/host.json')
    with (state / 'auto-update.lock').open('w') as lock:
        fcntl.flock(lock, fcntl.LOCK_EX | fcntl.LOCK_NB)
        if (state / 'pending.json').exists():
            raise RuntimeError('Unfinished deployment; inspect .deploy/pending.json')
        remote = subprocess.check_output(
            ['git', 'ls-remote', '--exit-code', 'origin', 'refs/heads/main'],
            cwd=root, text=True, timeout=60,
        ).split()
        revision = remote[0]
        if not re.fullmatch(r'[0-9a-f]{40}', revision):
            raise RuntimeError('Invalid main SHA')
        last = state / 'last-success.json'
        if last.exists() and json.loads(last.read_text()).get('revision') == revision:
            print('Current main is already deployed.')
            return
        headers = {'Accept': 'application/vnd.github+json', 'User-Agent': 'catforge-deploy'}
        if os.environ.get('GH_TOKEN'):
            headers['Authorization'] = 'Bearer ' + os.environ['GH_TOKEN']
        url = (f'https://api.github.com/repos/{repository}/actions/workflows/ci.yml/runs'
               f'?branch=main&event=push&head_sha={revision}&per_page=20')
        with urllib.request.urlopen(urllib.request.Request(url, headers=headers), timeout=30) as response:
            runs = json.load(response)['workflow_runs']
        if not successful_run(runs, revision, repository):
            print('Current main has no successful CI + publication; keeping the running version.')
            return
        subprocess.run(['bash', str(root / 'scripts/update.sh'), '--main-sha', revision],
                       cwd=root, check=True)


if __name__ == '__main__':
    main()
