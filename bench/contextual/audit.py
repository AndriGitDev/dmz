#!/usr/bin/env python3
"""Development-only error audit. Aggregate stdout; optional private review JSONL."""
import argparse
from collections import Counter, defaultdict
import json
from pathlib import Path
import re
from ner import external, features, rows, sha, spans, tokens_and_labels


def overlap(a, b):
    return a[0] < b[1] and b[0] < a[1]


def error_categories(gold, selected, candidates, other):
    """Exclusive explanations; partial matches count as both FP and FN."""
    missed, false = {}, {}
    for span in gold - selected:
        if span in candidates:
            missed[span] = 'removed_by_confidence_threshold'
        elif any(overlap(span, p) for p in candidates):
            missed[span] = 'boundary_or_merged_name'
        else:
            missed[span] = 'no_name_prediction'
    for span in selected - gold:
        if any(overlap(span, g) for g in gold):
            false[span] = 'boundary_or_merged_name'
        elif span in other:
            false[span] = 'other_entity_' + other[span]
        elif any(overlap(span, g) for g in other):
            false[span] = 'overlaps_other_entity'
        else:
            false[span] = 'unannotated_as_entity'
    return missed, false


def training_inventory(source):
    names, person_tokens, all_tokens = Counter(), Counter(), Counter()
    for row in rows(source, 'train'):
        tokens, offsets, labels = tokens_and_labels(row)
        all_tokens.update(t.lower() for t in tokens)
        for a, b, i, j in spans(labels, offsets):
            names.update([' '.join(tokens[i:j]).lower()])
            person_tokens.update(t.lower() for t in tokens[i:j])
    return names, person_tokens, all_tokens


def characteristics(words, inventory):
    names, person_tokens, all_tokens = inventory
    lower = [t.lower() for t in words]
    groups = ['single_token' if len(words) == 1 else 'multiple_tokens']
    groups += ['full_name_seen_in_training' if ' '.join(lower) in names else 'full_name_unseen_in_training']
    groups += ['all_components_seen_as_person' if all(w in person_tokens for w in lower) else 'unseen_person_component']
    if any(w not in all_tokens for w in lower): groups.append('unseen_word')
    if all(t.isupper() for t in words): groups.append('all_uppercase')
    if all(t.islower() for t in words): groups.append('all_lowercase')
    if any(re.fullmatch(r'[^\W\d_]\.?', t) for t in words): groups.append('initial')
    if any('-' in t for t in words): groups.append('hyphenated')
    if any(re.search(r'(sonar|syni|dóttur|burs)$', t) for t in lower): groups.append('oblique_patronymic_shape')
    if any(w in person_tokens and person_tokens[w] / all_tokens[w] < .5 for w in lower): groups.append('ambiguous_component')
    return groups


def audit(source, training, model, threshold, errors_out=None):
    import pycrfsuite
    inventory = training_inventory(training)
    metadata = json.loads(external(model).with_suffix('.metadata.json').read_text())
    if metadata['model_sha256'] != sha(model): raise ValueError('model checksum mismatch')
    if metadata['training_sha256'] != sha(training): raise ValueError('training checksum mismatch')
    tagger = pycrfsuite.Tagger(); tagger.open(str(model))
    fn, fp, counts = Counter(), Counter(), Counter()
    groups = defaultdict(Counter)
    details = []
    for row in rows(source, 'dev'):
        tokens, offsets, labels = tokens_and_labels(row)
        gold_spans = spans(labels, offsets)
        gold = {(a,b) for a,b,_,_ in gold_spans}
        predicted = tagger.tag(features(tokens, metadata.get('feature_version', 1)))
        candidates = {(a,b): min(tagger.marginal(predicted[k], k) for k in range(i,j)) for a,b,i,j in spans(predicted, offsets)}
        selected = {p for p, c in candidates.items() if c >= threshold}
        other = {(e['start'],e['end']):e['kind'] for e in row['entities'] if e['kind'] != 'Person'}
        missed, false = error_categories(gold, selected, candidates, other)
        fn.update(missed.values()); fp.update(false.values())
        counts.update(sentences=1, tp=len(gold & selected), fp=len(false), fn=len(missed))
        for a,b,i,j in gold_spans:
            for group in characteristics(tokens[i:j], inventory):
                groups[group]['gold'] += 1
                groups[group]['correct' if (a,b) in selected else 'missed'] += 1
        if errors_out and (missed or false):
            details.append(dict(id=row.get('id'), text=row['text'], entities=row['entities'],
                                missed=[dict(start=a,end=b,reason=r) for (a,b),r in missed.items()],
                                false=[dict(start=a,end=b,reason=r) for (a,b),r in false.items()],
                                predictions=[dict(start=a,end=b,score=c) for (a,b),c in candidates.items()]))
    if not counts['sentences']: raise ValueError('empty development corpus')
    if errors_out:
        with external(errors_out).open('x') as f:
            for item in details: f.write(json.dumps(item, ensure_ascii=False)+'\n')
    return dict(schema=1, split='dev', model_sha256=sha(model), input_sha256=sha(source), training_sha256=sha(training),
                threshold=threshold, counts=dict(counts), false_negatives=dict(fn), false_positives=dict(fp),
                gold_characteristics=dict(groups),
                caveats='Exclusive error reasons; overlapping characteristic groups. Automated shape/coverage audit, not human adjudication. No test data inspected.')


if __name__ == '__main__':
    p=argparse.ArgumentParser(description=__doc__)
    for flag in ('input','training','model'): p.add_argument('--'+flag,type=Path,required=True)
    p.add_argument('--threshold',type=float,default=.65)
    p.add_argument('--errors-out',type=Path)
    a=p.parse_args()
    if not 0 <= a.threshold <= 1:p.error('invalid threshold')
    print(json.dumps(audit(a.input,a.training,a.model,a.threshold,a.errors_out),indent=2))
