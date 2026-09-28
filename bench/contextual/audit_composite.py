#!/usr/bin/env python3
"""Aggregate exact-span development diagnostics from private prediction caches."""
import argparse
from collections import Counter, defaultdict
import json
from pathlib import Path
from audit import characteristics, error_categories, training_inventory
from encoder import digest
from ensemble import merge
from ner import external, rows, spans, tokens_and_labels


def audit(source, training, crf_path, neural_path):
    source,training,crf_path,neural_path=map(external,[source,training,crf_path,neural_path])
    crf=json.loads(crf_path.read_text());neural=json.loads(neural_path.read_text())
    for cache in [crf,neural]:
        if cache['input_sha256']!=digest(source) or cache['split']!='dev' or cache['text_mode']!='canonical':
            raise ValueError('prediction cache identity mismatch')
    inventory=training_inventory(training)
    counts=Counter();fn=Counter();fp=Counter();groups=defaultdict(Counter);variants=defaultdict(Counter)
    for row,c,n in zip(rows(source,'dev'),crf['records'],neural['records'],strict=True):
        words,offsets,labels=tokens_and_labels(row)
        gold_spans=spans(labels,offsets);gold={(s,e) for s,e,_,_ in gold_spans}
        if {tuple(v) for v in c['gold']}!=gold or c['gold']!=n['gold']:
            raise ValueError('cache annotation mismatch')
        chosen=merge(c['predictions'],n['predictions'],.75,0)
        proposals={(s,e) for s,e,_ in c['predictions']+n['predictions']}
        neural_spans={(s,e) for s,e,_ in n['predictions']}
        other={(e['start'],e['end']):e['kind'] for e in row['entities'] if e['kind']!='Person'}
        missed,false=error_categories(gold,chosen,proposals,other)
        for span,reason in missed.items():
            if span in neural_spans:reason='correct_neural_span_blocked_by_crf_overlap'
            elif reason=='removed_by_confidence_threshold':reason='exact_crf_span_below_threshold'
            fn[reason]+=1
        fp.update(false.values())
        counts.update(rows=1,tp=len(chosen&gold),fp=len(false),fn=len(missed))
        counts['negative_rows']+=not bool(gold)
        counts['negative_rows_with_findings']+=not bool(gold) and bool(chosen)
        for s,e,i,j in gold_spans:
            for group in characteristics(words[i:j],inventory):
                groups[group]['gold']+=1;groups[group]['correct' if (s,e) in chosen else 'missed']+=1
        if row.get('variant') in {'full','single','upper','initial','negative'}:
            variants[row['variant']].update(rows=1,tp=len(chosen&gold),fp=len(false),fn=len(missed))
    return dict(schema=1,split='dev',input_sha256=digest(source),training_sha256=digest(training),
        crf_sha256=crf['model_sha256'],neural_sha256=neural['model_sha256'],thresholds=dict(crf=.75,neural=0),
        counts=dict(counts),false_negatives=dict(fn),false_positives=dict(fp),
        gold_characteristics={k:dict(v) for k,v in groups.items()},synthetic_variants={k:dict(v) for k,v in variants.items()},
        independent_final_evaluation=False,caveat='Reused development; automated error/coverage categories, not human adjudication; characteristic groups overlap')


if __name__=='__main__':
    p=argparse.ArgumentParser(__doc__)
    for key in ['input','training','crf','neural']:p.add_argument('--'+key,type=Path,required=True)
    a=p.parse_args();print(json.dumps(audit(a.input,a.training,a.crf,a.neural),indent=2))
