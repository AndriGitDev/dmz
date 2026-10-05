"""Freeze a resource-bounded public English sample before candidate predictions."""
import argparse
import json
from pathlib import Path
from evaluate import digest, private_output
from evaluate_external import TEST_SHA
from external_selection import SEED, COUNT, selected_ids, load_selection


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--prior-partial', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    if digest(a.corpus / 'test.jsonl') != TEST_SHA:
        raise ValueError('wrong source')
    rows = [json.loads(line) for line in (a.corpus / 'test.jsonl').read_text().splitlines()]
    # Only identifiers are used from the stopped prefix; never outcomes or scores.
    excluded = sorted({json.loads(line)['id'] for line in a.prior_partial.read_text().splitlines()})
    ids = selected_ids(rows, set(excluded))
    selected = {r['id']: r for r in rows if r['id'] in set(ids)}
    result = {'schema': 1, 'seed': SEED, 'count': COUNT, 'corpus_test_sha256': TEST_SHA,
              'reason': 'Full Privacy Filter run projected beyond the requested work slice from elapsed runtime; fixed comparison sample for all tools.',
              'excluded_previously_inferred_ids': excluded, 'prior_partial_sha256': digest(a.prior_partial),
              'ids': ids, 'text_sha256': {key: selected[key]['text_sha256'] for key in ids},
              'independent_validation': False, 'measurement_sha256': digest(Path(__file__))}
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    load_selection(out, rows, TEST_SHA)
    print(json.dumps({'selected': len(ids), 'excluded_previously_inferred': len(excluded), 'selection_sha256': digest(out)}))


if __name__ == '__main__':
    main()
