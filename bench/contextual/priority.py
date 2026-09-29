#!/usr/bin/env python3
"""Fixed-threshold development comparison of overlap priority from private caches."""
import argparse
import json
from pathlib import Path
from encoder import digest
from ensemble import merge
from ner import external


def compare(crf, neural):
    for key in ('split', 'input_sha256', 'text_mode'):
        if crf[key] != neural[key]:
            raise ValueError('cache identity mismatch')
    if crf['split'] != 'dev' or not neural['records']:
        raise ValueError('nonempty development records required')
    counts = dict(tp=0, fp=0, fn=0)
    changes = dict(changed_records=0, recovered_spans=0, added_missed_spans=0,
                   added_false_positives=0, removed_false_positives=0)
    for c, n in zip(crf['records'], neural['records'], strict=True):
        if c['gold'] != n['gold']:
            raise ValueError('annotation mismatch')
        gold = {tuple(s) for s in n['gold']}
        old = merge(c['predictions'], n['predictions'], .75, 0)
        new = merge(n['predictions'], c['predictions'], 0, .75)
        counts['tp'] += len(new & gold); counts['fp'] += len(new - gold); counts['fn'] += len(gold - new)
        changes['changed_records'] += old != new
        changes['recovered_spans'] += len((new - old) & gold)
        changes['added_missed_spans'] += len((old - new) & gold)
        changes['added_false_positives'] += len((new - old) - gold)
        changes['removed_false_positives'] += len((old - new) - gold)
    counts['precision'] = counts['tp'] / max(1, counts['tp'] + counts['fp'])
    counts['recall'] = counts['tp'] / max(1, counts['tp'] + counts['fn'])
    counts['target_met'] = counts['precision'] >= .955 and counts['recall'] >= .955
    return dict(schema=1, split='dev', input_sha256=neural['input_sha256'], text_mode=neural['text_mode'],
                model_sha256=neural['model_sha256'], crf_sha256=crf['model_sha256'],
                thresholds=dict(crf=.75, neural=0), composition='neural_priority',
                scores=counts, versus_crf_priority=changes, independent_final_evaluation=False,
                method='Reused development; deterministic overlap-priority alternative, no threshold search')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(__doc__)
    for name in ('crf', 'neural', 'output'):
        parser.add_argument('--' + name, type=Path, required=True)
    args = parser.parse_args()
    crf, neural, output = map(external, (args.crf, args.neural, args.output))
    if output.exists(): raise ValueError('refusing to overwrite comparison')
    result = compare(json.loads(crf.read_text()), json.loads(neural.read_text()))
    result['prediction_cache_sha256'] = dict(crf=digest(crf), neural=digest(neural))
    with output.open('x') as stream: json.dump(result, stream, indent=2)
    print(json.dumps(result, indent=2))
