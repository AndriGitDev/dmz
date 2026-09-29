#!/usr/bin/env python3
"""Train small corrections on a frozen int4 backbone; keep evaluation separate."""
import argparse
import json
import math
from pathlib import Path
import random
import sys
import time

from encoder import digest
from finetune import detector, prepare, tensors
from ner import external


def linear_path(node):
    return '.'.join(node.name.strip('/').split('/')[:-1])


def dequantized_weight(node, initializers):
    """Decode ORT MatMulNBits packed weights to PyTorch [out, in] FP32."""
    import numpy as np
    from onnx import helper, numpy_helper
    attrs={a.name:helper.get_attribute_value(a) for a in node.attribute}
    if attrs['bits']!=4:raise ValueError('expected int4 weights')
    n,k,block=attrs['N'],attrs['K'],attrs['block_size'];blocks=(k+block-1)//block
    packed=numpy_helper.to_array(initializers[node.input[1]])
    q=np.stack((packed&15,packed>>4),axis=-1).reshape(n,blocks,block).astype(np.float32)
    scales=numpy_helper.to_array(initializers[node.input[2]]).reshape(n,blocks,1)
    zero=8.
    if len(node.input)>3 and node.input[3]:
        zp=numpy_helper.to_array(initializers[node.input[3]])
        if zp.dtype==np.uint8:
            zero=np.stack((zp&15,zp>>4),axis=-1).reshape(n,-1)[:,:blocks,None].astype(np.float32)
        else:zero=zp.reshape(n,blocks,1).astype(np.float32)
    return ((q-zero)*scales).reshape(n,-1)[:,:k].copy()


def identities(assets, selection, checkpoint, base, source=None):
    selected=json.loads(selection.read_text());export=json.loads((assets/'export.json').read_text())
    compressed=json.loads((assets/'int4-compression.json').read_text())
    if selected.get('frozen') is not True or selected['thresholds']!=dict(crf=.75,neural=0):
        raise ValueError('requires frozen reference and unchanged thresholds')
    if export['selection_sha256']!=digest(selection) or compressed['selection_sha256']!=digest(selection):
        raise ValueError('selection changed')
    if digest(assets/'fp32.onnx')!=export['model_sha256']:raise ValueError('FP32 export changed')
    if compressed['recipe']!='int4' or compressed['reference_sha256']!=export['model_sha256']:
        raise ValueError('requires original frozen-reference int4 recipe')
    if digest(assets/'int4.onnx')!=compressed['sha256'] or digest(checkpoint)!=selected['candidate']['model_sha256']:
        raise ValueError('model identity mismatch')
    if digest(base/'model.safetensors')!=selected['encoder']['base_weights_sha256']:
        raise ValueError('base identity mismatch')
    for name,expected in selected['encoder']['tokenizer_files'].items():
        if digest(base/name)!=expected:raise ValueError('tokenizer identity mismatch')
    training_path=checkpoint.parent/'training.json'
    if digest(training_path)!=selected['provenance']['training_sha256']:
        raise ValueError('frozen training manifest changed')
    original=json.loads(training_path.read_text())
    if source and (original['split']!='train' or digest(source)!=original['input_sha256']):
        raise ValueError('repair may only use the original training mixture')
    return dict(selection_sha256=digest(selection),frozen_checkpoint_sha256=digest(checkpoint),
        quantized_sha256=compressed['sha256'],input_sha256=original['input_sha256'],
        base_weights_sha256=selected['encoder']['base_weights_sha256'])


def build(base, checkpoint, graph_path, rank):
    import numpy as np
    import onnx
    import torch
    from safetensors.torch import load_file
    model=detector(base);model.load_state_dict(load_file(str(checkpoint)))
    graph=onnx.load(str(graph_path));initializers={v.name:v for v in graph.graph.initializer}
    linear_names=[];embeddings=[]
    with torch.no_grad():
        for node in graph.graph.node:
            if node.op_type!='MatMulNBits':continue
            name=linear_path(node);layer=model.get_submodule(name)
            if not isinstance(layer,torch.nn.Linear):raise ValueError('quantized node is not a linear layer')
            weight=torch.from_numpy(dequantized_weight(node,initializers))
            if layer.weight.shape!=weight.shape:raise ValueError('weight shape mismatch')
            layer.weight.copy_(weight);linear_names.append(name)
        for name,parameter in model.named_parameters():
            quantized=initializers.get(name+'_quantized')
            if quantized is None:continue
            q=onnx.numpy_helper.to_array(quantized).astype(np.float32)
            scale=onnx.numpy_helper.to_array(initializers[name+'_scale'])
            zero=onnx.numpy_helper.to_array(initializers[name+'_zero_point']).astype(np.float32)
            value=torch.from_numpy((q-zero)*scale)
            if value.shape!=parameter.shape:raise ValueError('embedding shape mismatch')
            parameter.copy_(value);embeddings.append(name)
    if len(linear_names)!=72 or len(embeddings)!=3:
        raise ValueError(f'incomplete quantized conversion: {len(linear_names)} linears, {len(embeddings)} embeddings')
    model.requires_grad_(False)
    class RepairLinear(torch.nn.Module):
        def __init__(self, layer):
            super().__init__();self.base=layer
            self.adapter_a=torch.nn.Linear(layer.in_features,rank,bias=False)
            self.adapter_b=torch.nn.Linear(rank,layer.out_features,bias=False)
            torch.nn.init.zeros_(self.adapter_b.weight)
        def forward(self,value):
            return self.base(value)+self.adapter_b(self.adapter_a(value))
    for name in linear_names:
        parent,attribute=name.rsplit('.',1);module=model.get_submodule(parent)
        setattr(module,attribute,RepairLinear(getattr(module,attribute)))
    model.head.requires_grad_(True)
    return model,dict(linears=linear_names,embeddings=embeddings,rank=rank,scale=1.,
        trainable_parameters=sum(p.numel() for p in model.parameters() if p.requires_grad))


def parity_probe(model, jobs, tokenizer, graph_path):
    import numpy as np
    import onnxruntime as ort
    import torch
    options=ort.SessionOptions();options.intra_op_num_threads=3;options.inter_op_num_threads=1
    session=ort.InferenceSession(str(graph_path),sess_options=options,providers=['CPUExecutionProvider'])
    maximum=0.;changed=0
    model.eval()
    # Training-only representative lengths; no development examples or labels.
    order=sorted(jobs,key=lambda j:len(j['ids']))
    selected=[order[round(i*(len(order)-1)/7)] for i in range(8)]
    with torch.inference_mode():
        for job in selected:
            batch=tensors([job],tokenizer.pad_token_id)
            actual=model(*batch[:3]).numpy()
            feeds={k:v.numpy() for k,v in zip(['input_ids','attention_mask','pooling'],batch[:3])}
            expected=session.run(['logits'],feeds)[0]
            maximum=max(maximum,float(np.max(np.abs(actual-expected))))
            changed+=int(np.sum(actual.argmax(-1)!=expected.argmax(-1)))
            if not np.allclose(actual,expected,rtol=5e-5,atol=5e-4):raise ValueError('dequantized backbone logit mismatch')
    if changed:raise ValueError('zero-adapter parity changed word labels')
    return dict(training_chunks=8,max_absolute_logit_error=maximum,changed_word_labels=changed)


def train(a):
    import torch
    from safetensors.torch import save_file, load_file
    from transformers import AutoTokenizer
    assets,base,checkpoint,source=map(external,[a.assets,a.base,a.checkpoint,a.input])
    identity=identities(assets,a.selection,checkpoint,base,source)
    output=external(a.output);output.mkdir(parents=True,exist_ok=False)
    torch.set_num_threads(a.threads);torch.manual_seed(a.seed)
    tokenizer=AutoTokenizer.from_pretrained(base,local_files_only=True,add_prefix_space=True,trust_remote_code=False)
    records,jobs=prepare(source,'train',tokenizer)
    model,conversion=build(base,checkpoint,assets/'int4.onnx',a.rank)
    comparison=assets/'int4.onnx';verified_export=None
    if a.repair_state:
        repair_state=external(a.repair_state);comparison=external(a.probe_graph)
        exported=json.loads(comparison.with_suffix('.json').read_text())
        if exported['sha256']!=digest(comparison) or exported['checkpoint_sha256']!=digest(repair_state) or exported['selection_sha256']!=identity['selection_sha256']:
            raise ValueError('repair export identity mismatch')
        state=load_file(str(repair_state))
        if set(state)!={n for n,p in model.named_parameters() if p.requires_grad}:raise ValueError('incomplete repair state')
        if any(v.dtype!=torch.float32 or not v.isfinite().all() for v in state.values()):raise ValueError('invalid repair state')
        model.load_state_dict(state,strict=False)
        verified_export=dict(model_sha256=digest(comparison),checkpoint_sha256=digest(repair_state))
    probe=parity_probe(model,jobs,tokenizer,comparison)
    adapters=[p for n,p in model.named_parameters() if p.requires_grad and not n.startswith('head.')]
    optimizer=torch.optim.AdamW([{'params':adapters,'lr':a.adapter_lr},{'params':model.head.parameters(),'lr':a.head_lr}],weight_decay=.01)
    total=math.ceil(math.ceil(len(jobs)/a.batch_size)/a.accumulation)*a.epochs;warmup=max(1,int(total*.1))
    def rate(step):return (step+1)/warmup if step<warmup else max(0.,(total-step)/max(1,total-warmup))
    scheduler=torch.optim.lr_scheduler.LambdaLR(optimizer,rate)
    report=dict(schema=1,method='Frozen int4 backbone with trainable FP32 rank-16 additive corrections and BIO head',
        identity=identity,conversion=conversion,parity_probe=probe,verified_export=verified_export,split='train',rows=len(records),chunks=len(jobs),
        settings={k:v for k,v in vars(a).items() if isinstance(v,(str,int,float,bool))},
        source_sha256=digest(Path(__file__)),torch=torch.__version__,checkpoints=[],independent_validation=False)
    (output/'protocol.json').write_text(json.dumps(report,indent=2)+'\n')
    if a.probe:print(json.dumps(report,indent=2));return
    model.train();rng=random.Random(a.seed);started=time.monotonic();steps=0
    parameters=[p for p in model.parameters() if p.requires_grad]
    for epoch in range(1,a.epochs+1):
        order=list(range(len(jobs)));rng.shuffle(order);batches=[]
        for begin in range(0,len(order),256):
            bucket=sorted(order[begin:begin+256],key=lambda i:len(jobs[i]['ids']))
            batches.extend(bucket[i:i+a.batch_size] for i in range(0,len(bucket),a.batch_size))
        rng.shuffle(batches);loss_sum=0.;tokens=0;complete=True;optimizer.zero_grad(set_to_none=True)
        for b,indices in enumerate(batches):
            batch=tensors([jobs[i] for i in indices],tokenizer.pad_token_id);logits=model(*batch[:3])
            loss=torch.nn.functional.cross_entropy(logits.reshape(-1,3),batch[3].reshape(-1))
            if not torch.isfinite(loss):raise ValueError('nonfinite loss')
            n=int((batch[3]!=-100).sum());loss_sum+=loss.item()*n;tokens+=n
            group=min(a.accumulation,len(batches)-(b//a.accumulation)*a.accumulation);(loss/group).backward()
            if (b+1)%a.accumulation==0 or b+1==len(batches):
                torch.nn.utils.clip_grad_norm_(parameters,1.);optimizer.step();scheduler.step();optimizer.zero_grad(set_to_none=True);steps+=1
                elapsed=time.monotonic()-started
                if steps%10==0:print(json.dumps(dict(epoch=epoch,step=steps,batches=b+1,epoch_batches=len(batches),loss=loss_sum/tokens,seconds=elapsed)),flush=True)
                if elapsed>=a.max_seconds:complete=b+1==len(batches);break
        path=output/f'epoch-{epoch}.safetensors'
        save_file({n:p.detach().contiguous() for n,p in model.named_parameters() if p.requires_grad},str(path))
        row=dict(epoch=epoch,complete_epoch=complete,file=path.name,sha256=digest(path),bytes=path.stat().st_size,
            steps=steps,tokens_seen=tokens,loss=loss_sum/tokens,seconds=time.monotonic()-started)
        report['checkpoints'].append(row);(output/'training.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(row),flush=True)
        if time.monotonic()-started>=a.max_seconds:break


def main():
    p=argparse.ArgumentParser(__doc__)
    for name in ['assets','base','checkpoint','input','output','selection']:p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--rank',type=int,default=16,choices=[16]);p.add_argument('--epochs',type=int,default=2)
    p.add_argument('--adapter-lr',type=float,default=1e-4);p.add_argument('--head-lr',type=float,default=5e-5)
    p.add_argument('--batch-size',type=int,default=8);p.add_argument('--accumulation',type=int,default=4)
    p.add_argument('--threads',type=int,default=3);p.add_argument('--seed',type=int,default=44)
    p.add_argument('--repair-state',type=Path);p.add_argument('--probe-graph',type=Path)
    p.add_argument('--max-seconds',type=float,default=14400);p.add_argument('--probe',action='store_true')
    a=p.parse_args()
    if (a.repair_state is None)!=(a.probe_graph is None) or (a.repair_state and not a.probe):p.error('repair-state and probe-graph require probe mode together')
    if min(a.epochs,a.batch_size,a.accumulation,a.threads,a.max_seconds,a.adapter_lr,a.head_lr)<=0:p.error('positive limits required')
    train(a)


if __name__=='__main__':main()
