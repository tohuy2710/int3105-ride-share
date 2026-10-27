"""Owned native processes for local verification and Kaggle (no Docker daemon)."""
import json
import os
from pathlib import Path
import shutil
import subprocess
import time
import urllib.request

ROOT = Path(__file__).resolve().parents[1]


def environment():
    env = os.environ.copy()
    defaults = {
        'JWT_SECRET': 'local-development-secret-at-least-32-bytes',
        'JWT_ISSUER': 'ride-share', 'JWT_AUDIENCE': 'ride-share-api',
        'INTERNAL_TOKEN': 'local-internal-token-change-for-deployment',
        'AUTH_FIXTURES': str(ROOT / 'fixtures/accounts.json'),
        'GOCACHE': str(ROOT / '.cache/go-build'),
        'GOMODCACHE': str(ROOT / '.cache/go-mod'),
        'DB_MAX_OPEN_CONNS': '10', 'DB_TIMEOUT': '2s',
        'REDIS_TIMEOUT': '100ms', 'REDIS_LOCK_TTL': '5s',
    }
    for key, value in defaults.items():
        env.setdefault(key, value)
    return env


class Processes:
    def __init__(self, directory):
        self.directory = Path(directory).resolve()
        self.directory.mkdir(parents=True, exist_ok=True)
        self.owned = []

    def start(self, name, command, env):
        log = (self.directory / (name + '.log')).open('w')
        process = subprocess.Popen(command, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
        self.owned.append((process, log))
        return process

    def stop(self):
        for process, _ in reversed(self.owned):
            if process.poll() is None:
                process.terminate()
        for process, log in reversed(self.owned):
            try:
                process.wait(timeout=12)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            log.close()
        self.owned.clear()

    def wait(self, url, process):
        deadline = time.monotonic() + 30
        while time.monotonic() < deadline:
            if process.poll() is not None:
                raise RuntimeError(f'{url}: process exited; inspect {self.directory}')
            try:
                with urllib.request.urlopen(url + '/readyz', timeout=2) as response:
                    if response.status == 200:
                        return
            except OSError:
                pass
            time.sleep(.1)
        raise RuntimeError(f'{url}: readiness timeout; inspect {self.directory}')


class Infrastructure:
    """Creates an isolated DB/Redis instance; never touches existing instances."""
    def __init__(self, directory, env):
        self.directory = Path(directory).resolve()
        self.directory.mkdir(parents=True, exist_ok=True)
        self.env = env.copy()
        self.processes = Processes(self.directory / 'logs')
        self.pgdata = self.directory / 'pgdata'
        self.pgctl = None
        self.as_user = []

    def start(self):
        postgres = shutil.which('postgres')
        redis = shutil.which('redis-server')
        if not postgres or not redis:
            raise RuntimeError('postgres and redis-server are required on PATH; see docs/kaggle.md')
        bindir = Path(postgres).resolve().parent
        initdb = str(bindir / 'initdb')
        self.pgctl = str(bindir / 'pg_ctl')
        if self.pgdata.exists():
            raise RuntimeError(f'{self.pgdata} already exists: choose a fresh run directory')
        self.pgdata.mkdir()
        # PostgreSQL refuses root. Kaggle notebooks often run as root.
        if os.geteuid() == 0:
            import pwd
            if not shutil.which('runuser'):
                raise RuntimeError('root runtime requires runuser and the nobody account')
            account = pwd.getpwnam('nobody')
            self.as_user = ['runuser', '-u', 'nobody', '--']
            for path in (self.pgdata,):
                os.chown(path, account.pw_uid, account.pw_gid)
        with (self.directory / 'initdb.log').open('w') as log:
            subprocess.run(self.as_user + [initdb, '-D', str(self.pgdata), '-U', 'ride', '-A', 'trust', '--no-locale', '--encoding=UTF8'], check=True, stdout=log, stderr=subprocess.STDOUT)
        pgport = self.env.get('NATIVE_POSTGRES_PORT', '15432')
        redisport = self.env.get('NATIVE_REDIS_PORT', '16379')
        # TCP only: Unix socket paths exceed macOS's limit in deep workspaces.
        import shlex
        options = shlex.join(['-h', '127.0.0.1', '-p', pgport, '-k', '', '-c', 'max_connections=50', '-c', 'shared_buffers=128MB', '-c', 'fsync=on', '-c', 'synchronous_commit=on'])
        subprocess.run(self.as_user + [self.pgctl, '-D', str(self.pgdata), '-l', str(self.pgdata / 'server.log'), '-o', options, '-w', 'start'], check=True)
        subprocess.run(self.as_user + [str(bindir / 'createdb'), '-h', '127.0.0.1', '-p', pgport, '-U', 'ride', 'rides'], check=True)
        self.env['DATABASE_URL'] = f'postgres://ride@127.0.0.1:{pgport}/rides?sslmode=disable'
        self.env['REDIS_URL'] = f'redis://127.0.0.1:{redisport}/0'
        self.redis = self.processes.start('redis', [redis, '--bind', '127.0.0.1', '--port', redisport, '--save', '', '--appendonly', 'no'], self.env)
        cli = shutil.which('redis-cli') or str(Path(redis).resolve().parent / 'redis-cli')
        for _ in range(100):
            if self.redis.poll() is not None:
                raise RuntimeError('owned Redis exited; inspect logs')
            ping = subprocess.run([cli, '-p', redisport, 'ping'], capture_output=True)
            if ping.returncode == 0:
                break
            time.sleep(.1)
        else:
            raise RuntimeError('Redis readiness timeout')
        subprocess.run([str(ROOT / 'bin/migrate')], cwd=ROOT, env=self.env, check=True)
        return self.env

    def stop(self):
        self.processes.stop()
        if self.pgctl and (self.pgdata / 'postmaster.pid').exists():
            subprocess.run(self.as_user + [self.pgctl, '-D', str(self.pgdata), '-m', 'fast', '-w', 'stop'], check=True)


def metadata(env):
    def output(args):
        try:
            return subprocess.check_output(args, cwd=ROOT, text=True, stderr=subprocess.STDOUT).strip()
        except (OSError, subprocess.CalledProcessError) as error:
            return str(error)
    result = {
        'commit': output(['git', 'rev-parse', 'HEAD']),
        'dirty': bool(output(['git', 'status', '--porcelain'])),
        'os': output(['uname', '-a']), 'cpuCount': os.cpu_count(),
        'cpu': Path('/proc/cpuinfo').read_text() if Path('/proc/cpuinfo').exists() else output(['sysctl', '-n', 'machdep.cpu.brand_string']),
        'memory': Path('/proc/meminfo').read_text() if Path('/proc/meminfo').exists() else output(['sysctl', '-n', 'hw.memsize']),
        'go': output(['go', 'version']), 'postgres': output(['postgres', '--version']),
        'redis': output(['redis-server', '--version']),
        'config': {key: env.get(key) for key in ('DB_MAX_OPEN_CONNS', 'DB_TIMEOUT', 'REDIS_TIMEOUT', 'REDIS_LOCK_TTL', 'GOMAXPROCS')},
        'timestampUTC': time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime()),
        'cpuAffinity': sorted(os.sched_getaffinity(0)) if hasattr(os, 'sched_getaffinity') else None,
        'cgroups': {name: Path('/sys/fs/cgroup', name).read_text().strip() for name in ('cpu.max', 'memory.max') if Path('/sys/fs/cgroup', name).exists()},
    }
    manifest = ROOT / 'source-manifest.json'
    if manifest.exists():
        result['sourceManifest'] = json.loads(manifest.read_text())
        result['commit'] = result['sourceManifest']['commit']
        result['dirty'] = result['sourceManifest']['dirty']
    return result
