#!/usr/bin/env python3
from pathlib import Path
import subprocess
import sys

files = [str(path) for directory in ('cmd', 'internal', 'migrations') for path in Path(directory).rglob('*.go')]
result = subprocess.check_output(['gofmt', '-l', *files], text=True)
if result:
    print(result, end='')
    sys.exit('Run make fmt')
