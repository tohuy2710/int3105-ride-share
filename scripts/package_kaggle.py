#!/usr/bin/env python3
"""Package current source (including uncommitted implementation) for a Kaggle Dataset."""
import hashlib
import io
import json
from pathlib import Path
import subprocess
import tarfile

root = Path(__file__).resolve().parents[1]
files = []
for name in ('cmd', 'internal', 'migrations', 'fixtures', 'infra', 'scripts', 'docs', 'notebooks', '.github'):
    files.extend(p for p in (root / name).rglob('*') if p.is_file() and '__pycache__' not in p.parts)
files.extend(root / name for name in ('.go-version', '.gitignore', '.dockerignore', 'go.mod', 'go.sum', 'Makefile', 'Dockerfile', 'compose.yaml', '.env.example', 'README.md', 'AGENTS.md'))
files = sorted(set(files))
manifest = {
    'commit': subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip(),
    'dirty': bool(subprocess.check_output(['git', 'status', '--porcelain'], cwd=root, text=True)),
    'files': {str(p.relative_to(root)): hashlib.sha256(p.read_bytes()).hexdigest() for p in files},
}
output = root / '.cache/kaggle-source.tar.gz'
output.parent.mkdir(exist_ok=True)
with tarfile.open(output, 'w:gz') as archive:
    for path in files:
        archive.add(path, arcname='int3105-ride-share/' + str(path.relative_to(root)), recursive=False)
    data = (json.dumps(manifest, indent=2) + '\n').encode()
    info = tarfile.TarInfo('int3105-ride-share/source-manifest.json')
    info.size = len(data)
    archive.addfile(info, io.BytesIO(data))
print(output)
