#!/usr/bin/env python3
"""Install pinned native tools without Docker, apt, sudo, or systemd."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import platform
import shutil
import subprocess
import tarfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--prefix', default='/kaggle/working/ride-share-tools')
    args = parser.parse_args()
    if platform.system() != 'Linux' or platform.machine() not in ('x86_64', 'amd64'):
        raise SystemExit('Pinned Kaggle bootstrap targets Linux amd64 CPU notebooks.')
    for tool in ('gcc', 'make', 'perl'):
        if not shutil.which(tool):
            raise SystemExit(f'Preflight: {tool} is required by native source builds.')
    if os.geteuid() == 0 and not shutil.which('runuser'):
        raise SystemExit('Preflight: root notebook needs runuser for PostgreSQL.')
    prefix = Path(args.prefix).resolve()
    prefix.mkdir(parents=True, exist_ok=True)
    sources = json.loads((ROOT / 'infra/versions.json').read_text())['sources']
    jobs = str(min(4, os.cpu_count() or 1))
    for name, source in sources.items():
        archive = prefix / source['url'].rsplit('/', 1)[1]
        if not archive.exists():
            print('Downloading', source['url'], flush=True)
            temporary = archive.with_suffix(archive.suffix + '.partial')
            request = urllib.request.Request(source['url'], headers={'User-Agent': 'ride-share-bootstrap/1.0'})
            for attempt in range(3):
                try:
                    with urllib.request.urlopen(request, timeout=60) as response, temporary.open('wb') as out:
                        shutil.copyfileobj(response, out)
                    break
                except (urllib.error.URLError, TimeoutError):
                    if attempt == 2:
                        raise
                    time.sleep(attempt + 1)
            temporary.replace(archive)
        if hashlib.sha256(archive.read_bytes()).hexdigest() != source['sha256']:
            raise SystemExit(f'Checksum mismatch: {archive}; remove it and retry.')
        destination = prefix / source['directory']
        if not destination.exists():
            with tarfile.open(archive) as package:
                package.extractall(prefix, filter='data')
        stamp = prefix / (name + '.installed')
        if stamp.exists() and stamp.read_text().strip() == source['sha256']:
            continue
        print('Installing', name, source['version'], flush=True)
        with (prefix / (name + '-build.log')).open('w') as log:
            def run(command, cwd=destination):
                subprocess.run(command, cwd=cwd, check=True, stdout=log, stderr=subprocess.STDOUT)
            if name == 'postgres':
                run(['./configure', '--prefix=' + str(prefix / 'postgres'), '--without-readline', '--without-zlib', '--without-icu'])
                run(['make', '-j', jobs])
                run(['make', 'install'])
            elif name == 'redis':
                run(['make', '-j', jobs, 'MALLOC=libc', 'BUILD_TLS=no', 'OPTIMIZATION=-O2'])
                run(['make', 'PREFIX=' + str(prefix / 'redis'), 'install'])
        stamp.write_text(source['sha256'] + '\n')
    paths = [str(prefix / directory / 'bin') for directory in ('go', 'postgres', 'redis')]
    result = {'PATH': os.pathsep.join(paths + [os.environ.get('PATH', '')]), 'GOTOOLCHAIN': 'local'}
    (prefix / 'tools.env.json').write_text(json.dumps(result, indent=2) + '\n')
    for binary, args in [('go', ['version']), ('postgres', ['--version']), ('redis-server', ['--version'])]:
        subprocess.run([binary, *args], env=dict(os.environ, **result), check=True)
    print('Preflight passed: native tools installed at', prefix)


if __name__ == '__main__':
    main()
