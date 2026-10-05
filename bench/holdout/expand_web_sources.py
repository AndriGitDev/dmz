#!/usr/bin/env python3
"""Freeze unused public article URLs from prior model-blind index snapshots."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re
from urllib.parse import urlsplit, urlunsplit

SEED = 'dmz-review-pool-20261004-v1'
SPECS = [
    ('hi', ['hi', 'hi-page2'], 'www.hi.is', r'/node/\d+/?', 'general', 'university-research-news'),
    ('stjornarradid', ['stjornarradid'], 'www.stjornarradid.is', r'/efst-a-baugi/frettir/stok-frett/\d{4}-\d{2}-\d{2}[^?]*', 'public-administration', 'government-news'),
    ('reykjavik', ['reykjavik'], 'reykjavik.is', r'/frettir/\d{4}/[^/]+/?', 'public-administration', 'municipal-news'),
    ('landsbankinn', ['landsbankinn'], 'www.landsbankinn.is', r'/frettir/\d{4}/\d{2}/\d{2}/[^/]+/?', 'financial', 'bank-news'),
    ('hagstofa', ['hagstofa'], 'hagstofa.is', r'/utgafur/frettasafn/[^/]+/[^/]+/?', 'financial', 'official-statistics-news'),
    ('personuvernd', ['personuvernd-news'], 'island.is', r'/s/personuvernd/frett/[^/]+/?', 'legal', 'regulator-news'),
]


def sha(data):
    return hashlib.sha256(data).hexdigest()


def canonical(url):
    p = urlsplit(url)
    if p.scheme != 'https' or p.username or p.password or p.port not in (None, 443) or not p.hostname:
        raise ValueError('public HTTPS URL required')
    return urlunsplit(('https', p.hostname.lower(), p.path or '/', p.query, ''))


def select(indexes, previous, limit=40, *, specs=SPECS, seed=SEED, snapshot='30 September snapshots', query_patterns=None):
    if type(limit) is not int or not 1 <= limit <= 100:
        raise ValueError('bounded publisher limit required')
    excluded = set()
    for item in previous:
        for doc in item['documents']:
            for key in ('url', 'resolved_url'):
                if doc.get(key):
                    excluded.add(canonical(doc[key]))
    documents, evidence = [], []
    for publisher, names, host, pattern, stratum, genre in specs:
        candidates = {}
        for name in names:
            raw = indexes[name]
            urls = json.loads(raw)
            if not isinstance(urls, list) or any(not isinstance(u, str) for u in urls):
                raise ValueError('invalid public link index')
            evidence.append(dict(key=name, links_sha256=sha(raw)))
            for url in urls:
                url = canonical(url)
                parsed = urlsplit(url)
                query_pattern = (query_patterns or {}).get(publisher, '')
                if (parsed.hostname != host or not re.fullmatch(query_pattern, parsed.query)
                        or not re.fullmatch(pattern, parsed.path) or url in excluded):
                    continue
                candidates.setdefault(url, set()).add(name)
        ordered = sorted(candidates, key=lambda url: sha((seed + url).encode()))[:limit]
        for url in ordered:
            documents.append(dict(id=publisher+'-'+sha(url.encode())[:12], publisher=publisher,
                                  stratum=stratum, genre=genre, url=url,
                                  discovery_keys=sorted(candidates[url])))
    if not documents:
        raise ValueError('no unused article candidates')
    return dict(schema=1, seed=seed, indexes=evidence, documents=documents,
                purpose='Unassigned model-blind review pool; no training labels or final-test designation',
                selection=f'Unused article-shaped URLs from {snapshot}, SHA256(seed+URL) order, at most {limit} per publisher; before inference.',
                limitations='Availability sample; collection date does not imply new publication; clustering, extraction and source-use review remain required.')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--indexes', type=Path, required=True)
    parser.add_argument('--exclude', type=Path, action='append', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    names = {name for _, names, *_ in SPECS for name in names}
    indexes = {name: (args.indexes/(name+'-links.json')).read_bytes() for name in names}
    result = select(indexes, [json.loads(path.read_text()) for path in args.exclude])
    result['exclusions_sha256'] = [sha(path.read_bytes()) for path in args.exclude]
    with args.output.open('x') as stream:
        stream.write(json.dumps(result, indent=2, ensure_ascii=False)+'\n')
    print(json.dumps(dict(documents=len(result['documents']), publishers=dict(Counter(d['publisher'] for d in result['documents'])))))
