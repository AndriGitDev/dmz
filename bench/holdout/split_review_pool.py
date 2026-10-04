#!/usr/bin/env python3
"""Freeze document-cluster review splits before labels or model predictions.

Raw documents/excerpts and split membership remain private. Review splits are
not training data or an independently approved final test set.
"""
import argparse
from collections import Counter, defaultdict
import json
from pathlib import Path

from collect import sha, shingles
from review import external

SEED = 'dmz-document-review-splits-20261004-v1'


def clusters(documents, threshold=.2):
    if not 0 < threshold <= 1:
        raise ValueError('invalid cluster threshold')
    identifiers = sorted(documents)
    parent = {identifier: identifier for identifier in identifiers}

    def find(identifier):
        while parent[identifier] != identifier:
            parent[identifier] = parent[parent[identifier]]
            identifier = parent[identifier]
        return identifier

    for i, left in enumerate(identifiers):
        for right in identifiers[i+1:]:
            a, b = documents[left], documents[right]
            # Empty shingle sets cannot establish lexical independence.
            if a and b and len(a & b) / min(len(a), len(b)) >= threshold:
                first, second = sorted([find(left), find(right)])
                parent[second] = first
    groups = defaultdict(list)
    for identifier in identifiers:
        groups[find(identifier)].append(identifier)
    result = {}
    for members in groups.values():
        cluster = sha('\n'.join(members).encode())
        bucket = int(sha((SEED + cluster).encode())[:8], 16) % 10
        split = 'train-review' if bucket < 6 else 'dev-review' if bucket < 8 else 'reserved-review'
        for identifier in members:
            result[identifier] = dict(cluster_sha256=cluster, split=split)
    return result


def freeze(collection, output):
    collection, output = external(collection), external(output)
    manifest_bytes = (collection/'manifest.json').read_bytes()
    manifest = json.loads(manifest_bytes)
    if manifest.get('model_evaluated') is not False or manifest.get('training_use') is not False:
        raise ValueError('requires model-blind collection')
    review_bytes = (collection/'review.jsonl').read_bytes()
    if sha(review_bytes) != manifest['review_sha256']:
        raise ValueError('collection excerpts changed')
    documents, grams, excluded = {}, {}, Counter()
    for doc in manifest['documents']:
        identifier = doc['id']
        if Path(identifier).name != identifier or identifier in documents:
            raise ValueError('invalid or duplicate document id')
        if 'text_sha256' not in doc or doc.get('excluded'):
            excluded['unavailable_or_collection_duplicate'] += 1
            continue
        text = (collection/(identifier+'.txt')).read_bytes()
        if sha(text) != doc['text_sha256']:
            raise ValueError('document changed')
        if doc['overlapping_reference_12grams']:
            excluded['overlaps_prior_text'] += 1
            continue
        document_grams = shingles(text.decode())
        if not document_grams:
            excluded['too_short_for_overlap_screen'] += 1
            continue
        documents[identifier] = (doc, text)
        grams[identifier] = document_grams
    if not documents:
        raise ValueError('no eligible review documents')
    assignment = clusters(grams)
    splits = defaultdict(list)
    seen = set()
    for line in review_bytes.decode().splitlines():
        row = json.loads(line)
        if row.get('entities') is not None or row.get('reviewed') is not False:
            raise ValueError('requires unlabelled, unreviewed excerpts')
        identifier = row['document_id']
        if identifier not in documents:
            continue
        key = (identifier, row['page'])
        if key in seen:
            raise ValueError('duplicate document excerpt')
        seen.add(key)
        text = documents[identifier][1]
        excerpt = row['text'].encode()
        a, b = row['document_byte_start'], row['document_byte_end']
        if type(a) is not int or type(b) is not int or not 0 <= a < b <= len(text):
            raise ValueError('invalid source offsets')
        if text[a:b] != excerpt or sha(excerpt) != row['text_sha256']:
            raise ValueError('excerpt identity mismatch')
        row = row | assignment[identifier] | dict(eligible_for_final=False)
        splits[row['split']].append(row)
    if {key[0] for key in seen} != set(documents):
        raise ValueError('eligible document missing excerpts')
    output.mkdir(parents=True, exist_ok=False)
    report = dict(schema=1, seed=SEED, status='Frozen unlabelled review allocation; human extraction and entity review required',
                  collection_manifest_sha256=sha(manifest_bytes), review_sha256=sha(review_bytes),
                  cluster_method='Transitive components at >=20% normalized exact 12-gram containment; not semantic topic detection',
                  excluded_documents=dict(excluded), source_use_review='pending; private storage only',
                  model_evaluated=False, training_use=False, independent_final_evaluation=False,
                  splits={})
    for split, rows in sorted(splits.items()):
        data = ''.join(json.dumps(row, ensure_ascii=False)+'\n' for row in rows).encode()
        (output/(split+'.jsonl')).write_bytes(data)
        report['splits'][split] = dict(documents=len({r['document_id'] for r in rows}), excerpts=len(rows),
            words=sum(len(r['text'].split()) for r in rows), clusters=len({r['cluster_sha256'] for r in rows}),
            strata=dict(Counter(r['stratum'] for r in rows)), sha256=sha(data))
    (output/'document-splits.json').write_text(json.dumps(assignment, indent=2)+'\n')
    report['document_splits_sha256'] = sha((output/'document-splits.json').read_bytes())
    (output/'manifest.json').write_text(json.dumps(report, indent=2)+'\n')
    return report


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--collection', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(freeze(args.collection, args.output), indent=2))
