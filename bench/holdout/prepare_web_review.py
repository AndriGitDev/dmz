#!/usr/bin/env python3
"""Freeze combined web candidates and create a blind human-review packet."""
import argparse
from collections import Counter
import csv
import json
from pathlib import Path
from collect import sha, shingles
from review import external, prepare


def main():
    p = argparse.ArgumentParser(__doc__)
    p.add_argument('--collection', type=Path, action='append', required=True)
    p.add_argument('--output', type=Path, required=True)
    args = p.parse_args(); output = external(args.output)
    output.mkdir(parents=True, exist_ok=False)
    rows = []; sources = []; documents = {}; excluded = []; accepted = []
    for directory in args.collection:
        directory = external(directory)
        manifest = json.loads((directory / 'manifest.json').read_text())
        data = (directory / 'review.jsonl').read_bytes()
        if sha(data) != manifest['review_sha256'] or manifest['model_evaluated'] or manifest['training_use']:
            raise ValueError('source changed or model-exposed')
        sources.append(dict(path=str(directory), manifest_sha256=sha((directory / 'manifest.json').read_bytes())))
        eligible = set()
        for doc in manifest['documents']:
            if 'text_sha256' not in doc or doc.get('excluded'):
                continue
            if doc['id'] in documents:
                raise ValueError('duplicate document ID across collections')
            text = (directory / (doc['id'] + '.txt')).read_bytes()
            if sha(text) != doc['text_sha256']:
                raise ValueError('document text changed')
            grams = shingles(text.decode())
            near = [name for name, other in accepted if len(grams & other) / max(1, min(len(grams), len(other))) >= 0.5]
            if near or doc['overlapping_reference_12grams']:
                excluded.append(dict(document_id=doc['id'], near_duplicate_documents=near,
                                     prior_overlap_12grams=doc['overlapping_reference_12grams']))
                continue
            accepted.append((doc['id'], grams)); documents[doc['id']] = doc; eligible.add(doc['id'])
        for line in data.decode().splitlines():
            row = json.loads(line)
            if row['document_id'] not in eligible:
                continue
            doc = documents[row['document_id']]
            text = (directory / (doc['id'] + '.txt')).read_bytes()
            excerpt = row['text'].encode()
            if sha(excerpt) != row['text_sha256'] or text[row['document_byte_start']:row['document_byte_end']] != excerpt:
                raise ValueError('excerpt offset or checksum mismatch')
            if row['entities'] is not None or row['reviewed']:
                raise ValueError('blind preparation requires null labels')
            rows.append(row)
    review = output / 'review.jsonl'
    review.write_text(''.join(json.dumps(r, ensure_ascii=False) + '\n' for r in rows))
    prepare([review], output / 'annotation-packet')
    with (output / 'catalog.csv').open('w') as f:
        writer = csv.writer(f)
        writer.writerow(['text_file', 'document_id', 'page', 'stratum', 'genre', 'source_url', 'sha256'])
        for i, row in enumerate(rows, 1):
            writer.writerow([f'page-{i:04d}.txt', row['document_id'], row['page'], row['stratum'], row['genre'], row['source_url'], row['text_sha256']])
    manifest = dict(schema=1, status='unlabelled held-out web candidates; human gold review pending', sources=sources,
                    documents=len({r['document_id'] for r in rows}), excerpts=len(rows),
                    words=sum(len(r['text'].split()) for r in rows),
                    publishers=dict(Counter(documents[r['document_id']]['publisher'] for r in rows)),
                    strata=dict(Counter(r['stratum'] for r in rows)),
                    excluded_documents=excluded, review_sha256=sha(review.read_bytes()),
                    human_review_complete=False, model_evaluated=False, training_use=False)
    (output / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    (output / 'README.md').write_text('''# Fresh Icelandic web evaluation candidates

This is a private, unlabelled corpus, not a scored test set or human gold standard.
No detector predictions were used for sampling, labels or selection. Do not train
or tune on it. Prior annotation pilots remain separate and unchanged.

Start with catalog.csv (source URLs, strata, text hashes) and annotation-packet/.
Follow the packet instructions for exact UTF-8 byte-offset Person spans. Null
labels mean unreviewed, not negative. Review all text, including name-free items.
Check extraction against saved source HTML before labelling; the underlying
collection paths and manifest hashes are in manifest.json. A spot-check is not a
substitute for this full extraction review. Do not edit frozen text in place.

Use two independent reviewers, adjudicate disagreements, then freeze label hashes
before evaluating the frozen detector once. Record clipped mentions at excerpt
boundaries and ambiguous Person-vs-organization/eponym cases for adjudication.
The legacy overlapping_mim_12grams field covers ALL inventoried prior text in
this packet, including synthetic training/dev and earlier review collections.

This is an availability sample of institutional web prose, not a representative
sample of all Icelandic or private customer contracts/case files. Some legal text
is anonymized. Shared wording below 12 tokens, semantic copying and base-model
pretraining exposure are not ruled out. Collection does not establish rights to
redistribute the text. Keep raw pages, excerpts and annotations outside Git and
public releases. Source-use review remains pending for any broader distribution.
''')
    print(json.dumps(manifest, indent=2))


if __name__ == '__main__': main()
