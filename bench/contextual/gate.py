#!/usr/bin/env python3
"""Check both exact-name quality targets; never substitute accuracy or F1."""
import argparse
import json
import math


def check(score, minimum=.955, allow_empty_predictions=False):
    for key in ('tp','fp','fn'):
        if type(score.get(key)) is not int or score[key] < 0:
            raise ValueError('invalid confusion counts')
    tp,fp,fn=(score[k] for k in ('tp','fp','fn'))
    if (tp+fp == 0 and not allow_empty_predictions) or tp+fn == 0:
        raise ValueError('no predictions or annotated positives')
    precision,recall=tp/max(1,tp+fp),tp/(tp+fn)
    for key,value in [('precision',precision),('recall',recall)]:
        if not math.isclose(score.get(key,-1),value,rel_tol=1e-12,abs_tol=1e-12):
            raise ValueError('reported metric disagrees with confusion counts')
    return dict(precision=precision,recall=recall,minimum=minimum,
                target_met=precision>=minimum and recall>=minimum)


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('report')
    parser.add_argument('--threshold',help='exact score key for a contextual report')
    args=parser.parse_args()
    with open(args.report) as f: report=json.load(f)
    if 'scores' in report:
        if args.threshold not in report['scores']:parser.error('select a recorded threshold')
        report=report['scores'][args.threshold]
    result=check(report)
    print(json.dumps(result,indent=2))
    raise SystemExit(0 if result['target_met'] else 1)

if __name__=='__main__':main()
