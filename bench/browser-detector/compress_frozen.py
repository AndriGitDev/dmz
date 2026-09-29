#!/usr/bin/env python3
"""Compress only a hash-frozen FP32 ONNX reference; no labels or threshold changes."""
import argparse
from collections import Counter
import json
from pathlib import Path
import sys
import time
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'contextual'))
from encoder import digest
from ner import external


def main():
    p=argparse.ArgumentParser(__doc__)
    p.add_argument('--assets',type=Path,required=True)
    p.add_argument('--selection',type=Path,required=True)
    p.add_argument('--recipe',choices=['int8','int4'],required=True)
    a=p.parse_args();assets=external(a.assets)
    selected=json.loads(a.selection.read_text());export=json.loads((assets/'export.json').read_text())
    if selected.get('frozen') is not True:raise ValueError('full-precision reference must be frozen first')
    if selected['thresholds']!=dict(crf=.75,neural=0):raise ValueError('threshold mismatch')
    if export['selection_sha256']!=digest(a.selection):raise ValueError('selection changed after export')
    source=assets/'fp32.onnx'
    if export['model_sha256']!=digest(source):raise ValueError('FP32 export changed')
    output=assets/(a.recipe+'.onnx')
    if output.exists():raise ValueError('refusing to replace compressed model')
    import onnx
    import onnxruntime as ort
    from onnxruntime.quantization import quantize_dynamic, QuantType
    started=time.monotonic()
    # The small decision head is retained in FP32; the encoder carries most bytes.
    graph=onnx.load(str(source))
    excluded=[n.name for n in graph.graph.node if '/head/' in n.name]
    del graph
    if a.recipe=='int8':
        # U8 weights avoid the S8-weight saturation hypothesis of the rejected run.
        quantize_dynamic(str(source),str(output),weight_type=QuantType.QUInt8,
            per_channel=True,op_types_to_quantize=['MatMul','Gather'],nodes_to_exclude=excluded,
            extra_options={'MatMulConstBOnly':True})
        description='Dynamic U8/U8 constant MatMul and U8 Gather; per-channel; FP32 head'
    else:
        from onnxruntime.quantization.matmul_nbits_quantizer import MatMulNBitsQuantizer
        intermediate=assets/'gather8-intermediate.onnx'
        if intermediate.exists():raise ValueError('intermediate already exists')
        quantize_dynamic(str(source),str(intermediate),weight_type=QuantType.QUInt8,
            per_channel=True,op_types_to_quantize=['Gather'])
        quantizer=MatMulNBitsQuantizer(str(intermediate),block_size=32,is_symmetric=True,
            accuracy_level=1,nodes_to_exclude=excluded,op_types_to_quantize=('MatMul',))
        quantizer.process()
        quantizer.model.save_model_to_file(str(output),use_external_data_format=False)
        # Retain intermediate provenance; remove only this newly generated temporary graph.
        intermediate_sha256=digest(intermediate);intermediate.unlink()
        description='Weight-only symmetric int4 MatMul, block 32, accuracy_level=1 (FP32 activations), FP32 head; U8 Gather'
    onnx.checker.check_model(str(output))
    graph=onnx.load(str(output))
    report=dict(schema=1,recipe=a.recipe,description=description,reference_sha256=digest(source),
        selection_sha256=digest(a.selection),sha256=digest(output),bytes=output.stat().st_size,
        seconds=time.monotonic()-started,onnxruntime=ort.__version__,onnx=onnx.__version__,
        operators=dict(Counter(n.domain+':'+n.op_type for n in graph.graph.node)),
        excluded_head_nodes=excluded,calibration=False,threshold_retuning=False,
        quality_status='not evaluated; size alone is not acceptance')
    if a.recipe=='int4':report['intermediate_sha256']=intermediate_sha256
    (assets/(a.recipe+'-compression.json')).write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))


if __name__=='__main__':main()
