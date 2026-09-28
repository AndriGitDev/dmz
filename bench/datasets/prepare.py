#!/usr/bin/env python3
"""Prepare owner-approved external data locally. Never emit source text to stdout."""
import argparse
import collections
import csv
import hashlib
import json
from pathlib import Path
import zipfile

LABELS = {'Person', 'Location', 'Organization', 'Miscellaneous', 'Date', 'Time', 'Money', 'Percent'}


def digest(data):
    return hashlib.sha256(data).hexdigest()


def sentence(tokens, tags):
    text = ' '.join(tokens)
    # Group exact duplicate text across sources into the same split, independent
    # of annotation differences. This is not a document-disjoint split.
    identity = digest(text.encode())
    bucket = int(identity[:8], 16) % 10
    split = 'test' if bucket == 0 else 'dev' if bucket == 1 else 'train'
    spans, offset, active, repairs = [], 0, None, 0
    for token, tag in zip(tokens, tags):
        start, end = offset, offset + len(token.encode())
        offset = end + 1
        prefix, _, kind = tag.partition('-')
        if tag != 'O' and (prefix not in ('B', 'I') or kind not in LABELS):
            raise ValueError('unknown BIO label')
        if prefix == 'I' and (active is None or active['kind'] != kind):
            # Standard CoNLL recovery, counted in the manifest for review.
            prefix = 'B'
            repairs += 1
        if prefix == 'B' or tag == 'O':
            active = None
        if prefix == 'B':
            active = dict(kind=kind, start=start, end=end)
            spans.append(active)
        elif prefix == 'I':
            active['end'] = end
    return dict(id=identity, split=split, text=text, entities=spans), repairs


def prepare(hms, mim, output):
    repo = Path(__file__).resolve().parents[2]
    output = output.resolve()
    if output == repo or repo in output.parents:
        raise ValueError('external data output must be outside the repository')
    output.mkdir(parents=True, exist_ok=False)
    report = dict(schema=1, inputs={}, hms={}, mim={})
    forms, rows = set(), 0
    with hms.open(encoding='utf-8-sig', newline='') as f:
        reader = csv.DictReader(f)
        if not {'HEITI_NF', 'HEITI_TGF', 'POSTNR'} <= set(reader.fieldnames or []):
            raise ValueError('HMS CSV missing required columns')
        for row in reader:
            rows += 1
            for column in ('HEITI_NF', 'HEITI_TGF'):
                value = row[column].strip()
                if value:
                    forms.add(value)
    # No house numbers, coordinates, property/employee IDs or complete addresses.
    (output / 'address-forms.json').write_text(json.dumps(sorted(forms), ensure_ascii=False) + '\n')
    report['hms'] = dict(rows=rows, unique_address_forms=len(forms), columns_used=['HEITI_NF', 'HEITI_TGF'])
    counts, entities, sources, seen = collections.Counter(), collections.Counter(), collections.Counter(), set()
    duplicates, repairs = 0, 0
    quarantined = []
    with zipfile.ZipFile(mim) as archive:
        if sum(i.file_size for i in archive.infolist()) > 100_000_000:
            raise ValueError('unexpectedly large MIM archive')
        # Read members directly; never extract archive paths to disk.
        files = [i for i in archive.infolist() if i.filename.endswith('.txt')]
        if not files:
            raise ValueError('no CoNLL text members')
        handles = {split: (output / (split + '.jsonl')).open('w', encoding='utf-8') for split in ('train', 'dev', 'test')}
        try:
            for member in sorted(files, key=lambda i: i.filename):
                tokens, tags, malformed = [], [], []
                for line_number, line in enumerate(archive.read(member).decode('utf-8-sig').splitlines() + [''], 1):
                    if line.strip():
                        fields = line.split()
                        if len(fields) != 2:
                            malformed.append(line_number)
                            continue
                        tokens.append(fields[0]); tags.append(fields[1])
                        continue
                    if malformed:
                        quarantined.append(dict(source=member.filename, malformed_lines=malformed))
                        tokens, tags, malformed = [], [], []
                        continue
                    if not tokens:
                        continue
                    item, repaired = sentence(tokens, tags)
                    repairs += repaired
                    # Deduplicate identical text+annotation; retain conflicting
                    # annotations in the same split for later adjudication.
                    key = (item['id'], tuple(tags))
                    if key in seen:
                        duplicates += 1
                    else:
                        seen.add(key)
                        item['source'] = member.filename
                        handles[item['split']].write(json.dumps(item, ensure_ascii=False) + '\n')
                        counts[item['split']] += 1
                        sources[member.filename] += 1
                        entities.update(e['kind'] for e in item['entities'])
                    tokens, tags = [], []
        finally:
            for handle in handles.values():
                handle.close()
    report['mim'] = dict(sentences=dict(counts), entities=dict(entities), sources=dict(sources), duplicate_sentences_removed=duplicates, quarantined_sentences=quarantined, orphan_inside_tags_recovered=repairs, reconstruction='single spaces between tokens; UTF-8 byte offsets', split='SHA256 reconstructed text, first 32 bits modulo 10: 0 test, 1 dev, remainder train; exact-text disjoint, not document-disjoint')
    report['inputs'] = {name: dict(sha256=digest(path.read_bytes()), bytes=path.stat().st_size) for name, path in [('hms', hms), ('mim', mim)]}
    (output / 'manifest.json').write_text(json.dumps(report, indent=2) + '\n')
    print(json.dumps(report, indent=2))


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--hms', required=True, type=Path)
    parser.add_argument('--mim', required=True, type=Path)
    parser.add_argument('--output', required=True, type=Path, help='new directory outside checkout')
    args = parser.parse_args()
    prepare(args.hms, args.mim, args.output)
