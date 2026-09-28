#!/usr/bin/env python3
"""Offline FP32 encoder fine-tuning; train and evaluation are separate commands."""
import argparse
import json
import math
from pathlib import Path
import random
import time

from encoder import LABELS, DECODE, chunks, classifier, digest
from ner import external, rows, spans, tokens_and_labels


def prepare(source, split, tokenizer):
    records, jobs = [], []
    for row in rows(source, split):
        words, offsets, labels = tokens_and_labels(row)
        index = len(records)
        records.append(dict(offsets=offsets, labels=labels))
        for start, part in chunks(words, tokenizer):
            encoded = tokenizer(part, is_split_into_words=True)
            jobs.append(dict(record=index, start=start, words=len(part),
                             ids=encoded['input_ids'], word_ids=encoded.word_ids(),
                             labels=[LABELS[v] for v in labels[start:start+len(part)]]))
    if not jobs:
        raise ValueError('empty input')
    return records, jobs


def tensors(jobs, pad):
    import torch
    batch, length, words = len(jobs), max(len(j['ids']) for j in jobs), max(j['words'] for j in jobs)
    ids = torch.full((batch, length), pad, dtype=torch.long)
    mask = torch.zeros_like(ids)
    pooling = torch.zeros((batch, words, length), dtype=torch.float32)
    target = torch.full((batch, words), -100, dtype=torch.long)
    for i, job in enumerate(jobs):
        n = len(job['ids'])
        ids[i, :n] = torch.tensor(job['ids']); mask[i, :n] = 1
        target[i, :job['words']] = torch.tensor(job['labels'])
        for w in range(job['words']):
            positions = [k for k, v in enumerate(job['word_ids']) if v == w]
            if not positions:
                raise ValueError('missing word representation')
            pooling[i, w, positions] = 1 / len(positions)
    return ids, mask, pooling, target


def detector(base, head_path=None):
    import torch
    from transformers import AutoModel
    from safetensors.torch import load_file
    class Detector(torch.nn.Module):
        def __init__(self):
            super().__init__()
            self.encoder = AutoModel.from_pretrained(base, local_files_only=True, use_safetensors=True,
                add_pooling_layer=False, trust_remote_code=False, attn_implementation='eager')
            self.head = classifier(self.encoder.config.hidden_size, 128)
            if head_path:
                self.head.load_state_dict(load_file(str(head_path)))

        def forward(self, input_ids, attention_mask, pooling):
            hidden = self.encoder(input_ids=input_ids, attention_mask=attention_mask).last_hidden_state
            return self.head(torch.matmul(pooling, hidden))
    return Detector()


def train(a):
    import torch
    from transformers import AutoTokenizer
    from safetensors.torch import save_file
    output = external(a.output); output.mkdir(parents=True, exist_ok=False)
    source, base, head = external(a.input), external(a.base), external(a.head)
    torch.set_num_threads(a.threads); torch.manual_seed(a.seed)
    tokenizer = AutoTokenizer.from_pretrained(base, local_files_only=True, add_prefix_space=True, trust_remote_code=False)
    records, jobs = prepare(source, 'train', tokenizer)
    model = detector(base, head).train()
    optimizer = torch.optim.AdamW([
        {'params': model.encoder.parameters(), 'lr': a.encoder_lr},
        {'params': model.head.parameters(), 'lr': a.head_lr}], weight_decay=.01)
    total_steps = math.ceil(math.ceil(len(jobs) / a.batch_size) / a.accumulation) * a.epochs
    warmup = max(1, int(total_steps * .1))
    def rate(step):
        if step < warmup: return (step + 1) / warmup
        return max(0., (total_steps - step) / max(1, total_steps - warmup))
    scheduler = torch.optim.lr_scheduler.LambdaLR(optimizer, rate)
    report = dict(schema=1, method='FP32 full encoder and BIO head fine-tuning; mean subword pooling without float16 round-trip',
        input_sha256=digest(source), split='train', rows=len(records), chunks=len(jobs),
        base_weights_sha256=digest(base/'model.safetensors'), initial_head_sha256=digest(head),
        settings={k: v for k, v in vars(a).items() if isinstance(v, (str, int, float, bool))},
        torch=torch.__version__, checkpoints=[], independent_final_evaluation=False)
    (output/'protocol.json').write_text(json.dumps(report, indent=2)+'\n')
    started = time.monotonic(); steps = 0; rng = random.Random(a.seed)
    for epoch in range(1, a.epochs + 1):
        order = list(range(len(jobs))); rng.shuffle(order)
        # Shuffle groups of similarly sized sequences, preserving every training chunk.
        batches = []
        for begin in range(0, len(order), 256):
            bucket = sorted(order[begin:begin+256], key=lambda i: len(jobs[i]['ids']))
            batches.extend(bucket[i:i+a.batch_size] for i in range(0, len(bucket), a.batch_size))
        rng.shuffle(batches)
        loss_sum = tokens = 0; complete = True
        optimizer.zero_grad(set_to_none=True)
        for b, indices in enumerate(batches):
            batch = tensors([jobs[i] for i in indices], tokenizer.pad_token_id)
            logits = model(*batch[:3])
            loss = torch.nn.functional.cross_entropy(logits.reshape(-1, 3), batch[3].reshape(-1))
            if not torch.isfinite(loss): raise ValueError('nonfinite training loss')
            n = int((batch[3] != -100).sum()); loss_sum += loss.item()*n; tokens += n
            group_size = min(a.accumulation, len(batches) - (b // a.accumulation)*a.accumulation)
            (loss / group_size).backward()
            if (b+1) % a.accumulation == 0 or b+1 == len(batches):
                torch.nn.utils.clip_grad_norm_(model.parameters(), 1.)
                optimizer.step(); scheduler.step(); optimizer.zero_grad(set_to_none=True); steps += 1
                elapsed = time.monotonic()-started
                if steps % 10 == 0:
                    print(json.dumps(dict(epoch=epoch, step=steps, batches=b+1, epoch_batches=len(batches), loss=loss_sum/tokens, seconds=elapsed)), flush=True)
                if (a.max_steps and steps >= a.max_steps) or elapsed >= a.max_seconds:
                    complete = b+1 == len(batches)
                    break
        record = dict(epoch=epoch, complete_epoch=complete, optimizer_steps=steps,
                      tokens_seen=tokens, loss=loss_sum/tokens, seconds=time.monotonic()-started)
        if not a.probe:
            path = output/f'epoch-{epoch}.safetensors'
            save_file(model.state_dict(), str(path))
            record.update(file=path.name, sha256=digest(path), bytes=path.stat().st_size)
        report['checkpoints'].append(record)
        (output/'training.json').write_text(json.dumps(report, indent=2)+'\n')
        print(json.dumps(record), flush=True)
        if (a.max_steps and steps >= a.max_steps) or time.monotonic()-started >= a.max_seconds:
            break
    return report


def evaluate(a):
    import torch
    from transformers import AutoTokenizer
    from safetensors.torch import load_file
    from ensemble import merge
    source, base, checkpoint = external(a.input), external(a.base), external(a.checkpoint)
    output = external(a.output)
    if output.exists(): raise ValueError('refusing to replace evaluation')
    training = json.loads((checkpoint.parent/'training.json').read_text())
    if not any(c.get('sha256') == digest(checkpoint) for c in training['checkpoints']):
        raise ValueError('checkpoint manifest mismatch')
    if training['base_weights_sha256'] != digest(base/'model.safetensors'):
        raise ValueError('base mismatch')
    torch.set_num_threads(a.threads)
    tokenizer = AutoTokenizer.from_pretrained(base, local_files_only=True, add_prefix_space=True, trust_remote_code=False)
    records, jobs = prepare(source, 'dev', tokenizer)
    model = detector(base); model.load_state_dict(load_file(str(checkpoint))); model.eval()
    labels = [[None]*len(r['labels']) for r in records]; confidence = [[None]*len(r['labels']) for r in records]
    jobs.sort(key=lambda j: len(j['ids'])); started=time.monotonic()
    with torch.inference_mode():
        for begin in range(0, len(jobs), a.batch_size):
            selected=jobs[begin:begin+a.batch_size]; batch=tensors(selected, tokenizer.pad_token_id)
            values, predicted = model(*batch[:3]).softmax(-1).max(-1)
            for i,j in enumerate(selected):
                r,s,n=j['record'],j['start'],j['words']
                labels[r][s:s+n]=[DECODE[v] for v in predicted[i,:n].tolist()]
                confidence[r][s:s+n]=values[i,:n].tolist()
    predictions=[]
    for r,l,c in zip(records,labels,confidence):
        if any(v is None for v in l+c): raise ValueError('incomplete prediction coverage')
        predictions.append(dict(gold=[[s,e] for s,e,_,_ in spans(r['labels'],r['offsets'])],
            predictions=[[s,e,min(c[i:j])] for s,e,i,j in spans(l,r['offsets'])]))
    result=dict(split='dev', input_sha256=digest(source), model_sha256=digest(checkpoint), text_mode='canonical', records=predictions)
    if a.predictions:
        with external(a.predictions).open('x') as f: json.dump(result,f)
    crf=json.loads(external(a.crf).read_text())
    for key in ('input_sha256','split','text_mode'):
        if crf[key]!=result[key]:raise ValueError('CRF identity mismatch')
    if len(crf['records'])!=len(predictions):raise ValueError('CRF length mismatch')
    scores={name:dict(tp=0,fp=0,fn=0) for name in ('neural','combined')}
    for p,c in zip(predictions,crf['records']):
        if p['gold']!=c['gold']:raise ValueError('annotation mismatch')
        gold={tuple(x) for x in p['gold']}
        choices=dict(neural={(s,e) for s,e,v in p['predictions'] if v>=a.neural_threshold},
                     combined=merge(c['predictions'],p['predictions'],a.crf_threshold,a.neural_threshold))
        for name,chosen in choices.items():
            scores[name]['tp']+=len(chosen&gold); scores[name]['fp']+=len(chosen-gold); scores[name]['fn']+=len(gold-chosen)
    for score in scores.values():
        score['precision']=score['tp']/max(1,score['tp']+score['fp']);score['recall']=score['tp']/max(1,score['tp']+score['fn'])
        score['target_met']=score['precision']>=.955 and score['recall']>=.955
    report={k:v for k,v in result.items() if k!='records'}
    report.update(scores=scores, crf_sha256=crf['model_sha256'], crf_threshold=a.crf_threshold,
        neural_threshold=a.neural_threshold, seconds=time.monotonic()-started, rows=len(records),
        independent_final_evaluation=False, method='Reused development; exact Person spans; FP32 fine-tuned encoder')
    with output.open('x') as f:json.dump(report,f,indent=2)
    return report


def main():
    p=argparse.ArgumentParser(__doc__); sub=p.add_subparsers(dest='command',required=True)
    t=sub.add_parser('train');t.add_argument('--head',type=Path,required=True)
    t.add_argument('--epochs',type=int,default=3);t.add_argument('--seed',type=int,default=42)
    t.add_argument('--encoder-lr',type=float,default=1e-5);t.add_argument('--head-lr',type=float,default=1e-4)
    t.add_argument('--accumulation',type=int,default=4);t.add_argument('--max-seconds',type=float,default=18000)
    t.add_argument('--max-steps',type=int,default=0);t.add_argument('--probe',action='store_true')
    e=sub.add_parser('evaluate');e.add_argument('--checkpoint',type=Path,required=True);e.add_argument('--crf',type=Path,required=True)
    e.add_argument('--predictions',type=Path);e.add_argument('--crf-threshold',type=float,default=.75);e.add_argument('--neural-threshold',type=float,default=0.)
    for q in (t,e):
        q.add_argument('--base',type=Path,required=True);q.add_argument('--input',type=Path,required=True);q.add_argument('--output',type=Path,required=True)
        q.add_argument('--batch-size',type=int,default=8);q.add_argument('--threads',type=int,default=3)
    a=p.parse_args()
    if a.batch_size<1 or a.threads<1:p.error('batch size and threads must be positive')
    if a.command=='train' and (a.accumulation<1 or a.epochs<1 or a.max_seconds<=0):p.error('invalid training limits')
    print(json.dumps(train(a) if a.command=='train' else evaluate(a),indent=2))


if __name__=='__main__':main()
