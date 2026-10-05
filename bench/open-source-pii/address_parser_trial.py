"""Small address parser calibration; not a postal validator or blind result."""
import argparse
import hashlib
import importlib.metadata
import json
import time
from pathlib import Path
from evaluate import ROOT, digest, load_corpus, offline, private_output
from compare import add_nonoverlapping
from quality import score

def main():
    p = argparse.ArgumentParser()
    p.add_argument('--reference', type=Path, required=True)
    p.add_argument('--gliner2', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    offline()
    import usaddress
    versions = {name: importlib.metadata.version(name) for name in ['usaddress', 'python-crfsuite', 'probableparsing']}
    if versions != {'usaddress': '0.5.16', 'python-crfsuite': '0.9.12', 'probableparsing': '0.0.1'}:
        raise ValueError('wrong parser version')
    rows, manifest = load_corpus(ROOT / 'bench/corpus/open-source-pii-v1')
    reference = json.loads(a.reference.read_text())
    neural = json.loads(a.gliner2.read_text())
    for cache in [reference, neural]:
        if cache['status'] != 'complete' or cache['corpus'] != manifest:
            raise ValueError('wrong cache')
    base = {r['id']: r for r in reference['records']}
    cached = {r['id']: r for r in neural['records']}
    if set(base) != set(cached) or set(base) != {r['id'] for r in rows}:
        raise ValueError('incomplete caches')

    from address_parser import parsed, qualifying, standalone

    started = time.monotonic()
    records = {'parser-filtered-gliner2': [], 'parser-standalone': []}
    failures = {}
    for row in rows:
        text = row['text']; sha = hashlib.sha256(text.encode()).hexdigest()
        if base[row['id']]['text_sha256'] != sha or cached[row['id']]['text_sha256'] != sha:
            raise ValueError('text mismatch')
        for variant in records:
            failure = None
            try:
                if variant == 'parser-standalone':
                    scored = standalone(text)
                else:
                    scored = []
                    for item in cached[row['id']]['scored']:
                        s, e, kind = item['span']
                        if kind == 'address' and qualifying(parsed(text.encode()[s:e].decode())):
                            scored.append(item)
            except Exception as error:
                failure = type(error).__name__; scored = []
                failures[variant + '/' + failure] = failures.get(variant + '/' + failure, 0) + 1
            records[variant].append({'id': row['id'], 'text_sha256': sha, 'gold': row['gold'], 'stratum': row['stratum'],
                                     'predicted': [s['span'] for s in scored], 'scored': scored,
                                     'abstained': failure is not None, 'candidate_failure': failure})
    runs = {}
    for variant, values in records.items():
        for mode in ['basic', 'trained']:
            combined = []
            for record in values:
                original = base[record['id']][mode]
                prediction = [] if original['abstained'] else add_nonoverlapping(original['predicted'], record['scored'])
                combined.append(record | {'predicted': prediction, 'abstained': original['abstained']})
            runs[variant + '-plus-' + mode] = score(combined, manifest['scope'])
    result = {'schema': 1, 'status': 'complete', 'independent_validation': False, 'scope': 'Calibration only',
              'versions': versions, 'model_sha256': digest(usaddress.MODEL_PATH), 'model_bytes': Path(usaddress.MODEL_PATH).stat().st_size,
              'parser_source_sha256': digest(usaddress.__file__), 'adapter_sha256': digest(Path(__file__).with_name('address_parser.py')), 'reference_sha256': digest(a.reference),
              'gliner2_sha256': digest(a.gliner2), 'measurement_sha256': digest(Path(__file__)),
              'corpus': manifest, 'seconds': time.monotonic() - started, 'candidate_failures': failures,
              'records': records, 'runs': runs}
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    for variant, run in runs.items():
        print(variant, {s: {k: r[k] for k in ['exact_excerpts', 'excerpts', 'tp', 'fp', 'fn']} for s, r in run['strata'].items()})


if __name__ == '__main__':
    main()
