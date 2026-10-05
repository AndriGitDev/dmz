"""Exercise completed-prefix recovery in a private copy; export only metadata."""
import argparse
import json
from pathlib import Path
import shutil
import subprocess

from evaluate import ROOT, digest


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--private', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    private, output = a.private.resolve(), a.output.resolve()
    if private.is_relative_to(ROOT) or not output.is_relative_to(ROOT / 'bench/results'):
        raise ValueError('private evidence and aggregate output required')
    original = private / 'external-final-browser.json'
    data = json.loads(original.read_text())
    measurement = ROOT / 'bench/open-source-pii/candidate-browser.mjs'
    assert data['status'] == 'complete' and len(data['records']) == 4996
    assert data['resumed_excerpts'] == data['external_requests'] == 0
    assert data['measurement_sha256'] == digest(measurement)
    copied = private / 'external-final-browser-resume-check.json'
    paths = [copied, Path(str(copied) + '.partial.jsonl'), Path(str(copied) + '.checkpoint.json')]
    if any(path.exists() for path in paths) or output.exists():
        raise ValueError('new recovery outputs required')
    for suffix in ['.partial.jsonl', '.checkpoint.json']:
        shutil.copy2(Path(str(original) + suffix), Path(str(copied) + suffix))
    # Deliberately simulate a write beyond the last atomic checkpoint. This
    # extra line is not a valid inference record and must never be reused.
    with paths[1].open('a') as stream:
        stream.write('{"uncommitted_tail":true}\n')
    command = ['node', str(measurement), str(copied),
               str(private / 'datasets/gretel-projected-v2'),
               str(private / 'final-browser-build-aab7d6e'),
               data['build_manifest_sha256'], '--resume']
    run = subprocess.run(command, cwd=ROOT, capture_output=True, text=True, timeout=90)
    log = private / 'browser-checkpoint-resume-verification.log'
    log.write_text(run.stdout + run.stderr)
    if run.returncode:
        raise RuntimeError('Recovery check failed; diagnostics retained in the private log')
    resumed = json.loads(copied.read_text())
    state = json.loads(paths[2].read_text())
    assert resumed['status'] == 'complete' and resumed['resumed_excerpts'] == 4996
    assert resumed['external_requests'] == state['external_requests'] == 0
    assert resumed['records'] == data['records']
    assert resumed['build'] == data['build'] and resumed['corpus'] == data['corpus']
    assert resumed['measurement_sha256'] == data['measurement_sha256']
    assert resumed['partial_sha256'] == state['partial_sha256'] == data['partial_sha256'] == digest(paths[1])
    assert state['completed'] == 4996
    negative_path = private / 'checkpoint-negative-verification.json'
    negative = json.loads(negative_path.read_text())
    assert negative['status'] == 'complete' and negative['measurement_sha256'] == data['measurement_sha256']
    assert 0 < negative['prefix_excerpts'] <= 4996
    assert {c['case'] for c in negative['checks']} == {'tampered', 'truncated'}
    assert all(c['rejected_before_browser_launch'] and c['exit_code'] != 0 for c in negative['checks'])
    result = {
        'schema': 1, 'status': 'complete', 'independent_validation': False,
        'limitation': 'Recovery integrity, not a second fresh inference or new quality result.',
        'resumed_excerpts': 4996, 'new_inference_excerpts': 0,
        'changed_records': 0, 'uncommitted_tail_discarded': True, 'external_requests': 0,
        'negative_prefix_excerpts': negative['prefix_excerpts'], 'negative_checks': negative['checks'],
        'browser_measurement_sha256': data['measurement_sha256'],
        'verifier_sha256': digest(Path(__file__)),
        'source_sha256': {'fresh_result': digest(original), 'resumed_result': digest(copied),
                          'resumed_partial': digest(paths[1]), 'resumed_checkpoint': digest(paths[2]),
                          'negative_checks': digest(negative_path), 'private_execution_log': digest(log)},
    }
    with output.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    print('Verified complete-prefix recovery, tail discard and recorded negative integrity checks')


if __name__ == '__main__':
    main()
