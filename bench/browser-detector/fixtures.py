#!/usr/bin/env python3
"""Create fabricated pretokenized browser vectors and native expected logits."""
import argparse
import json
from pathlib import Path
from evaluate import inputs,session,verify_tokenizer
from encoder import chunks,digest
from ner import external
from transformers import AutoTokenizer

p=argparse.ArgumentParser(__doc__);p.add_argument('--assets',type=Path,required=True)
p.add_argument('--output',type=Path,help='Fresh external fixture file; defaults to ASSETS/fixtures.json')
p.add_argument('--models',nargs='+',choices=['fp32','int8','int4','mixed','int4b16','int4a32','hqq4'],default=['fp32','int8']);a=p.parse_args()
a.assets=external(a.assets)
output=external(a.output) if a.output else a.assets/'fixtures.json'
if output.exists():raise ValueError('refusing to overwrite fixtures')
verify_tokenizer(a.assets/'tokenizer')
tokenizer=AutoTokenizer.from_pretrained(a.assets/'tokenizer',local_files_only=True,add_prefix_space=True)
phrase='Tilbúið próf Arna Jónsdóttir sendi samning til yfirferðar og greiðsla verður bókuð á morgun .'.split()
cases=[]
for size in [32,128,512]:
    words=(phrase*(size//len(phrase)+1))[:size]
    records=[]
    for shift,part in chunks(words,tokenizer):
        feed=inputs([part],tokenizer)
        records.append(dict(offset=shift,words=len(part),feeds={k:dict(shape=list(v.shape),data=v.flatten().tolist()) for k,v in feed.items()}))
    cases.append(dict(words=size,chunks=records))
for model in a.models:
    runtime=session(a.assets/(model+'.onnx'))
    for case in cases:
        for chunk in case['chunks']:
            import numpy as np
            feed={k:np.array(v['data'],dtype=np.float32 if k=='pooling' else np.int64).reshape(v['shape']) for k,v in chunk['feeds'].items()}
            logits=runtime.run(['logits'],feed)[0]
            chunk.setdefault('expected',{})[model]=dict(labels=logits.argmax(-1).flatten().tolist(),logits=logits.flatten().tolist())
    del runtime
output.write_text(json.dumps(dict(schema=1,scope='fabricated pretokenized words; excludes browser tokenization, CRF and masking',models={m:digest(a.assets/(m+'.onnx')) for m in a.models},cases=cases)))
print(json.dumps(dict(cases=len(cases),words=[c['words'] for c in cases])))
