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


def preserve_head(graph, originals):
    from onnx import TensorProto
    tensors={v.name:v for v in graph.graph.initializer}
    for name,raw in originals.items():
        if name not in tensors:raise ValueError('head initializer missing')
        original=TensorProto.FromString(raw)
        actual=TensorProto.FromString(tensors[name].SerializeToString())
        # ORT can explicitly serialize the protobuf's default storage location.
        if not original.HasField('data_location') and actual.HasField('data_location') and actual.data_location==TensorProto.DEFAULT:
            actual.ClearField('data_location')
        if actual.SerializeToString()!=raw:raise ValueError('head initializer changed')
        tensors[name].CopyFrom(original)


def main():
    p=argparse.ArgumentParser(__doc__)
    p.add_argument('--assets',type=Path,required=True)
    p.add_argument('--selection',type=Path,required=True)
    p.add_argument('--recipe',choices=['int8','int4','mixed','int4b16','int4a32','hqq4','int8fp16'],required=True)
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
    head_inputs={name for n in graph.graph.node if '/head/' in n.name for name in n.input}
    source_head={v.name:v.SerializeToString() for v in graph.graph.initializer if v.name in head_inputs}
    if a.recipe=='int8fp16' and len(source_head)!=6:raise ValueError('expected six FP32 head initializers')
    attention_and_gather=[n.name for n in graph.graph.node
        if n.op_type=='Gather' or (n.op_type=='MatMul' and '/attention/' in n.name)]
    del graph
    if a.recipe in ('int8','int8fp16'):
        # U8 weights avoid the S8-weight saturation hypothesis of the rejected run.
        quantize_dynamic(str(source),str(output),weight_type=QuantType.QUInt8,
            per_channel=True,op_types_to_quantize=['MatMul','Gather'] if a.recipe=='int8' else ['MatMul'],nodes_to_exclude=excluded,
            extra_options={'MatMulConstBOnly':True})
        description='Dynamic U8/U8 constant MatMul and U8 Gather; per-channel; FP32 head'
        if a.recipe=='int8fp16':
            from store_fp16 import convert
            graph=onnx.load(str(output))
            quantized=[n for n in graph.graph.node if n.op_type=='MatMulInteger']
            if len(quantized)!=72 or any(not n.name.startswith('/encoder/') for n in quantized):
                raise ValueError('expected exactly 72 quantized encoder matrices')
            converted=convert(graph,node_suffixes=('/Gather',))
            if converted!=dict(MatMul=0,Gather=3):raise ValueError('expected exactly three FP16 embedding tables')
            preserve_head(graph,source_head)
            after={v.name:v.SerializeToString() for v in graph.graph.initializer}
            if any(after.get(name)!=value for name,value in source_head.items()):raise ValueError('head initializer changed')
            onnx.save_model(graph,str(output))
            description='Dynamic U8/U8 constant MatMul; FP16 stored embeddings cast to FP32; per-channel; FP32 head'
    else:
        from onnxruntime.quantization.matmul_nbits_quantizer import MatMulNBitsQuantizer, HQQWeightOnlyQuantConfig
        intermediate=assets/(a.recipe+'-gather8-intermediate.onnx')
        if intermediate.exists():raise ValueError('intermediate already exists')
        if a.recipe=='mixed':
            quantize_dynamic(str(source),str(intermediate),weight_type=QuantType.QUInt8,
                per_channel=True,op_types_to_quantize=['MatMul','Gather'],
                nodes_to_quantize=attention_and_gather,nodes_to_exclude=excluded,
                extra_options={'MatMulConstBOnly':True})
        else:
            quantize_dynamic(str(source),str(intermediate),weight_type=QuantType.QUInt8,
                per_channel=True,op_types_to_quantize=['Gather'])
        block_size=16 if a.recipe=='int4b16' else 32
        symmetric=a.recipe not in ('int4a32','hqq4')
        algorithm=None
        if a.recipe=='hqq4':
            import torch
            torch.set_num_threads(3)
            algorithm=HQQWeightOnlyQuantConfig(block_size=32,bits=4,axis=1,op_types_to_quantize=('MatMul',))
        quantizer=MatMulNBitsQuantizer(str(intermediate),block_size=block_size,is_symmetric=symmetric,
            accuracy_level=1,nodes_to_exclude=excluded,op_types_to_quantize=('MatMul',),algo_config=algorithm)
        quantizer.process()
        if a.recipe=='hqq4':
            # ORT 1.23.2 HQQ does not propagate accuracy_level to generated nodes.
            for node in quantizer.model.model.graph.node:
                if node.op_type=='MatMulNBits':
                    retained=[attr for attr in node.attribute if attr.name!='accuracy_level']
                    del node.attribute[:];node.attribute.extend(retained)
                    node.attribute.append(onnx.helper.make_attribute('accuracy_level',1))
        quantizer.model.save_model_to_file(str(output),use_external_data_format=False)
        # Retain intermediate provenance; remove only this newly generated temporary graph.
        intermediate_sha256=digest(intermediate);intermediate.unlink()
        description=f"Weight-only {'symmetric' if symmetric else 'asymmetric'} int4 MatMul, block {block_size}, accuracy_level=1 (FP32 activations), FP32 head; U8 Gather"
        if a.recipe=='hqq4':
            description='HQQ weight-only int4 MatMul, block 32, FP32 floating zero points and activations, FP32 head; U8 Gather; no calibration data'
        if a.recipe=='mixed':
            description='Dynamic U8/U8 attention MatMul and U8 Gather; weight-only int4 feed-forward MatMul block 32 with FP32 activations; FP32 head'
    onnx.checker.check_model(str(output))
    graph=onnx.load(str(output))
    report=dict(schema=1,recipe=a.recipe,description=description,reference_sha256=digest(source),
        selection_sha256=digest(a.selection),sha256=digest(output),bytes=output.stat().st_size,
        seconds=time.monotonic()-started,onnxruntime=ort.__version__,onnx=onnx.__version__,
        operators=dict(Counter(n.domain+':'+n.op_type for n in graph.graph.node)),
        excluded_head_nodes=excluded,calibration=False,threshold_retuning=False,
        quality_status='not evaluated; size alone is not acceptance')
    if a.recipe=='int8fp16':
        report.update(converted_embeddings=converted,quantized_encoder_matrices=len(quantized),preserved_head_initializers=len(source_head),
            exporter_source_sha256=digest(Path(__file__)))
    elif a.recipe!='int8':
        report.update(intermediate_sha256=intermediate_sha256,block_size=block_size,is_symmetric=symmetric,accuracy_level=1)
    (assets/(a.recipe+'-compression.json')).write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))


if __name__=='__main__':main()
