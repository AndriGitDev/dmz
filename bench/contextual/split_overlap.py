#!/usr/bin/env python3
"""Aggregate exact/NFC/casefold token overlap; no inference or split changes."""
import argparse
from collections import defaultdict
import hashlib
import json
import os
from pathlib import Path
import unicodedata
from ner import external, tokens_and_labels, ENTITY_KINDS

MODES = ('exact_tokens','nfc_tokens','nfc_casefold_tokens')
LABELS = {'O'} | {prefix+'-'+kind for prefix in ('B','I') for kind in ENTITY_KINDS}


def sequence_key(words,mode):
    if mode not in MODES:raise ValueError('unknown comparison mode')
    if mode=='exact_tokens':return tuple(words)
    normalized=tuple(unicodedata.normalize('NFC',w) for w in words)
    return tuple(w.casefold() for w in normalized) if mode=='nfc_casefold_tokens' else normalized


def audit(training,development):
    for data in [training,*development.values()]:
        for words,labels in data:
            if not words or len(words)!=len(labels) or any(not isinstance(w,str) or not w for w in words) or not set(labels)<=LABELS:
                raise ValueError('invalid validated token/label sequence')
    comparisons={}
    for mode in MODES:
        index=defaultdict(set);counts=defaultdict(int)
        for words,labels in training:
            key=sequence_key(words,mode);index[key].add(tuple(labels));counts[key]+=1
        result=dict(unique_training_sequences=len(index),training_duplicate_sequences=sum(n>1 for n in counts.values()),
            training_sequences_with_multiple_full_labelings=sum(len(v)>1 for v in index.values()),strata={})
        for name,data in development.items():
            hits=[(words,labels) for words,labels in data if sequence_key(words,mode) in index]
            result['strata'][name]=dict(rows=len(data),overlapping_rows=len(hits),
                overlapping_rows_at_least_five_tokens=sum(len(w)>=5 for w,l in hits),
                overlapping_person_positive_rows=sum(any(t.endswith('-Person') for t in l) for w,l in hits),
                overlap_label_sequence_not_seen_in_training=sum(tuple(l) not in index[sequence_key(w,mode)] for w,l in hits))
        comparisons[mode]=result
    return dict(train_rows=len(training),comparisons=comparisons)


def load(path,split):
    content=external(path).read_bytes();result=[]
    for number,line in enumerate(content.decode('utf-8').splitlines(),1):
        row=json.loads(line)
        if row.get('split')!=split or not row.get('text') or not isinstance(row.get('entities'),list) or any(not isinstance(e,dict) or not isinstance(e.get('kind'),str) or not e['kind'] for e in row['entities']):
            raise ValueError(f'invalid canonical row {number}')
        words,_,labels=tokens_and_labels(row,label_mode='entities')
        result.append((tuple(words),tuple(labels)))
    if not result:raise ValueError('empty corpus')
    return result,hashlib.sha256(content).hexdigest()


def main():
    p=argparse.ArgumentParser(__doc__)
    for name in ('train','general','domain','output'):p.add_argument('--'+name,type=Path,required=True)
    a=p.parse_args();os.umask(0o077)
    output=external(a.output)
    if output.exists():raise ValueError('refusing to replace audit')
    data={};hashes={}
    for name in ('train','general','domain'):data[name],hashes[name]=load(getattr(a,name),'train' if name=='train' else 'dev')
    report=audit(data['train'],{k:data[k] for k in ('general','domain')})
    report.update(schema=1,scope='Read-only canonical token overlap audit of reused training/development; no inference or blind data',
        input_sha256=hashes,label_scope=sorted(ENTITY_KINDS),label_projection='O plus B/I for the four modelled entity types; other annotated kinds become background',code_sha256={n:hashlib.sha256(Path(__file__).with_name(n).read_bytes()).hexdigest() for n in ('split_overlap.py','ner.py')},
        independent_validation=False,limitations='Sequence matching does not establish document disjointness, exclude paraphrases, adjudicate labels or make reused development independent. Case changes may legitimately change labels. No inputs, labels, splits, thresholds or model decisions are changed.')
    with output.open('x') as f:json.dump(report,f,indent=2)
    print(json.dumps(dict(train_rows=report['train_rows'],strata={m:r['strata'] for m,r in report['comparisons'].items()}),indent=2))


if __name__=='__main__':main()
