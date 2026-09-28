#!/usr/bin/env python3
"""Select development candidates under the recorded two-dataset precision rule."""
import argparse
import json
import math
from pathlib import Path
from gate import check


def select(candidates):
    eligible=[];trials=[];identities=None;gold_counts=None;grid=None
    for name,general,domain in candidates:
        if general['split']!='dev' or domain['split']!='dev':raise ValueError('selection is development-only')
        pair=(general['input_sha256'],domain['input_sha256'],general.get('text_mode','canonical'),domain.get('text_mode','canonical'))
        if identities is None:identities=pair
        if identities!=pair:raise ValueError('candidates use different development inputs')
        model=lambda report:report.get('head_sha256',report.get('model_sha256'))
        if not model(general) or model(general)!=model(domain):raise ValueError('mismatched model pair')
        if set(general['scores'])!=set(domain['scores']):raise ValueError('mismatched threshold grids')
        current=set(general['scores'])
        if not current or any(not math.isfinite(float(t)) or not 0<=float(t)<=1 for t in current):raise ValueError('invalid threshold grid')
        if grid is None:grid=current
        if current!=grid:raise ValueError('candidates use different threshold grids')
        for threshold,g in general['scores'].items():
            d=domain['scores'][threshold];check(g);check(d)
            totals=(g['tp']+g['fn'],d['tp']+d['fn'])
            if gold_counts is None:gold_counts=totals
            if totals!=gold_counts:raise ValueError('inconsistent annotation counts')
            entry=dict(candidate=name,model_sha256=model(general),threshold=float(threshold),general=g,domain=d)
            entry['precision_qualified']=g['precision']>=.955 and d['precision']>=.955
            entry['lower_recall']=min(g['recall'],d['recall'])
            entry['target_met']=all(s['precision']>=.955 and s['recall']>=.955 for s in (g,d))
            trials.append(entry)
            if entry['precision_qualified']:eligible.append(entry)
    selected=max(eligible,key=lambda e:(e['lower_recall'],min(e['general']['precision'],e['domain']['precision']))) if eligible else None
    return dict(schema=1,split='dev',rule='Precision >=.955 on both datasets at one shared threshold; maximize the lower recall; ties maximize the minimum precision across datasets',
                inputs=identities,selected=selected,trials=trials,independent_final_evaluation=False)


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('manifest',type=Path)
    a=p.parse_args();manifest=json.loads(a.manifest.read_text());root=a.manifest.parent
    candidates=[(c['name'],json.loads((root/c['general']).read_text()),json.loads((root/c['domain']).read_text())) for c in manifest]
    print(json.dumps(select(candidates),indent=2))
