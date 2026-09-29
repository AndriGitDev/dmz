#!/usr/bin/env python3
"""Store encoder constants in FP16, explicitly cast to FP32 before computation."""
import argparse
import json
from pathlib import Path
import sys
import time
sys.path.insert(0, str(Path(__file__).resolve().parents[1] / 'contextual'))
from encoder import digest
from ner import external


def convert(graph, node_suffixes=None):
    import numpy as np
    from onnx import TensorProto, helper, numpy_helper
    initializers = {v.name: v for v in graph.graph.initializer}
    protected = {name for node in graph.graph.node if '/head/' in node.name for name in node.input}
    selected = {}
    for node in graph.graph.node:
        if not node.name.startswith('/encoder/') or node.op_type not in ('MatMul', 'Gather'):
            continue
        if node_suffixes is not None and not node.name.endswith(tuple(node_suffixes)):
            continue
        name = node.input[1 if node.op_type == 'MatMul' else 0]
        if name in initializers and name not in protected:
            if initializers[name].data_type != TensorProto.FLOAT:
                raise ValueError('requires FP32 source constants')
            selected[name] = node.op_type
    casts = []
    for name in selected:
        stored_name = name + '__fp16_storage'
        if stored_name in initializers:
            raise ValueError('stored initializer name collision')
        value = numpy_helper.to_array(initializers[name])
        if not np.isfinite(value).all() or np.max(np.abs(value)) > np.finfo(np.float16).max:
            raise ValueError('constant cannot be represented as finite FP16')
        stored = value.astype(np.float16)
        initializers[name].CopyFrom(numpy_helper.from_array(stored, stored_name))
        casts.append(helper.make_node('Cast', [stored_name], [name], to=TensorProto.FLOAT,
                                      name=name + '__restore_fp32'))
    original_nodes = list(graph.graph.node)
    del graph.graph.node[:]
    graph.graph.node.extend([*casts, *original_nodes])
    return {kind: sum(v == kind for v in selected.values()) for kind in ('MatMul', 'Gather')}


def main():
    p = argparse.ArgumentParser(__doc__)
    p.add_argument('--assets', type=Path, required=True)
    p.add_argument('--selection', type=Path, required=True)
    a = p.parse_args(); assets = external(a.assets)
    selected = json.loads(a.selection.read_text()); exported = json.loads((assets / 'export.json').read_text())
    if selected.get('frozen') is not True or selected['thresholds'] != dict(crf=.75, neural=0):
        raise ValueError('requires frozen reference and unchanged thresholds')
    if exported['selection_sha256'] != digest(a.selection):
        raise ValueError('selection changed')
    source = assets / 'fp32.onnx'; output = assets / 'fp16storage.onnx'
    report_path = assets / 'fp16storage-compression.json'
    if digest(source) != exported['model_sha256']:
        raise ValueError('FP32 export changed')
    if output.exists() or report_path.exists():
        raise ValueError('refusing to overwrite candidate')
    import onnx
    started = time.monotonic(); graph = onnx.load(str(source)); converted = convert(graph)
    if converted != dict(MatMul=72, Gather=3):
        raise ValueError('unexpected encoder constant coverage')
    onnx.checker.check_model(graph); onnx.save_model(graph, str(output))
    report = dict(schema=1, recipe='fp16storage',
        description='FP16 stored encoder MatMul/Gather constants cast to FP32; FP32 computation and head',
        reference_sha256=digest(source), selection_sha256=digest(a.selection),
        sha256=digest(output), bytes=output.stat().st_size, converted=converted,
        seconds=time.monotonic()-started, onnx=onnx.__version__, calibration=False,
        threshold_retuning=False, quality_status='not evaluated; storage reduction is not runtime memory reduction')
    report_path.write_text(json.dumps(report, indent=2) + '\n'); print(json.dumps(report, indent=2))


if __name__ == '__main__':
    main()
