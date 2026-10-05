#!/usr/bin/env python3
"""Freeze unused financial articles and tax rulings from bounded index snapshots."""
import argparse
from collections import Counter
import json
from pathlib import Path

from expand_web_sources import select, sha

SEED = 'dmz-domain-review-pool-20261005-v1'
SPECS = [
    ('yskn', ['yskn-2026', 'yskn-2025'], 'yskn.is',
     r'/urskurdir/skoda-urskurd/', 'legal', 'tax-ruling'),
    ('islandssjodir', ['islandssjodir-news'], 'islandssjodir.is',
     r'/frett/[^/]+/?', 'financial', 'fund-manager-news'),
]


def domain_sources(indexes, previous):
    return select(indexes, previous, specs=SPECS, seed=SEED,
                  snapshot='5 October 2026 explicit index snapshots', query_patterns={'yskn': r'nr=\d+'})


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--indexes', type=Path, required=True)
    parser.add_argument('--exclude', type=Path, action='append', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    names = {name for _, names, *_ in SPECS for name in names}
    indexes = {name: (args.indexes/(name+'-links.json')).read_bytes() for name in names}
    result = domain_sources(indexes, [json.loads(path.read_text()) for path in args.exclude])
    result['exclusions_sha256'] = [sha(path.read_bytes()) for path in args.exclude]
    with args.output.open('x') as stream:
        stream.write(json.dumps(result, indent=2, ensure_ascii=False)+'\n')
    print(json.dumps(dict(documents=len(result['documents']),
                         publishers=dict(Counter(d['publisher'] for d in result['documents'])))))
