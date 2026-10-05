#!/usr/bin/env python3
"""Fixed one-epoch auxiliary-supervision trial; private research artifacts only."""
import argparse
import copy
import json
import math
import os
import random
import time
from pathlib import Path
from encoder import digest
from finetune import detector, expand_entity_head, prepare, tensors, warm_start_record, ENTITY_DECODE
from ner import external

INPUT_SHA = '9f5020c0ea357b6f85fca5162aa6d01d6328669c189ae67ddf9e27731a22387a'
INITIAL_SHA = 'ec2ecc9579816fe059b83cc76cd61a18c7a6aad74253565fa98d0654536d8e9a'
SETTINGS = dict(epochs=1, seed=45, encoder_lr=5e-6, head_lr=5e-5,
                auxiliary_weight=.25, batch_size=8, accumulation=4,
                weight_decay=.01, gradient_clip=1., warmup_fraction=.1,
                threads=3, max_seconds=9000.)


def person_targets(target):
    """Retain O/B/I-Person and padding, map the other six BIO labels to O."""
    import torch
    if not torch.all((target == -100) | ((target >= 0) & (target <= 8))):
        raise ValueError('invalid nine-label target')
    result = target.clone()
    result[result > 2] = 0
    return result


def auxiliary_head(model):
    # A separate module keeps the original three-label head untouched.
    import torch
    holder = torch.nn.Module()
    holder.head = copy.deepcopy(model.head)
    expand_entity_head(holder)
    return holder.head


def forward(model, auxiliary, batch):
    hidden = model.encoder(input_ids=batch[0], attention_mask=batch[1]).last_hidden_state
    import torch
    pooled = torch.matmul(batch[2], hidden)
    return model.head(pooled), auxiliary(pooled)


def train(a):
    import torch
    from transformers import AutoTokenizer
    from safetensors.torch import load_file, save_file
    output, source, base, checkpoint = map(external, (a.output, a.input, a.base, a.initial_checkpoint))
    if digest(source) != INPUT_SHA or digest(checkpoint) != INITIAL_SHA:
        raise ValueError('fixed source or initialization differs')
    initial = warm_start_record(checkpoint, source, base)
    output.mkdir(parents=True, exist_ok=False)
    torch.set_num_threads(SETTINGS['threads']); torch.manual_seed(SETTINGS['seed'])
    tokenizer = AutoTokenizer.from_pretrained(base, local_files_only=True, add_prefix_space=True, trust_remote_code=False)
    records, jobs = prepare(source, 'train', tokenizer, label_mode='entities')
    if len(records) != 46196:
        raise ValueError('unexpected population')
    model = detector(base)
    state = load_file(str(checkpoint))
    if any(v.dtype != torch.float32 or not v.isfinite().all() for v in state.values()):
        raise ValueError('finite FP32 initialization required')
    model.load_state_dict(state); del state
    auxiliary = auxiliary_head(model)
    parity_batch = tensors(jobs[:2], tokenizer.pad_token_id)
    model.eval(); auxiliary.eval()
    with torch.no_grad():
        original = model(*parity_batch[:3])
        primary, _ = forward(model, auxiliary, parity_batch)
        if not torch.equal(original, primary):
            raise ValueError('primary initialization parity failed')
    del original, primary, parity_batch
    model.train(); auxiliary.train()
    parameters = list(model.parameters()) + list(auxiliary.parameters())
    optimizer = torch.optim.AdamW([
        {'params': model.encoder.parameters(), 'lr': SETTINGS['encoder_lr']},
        {'params': model.head.parameters(), 'lr': SETTINGS['head_lr']},
        {'params': auxiliary.parameters(), 'lr': SETTINGS['head_lr']}], weight_decay=SETTINGS['weight_decay'])
    total_steps = math.ceil(math.ceil(len(jobs)/SETTINGS['batch_size'])/SETTINGS['accumulation'])
    warmup = max(1, int(total_steps*SETTINGS['warmup_fraction']))
    def rate(step):
        if step < warmup: return (step+1)/warmup
        return max(0., (total_steps-step)/max(1,total_steps-warmup))
    scheduler = torch.optim.lr_scheduler.LambdaLR(optimizer, rate)
    report = dict(schema=1, method='Separate three-label Person head and nine-label auxiliary entity supervision',
        input_sha256=digest(source), split='train', rows=len(records), chunks=len(jobs),
        base_weights_sha256=digest(base/'model.safetensors'), warm_start=initial,
        label_mode='person', labels=['O','B','I'], auxiliary_labels=ENTITY_DECODE,
        primary_initialization_exact=True, settings=SETTINGS, probe=a.probe,
        torch=torch.__version__, independent_final_evaluation=False, checkpoints=[],
        code_sha256={name:digest(Path(__file__).with_name(name)) for name in
          ['auxiliary_person.py','auxiliary-person-protocol.md','finetune.py','encoder.py','ner.py']})
    (output/'protocol.json').write_text(json.dumps(report,indent=2)+'\n')
    rng=random.Random(SETTINGS['seed']);order=list(range(len(jobs)));rng.shuffle(order)
    batches=[]
    for start in range(0,len(order),256):
        bucket=sorted(order[start:start+256],key=lambda i:len(jobs[i]['ids']))
        batches.extend(bucket[i:i+SETTINGS['batch_size']] for i in range(0,len(bucket),SETTINGS['batch_size']))
    rng.shuffle(batches)
    started=time.monotonic();steps=tokens=0;primary_sum=auxiliary_sum=0.;complete=False
    optimizer.zero_grad(set_to_none=True)
    for b, indices in enumerate(batches):
        batch=tensors([jobs[i] for i in indices],tokenizer.pad_token_id)
        main, other=forward(model,auxiliary,batch)
        main_loss=torch.nn.functional.cross_entropy(main.reshape(-1,3),person_targets(batch[3]).reshape(-1))
        auxiliary_loss=torch.nn.functional.cross_entropy(other.reshape(-1,9),batch[3].reshape(-1))
        loss=main_loss+SETTINGS['auxiliary_weight']*auxiliary_loss
        if not torch.isfinite(loss): raise ValueError('nonfinite training loss')
        n=int((batch[3]!=-100).sum());tokens+=n
        primary_sum+=main_loss.item()*n;auxiliary_sum+=auxiliary_loss.item()*n
        accumulation=SETTINGS['accumulation']
        group_size=min(accumulation,len(batches)-(b//accumulation)*accumulation)
        (loss/group_size).backward()
        if (b+1)%accumulation==0 or b+1==len(batches):
            torch.nn.utils.clip_grad_norm_(parameters,SETTINGS['gradient_clip'])
            optimizer.step();scheduler.step();optimizer.zero_grad(set_to_none=True);steps+=1
            elapsed=time.monotonic()-started
            if steps%10==0:
                print(json.dumps(dict(step=steps,total_steps=total_steps,primary_loss=primary_sum/tokens,
                    auxiliary_loss=auxiliary_sum/tokens,seconds=elapsed)),flush=True)
            if (a.probe and steps>=2) or elapsed>=SETTINGS['max_seconds']:break
    complete=b+1==len(batches)
    item=dict(epoch=1,complete_epoch=complete,optimizer_steps=steps,tokens_seen=tokens,
        primary_loss=primary_sum/tokens,auxiliary_loss=auxiliary_sum/tokens,seconds=time.monotonic()-started)
    if not a.probe:
        for prefix,module in [('',model),('auxiliary_',auxiliary)]:
            path=output/('epoch-1.safetensors' if not prefix else 'auxiliary-epoch-1.safetensors')
            save_file(module.state_dict(),str(path))
            item.update({prefix+'file':path.name,prefix+'sha256':digest(path),prefix+'bytes':path.stat().st_size})
    report['checkpoints'].append(item)
    (output/'training.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(item),flush=True)
    return report


def main():
    os.umask(0o077)
    p=argparse.ArgumentParser(__doc__)
    for name in ['input','base','initial-checkpoint','output']:p.add_argument('--'+name,type=Path,required=True)
    p.add_argument('--probe',action='store_true')
    train(p.parse_args())


if __name__=='__main__':main()
