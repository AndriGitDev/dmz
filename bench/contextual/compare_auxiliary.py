"""Strict comparison for the single auxiliary Person trial, no promotion."""
from fractions import Fraction
from auxiliary_person import INPUT_SHA, INITIAL_SHA, SETTINGS
from compare_quality import metrics
from finetune import ENTITY_DECODE


def compare(training, reports, baselines, reference):
    if (training.get('schema') != 1 or training.get('input_sha256') != INPUT_SHA or
        training.get('rows') != 46196 or training.get('chunks') != 46236 or
        training.get('split') != 'train' or training.get('label_mode') != 'person' or
        training.get('labels') != ['O','B','I'] or training.get('auxiliary_labels') != ENTITY_DECODE or
        training.get('settings') != SETTINGS or
        any(isinstance(training['settings'][k],bool)!=isinstance(v,bool) for k,v in SETTINGS.items()) or
        training.get('probe') is not False or
        training.get('primary_initialization_exact') is not True or
        training.get('warm_start',{}).get('checkpoint_sha256') != INITIAL_SHA or
        training.get('independent_final_evaluation') is not False):
        raise ValueError('auxiliary recipe differs from predeclared trial')
    if (reference.get('frozen') is not True or reference.get('thresholds') != dict(crf=.75,neural=0) or
        reference['composition']['neural_sha256'] != INITIAL_SHA or
        reference['encoder']['base_weights_sha256'] != training.get('base_weights_sha256')):
        raise ValueError('frozen reference identity mismatch')
    checkpoints=training.get('checkpoints',[])
    if len(checkpoints)!=1:
        raise ValueError('exactly one checkpoint required')
    checkpoint=checkpoints[0]
    if (type(checkpoint.get('epoch')) is not int or checkpoint['epoch']!=1 or
        checkpoint.get('complete_epoch') is not True or checkpoint.get('optimizer_steps')!=1445 or
        checkpoint.get('tokens_seen')!=824869 or not checkpoint.get('auxiliary_sha256')):
        raise ValueError('complete fixed epoch required')
    if set(reports)!= {'general','domain'} or set(baselines)!=set(reports):
        raise ValueError('both strata required')
    failures=[];strata={};candidate_recalls=[];baseline_recalls=[]
    for dataset in ('general','domain'):
        report,baseline=reports[dataset],baselines[dataset]
        if (baseline.get('input_sha256')!=reference['evaluation_inputs'][dataset] or
            baseline.get('text_mode')!=('canonical' if dataset=='general' else 'original') or
            baseline.get('thresholds')!=reference['thresholds']):
            raise ValueError('baseline identity mismatch')
        if (report.get('model_sha256')!=checkpoint.get('sha256') or report.get('label_mode')!='person' or
            report.get('split')!='dev' or report.get('input_sha256')!=baseline['input_sha256'] or
            report.get('text_mode')!=baseline['text_mode'] or
            report.get('crf_sha256')!=reference['composition']['crf_sha256'] or
            report.get('crf_threshold')!=.75 or report.get('neural_threshold')!=0 or
            report.get('independent_final_evaluation') is not False):
            raise ValueError('evaluation identity mismatch')
        counts,ratios=metrics(report['quality']['combined']);old,old_ratios=metrics(baseline)
        if (counts['excerpts']!=old['excerpts'] or counts['positive_excerpts']!=old['positive_excerpts'] or
            counts['tp']+counts['fn']!=old['tp']+old['fn'] or report.get('rows')!=counts['excerpts']):
            raise ValueError('evaluation population changed')
        for metric in ('accuracy','precision'):
            if ratios[metric] is None or ratios[metric]<old_ratios[metric]:
                failures.append(f'{dataset} {metric} below frozen reference')
        candidate_recalls.append(ratios['recall']);baseline_recalls.append(old_ratios['recall'])
        strata[dataset]=dict(counts=counts,delta={k:float(ratios[k]-old_ratios[k]) if ratios[k] is not None else None for k in ratios},
            point_targets_met=ratios['accuracy']>=Fraction(99,100) and ratios['recall']>=Fraction(99,100))
    if min(candidate_recalls)<=min(baseline_recalls):failures.append('lower-stratum recall did not improve')
    return dict(schema=1,scope='Reused development; fixed auxiliary-supervision trial; Person only',
        research_gate_passed=not failures,failures=failures,strata=strata,
        lower_recall=float(min(candidate_recalls)),both_strata_point_targets_met=all(s['point_targets_met'] for s in strata.values()),
        decision='retain frozen reference' if failures else 'research candidate qualifies for further checks',
        selected_research_checkpoint=None if failures else checkpoint['sha256'],
        candidate_checkpoint=checkpoint['sha256'],reference_checkpoint=INITIAL_SHA,
        independent_final_evaluation=False,browser_promotion_approved=False)
