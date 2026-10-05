"""Prepare only the two already-used Icelandic dev sets for private core replay."""
import argparse
import hashlib
import json
from pathlib import Path
from evaluate import ROOT, digest
from evaluate_development import PRIVATE, PINNED, validate_reference, tokens_and_labels, spans, merge


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--output', type=Path, required=True)
    a = p.parse_args()
    out = a.output.resolve()
    if not out.is_relative_to(PRIVATE / 'experiments') or out.exists():
        raise ValueError('new private experiment directory required')
    selected = PRIVATE / 'experiments/full-precision-v1/frozen-selection.json'
    records, inputs = [], {}
    for dataset in ['general', 'domain']:
        src = PRIVATE / ('datasets/prepared-v2/dev.jsonl' if dataset == 'general' else 'experiments/recall-v2/domain-dev.jsonl')
        if digest(src) != PINNED[dataset]:
            raise ValueError('wrong development source')
        crf_path = PRIVATE / 'experiments/recall-v2' / ('crf-predictions-dev.json' if dataset == 'general' else 'crf-predictions-domain-original.json')
        neural_path = PRIVATE / 'experiments/full-precision-v1' / ('selected-repeat-general-predictions.json' if dataset == 'general' else 'selected-repeat-domain-original-predictions.json')
        validate_reference(src, crf_path, neural_path, selected, dataset)
        crf, neural = (json.loads(path.read_text()) for path in [crf_path, neural_path])
        mode = neural['text_mode']
        inputs[dataset] = {'source_sha256': digest(src), 'crf_sha256': digest(crf_path),
                           'neural_sha256': digest(neural_path), 'text_mode': mode}
        rows = [json.loads(line) for line in src.read_text().splitlines()]
        for i, (row, c, n) in enumerate(zip(rows, crf['records'], neural['records'], strict=True)):
            text = row['original_text'] if mode == 'original' else row['text']
            _, offsets, labels = tokens_and_labels(row, text_mode=mode)
            names = [{'start': start, 'end': end} for start, end in sorted(merge(c['predictions'], n['predictions'], .75, 0))]
            records.append({'id': f'{dataset}-{i}', 'stratum': dataset, 'text': text,
                            'text_sha256': hashlib.sha256(text.encode()).hexdigest(), 'names': names,
                            'gold': [[s, e, 'name_demo'] for s, e, _, _ in spans(labels, offsets)]})
    if len(records) != 5838:
        raise ValueError('incomplete development inputs')
    raw = ''.join(json.dumps(row, ensure_ascii=False) + '\n' for row in records).encode()
    manifest = {'schema': 1, 'scope': ['name_demo'], 'independent_validation': False,
                'inputs': inputs, 'selection_sha256': digest(selected), 'excerpts': len(records),
                'records_sha256': hashlib.sha256(raw).hexdigest(), 'measurement_sha256': digest(Path(__file__)),
                'limitation': 'Reused Person-only development with frozen cached names; not browser neural inference or full entity accuracy.'}
    out.mkdir()
    (out / 'records.jsonl').write_bytes(raw)
    (out / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps({'excerpts': len(records), 'records_sha256': manifest['records_sha256']}))


if __name__ == '__main__':
    main()
