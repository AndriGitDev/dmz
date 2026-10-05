"""Publish paired specialist scores only after corpus and WASM parity checks."""
import argparse
import hashlib
import json
from pathlib import Path

from compare_external import indexed
from evaluate import ROOT, digest, load_corpus
from export_summary import quality
from quality import score


def main():
    p = argparse.ArgumentParser()
    for name in ['corpus', 'before', 'after', 'wasm', 'output']:
        p.add_argument('--' + name, type=Path, required=True)
    a = p.parse_args()
    corpus, output = a.corpus.resolve(), a.output.resolve()
    if not corpus.is_relative_to(ROOT / 'bench/corpus') or not output.is_relative_to(ROOT / 'bench/results'):
        raise ValueError('synthetic corpus and aggregate destination required')
    if any(path.resolve().is_relative_to(ROOT) for path in [a.before, a.after, a.wasm]):
        raise ValueError('private evaluation caches required')
    rows, manifest = load_corpus(corpus)
    assert manifest['independent_validation'] is False
    assert len(rows) == sum(entry['excerpts'] for entry in manifest['sets'].values())
    gold = indexed(rows)
    caches = {key: indexed([json.loads(s) for s in path.read_text().splitlines()])
              for key, path in [('before', a.before), ('after', a.after)]}
    wasm = json.loads(a.wasm.read_text())
    assert wasm['status'] == 'complete' and wasm['corpus'] == manifest
    assert wasm['native_sha256'] == digest(a.after)
    runtime = indexed(wasm['records'])
    assert set(runtime) == set(gold)
    for label, cache in caches.items():
        assert set(cache) == set(gold)
        for key, row in gold.items():
            result = cache[key]
            assert result['gold'] == row['gold'] and result['stratum'] == row['stratum']
            assert result['text_sha256'] == hashlib.sha256(row['text'].encode()).hexdigest()
            assert result['exact_restoration']
            if label == 'after':
                assert runtime[key]['predicted'] == result['predicted']
                assert runtime[key]['exact_restoration']
    changes = dict(fixed=0, broken=0, both_correct=0, both_incorrect=0)
    for key in gold:
        correct = lambda r: not r['abstained'] and set(map(tuple, r['predicted'])) == set(map(tuple, r['gold']))
        before, after = (correct(caches[label][key]) for label in ['before', 'after'])
        changes['both_correct' if before and after else 'broken' if before else 'fixed' if after else 'both_incorrect'] += 1
    result = {'schema': 1, 'independent_validation': False,
              'limitation': 'Authored synthetic specialist probes; calibration and follow-up must be interpreted separately, not as field accuracy.',
              'corpus_sha256': digest(corpus / 'manifest.json'),
              'source_sha256': {label: digest(path) for label, path in [('before', a.before), ('after', a.after), ('wasm', a.wasm)]},
              'measurement_sha256': digest(Path(__file__)), 'wasm_sha256': wasm['wasm_sha256'],
              'exact_restoration': True, 'native_wasm_span_mismatches': 0,
              'runs': {label: quality(score(list(cache.values()), manifest['scope'])) for label, cache in caches.items()},
              'paired_exact_changes': changes}
    with output.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    print(json.dumps({label: run['overall'] for label, run in result['runs'].items()}))


if __name__ == '__main__':
    main()
