#!/usr/bin/env python3
"""Export a hash-frozen FP32 fine-tuned detector, without compression."""
import argparse
import json
from pathlib import Path
import sys
import time
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'contextual'))
from encoder import digest
from ner import external
from finetune import detector


def main():
    p=argparse.ArgumentParser(__doc__)
    for name in ['base','checkpoint','selection','output']:
        p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args()
    base,checkpoint,output=map(external,[a.base,a.checkpoint,a.output])
    selected=json.loads(a.selection.read_text())
    if selected.get('frozen') is not True:raise ValueError('freeze reference before export')
    if selected['composition']['neural_sha256']!=digest(checkpoint):raise ValueError('checkpoint mismatch')
    if selected['encoder']['base_weights_sha256']!=digest(base/'model.safetensors'):raise ValueError('base mismatch')
    if selected['thresholds']!=dict(crf=.75,neural=0):raise ValueError('threshold mismatch')
    output.mkdir(parents=True,exist_ok=False)
    import torch
    import onnx
    from safetensors.torch import load_file
    from transformers import AutoTokenizer
    torch.set_num_threads(3)
    model=detector(base);model.load_state_dict(load_file(str(checkpoint)));model.eval()
    tokenizer=AutoTokenizer.from_pretrained(base,local_files_only=True,add_prefix_space=True,trust_remote_code=False)
    tokenizer.save_pretrained(output/'tokenizer')
    ids=torch.tensor([[0,10,20,30,2]],dtype=torch.long)
    pooling=torch.tensor([[[0,1,0,0,0],[0,0,.5,.5,0]]],dtype=torch.float32)
    started=time.monotonic()
    torch.onnx.export(model,(ids,torch.ones_like(ids),pooling),str(output/'fp32.onnx'),
        input_names=['input_ids','attention_mask','pooling'],output_names=['logits'],
        opset_version=17,dynamo=False,dynamic_axes={'input_ids':{0:'batch',1:'sequence'},
        'attention_mask':{0:'batch',1:'sequence'},'pooling':{0:'batch',1:'words',2:'sequence'},
        'logits':{0:'batch',1:'words'}})
    onnx.checker.check_model(str(output/'fp32.onnx'))
    report=dict(schema=1,checkpoint_sha256=digest(checkpoint),selection_sha256=digest(a.selection),
        model_sha256=digest(output/'fp32.onnx'),bytes=(output/'fp32.onnx').stat().st_size,
        seconds=time.monotonic()-started,precision='FP32 encoder, pooling and head; no float16 round-trip',
        torch=torch.__version__,onnx=onnx.__version__,opset=17,compression=False)
    (output/'export.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))


if __name__=='__main__':main()
