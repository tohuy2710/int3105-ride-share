#!/usr/bin/env python3
"""Build, integration/fault tests, and real HTTP smoke with isolated native deps."""
import os
from pathlib import Path
import subprocess
import tempfile
from native import ROOT, Infrastructure, Processes, environment

env = environment()
subprocess.run(['make', 'build'], cwd=ROOT, env=env, check=True)
base = ROOT / '.cache/native-check'
base.mkdir(parents=True, exist_ok=True)
directory = Path(tempfile.mkdtemp(prefix='run-', dir=base))
directory.chmod(0o755)  # allow the unprivileged PostgreSQL owner to traverse on Kaggle
infra = Infrastructure(directory, env)
apps = Processes(directory / 'apps')
try:
    env = infra.start()
    env['TEST_DATABASE_URL'] = env['DATABASE_URL']
    env['TEST_REDIS_URL'] = env['REDIS_URL']
    subprocess.run(['go', 'test', '-race', '-tags=integration', './internal/integration', '-count=1', '-v'], cwd=ROOT, env=env, check=True)
    for name, port in [('ride', 18081), ('location', 18082), ('matching', 18083), ('notification', 18084), ('auth', 18085)]:
        child = env.copy()
        child['PORT'] = str(port)
        env[name.upper() + '_URL'] = f'http://127.0.0.1:{port}'
        process = apps.start(name, [str(ROOT / 'bin' / name)], child)
        apps.wait(env[name.upper() + '_URL'], process)
    subprocess.run(['python3', 'scripts/smoke.py'], cwd=ROOT, env=env, check=True)
    print('Native integration, fault, and smoke checks passed. Logs:', directory)
finally:
    apps.stop()
    infra.stop()
