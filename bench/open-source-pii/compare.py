"""Replay initial, predeclared additive compositions from verified caches."""
import argparse
import json
from pathlib import Path
from evaluate import load_corpus, ROOT, digest, private_output
from quality import score

def overlaps(a,b):return a[0]<b[1] and b[0]<a[1]
def add_nonoverlapping(base,scored):
    result=[list(s) for s in base]
    for item in sorted(scored,key=lambda x:(-x['score'],x['span'][0],x['span'][1],x['span'][2])):
        span=item['span']
        if not any(overlaps(span,old) for old in result):result.append(span)
    return sorted(result)

def compare(reference_path,candidate_paths,corpus):
    rows,manifest=load_corpus(corpus)
    reference=json.loads(reference_path.read_text())
    if reference['status']!='complete' or reference['corpus']!=manifest:raise ValueError('wrong baseline')
    base={r['id']:r for r in reference['records']}
    if set(base)!={r['id'] for r in rows}:raise ValueError('incomplete reference')
    result={'schema':1,'independent_validation':False,'reference_sha256':digest(reference_path),'corpus':manifest,'runs':{}}
    for mode in ['basic','trained']:
        records=[]
        for row in rows:
            rec=base[row['id']]
            import hashlib
            if rec['text_sha256']!=hashlib.sha256(row['text'].encode()).hexdigest():raise ValueError('text changed')
            records.append({'gold':row['gold'],'predicted':rec[mode]['predicted'],'abstained':rec[mode]['abstained'],'stratum':row['stratum']})
        result['runs']['frozen-'+mode]=score(records,manifest['scope'])
    for path in candidate_paths:
        d=json.loads(path.read_text())
        if d['status']!='complete' or d['corpus']!=manifest:raise ValueError('wrong candidate cache')
        predictions={r['id']:r for r in d['records']}
        if set(predictions)!=set(base):raise ValueError('incomplete candidate')
        for identifier,p in predictions.items():
            if p['text_sha256']!=base[identifier]['text_sha256']:raise ValueError('candidate text mismatch')
        result['runs'][d['candidate']+'-alone']=score(d['records'],manifest['scope'])
        for mode in ['basic','trained']:
            records=[]
            for row in rows:
                original=base[row['id']][mode]
                predicted=add_nonoverlapping(original['predicted'],predictions[row['id']]['scored']) if not original['abstained'] else []
                records.append({'gold':row['gold'],'predicted':predicted,'stratum':row['stratum'],'abstained':original['abstained']})
            result['runs'][d['candidate']+'-plus-'+mode]=score(records,manifest['scope'])
        result.setdefault('candidate_hashes',{})[d['candidate']]=digest(path)
    return result

if __name__=='__main__':
    p=argparse.ArgumentParser();p.add_argument('--reference',type=Path,required=True);p.add_argument('--candidate',type=Path,action='append',required=True);p.add_argument('--output',required=True);p.add_argument('--corpus',type=Path,default=ROOT/'bench/corpus/open-source-pii-v1');a=p.parse_args()
    out=private_output(a.output);result=compare(a.reference,a.candidate,a.corpus)
    with out.open('x') as f:json.dump(result,f,indent=2)
    for name,run in result['runs'].items():print(name,' | '.join(f'{s}: exact {v["exact_excerpts"]}/{v["excerpts"]}, TP/FP/FN {v["tp"]}/{v["fp"]}/{v["fn"]}' for s,v in run['strata'].items()))
