"""Frozen 36-configuration entity-aware development study, no model promotion."""
from fractions import Fraction
import hashlib
import json
from compare_quality import metrics
from ensemble import merge
from entity_veto import evaluate_veto
from quality import score

THRESHOLDS = (0, .5, .6, .65, .7, .75, .8, .9, .95)
PRIORITIES = ('crf', 'neural')


def compose(crf, person, typed, threshold, priority):
    if isinstance(threshold, bool) or threshold not in THRESHOLDS or priority not in PRIORITIES:
        raise ValueError('configuration outside declared family')
    records = []
    removed = dict(false_person=0, gold_person=0)
    for c,p,t in zip(crf['records'],person['records'],typed['records'],strict=True):
        gold={tuple(x) for x in p['gold']}
        other={(s,e) for s,e,k,v in t['predictions'] if k!='Person'}
        kept=[]
        for s,e,v in c['predictions']:
            if (s,e) in other:
                if v>=threshold:removed['gold_person' if (s,e) in gold else 'false_person']+=1
            else:kept.append([s,e,v])
        chosen=merge(kept,p['predictions'],threshold,0) if priority=='crf' else merge(p['predictions'],kept,0,threshold)
        records.append(dict(gold=[(*x,'Person') for x in gold],predicted=[(*x,'Person') for x in chosen]))
    return dict(quality=score(records,['Person']),vetoed=removed)


def study(candidates, baselines):
    if any(type(c['epoch']) is not int for c in candidates) or [c['epoch'] for c in candidates] != [1,2]:
        raise ValueError('both complete epochs required in declared order')
    baseline={d:metrics(baselines[d]) for d in ('general','domain')}
    baseline_recall=min(v[1]['recall'] for v in baseline.values())
    results=[]; qualified=[]
    for candidate in candidates:
        if set(candidate['caches'])!={'general','domain'}:raise ValueError('both strata required')
        for dataset,(crf,person,typed) in candidate['caches'].items():
            evaluate_veto(crf,person,typed)  # validate identities, labels, gold and Person parity
            if person['model_sha256']!=candidate['model_sha256']:raise ValueError('mixed candidate models')
        for threshold in THRESHOLDS:
            for priority in PRIORITIES:
                config=dict(epoch=candidate['epoch'],model_sha256=candidate['model_sha256'],crf_threshold=threshold,
                            neural_threshold=0,priority=priority,veto='exact non-Person argmax span')
                identifier=hashlib.sha256(json.dumps(config,sort_keys=True).encode()).hexdigest()
                strata={};failures=[];ratios=[]
                for dataset,caches in candidate['caches'].items():
                    item=compose(*caches,threshold,priority);counts,values=metrics(item['quality'])
                    old,old_values=baseline[dataset]
                    if (counts['excerpts']!=old['excerpts'] or counts['positive_excerpts']!=old['positive_excerpts'] or
                        counts['tp']+counts['fn']!=old['tp']+old['fn']):raise ValueError('population changed')
                    for key in ('accuracy','precision'):
                        if values[key] is None or values[key]<old_values[key]:failures.append(f'{dataset} {key} below frozen reference')
                    item['delta']={key:float(values[key]-old_values[key]) if values[key] is not None else None for key in values}
                    item['point_targets_met']=values['accuracy']>=Fraction(99,100) and values['recall']>=Fraction(99,100)
                    strata[dataset]=item;ratios.append(values)
                lower=min(v['recall'] for v in ratios)
                if lower<=baseline_recall:failures.append('lower-stratum recall did not improve')
                result=dict(id=identifier,configuration=config,strata=strata,failures=failures,
                            research_gate_passed=not failures,both_strata_point_targets_met=all(s['point_targets_met'] for s in strata.values()))
                results.append(result)
                if not failures:
                    rank=(lower,min(v['accuracy'] for v in ratios),min(v['precision'] for v in ratios),
                          -abs(Fraction(str(threshold))-Fraction(3,4)),priority=='crf',-candidate['epoch'],Fraction(str(threshold)))
                    qualified.append((rank,identifier))
    selected=max(qualified)[1] if qualified else None
    return dict(schema=1,scope='Adaptive reused-development Person-only study',expected_configurations=36,
                configurations=len(results),results=results,selected_research_configuration=selected,
                decision='research candidate qualifies for further checks' if selected else 'retain frozen reference',
                independent_validation=False,browser_promotion_approved=False)
