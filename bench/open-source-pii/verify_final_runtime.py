"""Bind immutable score reports to the final tested runtime without exporting rows."""
import argparse
import json
from pathlib import Path

from compare_external import indexed
from evaluate import ROOT, digest


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--private', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    private, output = a.private.resolve(), a.output.resolve()
    if private.is_relative_to(ROOT) or not output.is_relative_to(ROOT / 'bench/results'):
        raise ValueError('private evidence and aggregate destination required')
    trained, generic = (digest(private / name) for name in ['trained-contained-token.wasm', 'generic-contained-token.wasm'])
    checks, sources = {}, {}

    def read(name):
        path = private / name
        sources[name] = digest(path)
        return json.loads(path.read_text())

    for label, before, after, count in [
        ('initial', 'structured-core-initial-replay.json', 'initial-contained-token-core-replay.json', 100),
        ('followup', 'structured-core-followup-replay.json', 'followup-contained-token-core-replay.json', 90),
        ('public-english', 'external-stack128k-core-replay.json', 'external-contained-token-core-replay.json', 4996),
        ('icelandic-person', 'icelandic-core-stack128k.json', 'icelandic-core-contained-token.json', 5838),
    ]:
        x, y = read(before), read(after)
        assert x['status'] == y['status'] == 'complete'
        assert x['corpus'] == y['corpus'] and y['wasm_sha256'] == trained
        assert len(y['records']) == count and x['records'] == y['records']
        for row in y['records']:
            assert row['exact_restoration'] if label == 'icelandic-person' else all(row[m]['exact_restoration'] for m in ['basic', 'trained'])
        checks[label] = {'excerpts': count, 'changed_records': 0, 'exact_restoration': True}

    for label, old, new, wasm, count in [
        ('vendor', 'vendor-core-first.jsonl', 'vendor-final-native.jsonl', 'vendor-contained-token-wasm-parity.json', 102),
        ('vendor-followup', 'vendor-followup-after.jsonl', 'vendor-followup-final-native.jsonl', 'vendor-followup-contained-token-wasm-parity.json', 134),
        ('phone', 'phone-core-final.jsonl', 'phone-final-native.jsonl', 'phone-contained-token-wasm-parity.json', 36),
    ]:
        caches = []
        for name in [old, new]:
            path = private / name
            sources[name] = digest(path)
            caches.append(indexed([json.loads(s) for s in path.read_text().splitlines()]))
        x, y = caches
        assert x == y and len(y) == count
        w = read(wasm)
        assert w['status'] == 'complete' and w['native_sha256'] == sources[old] and w['wasm_sha256'] == trained
        wr = indexed(w['records'])
        assert wr.keys() == y.keys()
        assert all(wr[k]['predicted'] == y[k]['predicted'] and wr[k]['exact_restoration'] and y[k]['exact_restoration'] for k in y)
        checks[label] = {'excerpts': count, 'native_changed_records': 0, 'native_wasm_span_mismatches': 0, 'exact_restoration': True}

    for label, name, expected, binary in [
        ('phone-world', 'phone-world-wasm-contained-token.json', 49245, generic),
        ('global-network-generic', 'phone-nongeographic-generic-contained-token.json', 9, generic),
        ('global-network-trained', 'phone-nongeographic-trained-contained-token.json', 9, trained),
    ]:
        data = read(name)
        assert data['status'] == 'complete' and data['cases'] == expected and data['wasm_sha256'] == binary
        assert data['native_span_mismatches'] == 0 and data['exact_restoration']
        checks[label] = {k: data[k] for k in ['cases', 'fixture_sha256', 'native_span_mismatches', 'exact_restoration']}

    build = read('build-browser-aab7d6e-verification.json')
    assert build['wasm_sha256'] == trained
    published = {}
    directory = ROOT / 'bench/results/open-source-pii-v1'
    for name, cache in [('public-core-replay.json', 'external-stack128k-core-replay.json'),
                        ('icelandic-core-replay.json', 'icelandic-core-stack128k.json')]:
        path = directory / name
        assert json.loads(path.read_text())['input_sha256']['after'] == sources[cache]
        published[name] = digest(path)
    for label in ['initial', 'followup']:
        name = label + '-structured-replay.json'
        comparison = 'structured-core-' + label + '-comparison.json'
        data = read(comparison)
        assert data['cache_sha256']['after'] == sources['structured-core-' + label + '-replay.json']
        path = directory / name
        assert json.loads(path.read_text())['source_sha256'][comparison] == sources[comparison]
        published[name] = digest(path)
    for name, cache in [('vendor-specialist-calibration.json', 'vendor-core-first.jsonl'),
                        ('vendor-specialist-followup.json', 'vendor-followup-after.jsonl'),
                        ('phone-specialist.json', 'phone-core-final.jsonl')]:
        path = directory / name
        assert json.loads(path.read_text())['source_sha256']['after'] == sources[cache]
        published[name] = digest(path)
    result = {'schema': 1, 'status': 'complete', 'independent_validation': False,
              'limitation': 'Runtime/record parity; synthetic and reused development evidence, not independent field quality.',
              'product_source_commit': build['source_commit'], 'trained_wasm_sha256': trained,
              'generic_wasm_sha256': generic, 'measurement_sha256': digest(Path(__file__)),
              'checks': checks, 'published_score_sha256': published, 'private_source_sha256': sources}
    with output.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    print('Verified final runtime and native results against all recorded regression sets')


if __name__ == '__main__':
    main()
