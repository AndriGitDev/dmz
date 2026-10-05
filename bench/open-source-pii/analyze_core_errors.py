"""Post-evaluation typed-error relations; no labels, policies or scores are tuned."""
import argparse
from collections import Counter
import json
from pathlib import Path

from compare_external import indexed
from evaluate import ROOT, digest
from evaluate_external import TEST_SHA


def nested(counter):
    result = {}
    for (kind, relation), count in sorted(counter.items()):
        result.setdefault(kind, {})[relation] = count
    return result


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--private', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    private, output = a.private.resolve(), a.output.resolve()
    if private.is_relative_to(ROOT) or not output.is_relative_to(ROOT / 'bench/results'):
        raise ValueError('private evidence and aggregate output required')
    corpus = private / 'datasets/gretel-projected-v2'
    assert digest(corpus / 'test.jsonl') == TEST_SHA
    manifest = json.loads((corpus / 'manifest.json').read_text())
    scope = set(manifest['scope'])
    rows = indexed([json.loads(line) for line in (corpus / 'test.jsonl').read_text().splitlines()])
    reference = private / 'external-contained-token-core-replay.json'
    replay = json.loads(reference.read_text())
    assert replay['status'] == 'complete' and replay['external_requests'] == 0
    assert replay['corpus'] == manifest
    records = indexed(replay['records'])
    assert records.keys() == rows.keys() and len(rows) == 4996
    published = ROOT / 'bench/results/open-source-pii-v1/public-current-core-full-models.json'
    scores = json.loads(published.read_text())
    assert scores['reference_sha256'] == digest(reference)
    false, missing, pairs = Counter(), Counter(), Counter()
    for key, record in records.items():
        row = rows[key]
        assert record['text_sha256'] == row['text_sha256'] and record['stratum'] == row['stratum']
        assert record['trained']['exact_restoration']
        gold = {tuple(s) for s in row['gold']}
        # Exact quality uses projected types; masking overlap diagnostics retain
        # all core types so an out-of-scope covering mask is not called absent.
        all_predictions = {tuple(s) for s in record['trained']['predicted']}
        predicted = {s for s in all_predictions if s[2] in scope}
        for span in predicted - gold:
            hits = [g for g in gold if span[0] < g[1] and g[0] < span[1]]
            if not hits:
                relation = 'no_projected_gold_overlap'
            elif len(hits) > 1:
                relation = 'multiple_gold_spans'
            elif span[:2] == hits[0][:2]:
                relation = 'equal_span_wrong_type'
            elif hits[0][0] <= span[0] and span[1] <= hits[0][1]:
                relation = 'contained_in_gold'
            elif span[0] <= hits[0][0] and hits[0][1] <= span[1]:
                relation = 'contains_gold'
            else:
                relation = 'partial_overlap'
            false[span[2], relation] += 1
            for g in hits:
                pairs[span[2], g[2]] += 1
        for g in gold - predicted:
            hits = [s for s in all_predictions if s[0] < g[1] and g[0] < s[1]]
            if record['trained']['abstained']:
                # This best-effort replay restores exactly after whole-text
                # masking; unavailable entity spans do not mean no protection.
                relation = 'whole_text_abstention'
            elif not hits:
                relation = 'no_entity_span_overlap'
            elif any(s[0] <= g[0] and g[1] <= s[1] for s in hits):
                relation = 'covered_by_wrong_span_or_type'
            else:
                relation = 'partial_overlap'
            missing[g[2], relation] += 1
    per_type = scores['runs']['frozen-trained']['per_type']
    for kind, values in per_type.items():
        assert sum(v for (k, _), v in false.items() if k == kind) == values['fp']
        assert sum(v for (k, _), v in missing.items() if k == kind) == values['fn']
    result = {
        'schema': 1, 'independent_validation': False,
        'classification': 'Post-evaluation diagnostic; no relabelling or policy selection',
        'limitation': 'Overlap with projected synthetic labels; absence of gold overlap does not establish benign text or field errors.',
        'excerpts': len(rows), 'false_span_relations': nested(false),
        'missed_gold_relations': nested(missing), 'false_to_gold_overlap_pairs': nested(pairs),
        'pair_limitation': 'A false span can overlap multiple gold spans; pair counts are not distinct-span counts.',
        'private_reference_sha256': digest(reference), 'corpus_test_sha256': TEST_SHA,
        'published_score_sha256': digest(published), 'measurement_sha256': digest(Path(__file__)),
    }
    with output.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    print('Classified typed errors and reconciled every type with the published scores')


if __name__ == '__main__':
    main()
