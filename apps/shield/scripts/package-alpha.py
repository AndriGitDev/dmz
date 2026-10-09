"""Package a recorded, clean trained alpha and its colleague evaluation kit."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess
import zipfile

root = Path(__file__).resolve().parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--output', required=True, type=Path)
args = parser.parse_args()
output = args.output.resolve()
repo = root.parents[1]
if output.is_relative_to(repo):
    raise SystemExit('Keep the distribution archive outside the repository.')
if output.exists():
    raise SystemExit('Output already exists; choose a new archive path.')
if subprocess.check_output(['git', 'status', '--porcelain'], cwd=root).strip():
    raise SystemExit('Commit the alpha source before packaging.')
source = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=root, text=True).strip()
extension = root / '.output/chrome-mv3'
build = json.loads((extension / 'BUILD.json').read_text())
manifest = json.loads((extension / 'manifest.json').read_text())
if (build['sourceCommit'] != source or build.get('releaseLabel') != 'alpha.1'
        or build['scannerProfile'] != 'trained-fp16storage'
        or manifest['version'] != '0.1.1'
        or manifest['version_name'] != f'0.1.1 alpha.1 {source}'):
    raise SystemExit('Requires a clean, recorded Alpha 1 trained build.')
expected = set(build['sha256']) | {'BUILD.json'}
actual = {p.relative_to(extension).as_posix() for p in extension.rglob('*') if p.is_file()}
if actual != expected:
    raise SystemExit('Unexpected or missing extension files.')
files = {}
prefix = 'dmz-shield-alpha-1'
for name in sorted(expected):
    path = extension / name
    if path.is_symlink():
        raise SystemExit('Symlinks are not permitted in the package.')
    digest = hashlib.sha256(path.read_bytes()).hexdigest()
    if name != 'BUILD.json' and digest != build['sha256'][name]:
        raise SystemExit(f'Build checksum mismatch: {name}')
    files[f'{prefix}/extension/{name}'] = (path, digest)
for name in ['START-HERE.html', 'FEEDBACK.txt', 'SYNTHETIC-ATTACHMENT.txt']:
    path = root / 'alpha' / name
    files[f'{prefix}/{name}'] = (path, hashlib.sha256(path.read_bytes()).hexdigest())
output.parent.mkdir(parents=True, exist_ok=True)
with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED, compresslevel=6) as archive:
    for name, (path, _) in files.items():
        archive.write(path, name)
with zipfile.ZipFile(output) as archive:
    if archive.testzip() is not None:
        raise SystemExit('ZIP integrity check failed.')
    for name, (_, digest) in files.items():
        if hashlib.sha256(archive.read(name)).hexdigest() != digest:
            raise SystemExit(f'Archive checksum mismatch: {name}')
record = {'version': manifest['version'], 'releaseLabel': build['releaseLabel'],
          'sourceCommit': source, 'scannerProfile': build['scannerProfile'],
          'bytes': output.stat().st_size, 'sha256': hashlib.sha256(output.read_bytes()).hexdigest(),
          'files': {name: digest for name, (_, digest) in files.items()}}
output.with_suffix('.json').write_text(json.dumps(record, indent=2) + '\n')
print(json.dumps({k: v for k, v in record.items() if k != 'files'}, indent=2))
