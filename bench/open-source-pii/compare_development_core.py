"""Compare the two pinned Icelandic development replays without exposing rows."""
import argparse
import json
from pathlib import Path

from compare_external import indexed
from evaluate import digest, private_output
from quality import score


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--before', type=Path, required=True)
    p.add_argument('--after', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    data = {name: json.loads(path.read_text()) for name, path in [('before', a.before), ('after', a.after)]}
    corpus = data['before']['corpus']
    expected = {'general': 'ddf057b278e1a143da7d1667872cf0c5d66d7a94e63aeebc57fd8b1753f3a2a0',
                'domain': '6246db44f8ba0b2a0f33228c6c6deac2c4937fe442b2e17351292baf3cbd6b87'}
    assert corpus['scope'] == ['name_demo']
    for name, value in expected.items():
        assert corpus['inputs'][name]['source_sha256'] == value
    records = {}
    for name, run in data.items():
        assert run['status'] == 'complete' and run['independent_validation'] is False
        assert run['corpus'] == corpus
        records[name] = indexed(run['records'])
        assert len(records[name]) == 5838
        assert all(r['exact_restoration'] for r in run['records'])
    assert records['before'].keys() == records['after'].keys()
    changed = 0
    for key, before in records['before'].items():
        after = records['after'][key]
        for field in ['text_sha256', 'stratum', 'gold']:
            assert before[field] == after[field]
        changed += (before['abstained'] != after['abstained'] or
                    set(map(tuple, before['predicted'])) != set(map(tuple, after['predicted'])))
    result = {'schema': 1, 'independent_validation': False,
              'scope': 'Person-only reused Icelandic development with frozen cached names; not full-entity accuracy',
              'corpus': corpus, 'source_sha256': {'before': digest(a.before), 'after': digest(a.after)},
              'measurement_sha256': digest(Path(__file__)), 'changed_excerpts': changed,
              'before': score(list(records['before'].values()), ['name_demo']),
              'after': score(list(records['after'].values()), ['name_demo'])}
    with out.open('x') as stream:
        json.dump(result, stream, indent=2)
    print(json.dumps({'changed_excerpts': changed, 'before': result['before']['overall'], 'after': result['after']['overall']}))


if __name__ == '__main__':
    main()
