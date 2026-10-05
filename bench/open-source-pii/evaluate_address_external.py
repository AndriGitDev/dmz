"""Run the fixed small-parser grammar on the pinned public English diagnostic."""
import argparse
import hashlib
import importlib.metadata
import json
import time
from pathlib import Path
from evaluate import digest, private_output, offline
from evaluate_external import TEST_SHA
from quality import score


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    partial = private_output(str(out) + '.partial.jsonl')
    manifest = json.loads((a.corpus / 'manifest.json').read_text())
    if manifest['source']['revision'] != 'e06eb1499ca8d54470f085021cd8e54f9efac7fd' or digest(a.corpus / 'test.jsonl') != TEST_SHA:
        raise ValueError('wrong public projection')
    offline()
    import usaddress
    from address_parser import standalone
    versions = {name: importlib.metadata.version(name) for name in ['usaddress', 'python-crfsuite', 'probableparsing']}
    if versions != {'usaddress': '0.5.16', 'python-crfsuite': '0.9.12', 'probableparsing': '0.0.1'}:
        raise ValueError('wrong parser version')
    paths = [Path(__file__), Path(__file__).with_name('address_parser.py'), Path(usaddress.__file__), Path(usaddress.MODEL_PATH)]
    hashes = {str(p): digest(p) for p in paths}
    records, failures = [], {}
    began = time.monotonic()
    with partial.open('x') as log:
        for line in (a.corpus / 'test.jsonl').read_text().splitlines():
            row = json.loads(line)
            if hashlib.sha256(row['text'].encode()).hexdigest() != row['text_sha256']:
                raise ValueError('changed input')
            start = time.monotonic()
            failure = None
            try:
                scored = standalone(row['text'])
            except Exception as error:
                failure = type(error).__name__; scored = []
                failures[failure] = failures.get(failure, 0) + 1
            record = {k: row[k] for k in ['id', 'stratum', 'text_sha256', 'gold']}
            record.update(predicted=[s['span'] for s in scored], scored=scored, abstained=failure is not None,
                          candidate_failure=failure, seconds=time.monotonic() - start)
            records.append(record); log.write(json.dumps(record) + '\n')
    for path in paths:
        if digest(path) != hashes[str(path)]:
            raise ValueError('measurement changed')
    result = {'schema': 1, 'status': 'complete', 'candidate': 'usaddress-standalone', 'versions': versions,
              'corpus': manifest, 'measurement_sha256': hashes, 'partial_sha256': digest(partial),
              'candidate_failures': failures, 'seconds': time.monotonic() - began,
              'evaluated_excerpts': len(records), 'quality': score(records, manifest['scope']),
              'independent_validation': False, 'scope': 'Fixed parser grammar on public synthetic English; source annotation errors and training overlap unknown',
              'network': 'socket connections denied'}
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    print(json.dumps({'excerpts': len(records), 'seconds': result['seconds'], 'candidate_failures': failures,
                      'address_quality': result['quality']['per_type']['address']}))


if __name__ == '__main__':
    main()
