#!/usr/bin/env python3
"""Local frozen IceBERT experiment. Embeddings/weights stay outside the repository."""
import argparse
import hashlib
import json
from pathlib import Path
import time
import sys
from ner import external, rows, spans, tokens_and_labels

LABELS = {'O':0,'B':1,'I':2}
DECODE = {v:k for k,v in LABELS.items()}


def digest(path):
    h=hashlib.sha256()
    with path.open('rb') as f:
        for block in iter(lambda:f.read(1024*1024),b''):h.update(block)
    return h.hexdigest()


def chunks(words, tokenizer, limit=256):
    """Split at word boundaries and never silently truncate subwords."""
    pending=[(0,words)]
    while pending:
        start,part=pending.pop()
        ids=tokenizer(part,is_split_into_words=True,add_special_tokens=True)['input_ids']
        if len(ids)<=limit:
            yield start,part
        elif len(part)>1:
            middle=len(part)//2
            pending.append((start+middle,part[middle:]));pending.append((start,part[:middle]))
        else:raise ValueError('single word exceeds encoder context limit')


def encode(source, split, base, output, batch_size=16, threads=3, text_mode='canonical'):
    import numpy as np
    import torch
    from transformers import AutoModel, AutoTokenizer
    source=external(source);base=external(base);output=external(output)
    output.mkdir(parents=True,exist_ok=False)
    torch.set_num_threads(threads)
    tokenizer=AutoTokenizer.from_pretrained(base,local_files_only=True,add_prefix_space=True,trust_remote_code=False)
    model=AutoModel.from_pretrained(base,local_files_only=True,use_safetensors=True,add_pooling_layer=False,trust_remote_code=False).eval()
    records=[];total=0
    for row in rows(source,split):
        words,offsets,labels=tokens_and_labels(row,text_mode=text_mode)
        records.append((words,offsets,labels,total));total+=len(words)
    if not total:raise ValueError('empty corpus')
    matrix=np.lib.format.open_memmap(output/'x.npy',mode='w+',dtype=np.float16,shape=(total,model.config.hidden_size))
    target=np.lib.format.open_memmap(output/'y.npy',mode='w+',dtype=np.int8,shape=(total,))
    count=0;started=time.monotonic();pending=[];jobs=[]
    def flush():
        nonlocal count
        if not pending:return
        words=[part for _,part in pending]
        batch=tokenizer(words,is_split_into_words=True,padding=True,return_tensors='pt')
        with torch.inference_mode():hidden=model(**batch).last_hidden_state
        for i,(start,part) in enumerate(pending):
            ids=batch.word_ids(i)
            for word in range(len(part)):
                positions=[j for j,w in enumerate(ids) if w==word]
                if not positions:raise ValueError('missing word representation')
                matrix[start+word]=hidden[i,positions].mean(dim=0).numpy().astype(np.float16)
        count+=len(pending);pending.clear()
        if count%160==0:
            print(json.dumps(dict(event='encoded_chunks',chunks=count,seconds=round(time.monotonic()-started,1))),file=sys.stderr,flush=True)
    with (output/'records.jsonl').open('w') as index:
        for words,offsets,labels,start in records:
            target[start:start+len(words)]=[LABELS[l] for l in labels]
            index.write(json.dumps(dict(start=start,end=start+len(words),offsets=offsets))+'\n')
            for shift,part in chunks(words,tokenizer):
                jobs.append((start+shift,part))
    # Length bucketing avoids padding every short sentence to a long neighbor.
    # Destination offsets preserve the source order in the cache.
    jobs.sort(key=lambda job: len(tokenizer(job[1],is_split_into_words=True)['input_ids']))
    for job in jobs:
        pending.append(job)
        if len(pending)>=batch_size:flush()
    flush()
    matrix.flush();target.flush()
    metadata=dict(schema=1,input_sha256=digest(source),split=split,text_mode=text_mode,rows=len(records),tokens=total,hidden_size=model.config.hidden_size,
                  base_model='mideind/IceBERT',base_weights_sha256=digest(base/'model.safetensors'),pooling='mean subwords; <=256-subword chunks at word boundaries',
                  seconds=round(time.monotonic()-started,3),files={name:digest(output/name) for name in ['x.npy','y.npy','records.jsonl']})
    (output/'metadata.json').write_text(json.dumps(metadata,indent=2)+'\n')
    return metadata


def cache(path, split):
    import numpy as np
    path=external(path);meta=json.loads((path/'metadata.json').read_text())
    if meta['split']!=split:raise ValueError('wrong cache split')
    if set(meta['files']) != {'x.npy','y.npy','records.jsonl'}:raise ValueError('incomplete cache manifest')
    for name,value in meta['files'].items():
        if name not in ('x.npy','y.npy','records.jsonl') or digest(path/name)!=value:raise ValueError('cache checksum mismatch')
    x=np.load(path/'x.npy',mmap_mode='r',allow_pickle=False);y=np.load(path/'y.npy',mmap_mode='r',allow_pickle=False)
    if x.shape!=(meta['tokens'],meta['hidden_size']) or y.shape!=(meta['tokens'],):raise ValueError('cache shape mismatch')
    if not np.isfinite(x).all() or not np.isin(y,[0,1,2]).all():raise ValueError('invalid cache values')
    return x,y,meta


def classifier(size, hidden):
    import torch
    if hidden:
        return torch.nn.Sequential(torch.nn.LayerNorm(size),torch.nn.Linear(size,hidden),torch.nn.GELU(),torch.nn.Dropout(.1),torch.nn.Linear(hidden,3))
    return torch.nn.Linear(size,3)


def train(cache_dir, output, epochs=8, hidden=128, lr=.001, seed=42, threads=3):
    import numpy as np
    import torch
    from safetensors.torch import save_file
    x,y,meta=cache(cache_dir,'train');output=external(output);output.mkdir(parents=True,exist_ok=False)
    torch.set_num_threads(threads);torch.manual_seed(seed);rng=np.random.default_rng(seed)
    head=classifier(meta['hidden_size'],hidden)
    optimizer=torch.optim.AdamW(head.parameters(),lr=lr,weight_decay=.01)
    loss_fn=torch.nn.CrossEntropyLoss()
    started=time.monotonic();history=[]
    for epoch in range(1,epochs+1):
        head.train();order=rng.permutation(len(y));loss_sum=0
        for start in range(0,len(order),2048):
            selected=order[start:start+2048]
            inputs=torch.from_numpy(np.array(x[selected],dtype=np.float32))
            labels=torch.from_numpy(np.array(y[selected],dtype=np.int64))
            optimizer.zero_grad();loss=loss_fn(head(inputs),labels);loss.backward();optimizer.step()
            loss_sum+=loss.item()*len(selected)
        record=dict(epoch=epoch,loss=loss_sum/len(y),seconds=round(time.monotonic()-started,1))
        history.append(record);print(json.dumps(record),file=sys.stderr,flush=True)
        save_file(head.state_dict(),str(output/f'epoch-{epoch}.safetensors'))
    report=dict(schema=1,training_cache_sha256=digest(cache_dir/'metadata.json'),training_input_sha256=meta['input_sha256'],base_weights_sha256=meta['base_weights_sha256'],
                pooling=meta['pooling'],hidden_size=meta['hidden_size'],classifier_hidden=hidden,epochs=epochs,lr=lr,seed=seed,history=history,
                weights={f'epoch-{i}.safetensors':digest(output/f'epoch-{i}.safetensors') for i in range(1,epochs+1)},method='Frozen pretrained encoder; train BIO Person classifier only; not Shield')
    (output/'metadata.json').write_text(json.dumps(report,indent=2)+'\n')
    return report


def evaluate(cache_dir, head_dir, epoch, thresholds, threads=3):
    import numpy as np
    import torch
    from safetensors.torch import load_file
    x,y,meta=cache(cache_dir,'dev');head_dir=external(head_dir)
    training=json.loads((head_dir/'metadata.json').read_text());weights=head_dir/f'epoch-{epoch}.safetensors'
    if digest(weights)!=training['weights'][weights.name]:raise ValueError('head checksum mismatch')
    if meta['base_weights_sha256']!=training['base_weights_sha256'] or meta['pooling']!=training['pooling']:raise ValueError('encoder mismatch')
    torch.set_num_threads(threads);head=classifier(training['hidden_size'],training['classifier_hidden']);head.load_state_dict(load_file(str(weights)));head.eval()
    predictions=np.zeros(len(y),dtype=np.int8);confidence=np.zeros(len(y),dtype=np.float32)
    started=time.monotonic()
    with torch.inference_mode():
        for start in range(0,len(y),4096):
            probs=torch.softmax(head(torch.from_numpy(np.array(x[start:start+4096],dtype=np.float32))),dim=-1)
            values,labels=probs.max(dim=-1);predictions[start:start+len(labels)]=labels.numpy();confidence[start:start+len(labels)]=values.numpy()
    scores={t:dict(tp=0,fp=0,fn=0) for t in thresholds}
    cursor=0;record_count=0
    for line in (cache_dir/'records.jsonl').read_text().splitlines():
        r=json.loads(line);a,b=r['start'],r['end'];offsets=r['offsets']
        if a!=cursor or b<=a or b>len(y) or len(offsets)!=b-a:raise ValueError('invalid record coverage')
        if any(s<0 or e<=s or (i and s<offsets[i-1][1]) for i,(s,e) in enumerate(offsets)):raise ValueError('invalid record offsets')
        cursor=b;record_count+=1
        gold={(s,e) for s,e,_,_ in spans([DECODE[int(v)] for v in y[a:b]],offsets)}
        candidates=[(s,e,float(confidence[a+i:a+j].min())) for s,e,i,j in spans([DECODE[int(v)] for v in predictions[a:b]],offsets)]
        for t,score in scores.items():
            selected={(s,e) for s,e,c in candidates if c>=t}
            score['tp']+=len(selected&gold);score['fp']+=len(selected-gold);score['fn']+=len(gold-selected)
    if cursor!=len(y) or record_count!=meta['rows']:raise ValueError('incomplete record coverage')
    for score in scores.values():
        score['precision']=score['tp']/max(1,score['tp']+score['fp']);score['recall']=score['tp']/max(1,score['tp']+score['fn'])
        score['target_met']=score['precision']>=.955 and score['recall']>=.955
    return dict(input_sha256=meta['input_sha256'],base_weights_sha256=meta['base_weights_sha256'],head_sha256=digest(weights),epoch=epoch,split='dev',text_mode=meta['text_mode'],sentences=meta['rows'],scores=scores,seconds=round(time.monotonic()-started,3),method='Exact Person spans; frozen encoder and BIO classifier; min predicted token probability; development only')


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);sub=p.add_subparsers(dest='command',required=True)
    e=sub.add_parser('encode');e.add_argument('--input',type=Path,required=True);e.add_argument('--split',choices=['train','dev'],required=True);e.add_argument('--base',type=Path,required=True);e.add_argument('--output',type=Path,required=True);e.add_argument('--text-mode',choices=['canonical','original'],default='canonical')
    t=sub.add_parser('train');t.add_argument('--cache',type=Path,required=True);t.add_argument('--output',type=Path,required=True);t.add_argument('--epochs',type=int,default=8);t.add_argument('--hidden',type=int,default=128)
    v=sub.add_parser('evaluate');v.add_argument('--cache',type=Path,required=True);v.add_argument('--head',type=Path,required=True);v.add_argument('--epoch',type=int,required=True);v.add_argument('--thresholds',type=float,nargs='+',default=[0,.5,.6,.65,.7,.75,.8,.9,.95])
    for command in (e,t,v):command.add_argument('--threads',type=int,default=3)
    a=p.parse_args()
    if a.threads<1:p.error('threads must be positive')
    if a.command=='encode':result=encode(a.input,a.split,a.base,a.output,threads=a.threads,text_mode=a.text_mode)
    elif a.command=='train':
        if a.epochs<1 or a.hidden<0:p.error('invalid training settings')
        result=train(a.cache,a.output,a.epochs,a.hidden,threads=a.threads)
    else:
        if any(not 0<=t<=1 for t in a.thresholds):p.error('invalid threshold')
        result=evaluate(a.cache,a.head,a.epoch,a.thresholds,threads=a.threads)
    print(json.dumps(result,indent=2))
