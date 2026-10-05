"""Attribute additive-composition errors on complete pinned public caches.

This is a post-evaluation diagnostic, not a new candidate or an oracle score.
Only counts/hashes are written; retained or rejected text/record IDs are omitted.
"""
import argparse
from collections import Counter
import json
from pathlib import Path

from compare import add_nonoverlapping
from compare_external import compare, indexed
from evaluate import digest, private_output


def overlaps(a, b):
    return a[0] < b[1] and b[0] < a[1]


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--reference', type=Path, required=True)
    p.add_argument('--candidate', type=Path, required=True)
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    # Reuse the full cache/projection/gold/restoration/provenance checks.
    checked = compare(a.reference, [a.candidate], a.corpus)
    baseline = indexed(json.loads(a.reference.read_text())['records'])
    data = json.loads(a.candidate.read_text())
    candidate = indexed([json.loads(s) for s in Path(str(a.candidate) + '.partial.jsonl').read_text().splitlines()])
    scope = set(checked['corpus']['scope'])
    totals, type_pairs = {}, {}
    for mode in ['basic', 'trained']:
        counts, pairs = Counter(), Counter()
        for key, row in candidate.items():
            core = baseline[key][mode]
            gold = set(map(tuple, row['gold']))
            existing = list(map(tuple, core['predicted']))
            scored = row['scored']
            final = [] if core['abstained'] else add_nonoverlapping(core['predicted'], scored)
            final_set = {tuple(s) for s in final if s[2] in scope}
            retained = {s for s in existing if s in final_set}
            counts['excerpts'] += 1
            counts['retained_core_tp'] += len(retained & gold)
            counts['retained_core_fp'] += len(retained - gold)
            counts['added_candidate_tp'] += len((final_set - retained) & gold)
            counts['added_candidate_fp'] += len((final_set - retained) - gold)
            for span in {tuple(s['span']) for s in scored} & gold:
                if span in final_set:
                    counts['candidate_correct_present_in_final'] += 1
                elif core['abstained']:
                    counts['candidate_correct_lost_to_core_abstention'] += 1
                else:
                    blockers = [s for s in existing if overlaps(s, span)]
                    if blockers:
                        counts['candidate_correct_blocked_by_core'] += 1
                        for blocker in blockers:
                            label = 'correct' if blocker in gold else 'outside_scope' if blocker[2] not in scope else 'incorrect'
                            pairs[f'{label}:{blocker[2]} -> {span[2]}'] += 1
                    else:
                        counts['candidate_correct_lost_to_candidate_overlap'] += 1
        run = checked['runs'][f'{data["candidate"]}-all-plus-{mode}']['overall']
        assert counts['retained_core_tp'] + counts['added_candidate_tp'] == run['tp']
        assert counts['retained_core_fp'] + counts['added_candidate_fp'] == run['fp']
        totals[mode], type_pairs[mode] = dict(counts), dict(sorted(pairs.items()))
    result = {'schema': 1, 'independent_validation': False,
              'limitation': 'Post-evaluation attribution on public synthetic annotations; not a replacement-policy trial or field accuracy.',
              'reference_sha256': digest(a.reference), 'candidate_sha256': digest(a.candidate),
              'measurement_sha256': digest(Path(__file__)), 'candidate': data['candidate'],
              'counts': totals, 'core_blocker_type_pairs': type_pairs}
    with out.open('x') as stream:
        json.dump(result, stream, indent=2)
    print(json.dumps(result, indent=2))


if __name__ == '__main__':
    main()
