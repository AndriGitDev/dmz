#!/usr/bin/env python3
"""Measure the frozen Person reference under the excerpt-accuracy contract.

Reads existing private development caches only. Does not infer, tune, or read
blind annotation packets. Output is aggregate and explicitly Person-scoped.
"""
import argparse
import json
import math
from pathlib import Path

from ensemble import merge
from ner import external, sha, spans, tokens_and_labels
from quality import score


def evaluate(source, crf_path, neural_path, selection_path, dataset):
    source, crf_path, neural_path = map(external, (source, crf_path, neural_path))
    selection = json.loads(selection_path.read_text())
    crf, neural = (json.loads(p.read_text()) for p in (crf_path, neural_path))
    source_hash = sha(source)
    if selection.get('frozen') is not True:
        raise ValueError('requires a frozen reference')
    if selection['evaluation_inputs'][dataset] != source_hash:
        raise ValueError('source differs from frozen development input')
    mode = neural.get('text_mode')
    if mode not in ('canonical', 'original'):
        raise ValueError('explicit text mode required')
    for cache, name in ((crf, 'crf'), (neural, 'neural')):
        if (cache.get('split') != 'dev' or cache.get('input_sha256') != source_hash or
                cache.get('text_mode') != mode or
                cache.get('model_sha256') != selection['composition'][name + '_sha256']):
            raise ValueError('cache identity mismatch')
    thresholds = selection['thresholds']
    if thresholds != dict(crf=.75, neural=0):
        raise ValueError('unexpected frozen thresholds')
    rows = [json.loads(line) for line in source.read_text().splitlines()]
    if len(rows) != len(crf['records']) or len(rows) != len(neural['records']):
        raise ValueError('cache record count mismatch')
    records = []
    for row, c, n in zip(rows, crf['records'], neural['records']):
        if row.get('split') != 'dev' or not isinstance(row.get('entities'), list):
            raise ValueError('requires labelled development rows')
        text = row['original_text'] if mode == 'original' else row['text']
        data = text.encode('utf-8')
        _, offsets, labels = tokens_and_labels(row, text_mode=mode)
        gold = [[a, b] for a, b, _, _ in spans(labels, offsets)]
        for cache_row in (c, n):
            if cache_row.get('gold') != gold or not isinstance(cache_row.get('predictions'), list):
                raise ValueError('cached gold differs from source annotations')
            seen = set()
            for prediction in cache_row['predictions']:
                if not isinstance(prediction, list) or len(prediction) != 3:
                    raise ValueError('invalid prediction')
                a, b, confidence = prediction
                if (type(a) is not int or type(b) is not int or not 0 <= a < b <= len(data) or
                        isinstance(confidence, bool) or not isinstance(confidence, (int, float)) or
                        not math.isfinite(confidence) or not 0 <= confidence <= 1 or (a, b) in seen):
                    raise ValueError('invalid prediction')
                data[:a].decode('utf-8'); data[a:b].decode('utf-8'); data[b:].decode('utf-8')
                seen.add((a, b))
        predicted = merge(c['predictions'], n['predictions'], thresholds['crf'], thresholds['neural'])
        records.append(dict(gold=[(a, b, 'Person') for a, b in gold],
                            predicted=[(a, b, 'Person') for a, b in predicted], stratum=dataset))
    report = score(records, ['Person'])
    report.update(scope='Reused development; Person only, not all-entity or dmz accuracy',
                  text_mode=mode, input_sha256=source_hash,
                  crf_cache_sha256=sha(crf_path), neural_cache_sha256=sha(neural_path),
                  selection_sha256=sha(selection_path), thresholds=thresholds)
    return report


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    for key in ('source', 'crf', 'neural', 'selection'):
        parser.add_argument('--' + key, type=Path, required=True)
    parser.add_argument('--dataset', choices=['general', 'domain'], required=True)
    args = parser.parse_args()
    print(json.dumps(evaluate(args.source, args.crf, args.neural, args.selection, args.dataset), indent=2))
