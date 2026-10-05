"""Export aggregate scores and source hashes; never export prediction records/text."""
import argparse
import json
from pathlib import Path
from evaluate import ROOT, digest

GROUPS = {
    'initial-model-comparisons': ['initial-comparison.json', 'opf-comparison.json',
                                  'gravitee-comparison.json',
                                  'gliner-quint8-initial-comparison.json'],
    'gitleaks-phone-baseline-comparison': ['gitleaks-initial-comparison.json'],
    'followup-model-comparisons': ['selected-address-followup.json', 'gliner-quint8-followup-comparison.json'],
    'initial-structured-replay': ['structured-core-initial-comparison.json'],
    'followup-structured-replay': ['structured-core-followup-comparison.json'],
}


def quality(value):
    # Explicitly omit all record-level data and untrusted free-form metadata.
    return {k: value[k] for k in ['metric', 'annotation_scope', 'minimum', 'overall', 'per_type', 'strata']}


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--private', type=Path, required=True)
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    source, target = a.private.resolve(), a.output.resolve()
    if source.is_relative_to(ROOT) or not target.is_relative_to(ROOT / 'bench/results'):
        raise ValueError('expected private source and benchmark aggregate destination')
    results = {}
    for group, files in GROUPS.items():
        result = {'schema': 1, 'independent_validation': False,
                  'limitation': 'Authored synthetic diagnostics; reused comparisons and cached composition, not field acceptance.',
                  'source_sha256': {}, 'runs': {}}
        for name in files:
            path = source / name
            data = json.loads(path.read_text())
            if data['independent_validation'] is not False:
                raise ValueError('unexpected evidence classification')
            if 'corpus' in result and result['corpus'] != data['corpus']:
                raise ValueError('mixed corpora')
            result['corpus'] = data['corpus']
            reference = data.get('reference_sha256', data.get('baseline_sha256'))
            if reference:
                if 'reference_sha256' in result and result['reference_sha256'] != reference:
                    raise ValueError('mixed detector baselines')
                result['reference_sha256'] = reference
            result['source_sha256'][name] = digest(path)
            for run, scores in data['runs'].items():
                projected = quality(scores)
                if run in result['runs'] and result['runs'][run] != projected:
                    raise ValueError('conflicting duplicate run')
                result['runs'][run] = projected
        results[group] = result
    target.mkdir(parents=True, exist_ok=True)
    if any((target / (name + '.json')).exists() for name in results):
        raise ValueError('refusing to overwrite published aggregates')
    for name, result in results.items():
        with (target / (name + '.json')).open('x') as stream:
            json.dump(result, stream, indent=2)
            stream.write('\n')
    print(f'Exported {len(results)} aggregate reports without input text or prediction records')


if __name__ == '__main__':
    main()
