"""Compare a complete public browser baseline with an unchanged-name WASM replay."""
import argparse
import hashlib
import json
from pathlib import Path

from evaluate import digest, private_output
from evaluate_external import TEST_SHA
from compare_external import indexed
from quality import score


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--before', type=Path, required=True)
    p.add_argument('--after', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    manifest = json.loads((a.corpus / 'manifest.json').read_text())
    if digest(a.corpus / 'test.jsonl') != TEST_SHA:
        raise ValueError('wrong public projection')
    rows = indexed([json.loads(line) for line in (a.corpus / 'test.jsonl').read_text().splitlines()])
    sources = {key: json.loads(path.read_text()) for key, path in [('before', a.before), ('after', a.after)]}
    if sources['after']['reference_sha256'] != digest(a.before):
        raise ValueError('wrong replay reference')
    caches = {}
    for key, data in sources.items():
        if data['status'] != 'complete' or data['corpus'] != manifest or data['external_requests']:
            raise ValueError('incompatible or incomplete cache')
        caches[key] = indexed(data['records'])
        if set(caches[key]) != set(rows):
            raise ValueError('incomplete record set')
    for key, row in rows.items():
        expected = hashlib.sha256(row['text'].encode()).hexdigest()
        for cache in caches.values():
            if cache[key]['text_sha256'] != expected:
                raise ValueError('changed text')
            if not all(cache[key][mode]['exact_restoration'] for mode in ['basic', 'trained']):
                raise ValueError('restoration failure')
        if caches['before'][key]['names'] != caches['after'][key]['names']:
            raise ValueError('contextual name predictions changed')
    scope = set(manifest['scope'])
    runs, paired = {}, {}
    for mode in ['basic', 'trained']:
        predictions = {}
        for version, cache in caches.items():
            records = []
            predictions[version] = {}
            for key, row in rows.items():
                source = cache[key][mode]
                prediction = [s for s in source['predicted'] if s[2] in scope]
                records.append({'gold': row['gold'], 'predicted': prediction,
                                'abstained': source['abstained'], 'stratum': row['stratum']})
                predictions[version][key] = (not source['abstained'] and
                                             set(map(tuple, prediction)) == set(map(tuple, row['gold'])))
            runs[f'{version}-{mode}'] = score(records, scope)
        changes = {'fixed': [], 'broken': [], 'both_correct': 0, 'both_incorrect': 0}
        for key in rows:
            before, after = (predictions[v][key] for v in ['before', 'after'])
            if before == after:
                changes['both_correct' if before else 'both_incorrect'] += 1
            else:
                changes['fixed' if after else 'broken'].append(key)
        paired[mode] = changes
    result = {'schema': 1, 'independent_validation': False,
              'limitation': 'Public synthetic annotations; training overlap and label completeness unknown.',
              'execution': sources['after']['execution'], 'corpus': manifest,
              'source_sha256': {'before': digest(a.before), 'after': digest(a.after)},
              'measurement_sha256': digest(Path(__file__)),
              'wasm_sha256': sources['after']['wasm_sha256'], 'runs': runs, 'paired_exact_changes': paired}
    with out.open('x') as stream:
        json.dump(result, stream, indent=2)
    for name, run in runs.items():
        print(name, json.dumps(run['overall']))
    print(json.dumps({mode: {k: len(v) if isinstance(v, list) else v for k, v in c.items()}
                      for mode, c in paired.items()}))


if __name__ == '__main__':
    main()
