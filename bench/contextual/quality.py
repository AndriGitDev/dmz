#!/usr/bin/env python3
"""Exact entity-set accuracy and entity recall; no text or spans in reports.

These metrics describe the supplied, fully labelled scope. Reused development
scores cannot establish independent performance or complete Shield coverage.
"""
from collections import Counter, defaultdict
import math


def _spans(values):
    if not isinstance(values, (list, tuple, set)):
        raise ValueError('null or missing annotations are not negative labels')
    result = set()
    for value in values:
        if not isinstance(value, (list, tuple)) or len(value) != 3:
            raise ValueError('expected (start, end, kind) spans')
        start, end, kind = value
        if (type(start) is not int or type(end) is not int or
                start < 0 or end <= start or not isinstance(kind, str) or not kind):
            raise ValueError('invalid entity span')
        if tuple(value) in result:
            raise ValueError('duplicate entity span')
        result.add(tuple(value))
    return result


def _ratio(numerator, denominator):
    return numerator / denominator if denominator else None


def _metrics(counts):
    tp, fp, fn = (counts[k] for k in ('tp', 'fp', 'fn'))
    return dict(tp=tp, fp=fp, fn=fn, precision=_ratio(tp, tp + fp),
                recall=_ratio(tp, tp + fn), f1=_ratio(2 * tp, 2 * tp + fp + fn))


def score(records, kinds, minimum=.99):
    """Score records containing gold/predicted (UTF-8 start,end,kind) triples.

    Callers must verify source/cache identity, byte boundaries, and complete
    annotation for every declared kind before constructing records. Unexpected
    kinds fail instead of being silently dropped. An excerpt is correct only
    if the two complete sets agree, including reviewed negative excerpts.
    """
    if (isinstance(minimum, bool) or not isinstance(minimum, (float, int)) or
            not math.isfinite(minimum) or not 0 < minimum <= 1):
        raise ValueError('invalid target')
    if (not isinstance(kinds, (list, tuple, set)) or not kinds or
            any(not isinstance(k, str) or not k for k in kinds) or
            len(set(kinds)) != len(kinds)):
        raise ValueError('declare a nonempty unique entity scope')
    kinds = set(kinds)
    total, by_kind, by_stratum = Counter(), defaultdict(Counter), defaultdict(list)
    for row in records:
        gold, predicted = _spans(row.get('gold')), _spans(row.get('predicted'))
        if {s[2] for s in gold | predicted} - kinds:
            raise ValueError('entity outside declared annotation scope')
        stratum = row.get('stratum', 'unspecified')
        if not isinstance(stratum, str) or not stratum:
            raise ValueError('invalid stratum')
        counts = Counter(tp=len(gold & predicted), fp=len(predicted - gold),
                         fn=len(gold - predicted), excerpts=1,
                         exact_excerpts=int(gold == predicted),
                         positive_excerpts=int(bool(gold)),
                         exact_positive_excerpts=int(bool(gold) and gold == predicted),
                         negative_excerpts=int(not gold),
                         false_positive_negative_excerpts=int(not gold and bool(predicted)))
        total.update(counts)
        by_stratum[stratum].append(counts)
        for kind in kinds:
            g, p = {s for s in gold if s[2] == kind}, {s for s in predicted if s[2] == kind}
            by_kind[kind].update(tp=len(g & p), fp=len(p - g), fn=len(g - p))
    if not total['excerpts']:
        raise ValueError('empty evaluation')

    def summarize(counts):
        result = dict(counts) | _metrics(counts)
        result.update(accuracy=counts['exact_excerpts'] / counts['excerpts'],
                      positive_excerpt_accuracy=_ratio(counts['exact_positive_excerpts'], counts['positive_excerpts']),
                      negative_excerpt_false_positive_rate=_ratio(counts['false_positive_negative_excerpts'], counts['negative_excerpts']))
        result['point_targets_met'] = (result['accuracy'] >= minimum and
                                       result['recall'] is not None and result['recall'] >= minimum)
        return result

    strata = {}
    for name, rows in sorted(by_stratum.items()):
        counts = Counter()
        for row in rows:
            counts.update(row)
        strata[name] = summarize(counts)
    return dict(schema=1, metric='exact entity-set accuracy per excerpt',
                annotation_scope=sorted(kinds), minimum=minimum,
                overall=summarize(total), per_type={k: _metrics(by_kind[k]) for k in sorted(kinds)},
                strata=strata, independent_validation=False,
                limitation='Point estimates only; annotation independence, sampling, and system masking require separate evidence.')
