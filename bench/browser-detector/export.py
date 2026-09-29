#!/usr/bin/env python3
"""Export the frozen local encoder + pooling + BIO head; no source downloads."""
import argparse
import json
from pathlib import Path
import sys
import time
sys.path.insert(0, str(Path(__file__).resolve().parents[1]/'contextual'))
from encoder import classifier, digest
from ner import external


def main():
    p=argparse.ArgumentParser(__doc__)
    p.add_argument('--base',type=Path,required=True)
    p.add_argument('--head',type=Path,required=True)
    p.add_argument('--output',type=Path,required=True)
    a=p.parse_args();a.base=external(a.base);a.head=external(a.head);a.output=external(a.output)
    selected=json.loads((Path(__file__).resolve().parents[1]/'results/error-audit-v2/selected-candidate.json').read_text())
    assert digest(a.base/'model.safetensors')==selected['encoder']['base_weights_sha256']
    assert digest(a.head)==selected['composition']['neural_sha256']
    a.output.mkdir(parents=True,exist_ok=False)
    import torch
    import onnx
    import onnxruntime as ort
    from onnxruntime.quantization import quantize_dynamic, QuantType
    from transformers import AutoModel, AutoTokenizer
    from safetensors.torch import load_file
    torch.set_num_threads(3)
    class Detector(torch.nn.Module):
        def __init__(self):
            super().__init__()
            self.encoder=AutoModel.from_pretrained(a.base,local_files_only=True,use_safetensors=True,add_pooling_layer=False,trust_remote_code=False,attn_implementation='eager')
            self.head=classifier(768,128)
            self.head.load_state_dict(load_file(str(a.head)))
        def forward(self,input_ids,attention_mask,pooling):
            hidden=self.encoder(input_ids=input_ids,attention_mask=attention_mask).last_hidden_state
            pooled=torch.matmul(pooling,hidden).half().float()
            return self.head(pooled)
    model=Detector().eval()
    tokenizer=AutoTokenizer.from_pretrained(a.base,local_files_only=True,add_prefix_space=True,trust_remote_code=False)
    tokenizer.save_pretrained(a.output/'tokenizer')
    ids=torch.tensor([[0,10,20,30,2]],dtype=torch.long)
    pooling=torch.tensor([[[0,1,0,0,0],[0,0,.5,.5,0]]],dtype=torch.float32)
    started=time.monotonic()
    torch.onnx.export(model,(ids,torch.ones_like(ids),pooling),str(a.output/'fp32.onnx'),input_names=['input_ids','attention_mask','pooling'],output_names=['logits'],opset_version=17,dynamo=False,dynamic_axes={'input_ids':{0:'batch',1:'sequence'},'attention_mask':{0:'batch',1:'sequence'},'pooling':{0:'batch',1:'words',2:'sequence'},'logits':{0:'batch',1:'words'}})
    onnx.checker.check_model(str(a.output/'fp32.onnx'))
    del model
    quantize_dynamic(str(a.output/'fp32.onnx'),str(a.output/'int8.onnx'),weight_type=QuantType.QInt8,per_channel=True,op_types_to_quantize=['MatMul','Gather'],extra_options={'MatMulConstBOnly':True})
    onnx.checker.check_model(str(a.output/'int8.onnx'))
    report=dict(schema=1,reference=selected['candidate']['model_sha256'],encoder_sha256=selected['encoder']['base_weights_sha256'],head_sha256=digest(a.head),torch=torch.__version__,onnx=onnx.__version__,onnxruntime=ort.__version__,opset=17,exporter='torchscript dynamo=False; eager attention; dynamic batch/sequence/words',quantization='dynamic signed int8 per-channel constant MatMul and Gather; no calibration or retuning',seconds=time.monotonic()-started,files={n:dict(bytes=(a.output/n).stat().st_size,sha256=digest(a.output/n)) for n in ['fp32.onnx','int8.onnx']})
    (a.output/'export.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))

if __name__=='__main__':main()
