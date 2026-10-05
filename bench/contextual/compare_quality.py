#!/usr/bin/env python3
"""Apply the predeclared nine-label research gate; never promote a browser model.

Only complete checkpoints may qualify. Both development strata must preserve
frozen precision and exact excerpt accuracy, and the lower recall must improve.
"""
import argparse
from fractions import Fraction
import json
import math
from pathlib import Path

from ner import external, sha

DATASETS = ('general', 'domain')
TRAINING_INPUT = '9f5020c0ea357b6f85fca5162aa6d01d6328669c189ae67ddf9e27731a22387a'
TRAINING_LABELS = ['O', 'B-Person', 'I-Person', 'B-Organization', 'I-Organization',
                   'B-Location', 'I-Location', 'B-Miscellaneous', 'I-Miscellaneous']
TRAINING_SETTINGS = dict(command='train', epochs=2, seed=44, label_mode='entities',
                         encoder_lr=5e-6, head_lr=5e-5, accumulation=4,
                         max_seconds=16200.0, max_steps=0, probe=False,
                         batch_size=8, threads=3)


def validate_training_recipe(training):
    if (training.get('schema') != 1 or training.get('input_sha256') != TRAINING_INPUT or
            training.get('rows') != 46196 or training.get('labels') != TRAINING_LABELS or
            training.get('initial_head_sha256') is not None or
            training.get('independent_final_evaluation') is not False):
        raise ValueError('training source, label schema or initialization differs from protocol')
    settings = training.get('settings', {})
    if any(settings.get(key) != value or isinstance(settings.get(key), bool) != isinstance(value, bool)
           for key, value in TRAINING_SETTINGS.items()):
        raise ValueError('training settings differ from protocol')
    epochs = set()
    for item in training['checkpoints']:
        epoch = item.get('epoch')
        if (type(epoch) is not int or epoch not in (1, 2) or epoch in epochs or
                type(item.get('complete_epoch')) is not bool):
            raise ValueError('invalid or duplicate training epoch')
        epochs.add(epoch)


def metrics(report):
    if (report.get('schema') != 1 or report.get('annotation_scope') != ['Person'] or
            report.get('metric') != 'exact entity-set accuracy per excerpt' or
            report.get('minimum') != .99 or report.get('independent_validation') is not False):
        raise ValueError('incompatible metric contract')
    values = report['overall']
    for key in ('tp', 'fp', 'fn', 'excerpts', 'exact_excerpts', 'positive_excerpts'):
        if type(values.get(key)) is not int or values[key] < 0:
            raise ValueError('invalid metric counts')
    if (not values['excerpts'] or not values['tp'] + values['fn'] or
            values['exact_excerpts'] > values['excerpts'] or
            values['positive_excerpts'] > values['excerpts']):
        raise ValueError('invalid evaluation population')
    ratios = dict(accuracy=Fraction(values['exact_excerpts'], values['excerpts']),
                  recall=Fraction(values['tp'], values['tp'] + values['fn']),
                  precision=Fraction(values['tp'], values['tp'] + values['fp'])
                  if values['tp'] + values['fp'] else None)
    for key, expected in ratios.items():
        actual = values.get(key)
        if expected is None:
            if actual is not None:
                raise ValueError('ratio differs from counts')
        elif (isinstance(actual, bool) or not isinstance(actual, (int, float)) or
              not math.isfinite(actual) or abs(actual-float(expected)) > 1e-12):
            raise ValueError('ratio differs from counts')
    return values, ratios


def compare(training, candidates, baselines, reference):
    validate_training_recipe(training)
    if reference.get('frozen') is not True or reference.get('thresholds') != dict(crf=.75, neural=0):
        raise ValueError('requires unchanged frozen reference')
    if (training.get('label_mode') != 'entities' or training.get('split') != 'train' or
            training.get('base_weights_sha256') != reference['encoder']['base_weights_sha256'] or
            training.get('warm_start', {}).get('checkpoint_sha256') != reference['composition']['neural_sha256']):
        raise ValueError('training differs from the declared warm-start experiment')
    complete = {}
    for item in training['checkpoints']:
        if item.get('complete_epoch') is True:
            if item['sha256'] in complete:
                raise ValueError('duplicate checkpoint identity')
            complete[item['sha256']] = item
    if not complete:
        raise ValueError('no complete checkpoint to compare')
    baseline_metrics = {}
    for dataset in DATASETS:
        baseline = baselines[dataset]
        if (baseline['input_sha256'] != reference['evaluation_inputs'][dataset] or
                baseline['text_mode'] != ('original' if dataset == 'domain' else 'canonical') or
                baseline['thresholds'] != reference['thresholds']):
            raise ValueError('baseline identity mismatch')
        baseline_metrics[dataset] = metrics(baseline)
    reference_recall = min(pair[1]['recall'] for pair in baseline_metrics.values())
    results, seen = [], set()
    for reports in candidates:
        if set(reports) != set(DATASETS):
            raise ValueError('both development strata are required')
        identity = reports['general']['model_sha256']
        if identity not in complete or identity in seen:
            raise ValueError('incomplete, unknown or duplicate candidate checkpoint')
        seen.add(identity)
        failures, strata, recalls = [], {}, []
        for dataset in DATASETS:
            report, baseline = reports[dataset], baselines[dataset]
            if (report.get('model_sha256') != identity or report.get('label_mode') != 'entities' or
                    report.get('split') != 'dev' or report.get('input_sha256') != baseline['input_sha256'] or
                    report.get('text_mode') != baseline['text_mode'] or
                    report.get('crf_sha256') != reference['composition']['crf_sha256'] or
                    report.get('crf_threshold') != .75 or report.get('neural_threshold') != 0 or
                    report.get('independent_final_evaluation') is not False):
                raise ValueError('candidate evaluation identity mismatch')
            counts, ratios = metrics(report['quality']['combined'])
            old_counts, old_ratios = baseline_metrics[dataset]
            if (counts['excerpts'] != old_counts['excerpts'] or
                    counts['positive_excerpts'] != old_counts['positive_excerpts'] or
                    counts['tp'] + counts['fn'] != old_counts['tp'] + old_counts['fn'] or
                    report.get('rows') != counts['excerpts']):
                raise ValueError('candidate evaluation population changed')
            for metric in ('accuracy', 'precision'):
                if ratios[metric] is None or ratios[metric] < old_ratios[metric]:
                    failures.append(f'{dataset} {metric} below frozen reference')
            recalls.append(ratios['recall'])
            strata[dataset] = dict(counts=counts, delta={
                key: float(ratios[key]-old_ratios[key]) if ratios[key] is not None else None
                for key in ratios}, point_targets_met=ratios['accuracy'] >= Fraction(99, 100)
                and ratios['recall'] >= Fraction(99, 100))
        lower = min(recalls)
        if lower <= reference_recall:
            failures.append('lower-stratum recall did not improve')
        results.append(dict(epoch=complete[identity]['epoch'], model_sha256=identity,
                            lower_recall=float(lower), research_gate_passed=not failures,
                            failures=failures, strata=strata,
                            both_strata_point_targets_met=all(s['point_targets_met'] for s in strata.values())))
    if seen != set(complete):
        raise ValueError('evaluate every complete checkpoint before choosing')
    # Prefer better worst-stratum recall; ties retain the earlier checkpoint.
    qualified = [item for item in results if item['research_gate_passed']]
    selected = max(qualified, key=lambda item: (item['lower_recall'], -item['epoch'])) if qualified else None
    return dict(schema=1, scope='Reused development; Person only', candidates=results,
                decision='research candidate qualifies for further checks' if selected else 'retain frozen reference',
                selected_research_checkpoint=selected['model_sha256'] if selected else None,
                reference_checkpoint=reference['composition']['neural_sha256'],
                independent_final_evaluation=False, browser_promotion_approved=False,
                limitations='Development selection only. Export, browser parity, full-entity evaluation and independent labels remain separate gates.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--training', type=Path, required=True)
    parser.add_argument('--candidate', type=Path, nargs=2, action='append', required=True,
                        metavar=('GENERAL', 'DOMAIN'))
    parser.add_argument('--reference', type=Path, required=True)
    parser.add_argument('--baseline', type=Path, nargs=2, required=True, metavar=('GENERAL', 'DOMAIN'))
    args = parser.parse_args()
    training_path = external(args.training)
    training = json.loads(training_path.read_text())
    for checkpoint in training['checkpoints']:
        path = external(training_path.parent/checkpoint['file'])
        if path.parent != training_path.parent or sha(path) != checkpoint['sha256']:
            raise ValueError('checkpoint artifact mismatch')
    paths = [training_path, args.reference, *args.baseline, *(p for pair in args.candidate for p in pair)]
    report = compare(training,
        [dict(zip(DATASETS, [json.loads(external(p).read_text()) for p in pair])) for pair in args.candidate],
        dict(zip(DATASETS, [json.loads(p.read_text()) for p in args.baseline])),
        json.loads(args.reference.read_text()))
    report['input_hashes'] = [dict(file=p.name, sha256=sha(p)) for p in paths]
    print(json.dumps(report, indent=2))
