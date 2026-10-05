"""Verify completed fresh inference against scored replay; publish only provenance."""
import argparse
import json
from pathlib import Path

from compare_external import indexed
from evaluate import ROOT, digest


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument('--private', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    private, output = args.private.resolve(), args.output.resolve()
    if private.is_relative_to(ROOT) or not output.is_relative_to(ROOT / 'bench/results'):
        raise ValueError('private evidence and aggregate destination required')
    sources = {}

    def read(name):
        path = private / name
        sources[name] = digest(path)
        return json.loads(path.read_text())

    fresh = read('external-final-browser.json')
    replay = read('external-contained-token-core-replay.json')
    state = read('external-final-browser.json.checkpoint.json')
    partial = private / 'external-final-browser.json.partial.jsonl'
    assert fresh['status'] == replay['status'] == 'complete'
    assert fresh['external_requests'] == replay['external_requests'] == 0
    assert fresh['resumed_excerpts'] == 0
    assert fresh['corpus'] == replay['corpus']
    assert fresh['build']['sha256']['dmz.wasm'] == replay['wasm_sha256']
    assert fresh['build']['sha256']['wasm_exec.js'] == replay['runtime_sha256']
    assert fresh['build'] == read('final-browser-build-aab7d6e/BUILD.json')
    assert fresh['build_manifest_sha256'] == digest(private / 'final-browser-build-aab7d6e/BUILD.json')
    assert fresh['measurement_sha256'] == digest(ROOT / 'bench/open-source-pii/candidate-browser.mjs')
    assert fresh['partial_sha256'] == state['partial_sha256'] == digest(partial)
    assert state['completed'] == len(fresh['records']) == 4996
    assert state['external_requests'] == 0
    assert state['identity']['expected_excerpts'] == 4996
    assert state['identity']['build_manifest_sha256'] == fresh['build_manifest_sha256']
    assert state['identity']['measurement_sha256'] == fresh['measurement_sha256']
    assert [json.loads(line) for line in partial.read_text().splitlines()] == fresh['records']
    before, after = indexed(replay['records']), indexed(fresh['records'])
    assert before.keys() == after.keys()
    for key, expected in before.items():
        observed = after[key]
        for field in ['id', 'text_sha256', 'stratum', 'names', 'scanner_code']:
            assert observed[field] == expected[field], 'Fresh inference differs; rescore before publishing'
        for mode in ['basic', 'trained']:
            for field in ['predicted', 'abstained', 'exact_restoration', 'altered']:
                assert observed[mode][field] == expected[mode][field], 'Fresh masking differs; rescore before publishing'
            assert observed[mode]['exact_restoration']
    directory = ROOT / 'bench/results/open-source-pii-v1'
    runtime = json.loads((directory / 'final-runtime-validation.json').read_text())
    assert runtime['private_source_sha256']['external-contained-token-core-replay.json'] == sources['external-contained-token-core-replay.json']
    assert runtime['trained_wasm_sha256'] == replay['wasm_sha256']
    published = {'final-runtime-validation.json': digest(directory / 'final-runtime-validation.json')}
    for name in ['public-current-core-full-models.json', 'public-current-core-sample-models.json']:
        path = directory / name
        assert json.loads(path.read_text())['reference_sha256'] == sources['external-contained-token-core-replay.json']
        published[name] = digest(path)
    result = {
        'schema': 1, 'status': 'complete', 'independent_validation': False,
        'execution': fresh['execution'],
        'limitation': 'Fresh runtime confirmation on the same public synthetic projection; not independent field quality.',
        'excerpts': len(after), 'changed_prediction_records': 0,
        'exact_restoration': True, 'external_requests': 0, 'resumed_excerpts': 0,
        'seconds': fresh['seconds'], 'product_source_commit': fresh['build']['sourceCommit'],
        'build_manifest_sha256': fresh['build_manifest_sha256'],
        'trained_wasm_sha256': replay['wasm_sha256'],
        'browser_measurement_sha256': fresh['measurement_sha256'],
        'verifier_sha256': digest(Path(__file__)),
        'private_source_sha256': sources, 'partial_sha256': digest(partial),
        'published_score_sha256': published,
    }
    with output.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    print('Fresh browser predictions and restoration match all 4996 scored replay records')


if __name__ == '__main__':
    main()
