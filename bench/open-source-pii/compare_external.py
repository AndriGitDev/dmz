"""Compare complete public-English caches without treating them as blind evidence."""
import argparse
import hashlib
import json
from pathlib import Path
from evaluate import digest, private_output
from evaluate_external import TEST_SHA
from compare import add_nonoverlapping
from quality import score


def indexed(records):
    result = {r['id']: r for r in records}
    if len(result) != len(records):
        raise ValueError('duplicate cache identifier')
    return result


def compare(reference_path, candidate_paths, corpus, selection_path=None):
    manifest = json.loads((corpus / 'manifest.json').read_text())
    if digest(corpus / 'test.jsonl') != TEST_SHA:
        raise ValueError('wrong public projection')
    all_rows = indexed([json.loads(s) for s in (corpus / 'test.jsonl').read_text().splitlines()])
    rows, selection = all_rows, None
    if selection_path:
        from external_selection import load_selection
        selected, selection = load_selection(selection_path, list(all_rows.values()), TEST_SHA)
        rows = indexed(selected)
    reference = json.loads(reference_path.read_text())
    if reference['status'] != 'complete' or reference['corpus'] != manifest or reference['external_requests']:
        raise ValueError('incomplete or incompatible browser baseline')
    base = indexed(reference['records'])
    if set(base) != set(all_rows):
        raise ValueError('incomplete browser baseline')
    for key, row in rows.items():
        if row['text_sha256'] != hashlib.sha256(row['text'].encode()).hexdigest() or base[key]['text_sha256'] != row['text_sha256']:
            raise ValueError('baseline text mismatch')
        if not all(base[key][m]['exact_restoration'] for m in ['basic', 'trained']):
            raise ValueError('restoration failure')
    scope = set(manifest['scope'])

    def record(row, prediction, abstained):
        # Resolve overlaps against ALL core spans before projecting the metric scope.
        return {'gold': row['gold'], 'predicted': [s for s in prediction if s[2] in scope],
                'abstained': abstained, 'stratum': row['stratum']}

    runs = {}
    baseline_records = {}
    for mode in ['basic', 'trained']:
        baseline_records[mode] = [record(r, base[k][mode]['predicted'], base[k][mode]['abstained']) for k, r in rows.items()]
        runs['frozen-' + mode] = score(baseline_records[mode], scope)
    result = {'schema': 1, 'independent_validation': False,
              'limitation': 'Public synthetic annotations; training overlap and label completeness unknown.',
              'reference_sha256': digest(reference_path), 'corpus': manifest,
              'selection': selection, 'evaluated_excerpts': len(rows),
              'runs': runs, 'candidate_hashes': {}, 'paired_exact_changes': {}}
    for path in candidate_paths:
        data = json.loads(path.read_text())
        partial = Path(str(path) + '.partial.jsonl')
        if data['status'] != 'complete' or data['corpus'] != manifest or digest(partial) != data['partial_sha256']:
            raise ValueError('incomplete or changed candidate')
        predictions = indexed([json.loads(s) for s in partial.read_text().splitlines()])
        candidate_selection = data.get('selection')
        if candidate_selection:
            if not selection or candidate_selection['manifest_sha256'] != selection['manifest_sha256'] or set(predictions) != set(rows):
                raise ValueError('wrong candidate selection')
        elif set(predictions) != set(all_rows):
            raise ValueError('incomplete candidate records')
        predictions = {key: predictions[key] for key in rows}
        for key, pred in predictions.items():
            if pred['text_sha256'] != rows[key]['text_sha256'] or pred['gold'] != rows[key]['gold']:
                raise ValueError('candidate text or gold mismatch')
        name = data['candidate']
        if name in result['candidate_hashes']:
            raise ValueError('duplicate candidate name')
        result['candidate_hashes'][name] = digest(path)
        runs[name + '-alone'] = score(list(predictions.values()), scope)
        # The address-only GLiNER2 choice was fixed before fresh evaluation.
        variants = ['all', 'address-only'] if name in {'gliner2', 'gliner2-onnx-fp32'} else ['all']
        for variant in variants:
            for mode in ['basic', 'trained']:
                records = []
                for key, row in rows.items():
                    original = base[key][mode]
                    extra = predictions[key]['scored']
                    if variant == 'address-only':
                        extra = [s for s in extra if s['span'][2] == 'address']
                    combined = [] if original['abstained'] else add_nonoverlapping(original['predicted'], extra)
                    records.append(record(row, combined, original['abstained']))
                run_name = f'{name}-{variant}-plus-{mode}'
                runs[run_name] = score(records, scope)
                changed = {'fixed': 0, 'broken': 0, 'both_correct': 0, 'both_incorrect': 0}
                for before, after in zip(baseline_records[mode], records):
                    correct = lambda r: not r['abstained'] and set(map(tuple, r['gold'])) == set(map(tuple, r['predicted']))
                    b, a = correct(before), correct(after)
                    changed['both_correct' if b and a else 'broken' if b else 'fixed' if a else 'both_incorrect'] += 1
                result['paired_exact_changes'][run_name] = changed
    return result


if __name__ == '__main__':
    p = argparse.ArgumentParser()
    p.add_argument('--reference', type=Path, required=True)
    p.add_argument('--candidate', type=Path, action='append', required=True)
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--output', required=True)
    p.add_argument('--selection', type=Path)
    a = p.parse_args()
    out = private_output(a.output)
    result = compare(a.reference, a.candidate, a.corpus, a.selection)
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    for name, run in result['runs'].items():
        r = run['overall']
        print(f'{name}: exact {r["exact_excerpts"]}/{r["excerpts"]}; TP/FP/FN {r["tp"]}/{r["fp"]}/{r["fn"]}')
