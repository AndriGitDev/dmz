"""Publish only aggregate public/development scores, never record identifiers."""
import argparse
import json
from pathlib import Path

from evaluate import ROOT, digest
from export_summary import quality


def count_changes(changes):
    return {name: {key: len(value) if isinstance(value, list) else value
                   for key, value in values.items()
                   if key in {'fixed', 'broken', 'both_correct', 'both_incorrect'}}
            for name, values in changes.items()}


def project(data, kind):
    if data['independent_validation'] is not False:
        raise ValueError('unexpected evidence classification')
    result = {'schema': 1, 'independent_validation': False, 'kind': kind}
    if kind in {'public-models', 'public-core'}:
        result['limitation'] = 'Public synthetic annotations; training overlap and label completeness unknown.'
        # Scope/provenance only: omit any record IDs, source paths and raw rows.
        corpus = data['corpus']
        result['source_revision'] = corpus['source']['revision']
        result['annotation_scope'] = corpus['scope']
        result['runs'] = {name: quality(scores) for name, scores in data['runs'].items()}
        result['paired_exact_changes'] = count_changes(data['paired_exact_changes'])
        if kind == 'public-models':
            result['evaluated_excerpts'] = data['evaluated_excerpts']
            result['reference_sha256'] = data['reference_sha256']
            result['candidate_hashes'] = data['candidate_hashes']
            selection = data.get('selection')
            result['selection_manifest_sha256'] = selection['manifest_sha256'] if selection else None
        else:
            result['execution'] = 'WASM core replay with unchanged cached browser name spans; no neural inference rerun'
            result['input_sha256'] = data['source_sha256']
            result['wasm_sha256'] = data['wasm_sha256']
    elif kind == 'development-core':
        result['limitation'] = 'Person-only reused Icelandic development; cached names, not full entity accuracy or independent validation.'
        result['input_sha256'] = data['source_sha256']
        result['changed_excerpts'] = data['changed_excerpts']
        result['runs'] = {name: quality(data[name]) for name in ['before', 'after']}
    elif kind == 'address-selection':
        if data['status'] != 'complete':
            raise ValueError('incomplete validation')
        result['limitation'] = 'Validation-only confidence selection on public synthetic data; not certified field precision.'
        for key in ['candidate_sha256', 'measurement_sha256', 'protocol_sha256', 'selected_threshold', 'decision']:
            result[key] = data[key]
        result['trials'] = [dict(threshold=t['threshold'],
                                 precision_wilson95_lower=t['precision_wilson95_lower'],
                                 selection_guard_passed=t['selection_guard_passed'],
                                 quality=quality(t['quality'])) for t in data['trials']]
    return result


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--source', type=Path, required=True)
    p.add_argument('--kind', choices=['public-models', 'public-core', 'development-core', 'address-selection'], required=True)
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    source, output = a.source.resolve(), a.output.resolve()
    if source.is_relative_to(ROOT) or not output.is_relative_to(ROOT / 'bench/results'):
        raise ValueError('private input and benchmark aggregate destination required')
    result = project(json.loads(source.read_text()), a.kind)
    result['private_report_sha256'] = digest(source)
    result['exporter_sha256'] = digest(Path(__file__))
    with output.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    print('Exported aggregate scores and hashes without prediction records or identifiers')


if __name__ == '__main__':
    main()
