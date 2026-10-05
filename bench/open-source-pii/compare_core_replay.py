"""Isolate portable-phone gains and the previously selected address supplement."""
import argparse
import hashlib
import json
from pathlib import Path
from evaluate import load_corpus, digest, private_output
from compare import add_nonoverlapping
from quality import score


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--before', type=Path, required=True)
    p.add_argument('--after', type=Path, required=True)
    p.add_argument('--address', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    rows, manifest = load_corpus(a.corpus)
    sources = {name: json.loads(path.read_text()) for name, path in [('before', a.before), ('after', a.after), ('address', a.address)]}
    for data in sources.values():
        if data['status'] != 'complete' or data['corpus'] != manifest:
            raise ValueError('incompatible cache')
    if sources['after']['reference_sha256'] != digest(a.before):
        raise ValueError('wrong replay source')
    candidate = sources['address']
    if candidate['candidate'] != 'gliner2' or candidate['config']['threshold'] != .5 or list(candidate['config']['mapping']) != ['person', 'address', 'phone_number', 'email', 'iban', 'payment_card', 'secret']:
        raise ValueError('wrong preselected candidate')
    indexed = {}
    for name, source in sources.items():
        indexed[name] = {r['id']: r for r in source['records']}
        if len(indexed[name]) != len(rows) or set(indexed[name]) != {r['id'] for r in rows}:
            raise ValueError('incomplete or duplicate records')
    for row in rows:
        for source in indexed.values():
            if source[row['id']]['text_sha256'] != hashlib.sha256(row['text'].encode()).hexdigest():
                raise ValueError('changed text')
        if indexed['before'][row['id']]['names'] != indexed['after'][row['id']]['names']:
            raise ValueError('contextual names changed')
        if not all(indexed[n][row['id']][m]['exact_restoration'] for n in ['before', 'after'] for m in ['basic', 'trained']):
            raise ValueError('restoration failure')
    runs = {}
    for core in ['before', 'after']:
        for mode in ['basic', 'trained']:
            for supplement in [False, True]:
                records = []
                for row in rows:
                    base = indexed[core][row['id']][mode]
                    extra = [r for r in indexed['address'][row['id']]['scored'] if r['span'][2] == 'address' and r['score'] >= .5] if supplement else []
                    predicted = [] if base['abstained'] else add_nonoverlapping(base['predicted'], extra)
                    records.append({'gold': row['gold'], 'predicted': predicted, 'abstained': base['abstained'], 'stratum': row['stratum']})
                name = f'{core}-{mode}' + ('-address' if supplement else '')
                runs[name] = score(records, manifest['scope'])
    result = {'schema': 1, 'independent_validation': False, 'corpus': manifest,
              'cache_sha256': {k: digest(v) for k, v in [('before', a.before), ('after', a.after), ('address', a.address)]},
              'measurement_sha256': digest(Path(__file__)), 'wasm_sha256': sources['after']['wasm_sha256'],
              'scope': 'Cached composition with unchanged contextual names; selected GLiNER2 address-only at 0.5.', 'runs': runs}
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    for name, run in runs.items():
        print(name, ' | '.join(f'{s}: exact {r["exact_excerpts"]}/{r["excerpts"]}; TP/FP/FN {r["tp"]}/{r["fp"]}/{r["fn"]}' for s, r in run['strata'].items()))


if __name__ == '__main__':
    main()
