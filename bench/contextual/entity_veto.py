"""Research-only exact non-Person veto; no thresholds or browser promotion."""
import math
from ensemble import merge
from ner import spans
from quality import score

KINDS = ('Person', 'Organization', 'Location', 'Miscellaneous')
LABELS = ['O'] + [f'{prefix}-{kind}' for kind in KINDS for prefix in ('B','I')]
METHOD = 'exact non-Person argmax span veto before CRF-priority merge; CRF=.75 neural=0'


def typed_spans(labels, offsets, confidence):
    if len(labels) != len(offsets) or len(labels) != len(confidence):
        raise ValueError('token coverage differs')
    found, active, kind = [], None, None
    previous_end = 0
    for i, (label, (start, end), probability) in enumerate(zip(labels, offsets, confidence)):
        if (type(start) is not int or type(end) is not int or start < previous_end or end <= start or
                isinstance(probability, bool) or not isinstance(probability, (int, float)) or
                not math.isfinite(probability) or not 0 <= probability <= 1):
            raise ValueError('invalid token interval or confidence')
        previous_end = end
        prefix, current = ('O', None) if label == 'O' else label.split('-', 1)
        if label != 'O' and (prefix not in ('B', 'I') or current not in KINDS):
            raise ValueError('invalid entity label')
        if active is not None and (prefix != 'I' or current != kind):
            found.append([offsets[active][0], offsets[i-1][1], kind, min(confidence[active:i])])
            active = None
        if prefix != 'O' and active is None:
            active, kind = i, current
    if active is not None:
        found.append([offsets[active][0], offsets[-1][1], kind, min(confidence[active:])])
    # Preserve the established Person decoder exactly, including orphan I tags.
    person = [[s, e, min(confidence[i:j])] for s, e, i, j in spans(labels, offsets)]
    if [[s, e, p] for s, e, k, p in found if k == 'Person'] != person:
        raise ValueError('Person decoder parity failure')
    return found


def evaluate_veto(crf, person, typed):
    for key in ('input_sha256', 'split', 'text_mode'):
        if crf[key] != person[key] or person[key] != typed[key]:
            raise ValueError('cache identity mismatch')
    if (typed.get('schema') != 1 or typed.get('label_mode') != 'entities' or typed.get('labels') != LABELS or
            typed['model_sha256'] != person['model_sha256'] or typed['split'] != 'dev'):
        raise ValueError('requires matching nine-label development cache')
    records, original = [], []
    counts = dict(vetoed_crf=0, vetoed_gold_person=0, vetoed_false_person=0)
    for c, p, t in zip(crf['records'], person['records'], typed['records'], strict=True):
        if c['gold'] != p['gold'] or p['gold'] != t['gold']:
            raise ValueError('gold mismatch')
        parsed, previous_end = [], 0
        for start, end, kind, confidence in t['predictions']:
            if (type(start) is not int or type(end) is not int or start < previous_end or end <= start or
                    kind not in KINDS or isinstance(confidence, bool) or not isinstance(confidence, (int, float)) or
                    not math.isfinite(confidence) or not 0 <= confidence <= 1):
                raise ValueError('invalid typed span')
            previous_end = end
            parsed.append([start, end, kind, confidence])
        if [[s,e,v] for s,e,k,v in parsed if k == 'Person'] != p['predictions']:
            raise ValueError('replayed Person predictions changed')
        other = {(s,e) for s,e,k,_ in parsed if k != 'Person'}
        kept = []
        gold = {tuple(x) for x in p['gold']}
        for start, end, confidence in c['predictions']:
            if confidence >= .75 and (start, end) in other:
                counts['vetoed_crf'] += 1
                counts['vetoed_gold_person' if (start,end) in gold else 'vetoed_false_person'] += 1
            else:
                kept.append([start,end,confidence])
        predicted = merge(kept, p['predictions'], .75, 0)
        before = merge(c['predictions'], p['predictions'], .75, 0)
        records.append(dict(gold=[(*x,'Person') for x in gold], predicted=[(*x,'Person') for x in predicted]))
        original.append(dict(gold=[(*x,'Person') for x in gold], predicted=[(*x,'Person') for x in before]))
    if not records:
        raise ValueError('empty evaluation')
    return dict(method=METHOD, quality=score(records,['Person']),
                original_quality=score(original,['Person']), diagnostics=counts)
