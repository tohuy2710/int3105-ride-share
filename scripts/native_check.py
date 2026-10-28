#!/usr/bin/env python3
"""Build, integration/fault tests, and real HTTP smoke with isolated native deps."""
import os
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import urllib.request
from urllib.parse import urlsplit, urlunsplit
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
    # Each service owns a separate, disposable test DB and applies its SQL only
    # through explicit integration-test setup, never from an application replica.
    database = urlsplit(env['DATABASE_URL'])
    bindir = Path(shutil.which('postgres')).resolve().parent
    for service in ('location', 'notification', 'matching'):
        name = service + '_test'
        subprocess.run([str(bindir / 'createdb'), '-h', database.hostname,
                        '-p', str(database.port), '-U', database.username, name], check=True)
        env['TEST_' + service.upper() + '_DATABASE_URL'] = urlunsplit(database._replace(path='/' + name))
        subprocess.run(['go', 'test', '-race', '-tags=integration',
                        './internal/' + service + '/...', '-count=1', '-v'], cwd=ROOT, env=env, check=True)
    subprocess.run(['go', 'test', '-race', '-tags=integration', './internal/integration', '-count=1', '-v'], cwd=ROOT, env=env, check=True)
    for name, port in [('ride', 18081), ('location', 18082), ('matching', 18083), ('notification', 18084), ('auth', 18085)]:
        child = env.copy()
        child['PORT'] = str(port)
        env[name.upper() + '_URL'] = f'http://127.0.0.1:{port}'
        process = apps.start(name, [str(ROOT / 'bin' / name)], child)
        apps.wait(env[name.upper() + '_URL'], process)
        for path in ('index.html', 'swagger-ui.css', 'swagger-ui-bundle.js', 'openapi.json'):
            with urllib.request.urlopen(env[name.upper() + '_URL'] + '/swagger/' + path, timeout=5) as response:
                assert response.status == 200 and response.read(), (name, path)
        with urllib.request.urlopen(env[name.upper() + '_URL'] + '/swagger/openapi.json', timeout=5) as response:
            assert json.load(response)['servers'] == [{'url': '/'}], name
    subprocess.run(['python3', 'scripts/smoke.py'], cwd=ROOT, env=env, check=True)
    print('Service PostgreSQL, baseline integration/fault, Swagger, and HTTP smoke checks passed. Logs:', directory)
finally:
    apps.stop()
    infra.stop()
