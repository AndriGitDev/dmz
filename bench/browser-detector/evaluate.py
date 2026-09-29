#!/usr/bin/env python3
"""Fixed-threshold development parity; aggregates only in stdout/report."""
import argparse
import json
from pathlib import Path
import sys
import time
import numpy as np
import onnxruntime as ort
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'contextual'))
from encoder import chunks,digest,DECODE
from ner import external,rows,tokens_and_labels,spans
from ensemble import merge


def inputs(parts, tokenizer):
    batch=tokenizer(parts,is_split_into_words=True,padding=True,return_tensors='np')
    length=batch['input_ids'].shape[1]
    pooling=np.zeros((len(parts),max(map(len,parts)),length),dtype=np.float32)
    for i,part in enumerate(parts):
        ids=batch.word_ids(i)
        for word in range(len(part)):
            positions=[j for j,w in enumerate(ids) if w==word]
            if not positions:raise ValueError('missing word representation')
            pooling[i,word,positions]=1/len(positions)
    return dict(input_ids=batch['input_ids'].astype(np.int64),attention_mask=batch['attention_mask'].astype(np.int64),pooling=pooling)


def score(records, crf=None):
    tp=fp=fn=0
    for i,row in enumerate(records):
        gold={tuple(x) for x in row['gold']}
        predicted=merge(crf[i]['predictions'],row['predictions'],.75,0) if crf else {tuple(x[:2]) for x in row['predictions']}
        tp+=len(predicted&gold);fp+=len(predicted-gold);fn+=len(gold-predicted)
    return dict(tp=tp,fp=fp,fn=fn,precision=tp/max(1,tp+fp),recall=tp/max(1,tp+fn))


def session(model,threads=3):
    options=ort.SessionOptions();options.intra_op_num_threads=threads;options.inter_op_num_threads=1
    return ort.InferenceSession(str(model),sess_options=options,providers=['CPUExecutionProvider'])


def verify_tokenizer(directory):
    manifest=directory.parent/'export.json'
    if manifest.exists():
        export=json.loads(manifest.read_text())
        if 'tokenizer_files' in export and {p.name for p in directory.iterdir() if p.is_file()}!=set(export['tokenizer_files']):
            raise ValueError('exported tokenizer file set mismatch')
        for name,expected in export.get('tokenizer_files',{}).items():
            if Path(name).name!=name or digest(directory/name)!=expected:
                raise ValueError('exported tokenizer integrity mismatch')


def main():
    p=argparse.ArgumentParser(__doc__)
    for key in ['model','tokenizer','input','crf','reference','output','predictions']:p.add_argument('--'+key,type=Path,required=True)
    p.add_argument('--batch-size',type=int,default=8)
    p.add_argument('--selection',type=Path,default=Path(__file__).resolve().parents[1]/'results/error-audit-v2/selected-candidate.json')
    a=p.parse_args()
    for key in ['model','tokenizer','input','crf','reference','predictions']:setattr(a,key,external(getattr(a,key)))
    if a.predictions.exists():raise ValueError('refusing to overwrite prediction cache')
    if a.output.exists():raise ValueError('refusing to overwrite evaluation report')
    from transformers import AutoTokenizer
    verify_tokenizer(a.tokenizer)
    tokenizer=AutoTokenizer.from_pretrained(a.tokenizer,local_files_only=True,add_prefix_space=True,trust_remote_code=False)
    crf=json.loads(a.crf.read_text());reference=json.loads(a.reference.read_text())
    sha=digest(a.input)
    for cache in [crf,reference]:
        if cache['input_sha256']!=sha or cache['split']!='dev' or cache['text_mode']!='canonical':raise ValueError('cache identity mismatch')
    selected=json.loads(a.selection.read_text())
    assert selected['thresholds']==dict(crf=.75,neural=0)
    assert crf['model_sha256']==selected['composition']['crf_sha256']
    assert reference['model_sha256']==selected['composition']['neural_sha256']
    records=[];jobs=[];predictions=[]
    started=time.monotonic();runtime=session(a.model);load=time.monotonic()-started
    for index,row in enumerate(rows(a.input,'dev')):
        words,offsets,labels=tokens_and_labels(row)
        gold=[[s,e] for s,e,_,_ in spans(labels,offsets)]
        if gold!=reference['records'][index]['gold'] or gold!=crf['records'][index]['gold']:raise ValueError('gold mismatch')
        records.append(dict(gold=gold,offsets=offsets));predictions.append(np.zeros(len(words),dtype=np.int8))
        for shift,part in chunks(words,tokenizer):jobs.append((index,shift,part))
    assert len(records)==len(reference['records'])==len(crf['records'])
    jobs.sort(key=lambda j:len(tokenizer(j[2],is_split_into_words=True)['input_ids']))
    inference_start=time.monotonic()
    for i in range(0,len(jobs),a.batch_size):
        group=jobs[i:i+a.batch_size];feed=inputs([j[2] for j in group],tokenizer)
        logits=runtime.run(['logits'],feed)[0]
        if not np.isfinite(logits).all():raise ValueError('nonfinite logits')
        for j,(record,start,part) in enumerate(group):predictions[record][start:start+len(part)]=logits[j,:len(part)].argmax(-1)
        if i%256==0:print(json.dumps(dict(event='inference',chunks=i,total=len(jobs),seconds=round(time.monotonic()-inference_start,1))),file=sys.stderr,flush=True)
    output=[];changed=0;combined_changed=0
    for i,(record,labels) in enumerate(zip(records,predictions)):
        pred=[[s,e,1.] for s,e,_,_ in spans([DECODE[int(v)] for v in labels],record['offsets'])]
        output.append(dict(gold=record['gold'],predictions=pred))
        changed+=({tuple(s[:2]) for s in pred}!={tuple(s[:2]) for s in reference['records'][i]['predictions']})
        combined_changed+=(merge(crf['records'][i]['predictions'],pred,.75,0)!=merge(crf['records'][i]['predictions'],reference['records'][i]['predictions'],.75,0))
    report=dict(schema=1,scope='fixed-threshold development exact Person spans; no training or threshold retuning',input_sha256=sha,model_sha256=digest(a.model),model_bytes=a.model.stat().st_size,rows=len(records),chunks=len(jobs),threads=3,batch_size=a.batch_size,load_seconds=load,inference_seconds=time.monotonic()-inference_start,onnxruntime=ort.__version__,neural=score(output),combined=score(output,crf['records']),reference_neural=score(reference['records']),reference_combined=score(reference['records'],crf['records']),changed_neural_records=changed,changed_combined_records=combined_changed,independent_validation=False)
    a.predictions.write_text(json.dumps(dict(split='dev',input_sha256=sha,model_sha256=report['model_sha256'],records=output)))
    a.output.write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))

if __name__=='__main__':main()
