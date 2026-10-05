"""Small address parser calibration; not a postal validator or blind result."""
import argparse
import hashlib
import importlib.metadata
import json
import re
import time
from pathlib import Path
from evaluate import ROOT, digest, load_corpus, offline, private_output
from compare import add_nonoverlapping
from quality import score

STREET = {'AddressNumber', 'AddressNumberSuffix', 'StreetNamePreModifier', 'StreetNamePreDirectional',
          'StreetNamePreType', 'StreetName', 'StreetNamePostType', 'StreetNamePostDirectional',
          'OccupancyType', 'OccupancyIdentifier', 'SubaddressType', 'SubaddressIdentifier'}
REQUIRED = {'AddressNumber', 'StreetName', 'StreetNamePostType'}
STATES = set('AL AK AZ AR CA CO CT DE FL GA HI ID IL IN IA KS KY LA ME MD MA MI MN MS MO MT NE NV NH NJ NM NY NC ND OH OK OR PA RI SC SD TN TX UT VT VA WA WV WI WY DC AS GU MP PR VI'.split())
ABBREVIATIONS = set('st rd ave blvd ln ct dr pl pkwy hwy sq ter trl cir'.split())


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

    def parsed(text):
        if len(text.encode()) > 4096 or re.search(r'&(?:amp;|#38;)', text):
            raise ValueError('unsupported input')
        tokens = usaddress.tokenize(text)
        if not tokens:
            return []
        labels = usaddress.TAGGER.tag(usaddress.tokens2features(tokens))
        result, cursor = [], 0
        for i, (token, label) in enumerate(zip(tokens, labels)):
            start = text.find(token, cursor)
            if start < 0:
                raise ValueError('token alignment failure')
            end = start + len(token)
            result.append({'start': start, 'end': end, 'token': token, 'label': label,
                           'probability': usaddress.TAGGER.marginal(label, i)})
            cursor = end
        return result

    def qualifying(tokens):
        if not REQUIRED.issubset({t['label'] for t in tokens}):
            return False
        required = [t for t in tokens if t['label'] in REQUIRED]
        if min(t['probability'] for t in required) < .90:
            return False
        numbers = [t['token'].strip('(),;') for t in required if t['label'] == 'AddressNumber']
        suffixes = [t['token'].strip('().,;\n').lower() for t in required if t['label'] == 'StreetNamePostType']
        return all(re.fullmatch(r'[0-9]{1,6}[A-Za-z]?(?:-[0-9]{1,6}[A-Za-z]?)?', n) for n in numbers) and all(s in usaddress.STREET_NAMES for s in suffixes)

    def standalone(text):
        tokens = parsed(text)
        result = []
        for i, token in enumerate(tokens):
            if token['label'] != 'AddressNumber':
                continue
            end = i + 1
            while end < len(tokens) and tokens[end]['label'] in STREET and tokens[end]['label'] != 'AddressNumber':
                end += 1
            part = tokens[i:end]
            if not qualifying(part):
                continue
            tail = end
            while tail < len(tokens) and tokens[tail]['label'] in {'PlaceName', 'StateName', 'ZipCode'}:
                tail += 1
            extra = tokens[end:tail]
            state = [t['token'].strip('.,;()').upper() for t in extra if t['label'] == 'StateName']
            postal = [t['token'].strip('.,;()') for t in extra if t['label'] == 'ZipCode']
            if len(state) == 1 and state[0] in STATES and len(postal) == 1 and re.fullmatch(r'[0-9]{5}(?:-[0-9]{4})?', postal[0]):
                end = tail
                part = tokens[i:end]
            s, e = token['start'], part[-1]['end']
            while e > s and text[e-1] in ',;)\n\r':
                e -= 1
            last = text[part[-1]['start']:e]
            if last.endswith('.') and not (part[-1]['label'] == 'StreetNamePostType' and last[:-1].lower() in ABBREVIATIONS):
                e -= 1
            result.append({'span': [len(text[:s].encode()), len(text[:e].encode()), 'address'],
                           'score': min(t['probability'] for t in part)})
        return result

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
              'parser_source_sha256': digest(usaddress.__file__), 'reference_sha256': digest(a.reference),
              'gliner2_sha256': digest(a.gliner2), 'measurement_sha256': digest(Path(__file__)),
              'corpus': manifest, 'seconds': time.monotonic() - started, 'candidate_failures': failures,
              'records': records, 'runs': runs}
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    for variant, run in runs.items():
        print(variant, {s: {k: r[k] for k in ['exact_excerpts', 'excerpts', 'tp', 'fp', 'fn']} for s, r in run['strata'].items()})


if __name__ == '__main__':
    main()
