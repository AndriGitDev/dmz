#!/usr/bin/env python3
"""Offline contextual Person baseline. Corpus/model files must remain outside Git."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import re
import time

REPO = Path(__file__).resolve().parents[2]
PATRONYMIC = re.compile(r'(sonar|syni|son|dóttir|dóttur|burs|bur)$')


def external(path):
    path = path.resolve()
    if path == REPO or REPO in path.parents:
        raise ValueError('corpus/model artifacts must remain outside the repository')
    return path


def rows(path, split):
    with external(path).open() as source:
        for number, line in enumerate(source, 1):
            row = json.loads(line)
            if row.get('split') != split or not row.get('text'):
                raise ValueError(f'invalid split/text at row {number}')
            yield row


def tokens_and_labels(row):
    text = row['text']
    tokens, offsets = [], []
    cursor = 0
    for token in text.split(' '):
        if not token:
            raise ValueError('expected canonical token-spaced text')
        end = cursor + len(token.encode('utf-8'))
        tokens.append(token)
        offsets.append((cursor, end))
        cursor = end + 1
    labels = ['O'] * len(tokens)
    starts = {a: i for i, (a, _) in enumerate(offsets)}
    ends = {b: i for i, (_, b) in enumerate(offsets)}
    for entity in row['entities']:
        a, b = entity['start'], entity['end']
        if a not in starts or b not in ends or a >= b:
            raise ValueError('entity does not align with UTF-8 token boundaries')
        if entity['kind'] != 'Person':
            continue
        i, j = starts[a], ends[b]
        if any(label != 'O' for label in labels[i:j+1]):
            raise ValueError('overlapping Person annotations')
        labels[i:j+1] = ['B'] + ['I'] * (j-i)
    return tokens, offsets, labels


def shape(word):
    return re.sub(r'(.)\1+', r'\1', ''.join('X' if c.isupper() else 'x' if c.islower() else 'd' if c.isdigit() else c for c in word))


def features(tokens, version=1):
    if version not in (1, 2):
        raise ValueError("unsupported feature version")
    basic = []
    for token in tokens:
        lower = token.lower()
        values = ['word='+lower, 'shape='+shape(token), 'title='+str(token.istitle()),
                  'upper='+str(token.isupper()), 'digit='+str(token.isdigit()),
                  'patronymic='+str(bool(PATRONYMIC.search(lower)))]
        values += ['suffix'+str(n)+'='+lower[-n:] for n in range(1, 6)]
        values += ['prefix'+str(n)+'='+lower[:n] for n in range(1, 4)]
        basic.append(values)
    result = []
    for i in range(len(tokens)):
        fs = ['bias']
        for delta in [-2, -1, 0, 1, 2]:
            j = i + delta
            if 0 <= j < len(tokens):
                selected = basic[j] if delta == 0 or version == 2 else basic[j][:6]
                fs.extend(str(delta)+':'+f for f in selected)
            else:
                fs.append(str(delta)+':boundary')
        if version == 2:
            for left, right in [(-1, 0), (0, 1), (-1, 1)]:
                a, b = i + left, i + right
                if a >= 0 and b < len(tokens):
                    fs.append(f'pair{left},{right}=' + tokens[a].lower() + '/' + tokens[b].lower())
                    fs.append(f'shapes{left},{right}=' + shape(tokens[a]) + '/' + shape(tokens[b]))
        result.append(fs)
    return result


def spans(labels, offsets):
    found = []
    start = None
    for i, label in enumerate(labels):
        if label not in ('B', 'I', 'O'):
            raise ValueError('unknown label')
        if start is not None and label != 'I':
            found.append((offsets[start][0], offsets[i-1][1], start, i))
            start = None
        if label == 'B' or (label == 'I' and start is None):
            start = i
    if start is not None:
        found.append((offsets[start][0], offsets[-1][1], start, len(labels)))
    return found


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def train(source, model, iterations, c1, c2, version=1):
    import pycrfsuite
    external(model)
    if model.exists() or model.with_suffix('.metadata.json').exists():
        raise ValueError('refusing to overwrite an existing model')
    trainer = pycrfsuite.Trainer(verbose=False)
    count = 0
    for row in rows(source, 'train'):
        tokens, _, labels = tokens_and_labels(row)
        trainer.append(features(tokens, version), labels)
        count += 1
    if count == 0:
        raise ValueError('empty training corpus')
    trainer.set_params({'c1': c1, 'c2': c2, 'max_iterations': iterations,
                        'feature.possible_transitions': True})
    started = time.monotonic()
    trainer.train(str(model))
    metadata = dict(schema=1, training_sha256=sha(source), model_sha256=sha(model),
                    feature_version=version, training_sentences=count, iterations=iterations, c1=c1, c2=c2,
                    seconds=round(time.monotonic()-started, 3), bytes=model.stat().st_size,
                    features='word/shape/affixes/patronymic; +/-2 token context; BIO Person; v2 adds neighbor affixes and pairs',
                    runtime='python-crfsuite 0.9.12; experimental offline baseline, not Shield')
    model.with_suffix('.metadata.json').write_text(json.dumps(metadata, indent=2)+'\n')
    return metadata


def evaluate(source, model, split, thresholds):
    import pycrfsuite
    tagger = pycrfsuite.Tagger()
    tagger.open(str(external(model)))
    metadata = json.loads(model.with_suffix('.metadata.json').read_text())
    if metadata['model_sha256'] != sha(model):
        raise ValueError('model checksum differs from training metadata')
    version = metadata.get('feature_version', 1)
    scores = {t: dict(tp=0, fp=0, fn=0) for t in thresholds}
    count = 0
    started = time.monotonic()
    for row in rows(source, split):
        tokens, offsets, gold_labels = tokens_and_labels(row)
        gold = {(a,b) for a,b,_,_ in spans(gold_labels, offsets)}
        predicted = tagger.tag(features(tokens, version))
        candidates = [(a,b,min(tagger.marginal(predicted[k],k) for k in range(i,j))) for a,b,i,j in spans(predicted, offsets)]
        for threshold, score in scores.items():
            selected = {(a,b) for a,b,confidence in candidates if confidence >= threshold}
            score['tp'] += len(selected & gold)
            score['fp'] += len(selected - gold)
            score['fn'] += len(gold - selected)
        count += 1
    if count == 0:
        raise ValueError('empty evaluation corpus')
    for score in scores.values():
        score['precision'] = score['tp'] / max(1,score['tp']+score['fp'])
        score['recall'] = score['tp'] / max(1,score['tp']+score['fn'])
        score['target_met'] = score['precision'] >= .955 and score['recall'] >= .955
    return dict(input_sha256=sha(source),model_sha256=sha(model),split=split,sentences=count,
                seconds=round(time.monotonic()-started,3),scores=scores,
                method='exact UTF-8 Person spans; confidence=min BIO token marginal; offline CRF only, not full Shield pipeline')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    sub = parser.add_subparsers(dest='command', required=True)
    t = sub.add_parser('train')
    t.add_argument('--input',type=Path,required=True)
    t.add_argument('--model',type=Path,required=True)
    t.add_argument('--feature-version',type=int,choices=[1,2],default=1)
    t.add_argument('--iterations',type=int,default=100)
    t.add_argument('--c1',type=float,default=.1)
    t.add_argument('--c2',type=float,default=.1)
    e = sub.add_parser('evaluate')
    e.add_argument('--input',type=Path,required=True)
    e.add_argument('--model',type=Path,required=True)
    e.add_argument('--split',choices=['dev','test'],default='dev')
    e.add_argument('--thresholds',type=float,nargs='+',default=[0,.5,.8,.9,.95])
    args = parser.parse_args()
    if args.command == 'train':
        if args.iterations < 1 or not all(math.isfinite(v) and v >= 0 for v in (args.c1, args.c2)): parser.error('invalid training parameters')
        result = train(args.input,args.model,args.iterations,args.c1,args.c2,args.feature_version)
    else:
        if any(not math.isfinite(t) or t<0 or t>1 for t in args.thresholds): parser.error('invalid threshold')
        result = evaluate(args.input,args.model,args.split,args.thresholds)
    print(json.dumps(result,indent=2))


if __name__ == '__main__':
    main()
