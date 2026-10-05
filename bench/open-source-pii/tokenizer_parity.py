"""Check a local JS tokenizer against pinned Python assets on synthetic words."""
import argparse
import hashlib
import json
import subprocess
from pathlib import Path
from evaluate import ROOT, digest, load_corpus, offline, private_output


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--export', type=Path, required=True)
    p.add_argument('--package', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    package = json.loads((a.package / 'package.json').read_text())
    if package['name'] != '@huggingface/tokenizers' or package['version'] != '0.2.0':
        raise ValueError('wrong tokenizer package')
    offline()
    from transformers import AutoTokenizer
    import re
    splitter = re.compile(r'(?:https?://[^\s]+|www\.[^\s]+)|[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}|@[a-z0-9_]+|\w+(?:[-_]\w+)*|\S', re.I)
    words = ['(', ')', 'entities', 'person', 'address', 'phone_number', 'email', 'iban', 'payment_card', 'secret']
    manifests = []
    for name in ['open-source-pii-v1', 'open-source-pii-followup-v1']:
        rows, manifest = load_corpus(ROOT / 'bench/corpus' / name)
        manifests.append(manifest)
        words.extend(m.group().lower() for row in rows for m in splitter.finditer(row['text']))
    # Length-changing lowercase, decomposed accents, astral characters and controls.
    words.extend(['İ', 'i\u0307', 'Jo\u0301n', 'Sigríður', 'Åse', '𐐀', '😀', '\u200b', '\ufeff', '\x00'])
    words = sorted(set(words))
    tokenizer = AutoTokenizer.from_pretrained(str(a.export), local_files_only=True)
    expected = [tokenizer.encode(word, add_special_tokens=False) for word in words]
    js = """
import {readFileSync} from 'node:fs';
import {pathToFileURL} from 'node:url';
const [pkg,dir] = process.argv.slice(1);
const {Tokenizer} = await import(pathToFileURL(pkg+'/dist/tokenizers.mjs'));
const model=JSON.parse(readFileSync(dir+'/tokenizer.json'));
if(model.model.type!=='Unigram')throw Error('This audit only covers the inspected cache-free Unigram implementation');
const tokenizer=new Tokenizer(model,JSON.parse(readFileSync(dir+'/tokenizer_config.json')));
const words=JSON.parse(readFileSync(0,'utf8'));
process.stdout.write(JSON.stringify(words.map(word=>tokenizer.encode(word,{add_special_tokens:false}).ids)));
"""
    response = subprocess.run(['node', '--input-type=module', '-e', js, str(a.package), str(a.export)],
                              input=json.dumps(words), text=True, capture_output=True)
    if response.returncode:
        raise ValueError('JS tokenizer failed; no input-bearing error output retained')
    actual = json.loads(response.stdout)
    if len(actual) != len(expected):
        raise ValueError('wrong result count')
    mismatches = [{'word_sha256': hashlib.sha256(w.encode()).hexdigest(), 'python_length': len(e), 'js_length': len(r)}
                  for w, e, r in zip(words, expected, actual) if e != r]
    result = {'schema': 1, 'status': 'complete', 'words': len(words), 'mismatch_count': len(mismatches),
              'mismatches': mismatches, 'corpora': manifests, 'package': package['name'] + '@' + package['version'],
              'package_sha256': digest(a.package / 'dist/tokenizers.mjs'),
              'tokenizer_sha256': digest(a.export / 'tokenizer.json'),
              'tokenizer_config_sha256': digest(a.export / 'tokenizer_config.json'),
              'measurement_sha256': digest(Path(__file__)),
              'limitation': 'Exact token IDs for these synthetic words only, not model or browser inference parity.'}
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    print(json.dumps({'words': len(words), 'mismatch_count': len(mismatches)}))


if __name__ == '__main__':
    main()
