#!/usr/bin/env python3
"""Run both modes with identical owned replicas and native PostgreSQL/Redis."""
import argparse
import csv
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time
from native import ROOT, Infrastructure, Processes, environment, metadata


def sample_resources(path, mode, roots, stop):
    """Component RSS and CPU; Linux CPU uses interval deltas from /proc."""
    previous = {}
    ticks = os.sysconf('SC_CLK_TCK')
    with path.open('w', newline='') as stream:
        writer = csv.writer(stream)
        writer.writerow(['timestampUTC', 'mode', 'component', 'cpuPercent', 'rssKiB'])
        while not stop.is_set():
            now = time.monotonic()
            rows = subprocess.check_output(['ps', '-axo', 'pid=,ppid=,pcpu=,rss='], text=True, env=dict(os.environ, LC_ALL='C'))
            processes = {}
            for row in rows.splitlines():
                parts = row.split()
                if len(parts) == 4:
                    processes[int(parts[0])] = (int(parts[1]), float(parts[2]), int(parts[3]))
            owners = dict(roots())
            totals = {}
            for pid, (parent, reported_cpu, rss) in processes.items():
                ancestor, visited = pid, set()
                while ancestor not in owners and ancestor in processes and ancestor not in visited:
                    visited.add(ancestor)
                    ancestor = processes[ancestor][0]
                if ancestor not in owners:
                    continue
                component = owners[ancestor]
                cpu = reported_cpu
                stat = Path(f'/proc/{pid}/stat')
                if stat.exists():
                    try:
                        fields = stat.read_text().rsplit(')', 1)[1].split()
                        total = (int(fields[11]) + int(fields[12])) / ticks
                        last = previous.get(pid)
                        cpu = 0 if last is None else max(0, (total - last[1]) / (now - last[0]) * 100)
                        previous[pid] = (now, total)
                    except (OSError, ValueError):
                        continue
                values = totals.setdefault(component, [0.0, 0])
                values[0] += cpu
                values[1] += rss
            stamp = time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())
            for component, (cpu, rss) in totals.items():
                writer.writerow([stamp, mode, component, round(cpu, 2), rss])
            stream.flush()
            stop.wait(.5)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--concurrency', default='2,10,50,100')
    parser.add_argument('--groups', type=int, default=100)
    parser.add_argument('--warmup', type=int, default=10)
    parser.add_argument('--repeats', type=int, default=5)
    parser.add_argument('--output', default='.cache/benchmark')
    parser.add_argument('--no-plots', action='store_true', help='skip matplotlib only for quick local verification')
    args = parser.parse_args()
    os.environ.setdefault('MPLCONFIGDIR', str(ROOT / '.cache/matplotlib'))
    env = environment()
    if not args.no_plots:
        # Fail before measuring if chart dependencies are missing.
        import matplotlib  # noqa: F401
    base = Path(args.output).resolve()
    base.mkdir(parents=True, exist_ok=True)
    directory = Path(tempfile.mkdtemp(prefix='run-', dir=base))
    directory.chmod(0o755)  # PostgreSQL runs as nobody when the notebook user is root
    infra = Infrastructure(directory / 'infra', env)
    auth = Processes(directory / 'auth')
    try:
        env = infra.start()
        auth_env = dict(env, PORT='18085')
        auth_process = auth.start('auth', [str(ROOT / 'bin/auth')], auth_env)
        env['AUTH_URL'] = 'http://127.0.0.1:18085'
        auth.wait(env['AUTH_URL'], auth_process)
        info = metadata(env)
        info['binaries'] = {name: hashlib.sha256((ROOT / 'bin' / name).read_bytes()).hexdigest() for name in ('ride', 'auth', 'benchmark')}
        info['fixtureSHA256'] = hashlib.sha256(Path(env['AUTH_FIXTURES']).read_bytes()).hexdigest()
        info['benchmark'] = vars(args)
        info['durability'] = {'fsync': True, 'synchronous_commit': True, 'shared_buffers': '128MB', 'max_connections': 50}
        info['limitations'] = ['Load generator and backend share hardware.', 'RSS sums can count shared pages more than once.', 'Core-only acceptance run: Matching/Location/Notification are not started; outbox work is deferred or skipped after acceptance. No AMQP delivery.']
        (directory / 'environment.json').write_text(json.dumps(info, indent=2) + '\n')
        # Alternate order across suite invocations if investigating order effects.
        for mode in ('sql', 'redis_sql'):
            replicas = Processes(directory / mode)
            stop = threading.Event()
            sampling = None
            try:
                env['ACCEPT_MODE'] = mode
                env['RIDE_URLS'] = 'http://127.0.0.1:18081,http://127.0.0.1:19081'
                env['RIDE_URL'] = env['RIDE_URLS'].split(',')[0]
                owned = [(int((infra.pgdata / 'postmaster.pid').read_text().splitlines()[0]), 'postgres'), (infra.redis.pid, 'redis'), (auth_process.pid, 'auth')]
                for port in (18081, 19081):
                    process = replicas.start('ride-' + str(port), [str(ROOT / 'bin/ride')], dict(env, PORT=str(port)))
                    replicas.wait('http://127.0.0.1:' + str(port), process)
                    owned.append((process.pid, 'ride'))
                subprocess.run([sys.executable, 'scripts/smoke.py', '--core-only'], cwd=ROOT, env=env, check=True)
                command = [str(ROOT / 'bin/benchmark'), '-mode', mode, '-concurrency', args.concurrency, '-groups', str(args.groups), '-warmup', str(args.warmup), '-repeats', str(args.repeats), '-out', str(directory / (mode + '.json'))]
                with (directory / (mode + '-benchmark.log')).open('w') as log:
                    load = subprocess.Popen(command, cwd=ROOT, env=env, stdout=log, stderr=subprocess.STDOUT)
                    owned.append((load.pid, 'loadgen'))
                    sampling = threading.Thread(target=sample_resources, args=(directory / (mode + '-resources.csv'), mode, lambda: list(owned), stop))
                    sampling.start()
                    try:
                        if load.wait() != 0:
                            raise RuntimeError(f'{mode} benchmark failed; inspect {log.name}')
                    finally:
                        if load.poll() is None:
                            load.terminate()
                            try:
                                load.wait(timeout=12)
                            except subprocess.TimeoutExpired:
                                load.kill()
                                load.wait()
                print((directory / (mode + '-benchmark.log')).read_text())
            finally:
                stop.set()
                if sampling:
                    sampling.join(timeout=5)
                replicas.stop()
        if not args.no_plots:
            subprocess.run([sys.executable, 'scripts/plot_benchmark.py', str(directory)], cwd=ROOT, check=True)
        print('Benchmark artifacts:', directory)
    finally:
        auth.stop()
        infra.stop()


if __name__ == '__main__':
    main()
