#!/usr/bin/env python3
"""Model-blind, bounded private web collection; never labels or trains a model."""
import argparse
from collections import Counter
from datetime import datetime, timezone
import json
from pathlib import Path
import re
import time
from urllib.parse import urljoin, urlsplit, urlunsplit
from urllib.robotparser import RobotFileParser
import requests
from collect import sha, shingles
from collect_general import Tree, Node
from review import external

AGENT = 'DMZ-evaluation-research'
LIMIT = 2_000_000


def canonical(url):
    p = urlsplit(url)
    if p.scheme != 'https' or not p.hostname or p.username or p.password or p.port not in (None, 443):
        raise ValueError('HTTPS public source required')
    return urlunsplit(('https', p.hostname.lower(), p.path or '/', p.query, ''))


class Fetcher:
    def __init__(self, output):
        self.output = output
        self.session = requests.Session()
        self.session.headers['User-Agent'] = AGENT + '/0.2 (private Icelandic evaluation corpus)'
        self.robots = {}
        self.last = {}
        self.evidence = []

    def request(self, url, delay=1.0):
        host = urlsplit(url).netloc
        time.sleep(max(0, delay - (time.monotonic() - self.last.get(host, 0))))
        self.last[host] = time.monotonic()
        with self.session.get(url, timeout=(15, 35), stream=True, allow_redirects=False) as r:
            data = bytearray()
            for chunk in r.iter_content(65536):
                data.extend(chunk)
                if len(data) > LIMIT:
                    raise ValueError('response exceeds size bound')
            return r.status_code, {key.lower(): value for key, value in r.headers.items()}, bytes(data)

    def rules(self, url):
        p = urlsplit(url); origin = p.scheme + '://' + p.netloc
        if origin not in self.robots:
            status, headers, body = self.request(origin + '/robots.txt')
            # Fail closed on robots errors/redirects. No access-control workaround.
            parser = RobotFileParser()
            if status == 200:
                if b'<html' in body.lower():
                    raise ValueError('robots response is HTML')
                parser.parse(body.decode('utf-8', errors='replace').splitlines())
            elif status == 404:
                parser.parse([])
            else:
                raise ValueError('robots unavailable: ' + str(status))
            name = 'robots-' + sha(origin.encode())[:12] + '.txt'
            (self.output / name).write_bytes(body)
            self.evidence.append(dict(origin=origin, status=status, file=name, sha256=sha(body)))
            self.robots[origin] = parser
        parser = self.robots[origin]
        if not parser.can_fetch(AGENT, url):
            raise ValueError('robots disallowed')
        delay = max(1.0, parser.crawl_delay(AGENT) or 0)
        if delay > 30:
            raise ValueError('crawl delay exceeds collection budget')
        return delay

    def get(self, url):
        url = canonical(url)
        initial = urlsplit(url).hostname.removeprefix('www.')
        for _ in range(5):
            delay = self.rules(url)
            status, headers, body = self.request(url, delay)
            if status in (301, 302, 303, 307, 308):
                url = canonical(urljoin(url, headers.get('location', '')))
                if urlsplit(url).hostname.removeprefix('www.') != initial:
                    raise ValueError('cross-publisher redirect; needs explicit source entry')
                continue
            if status != 200:
                raise ValueError('HTTP ' + str(status))
            if 'text/html' not in headers.get('content-type', ''):
                raise ValueError('not HTML')
            return url, headers, body
        raise ValueError('redirect limit')


def extract(body):
    tree = Tree(); tree.feed(body.decode('utf-8-sig'))
    # Eliminate navigation/related-content subtrees before extracting prose.
    for node in tree.nodes:
        attrs = (node.attrs.get('class', '') + ' ' + node.attrs.get('id', '')).lower()
        if node.tag in {'nav', 'footer', 'script', 'style', 'noscript', 'iframe', 'form', 'button', 'aside'} or any(
            term in attrs for term in ['related-content', 'related-articles', 'cookie-banner', 'feedback-form', 'social-share']
        ):
            node.children = []
    title = next((n.attrs.get('content', '') for n in tree.nodes if n.tag == 'meta' and n.attrs.get('property') == 'og:title'), '')
    if any('PageHero_' in n.attrs.get('class', '') for n in tree.nodes) and not any('ArticleMeta_' in n.attrs.get('class', '') for n in tree.nodes):
        raise ValueError('municipal category page, not an article')
    selectors = [
        ('field--name-body', lambda n: 'field--name-body' in n.attrs.get('class', '').split()),
        ('rs-content', lambda n: 'rs-content' in n.attrs.get('class', '').split()),
        ('section.case', lambda n: n.tag == 'section' and 'case' in n.attrs.get('class', '').split()),
        ('article', lambda n: n.tag == 'article'),
        ('div.article', lambda n: n.tag == 'div' and 'article' in n.attrs.get('class', '').split()),
        ('main', lambda n: n.tag == 'main'),
        ('role-main', lambda n: n.attrs.get('role') == 'main'),
    ]
    for label, predicate in selectors:
        candidates = [n for n in tree.nodes if predicate(n)]
        if not candidates:
            continue
        raw = max((n.text() for n in candidates), key=len)
        lines = [re.sub(r'\s+', ' ', x).strip() for x in raw.splitlines() if x.strip()]
        text = '\n'.join(lines).replace('\u00ad', '')
        title = title.replace('\u00ad', '')
        if title and title not in text:
            text = title + '\n' + text
        if len(text.split()) >= 60:
            if len(re.findall('[ðþæöáéíóúýÐÞÆÖÁÉÍÓÚÝ]', text)) < 8:
                raise ValueError('Icelandic heuristic failed; review language separately')
            dates = [n.attrs.get('content') or n.attrs.get('datetime') for n in tree.nodes
                     if n.tag == 'time' or n.attrs.get('property') in {'article:published_time', 'article:modified_time'}]
            return text, label, [x for x in dates if x]
    raise ValueError('no bounded Icelandic prose extraction')


def segments(text, maximum=1800):
    """Contiguous original substrings; UTF-8 offsets computed after selection."""
    start = 0
    while start < len(text):
        end = min(len(text), start + maximum)
        if end < len(text):
            boundary = text.rfind('\n', start + maximum // 2, end)
            if boundary < 0:
                boundary = text.rfind(' ', start + maximum // 2, end)
            if boundary >= 0:
                end = boundary
        left = start
        while left < end and text[left].isspace():
            left += 1
        right = end
        while right > left and text[right - 1].isspace():
            right -= 1
        if right - left >= 200:
            yield left, right, text[left:right]
        start = end
        while start < len(text) and text[start].isspace():
            start += 1


def overlap_inventory(paths):
    known = set(); evidence = []
    for path in paths:
        path = Path(path).resolve(); data = path.read_bytes(); records = 0
        for line in data.decode().splitlines():
            row = json.loads(line)
            text = row.get('original_text') or row.get('text')
            if isinstance(text, str):
                known.update(shingles(text)); records += 1
        evidence.append(dict(path=str(path), sha256=sha(data), text_records=records))
    if not known:
        raise ValueError('empty overlap reference inventory')
    return known, evidence


def main():
    p = argparse.ArgumentParser(__doc__)
    p.add_argument('--sources', type=Path, required=True)
    p.add_argument('--references', type=Path, required=True, help='JSON array of prior text JSONL paths')
    p.add_argument('--output', type=Path, required=True)
    p.add_argument('--cached', type=Path, help='Re-extract a checksummed previous collection without network requests')
    a = p.parse_args(); output = external(a.output); output.mkdir(parents=True, exist_ok=False)
    collector_sha256 = sha(Path(__file__).read_bytes())
    sources = json.loads(a.sources.read_text()); refs = json.loads(a.references.read_text())
    (output / 'sources.json').write_bytes(a.sources.read_bytes())
    known, evidence = overlap_inventory(refs)
    cache = external(a.cached) if a.cached else None
    cached_report = json.loads((cache / 'manifest.json').read_text()) if cache else None
    if cache and cached_report['sources_sha256'] != sha(a.sources.read_bytes()):
        raise ValueError('cached source list differs')
    cached_docs = {d['id']: d for d in cached_report['documents']} if cache else {}
    fetcher = Fetcher(output); docs = []; rows = []; accepted_shingles = []; exact = set()
    for source in sources['documents']:
        entry = dict(source)
        if not re.fullmatch('[a-z0-9-]+', source['id']):
            raise ValueError('invalid source id')
        try:
            if cache:
                old = cached_docs[source['id']]
                file = cache / (source['id'] + '.html')
                if not file.exists():
                    raise ValueError('prior fetch unavailable: ' + old.get('error', 'unknown'))
                body = file.read_bytes()
                if not old.get('html_sha256') or sha(body) != old['html_sha256']:
                    raise ValueError('cached HTML checksum mismatch')
                url = old.get('resolved_url', source['url'])
            else:
                url, headers, body = fetcher.get(source['url'])
            (output / (source['id'] + '.html')).write_bytes(body)
            entry.update(resolved_url=url, html_sha256=sha(body))
            text, selector, dates = extract(body)
            (output / (source['id'] + '.txt')).write_text(text)
            grams = shingles(text); normalized_sha = sha(' '.join(text.casefold().split()).encode())
            similarities = [(doc, len(grams & other) / max(1, min(len(grams), len(other)))) for doc, other in accepted_shingles]
            near = [doc for doc, score in similarities if score >= 0.5]
            entry.update(resolved_url=url, html_sha256=sha(body), text_sha256=sha(text.encode()),
                         text_characters=len(text), words=len(text.split()), extraction_selector=selector,
                         publication_date_evidence=dates, exact_duplicate=normalized_sha in exact,
                         near_duplicate_documents=near, overlapping_reference_12grams=len(grams & known),
                         extraction_reviewed=False, source_use_review='pending; private evaluation candidate only')
            if normalized_sha in exact or near:
                entry['excluded'] = 'duplicate-or-near-duplicate-document'
            else:
                exact.add(normalized_sha); accepted_shingles.append((source['id'], grams))
                parts = list(segments(text))
                selected = sorted(parts, key=lambda part: sha((sources['seed'] + source['id'] + str(part[0])).encode()))[:3]
                for page, (start, end, excerpt) in enumerate(sorted(selected), 1):
                    rows.append(dict(document_id=source['id'], page=page, source_url=url,
                        stratum=source['stratum'], genre=source['genre'], text=excerpt,
                        text_sha256=sha(excerpt.encode()), document_byte_start=len(text[:start].encode()),
                        document_byte_end=len(text[:end].encode()), entities=None, reviewed=False,
                        overlapping_mim_12grams=len(shingles(excerpt) & known),
                        overlap_reference_scope='all inventoried prior text, not MIM alone', eligible_for_final=False))
        except (requests.RequestException, ValueError, UnicodeError) as error:
            entry['error'] = str(error)[:180]
        docs.append(entry)
        print(json.dumps(dict(processed=len(docs), total=len(sources['documents']), collected=sum('text_sha256' in d for d in docs), excerpts=len(rows))), flush=True)
    with (output / 'review.jsonl').open('x') as f:
        for row in rows:
            f.write(json.dumps(row, ensure_ascii=False) + '\n')
    report = dict(schema=1, collector_sha256=collector_sha256, collected_at=datetime.now(timezone.utc).isoformat(),
        status='frozen unlabelled web evaluation candidates; not human-labelled gold',
        sources_sha256=sha(a.sources.read_bytes()), references=evidence, robots=cached_report['robots'] if cache else fetcher.evidence, documents=docs,
        extraction_normalization='HTML entities decoded; block whitespace collapsed; discretionary soft hyphens removed; other Unicode retained',
        cached_manifest_sha256=sha((cache / 'manifest.json').read_bytes()) if cache else None,
        excerpts=len(rows), excerpt_words=sum(len(r['text'].split()) for r in rows),
        strata=dict(Counter(r['stratum'] for r in rows)),
        flagged_overlap_excerpts=sum(r['overlapping_mim_12grams'] > 0 for r in rows),
        model_evaluated=False, training_use=False, human_review_complete=False,
        independence_limits='Checked normalized exact 12-grams and document near duplicates; semantic copying and base-model pretraining exposure not ruled out.',
        review_sha256=sha((output / 'review.jsonl').read_bytes()))
    (output / 'manifest.json').write_text(json.dumps(report, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({k: report[k] for k in ['status','excerpts','excerpt_words','strata','flagged_overlap_excerpts']}))


if __name__ == '__main__':
    main()
