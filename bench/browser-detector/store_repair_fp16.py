#!/usr/bin/env python3
"""Reduce stored correction bytes while preserving the quantized backbone/head."""
import argparse
import json
import math
from pathlib import Path
import time
from store_fp16 import convert, digest, external


def require_quality_preservation(quality, source_sha, reference_scores):
    if quality['model_sha256'] != source_sha or quality.get('text_mode') != 'canonical':
        raise ValueError('quality report does not match the canonical parent graph')
    if quality.get('independent_validation') is not False or quality.get('externalRequests') != []:
        raise ValueError('requires the disclosed development Chromium comparison')
    if not quality.get('runtime', '').startswith('onnxruntime-web ') or not quality.get('browser'):
        raise ValueError('requires an actual browser report')
    rows = quality['sets']
    if len(rows) != 2 or {r['name'] for r in rows} != {'general', 'domain'}:
        raise ValueError('both development sets are required')
    for row in rows:
        reference = reference_scores[row['name']]
        if any(row['fp32_browser_combined'][key] != reference[key] for key in ['tp', 'fp', 'fn', 'precision', 'recall']):
            raise ValueError('browser reference differs from frozen selection')
        score = row['combined']
        if any(type(score[k]) is not int or score[k] < 0 for k in ['tp', 'fp', 'fn']):
            raise ValueError('invalid quality counts')
        if score['tp'] + score['fn'] != reference['tp'] + reference['fn']:
            raise ValueError('quality counts do not cover the reference gold spans')
        for metric, denominator in [('precision', score['tp']+score['fp']), ('recall', score['tp']+score['fn'])]:
            if denominator <= 0 or not math.isclose(score[metric], score['tp']/denominator, rel_tol=0, abs_tol=1e-15):
                raise ValueError('invalid quality score arithmetic')
            if score[metric] < reference[metric]:
                raise ValueError('parent does not preserve frozen-reference quality')


def main():
    p = argparse.ArgumentParser(__doc__)
    p.add_argument('--assets', type=Path, required=True)
    p.add_argument('--selection', type=Path, required=True)
    p.add_argument('--source-model', choices=['repair1', 'repair2'], required=True)
    p.add_argument('--quality-report', type=Path, required=True)
    a = p.parse_args(); assets = external(a.assets)
    selected = json.loads(a.selection.read_text())
    parent = json.loads((assets / (a.source_model + '.json')).read_text())
    frozen = json.loads((assets / 'export.json').read_text())
    source = assets / (a.source_model + '.onnx')
    output = assets / 'repairhalf.onnx'; report_path = assets / 'repairhalf-compression.json'
    if output.exists() or report_path.exists():
        raise ValueError('refusing to overwrite candidate')
    if selected.get('frozen') is not True or selected['thresholds'] != dict(crf=.75, neural=0):
        raise ValueError('requires frozen reference and unchanged thresholds')
    selection_sha = digest(a.selection)
    if parent['selection_sha256'] != selection_sha or frozen['selection_sha256'] != selection_sha:
        raise ValueError('selection changed')
    quality = json.loads(a.quality_report.read_text())
    require_quality_preservation(quality, parent['sha256'], selected['candidate']['scores'])
    if digest(source) != parent['sha256'] or digest(assets/'fp32.onnx') != frozen['model_sha256']:
        raise ValueError('parent or FP32 reference changed')
    if digest(assets/'int4.onnx') != parent['base_quantized_sha256']:
        raise ValueError('quantized backbone changed')
    import onnx
    started = time.monotonic(); graph = onnx.load(str(source))
    before = {v.name: v.SerializeToString() for v in graph.graph.initializer}
    converted = convert(graph, node_suffixes=('__repair_A', '__repair_B'))
    if converted != dict(MatMul=144, Gather=0):
        raise ValueError('expected exactly 144 correction matrices')
    after = {v.name: v for v in graph.graph.initializer}
    removed = set(before) - set(after)
    if len(removed) != 144 or len(before) != len(after):
        raise ValueError('unexpected initializer changes')
    for name, value in before.items():
        if name in removed:
            if after[name+'__fp16_storage'].data_type != onnx.TensorProto.FLOAT16:
                raise ValueError('correction storage is not FP16')
        elif after[name].SerializeToString() != value:
            raise ValueError('backbone or head initializer changed')
    onnx.checker.check_model(graph); onnx.save_model(graph, str(output))
    report = dict(schema=1, recipe='repairhalf', source_model=a.source_model,
        description='FP16 stored rank-16 corrections cast to FP32; preserved int4 backbone and FP32 head',
        source_sha256=parent['sha256'], reference_sha256=frozen['model_sha256'],
        source_quality_report_sha256=digest(a.quality_report),
        selection_sha256=selection_sha, sha256=digest(output), bytes=output.stat().st_size,
        converted=converted, preserved_initializers=len(before)-len(removed),
        seconds=time.monotonic()-started, onnx=onnx.__version__, calibration=False,
        threshold_retuning=False, quality_status='not evaluated; rounding can change predictions')
    report_path.write_text(json.dumps(report, indent=2)+'\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
