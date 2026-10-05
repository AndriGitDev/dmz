"""Apply the predeclared address-only validation guard without reading test data."""
import argparse
import json
import math
from pathlib import Path
from evaluate import ROOT, digest, private_output
from quality import score

THRESHOLDS = [.3, .5, .7, .9, .95, .99, .995, .999]


def wilson_lower(successes, total):
    if not total:
        return 0.0
    z = 1.959963984540054
    p = successes / total
    return (p + z*z/(2*total) - z*math.sqrt(p*(1-p)/total + z*z/(4*total*total))) / (1 + z*z/total)


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--validation', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    data = json.loads(a.validation.read_text())
    if data['status'] != 'complete' or data['split'] != 'validation' or data['candidate'] != 'gliner-published-quint8':
        raise ValueError('wrong candidate or split')
    if data['graph_sha256'] != '0514c8fd86d0513ce5351a3267f132b57d5bcd8f99a90d43cde1228092881d19':
        raise ValueError('wrong graph')
    if data['corpus']['sets']['validation']['sha256'] != '0363cb20728c0fc53b3c27cae5ecf9d03d04388ce4147907d01e9260bf5fb1de':
        raise ValueError('wrong validation corpus')
    records = data['records']
    if len(records) != 4995 or len({r['id'] for r in records}) != 4995:
        raise ValueError('incomplete records')
    trials, eligible = [], []
    for threshold in THRESHOLDS:
        filtered = [{'gold': [s for s in r['gold'] if s[2] == 'address'],
                     'predicted': [s['span'] for s in r['scored'] if s['span'][2] == 'address' and s['score'] >= threshold],
                     'abstained': r['abstained'], 'stratum': r['stratum']} for r in records]
        quality = score(filtered, ['address'])
        q = quality['overall']
        lower = wilson_lower(q['tp'], q['tp'] + q['fp'])
        passes = q['tp'] >= 50 and lower >= .99
        trial = {'threshold': threshold, 'precision_wilson95_lower': lower, 'selection_guard_passed': passes, 'quality': quality}
        trials.append(trial)
        if passes:
            eligible.append(trial)
    selected = max(eligible, key=lambda x: (x['quality']['overall']['recall'], x['threshold'])) if eligible else None
    result = {'schema': 1, 'status': 'complete', 'independent_validation': False,
              'selection': 'address-only; fixed threshold grid; validation only',
              'limitation': 'Wilson statistic is a selection guard on synthetic correlated data, not a certified field precision bound.',
              'candidate_sha256': digest(a.validation), 'measurement_sha256': digest(Path(__file__)),
              'protocol_sha256': digest(ROOT / 'bench/open-source-pii/address-confidence-protocol.md'),
              'selected_threshold': selected['threshold'] if selected else None,
              'decision': 'requires-separate-followup' if selected else 'reject-confidence-only-route', 'trials': trials}
    with out.open('x') as stream:
        json.dump(result, stream, indent=2)
    print(json.dumps({'decision': result['decision'], 'selected_threshold': result['selected_threshold']}))
    for trial in trials:
        print(trial['threshold'], json.dumps(trial['quality']['overall']), trial['precision_wilson95_lower'])


if __name__ == '__main__':
    main()
