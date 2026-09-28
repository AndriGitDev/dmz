#!/usr/bin/env python3
"""Development-only CRF/encoder combination; private span caches remain outside Git."""
import argparse
import hashlib
import json
from pathlib import Path
from ner import external, rows, tokens_and_labels, features, spans, sha

GRID=[0,.5,.6,.65,.7,.75,.8,.9,.95]


def crf_cache(source,model,text_mode='canonical'):
    import pycrfsuite
    metadata=json.loads(model.with_suffix('.metadata.json').read_text())
    if sha(model)!=metadata['model_sha256']:raise ValueError('model checksum mismatch')
    tagger=pycrfsuite.Tagger();tagger.open(str(model));records=[]
    for row in rows(source,'dev'):
        words,offsets,labels=tokens_and_labels(row,text_mode=text_mode)
        predicted=tagger.tag(features(words,metadata.get('feature_version',1)))
        records.append(dict(gold=[[s,e] for s,e,_,_ in spans(labels,offsets)],predictions=[[s,e,min(tagger.marginal(predicted[k],k) for k in range(i,j))] for s,e,i,j in spans(predicted,offsets)]))
    return dict(split='dev',input_sha256=sha(source),model_sha256=sha(model),text_mode=text_mode,records=records)


def neural_cache(directory,head_dir,epoch):
    import numpy as np
    import torch
    from safetensors.torch import load_file
    from encoder import cache,classifier,digest,DECODE
    x,y,metadata=cache(directory,'dev');head_dir=external(head_dir)
    training=json.loads((head_dir/'metadata.json').read_text())
    epochs=[epoch] if isinstance(epoch,int) else epoch
    if not epochs or len(set(epochs))!=len(epochs):raise ValueError('invalid checkpoint list')
    weights=[head_dir/f'epoch-{number}.safetensors' for number in epochs]
    hashes=[digest(path) for path in weights]
    if any(value!=training['weights'][path.name] for path,value in zip(weights,hashes)):raise ValueError('head checksum mismatch')
    if metadata['base_weights_sha256']!=training['base_weights_sha256'] or metadata['pooling']!=training['pooling']:raise ValueError('encoder mismatch')
    torch.set_num_threads(3);heads=[]
    for path in weights:
        head=classifier(training['hidden_size'],training['classifier_hidden']);head.load_state_dict(load_file(str(path)));head.eval();heads.append(head)
    prediction=np.zeros(len(y),dtype=np.int8);confidence=np.zeros(len(y),dtype=np.float32)
    with torch.inference_mode():
        for a in range(0,len(y),4096):
            inputs=torch.from_numpy(np.array(x[a:a+4096],dtype=np.float32))
            probabilities=torch.stack([torch.softmax(head(inputs),dim=-1) for head in heads]).mean(dim=0)
            values,labels=probabilities.max(dim=-1);prediction[a:a+len(labels)]=labels.numpy();confidence[a:a+len(labels)]=values.numpy()
    records=[];cursor=0
    for line in (directory/'records.jsonl').read_text().splitlines():
        row=json.loads(line);a,b=row['start'],row['end'];offsets=row['offsets']
        if a!=cursor or b<=a or b>len(y) or len(offsets)!=b-a:raise ValueError('invalid record coverage')
        cursor=b
        records.append(dict(gold=[[s,e] for s,e,_,_ in spans([DECODE[int(v)] for v in y[a:b]],offsets)],predictions=[[s,e,float(confidence[a+i:a+j].min())] for s,e,i,j in spans([DECODE[int(v)] for v in prediction[a:b]],offsets)]))
    if cursor!=len(y) or len(records)!=metadata['rows']:raise ValueError('incomplete record coverage')
    composition=dict(method='mean token probabilities',epochs=epochs,head_sha256=hashes)
    identity=hashes[0] if len(hashes)==1 else hashlib.sha256(json.dumps(composition,sort_keys=True).encode()).hexdigest()
    return dict(split='dev',input_sha256=metadata['input_sha256'],model_sha256=identity,text_mode=metadata['text_mode'],records=records,composition=composition)


def merge(primary,secondary,primary_threshold,secondary_threshold):
    chosen={(a,b) for a,b,c in primary if c>=primary_threshold}
    additions={(a,b) for a,b,c in secondary if c>=secondary_threshold and not any(a<e and s<b for s,e in chosen)}
    return chosen|additions


def evaluate(primary,secondary):
    for key in ['split','input_sha256','text_mode']:
        if primary[key]!=secondary[key]:raise ValueError('cache identity mismatch')
    if primary['split']!='dev':raise ValueError('development only')
    if not primary['records'] or len(primary['records'])!=len(secondary['records']):raise ValueError('record count mismatch')
    for p,n in zip(primary['records'],secondary['records']):
        if p['gold']!=n['gold']:raise ValueError('annotation mismatch')
    models=[]
    for ct in GRID:
        spec=dict(crf_sha256=primary['model_sha256'],neural_sha256=secondary['model_sha256'],crf_threshold=ct,overlap_policy='CRF priority; add only non-overlapping neural spans')
        identity=hashlib.sha256(json.dumps(spec,sort_keys=True).encode()).hexdigest();scores={}
        for nt in GRID:
            tp=fp=fn=0
            for p,n in zip(primary['records'],secondary['records']):
                gold={tuple(s) for s in p['gold']};selected=merge(p['predictions'],n['predictions'],ct,nt)
                tp+=len(gold&selected);fp+=len(selected-gold);fn+=len(gold-selected)
            precision=tp/max(1,tp+fp);recall=tp/max(1,tp+fn)
            scores[nt]=dict(tp=tp,fp=fp,fn=fn,precision=precision,recall=recall,target_met=precision>=.955 and recall>=.955)
        models.append(dict(crf_threshold=ct,report=dict(split='dev',input_sha256=primary['input_sha256'],model_sha256=identity,text_mode=primary['text_mode'],sentences=len(primary['records']),scores=scores,composition=spec,method='Exact Person spans; added development ensemble; confidence thresholds are model-specific ranking scores')))
    return dict(models=models,independent_final_evaluation=False)


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);sub=p.add_subparsers(dest='command',required=True)
    c=sub.add_parser('crf-cache');c.add_argument('--input',type=Path,required=True);c.add_argument('--model',type=Path,required=True);c.add_argument('--output',type=Path,required=True)
    c.add_argument('--text-mode',choices=['canonical','original'],default='canonical')
    n=sub.add_parser('neural-cache');n.add_argument('--cache',type=Path,required=True);n.add_argument('--head',type=Path,required=True);n.add_argument('--output',type=Path,required=True)
    checkpoint=n.add_mutually_exclusive_group(required=True);checkpoint.add_argument('--epoch',type=int);checkpoint.add_argument('--epochs',type=int,nargs='+')
    e=sub.add_parser('evaluate');e.add_argument('--crf',type=Path,required=True);e.add_argument('--neural',type=Path,required=True)
    a=p.parse_args()
    if a.command=='evaluate':print(json.dumps(evaluate(json.loads(external(a.crf).read_text()),json.loads(external(a.neural).read_text())),indent=2))
    else:
        output=external(a.output)
        if output.exists():raise ValueError('refusing to overwrite private predictions')
        result=crf_cache(a.input,a.model,a.text_mode) if a.command=='crf-cache' else neural_cache(a.cache,a.head,a.epochs or a.epoch)
        with output.open('x') as f:json.dump(result,f)
        print(json.dumps(dict(records=len(result['records']),input_sha256=result['input_sha256'],model_sha256=result['model_sha256'])))
