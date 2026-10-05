"""Person-only guard on the two hash-pinned, already-used Icelandic dev sets.

This explicitly refuses arbitrary private source files. It never accepts or reads
blind web packets. Predictions remain private and scope is Person-only.
"""
import argparse
import hashlib
import json
from pathlib import Path
import resource
import sys
import time
from evaluate import ROOT, digest, private_output, runner, offline
from compare import add_nonoverlapping
sys.path.insert(0,str(ROOT/'bench/contextual'))
from quality import score
from quality_frozen import evaluate as validate_reference
from ner import spans,tokens_and_labels
from ensemble import merge

PRIVATE=Path('/root/.local/share/dmz')
PINNED={'general':'ddf057b278e1a143da7d1667872cf0c5d66d7a94e63aeebc57fd8b1753f3a2a0','domain':'6246db44f8ba0b2a0f33228c6c6deac2c4937fe442b2e17351292baf3cbd6b87'}

def main():
    p=argparse.ArgumentParser();p.add_argument('--candidate',choices=['presidio','bert','gliner','gliner2','opf'],required=True);p.add_argument('--models',type=Path,required=True);p.add_argument('--output',required=True);p.add_argument('--max-seconds',type=int,default=18000);a=p.parse_args()
    out=private_output(a.output);partial=private_output(str(out)+'.partial.jsonl')
    config=json.loads((ROOT/'bench/open-source-pii/candidates.json').read_text())['candidates'][a.candidate]
    model_dir=a.models/config['model'].replace('/','--')
    if a.candidate!='presidio':
        identity=json.loads((model_dir/'download-manifest.json').read_text())
        if identity['revision']!=config['revision']:raise ValueError('wrong model revision')
        for name,entry in identity['files'].items():
            if digest(model_dir/name)!=entry['sha256']:raise ValueError('changed model asset')
    else:identity={'model':config['model']}
    paths=[Path(__file__),ROOT/'bench/open-source-pii/evaluate.py',ROOT/'bench/open-source-pii/compare.py',ROOT/'bench/contextual/quality.py',ROOT/'bench/contextual/quality_frozen.py',ROOT/'bench/contextual/ner.py',ROOT/'bench/contextual/ensemble.py']
    source_hashes={str(path.relative_to(ROOT)):digest(path) for path in paths}
    selected=PRIVATE/'experiments/full-precision-v1/frozen-selection.json'
    data=[];references={};inputs={}
    for dataset in ['general','domain']:
        src=PRIVATE/('datasets/prepared-v2/dev.jsonl' if dataset=='general' else 'experiments/recall-v2/domain-dev.jsonl')
        if digest(src)!=PINNED[dataset]:raise ValueError('unexpected development input')
        crf_path=PRIVATE/'experiments/recall-v2'/('crf-predictions-dev.json' if dataset=='general' else 'crf-predictions-domain-original.json')
        neural_path=PRIVATE/'experiments/full-precision-v1'/('selected-repeat-general-predictions.json' if dataset=='general' else 'selected-repeat-domain-original-predictions.json')
        references[dataset]=validate_reference(src,crf_path,neural_path,selected,dataset)
        inputs[dataset]={'source_sha256':digest(src),'crf_sha256':digest(crf_path),'neural_sha256':digest(neural_path)}
        rows=[json.loads(line) for line in src.read_text().splitlines()]
        c=json.loads(crf_path.read_text());n=json.loads(neural_path.read_text());mode=n['text_mode']
        for index,(row,crf,neural) in enumerate(zip(rows,c['records'],n['records'],strict=True)):
            text=row['original_text'] if mode=='original' else row['text']
            _,offsets,labels=tokens_and_labels(row,text_mode=mode)
            gold=[[start,end,'name_demo'] for start,end,_,_ in spans(labels,offsets)]
            frozen=[[start,end,'name_demo'] for start,end in merge(crf['predictions'],neural['predictions'],.75,0)]
            data.append({'id':dataset+'-'+str(index),'stratum':dataset,'text':text,'gold':gold,'frozen':frozen})
    offline();start=time.monotonic();predict=runner(a.candidate,config,model_dir);load_seconds=time.monotonic()-start
    records=[];status='complete'
    import torch
    with partial.open('x') as log,torch.inference_mode():
        for i,row in enumerate(data):
            if time.monotonic()-start>a.max_seconds:status='time_limit';break
            entities=predict(row['text']);scored=[];raw=[]
            for e in entities:
                if config['mapping'].get(e['label'])!='name_demo':continue
                x,y=int(e['start']),int(e['end']);text=row['text']
                if not 0<=x<y<=len(text):raise ValueError('invalid offsets')
                span=[len(text[:x].encode()),len(text[:y].encode()),'name_demo']
                if span not in raw:raw.append(span)
                scored.append({'span':span,'score':float(e['score'])})
            r={k:row[k] for k in ['id','stratum','gold','frozen']}
            r.update(text_sha256=hashlib.sha256(row['text'].encode()).hexdigest(),predicted=raw,scored=scored,combined=add_nonoverlapping(row['frozen'],scored))
            records.append(r);log.write(json.dumps(r)+'\n');log.flush()
            if (i+1)%100==0:print(json.dumps({'candidate':a.candidate,'completed':i+1,'total':len(data),'seconds':time.monotonic()-start}),flush=True)
    for path in paths:
        if digest(path)!=source_hashes[str(path.relative_to(ROOT))]:raise ValueError('measurement changed during guard')
    result={'schema':1,'status':status,'candidate':a.candidate,'config':config,'model':identity,'inputs':inputs,'selection_sha256':digest(selected),'measurement_sha256':source_hashes,'scope':'Person only on reused Icelandic development; not all-entity system accuracy or independent acceptance','independent_validation':False,'seconds':time.monotonic()-start,'load_seconds':load_seconds,'peak_rss_kib':resource.getrusage(resource.RUSAGE_SELF).ru_maxrss,'evaluated_excerpts':len(records),'expected_excerpts':len(data),'partial_sha256':digest(partial),'network':'socket connections denied throughout model loading and inference','reference':references}
    if status=='complete':
        result['candidate_alone']=score(records,['name_demo'])
        result['additive']=score([r|{'predicted':r['combined']} for r in records],['name_demo'])
    with out.open('x') as f:json.dump(result,f,indent=2)
    print(json.dumps({k:result[k] for k in ['status','candidate','seconds','evaluated_excerpts','peak_rss_kib']}),flush=True)
if __name__=='__main__':main()
