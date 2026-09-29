#!/usr/bin/env python3
"""Attach trained FP32 corrections without requantizing the frozen int4 backbone."""
import argparse
import json
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'contextual'))
from encoder import digest
from ner import external
from quantized_repair import linear_path


def attach(graph, state):
    import numpy as np
    import onnx
    from onnx import helper,numpy_helper
    initializers={v.name:v for v in graph.graph.initializer}
    original={name:value.SerializeToString() for name,value in initializers.items()}
    nodes=[];consumed=set();count=0
    for node in graph.graph.node:
        nodes.append(node)
        if node.op_type!='MatMulNBits':continue
        name=linear_path(node);ak=name+'.adapter_a.weight';bk=name+'.adapter_b.weight'
        a=state[ak].numpy().T.copy();b=state[bk].numpy().T.copy();consumed.update([ak,bk])
        attrs={v.name:helper.get_attribute_value(v) for v in node.attribute}
        if a.shape!=(attrs['K'],16) or b.shape!=(16,attrs['N']):raise ValueError('adapter shape mismatch')
        prefix=node.name+'__repair';out=node.output[0];node.output[0]=prefix+'_base'
        an,bn=prefix+'_a_weight',prefix+'_b_weight'
        graph.graph.initializer.extend([numpy_helper.from_array(a,an),numpy_helper.from_array(b,bn)])
        nodes.extend([
            helper.make_node('MatMul',[node.input[0],an],[prefix+'_a'],name=prefix+'_A'),
            helper.make_node('MatMul',[prefix+'_a',bn],[prefix+'_b'],name=prefix+'_B'),
            helper.make_node('Add',[node.output[0],prefix+'_b'],[out],name=prefix+'_Add')])
        count+=1
    if count!=72:raise ValueError('expected all 72 encoder matrix corrections')
    graph.graph.ClearField('node');graph.graph.node.extend(nodes)
    lookup={n.name:n for n in graph.graph.node};replaced=set()
    mappings={
        'head.0.weight':('/head/head.0/LayerNormalization',1,False),
        'head.0.bias':('/head/head.0/LayerNormalization',2,False),
        'head.1.weight':('/head/head.1/MatMul',1,True),
        'head.1.bias':('/head/head.1/Add',1,False),
        'head.4.weight':('/head/head.4/MatMul',1,True),
        'head.4.bias':('/head/head.4/Add',1,False)}
    for key,(node,index,transpose) in mappings.items():
        operator=lookup[node]
        if operator.op_type=='Add':
            constants=[name for name in operator.input if name in initializers]
            if len(constants)!=1:raise ValueError('head bias Add requires exactly one initializer')
            target=constants[0]
        else:
            target=operator.input[index]
        value=state[key].numpy()
        if any(target in consumer.input and not consumer.name.startswith('/head/') for consumer in graph.graph.node):
            raise ValueError('head initializer is shared with the frozen backbone')
        if transpose:value=value.T
        if list(value.shape)!=list(initializers[target].dims):raise ValueError('head shape mismatch')
        initializers[target].CopyFrom(numpy_helper.from_array(np.ascontiguousarray(value),target))
        replaced.add(target);consumed.add(key)
    if consumed!=set(state):raise ValueError('unexpected or unused trainable tensors')
    for name,value in initializers.items():
        if name not in replaced and value.SerializeToString()!=original[name]:raise ValueError('frozen backbone initializer changed')
    return dict(adapter_matrices=count*2,rank=16,head_tensors_replaced=len(replaced),
        frozen_initializers_preserved=len(original)-len(replaced),backbone_requantized=False)


def main():
    p=argparse.ArgumentParser(__doc__)
    for name in ['assets','selection','checkpoint','output']:p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();assets=external(a.assets);checkpoint=external(a.checkpoint);output=external(a.output)
    if output.exists() or output.with_suffix('.json').exists():raise ValueError('refusing to overwrite export')
    training=json.loads((checkpoint.parent/'training.json').read_text())
    selected=json.loads(a.selection.read_text());identity=training['identity']
    if not selected.get('frozen') or selected['thresholds']!=dict(crf=.75,neural=0):raise ValueError('reference is not frozen')
    if identity['selection_sha256']!=digest(a.selection):raise ValueError('selection changed')
    if digest(assets/'int4.onnx')!=identity['quantized_sha256']:raise ValueError('quantized backbone changed')
    records=[c for c in training['checkpoints'] if c['sha256']==digest(checkpoint)]
    if len(records)!=1 or records[0]['complete_epoch'] is not True:raise ValueError('requires intact complete training checkpoint')
    import onnx
    import torch
    from safetensors.torch import load_file
    state=load_file(str(checkpoint))
    if any(v.dtype!=torch.float32 or not v.isfinite().all() for v in state.values()):raise ValueError('requires finite FP32 corrections')
    graph=onnx.load(str(assets/'int4.onnx'));details=attach(graph,state)
    onnx.checker.check_model(graph);onnx.save_model(graph,str(output))
    report=dict(schema=1,method='Frozen int4 backbone plus FP32 rank-16 corrections and repaired head',
        selection_sha256=digest(a.selection),base_quantized_sha256=identity['quantized_sha256'],
        training_sha256=digest(checkpoint.parent/'training.json'),checkpoint_sha256=digest(checkpoint),
        sha256=digest(output),bytes=output.stat().st_size,details=details,threshold_retuning=False,
        quality_status='not evaluated; unchanged quantized weights do not guarantee unchanged predictions')
    output.with_suffix('.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()
