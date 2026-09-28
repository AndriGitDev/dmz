#!/usr/bin/env python3
"""Build an experimental local lexicon using only MIM training rows and HMS forms."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re


def train(path, addresses, output, minimum=2, purity=0.95):
    repo = Path(__file__).resolve().parents[2]
    if repo == output.resolve() or repo in output.resolve().parents:
        raise ValueError('generated data must stay outside the repository')
    rows, positives = [], Counter()
    for line in path.read_text().splitlines():
        row = json.loads(line)
        if row['split'] != 'train':
            raise ValueError('only training rows may build the lexicon')
        text = row['text']
        rows.append(text.split(' '))
        raw = text.encode()
        for entity in row['entities']:
            if entity['kind'] == 'Person':
                phrase = raw[entity['start']:entity['end']].decode()
                tokens = phrase.split(' ')
                if 1 <= len(tokens) <= 8 and all(re.fullmatch(r"[^\W\d_]+(?:[-’'][^\W\d_]+)*|\.", t) for t in tokens):
                    positives[tuple(tokens)] += 1
    candidates = {p for p, count in positives.items() if count >= minimum}
    totals = Counter()
    lengths = sorted({len(p) for p in candidates})
    for tokens in rows:
        for i in range(len(tokens)):
            for length in lengths:
                phrase = tuple(tokens[i:i+length])
                if phrase in candidates:
                    totals[phrase] += 1
    names = sorted(' '.join(p) for p in candidates if positives[p] / totals[p] >= purity)
    address_forms = sorted({' '.join(s.split()) for s in json.loads(addresses.read_text()) if 1 <= len(s.split()) <= 8 and all(re.fullmatch(r"[^\W\d_]+(?:[-’'][^\W\d_]+)*", t) for t in s.split())})
    pack = dict(schema=1, names=names, addresses=address_forms)
    data = (json.dumps(pack, ensure_ascii=False, separators=(',', ':'))+'\n').encode()
    output.write_bytes(data)
    report = dict(training_sha256=hashlib.sha256(path.read_bytes()).hexdigest(), address_forms_sha256=hashlib.sha256(addresses.read_bytes()).hexdigest(), pack_sha256=hashlib.sha256(data).hexdigest(), names=len(names), addresses=len(address_forms), bytes=len(data), minimum=minimum, purity=purity, training_sentences=len(rows))
    output.with_suffix('.metadata.json').write_text(json.dumps(report, indent=2)+'\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--train', type=Path, required=True)
    parser.add_argument('--addresses', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--minimum', type=int, default=2)
    parser.add_argument('--purity', type=float, default=.95)
    args=parser.parse_args()
    if args.minimum < 1 or not 0 < args.purity <= 1:
        parser.error('invalid thresholds')
    train(args.train,args.addresses,args.output,args.minimum,args.purity)
