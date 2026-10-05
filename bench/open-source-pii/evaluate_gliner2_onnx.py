"""Offline parity/quality trial for a pinned community GLiNER2 ONNX export.

Research only: upstream splitting/decoding is deliberately preserved so parity
differences remain visible before considering a browser adapter.
"""
import argparse
import json
import resource
import subprocess
import sys
import time
from pathlib import Path
from evaluate import ROOT, digest, load_corpus, offline, private_output
from quality import score

SOURCE_REVISION = '35655a6b18fa24e81584645415a1733e9d47e321'


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--source', type=Path, required=True)
    p.add_argument('--export', type=Path, required=True)
    p.add_argument('--quantized', type=Path)
    p.add_argument('--variant', choices=['fp32', 'encoder-q8', 'all-q8'], required=True)
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    if subprocess.check_output(['git', '-C', str(a.source), 'rev-parse', 'HEAD'], text=True).strip() != SOURCE_REVISION:
        raise ValueError('wrong runtime revision')
    if subprocess.check_output(['git', '-C', str(a.source), 'status', '--porcelain'], text=True).strip():
        raise ValueError('modified runtime checkout')
    files = {n: a.export / 'onnx' / (n + '.onnx') for n in ['encoder', 'span_rep', 'count_embed']}
    if a.variant != 'fp32':
        if a.quantized is None:
            raise ValueError('quantized export required')
        quant = json.loads((a.quantized / 'manifest.json').read_text())
        for name in (['encoder'] if a.variant == 'encoder-q8' else list(files)):
            entry = quant['files'][name]
            if digest(files[name]) != entry['source_sha256']:
                raise ValueError('wrong FP32 export')
            files[name] = a.quantized / (name + '.onnx')
            if digest(files[name]) != entry['sha256']:
                raise ValueError('changed quantized export')
    identities = {n: {'sha256': digest(f), 'bytes': f.stat().st_size} for n, f in files.items()}
    runtime_files = list((a.source / 'gliner2_onnx').glob('*.py'))
    paths = [Path(__file__), ROOT / 'bench/open-source-pii/evaluate.py', ROOT / 'bench/open-source-pii/candidates.json', *runtime_files]
    hashes = {str(f): digest(f) for f in paths}
    rows, manifest = load_corpus(a.corpus)
    config = json.loads((ROOT / 'bench/open-source-pii/candidates.json').read_text())['candidates']['gliner2']
    sys.path.insert(0, str(a.source))
    offline()
    import onnxruntime as ort
    from gliner2_onnx.runtime import GLiNER2ONNXRuntime

    class NEROnlyRuntime(GLiNER2ONNXRuntime):
        def _load_onnx_models(self, onnx_files, providers):
            # Classification is unused for NER, so it need not occupy memory.
            for name, path in files.items():
                options = ort.SessionOptions()
                options.intra_op_num_threads = 2
                options.inter_op_num_threads = 1
                setattr(self, name, ort.InferenceSession(str(path), sess_options=options, providers=providers))

    started = time.monotonic()
    model = NEROnlyRuntime(a.export, providers=['CPUExecutionProvider'])
    load_seconds = time.monotonic() - started
    records, failures = [], {}
    for row in rows:
        began = time.monotonic()
        scored, failure = [], None
        try:
            entities = model.extract_entities(row['text'], list(config['mapping']), threshold=config['threshold'])
            for entity in entities:
                if not 0 <= entity.start < entity.end <= len(row['text']):
                    raise ValueError('invalid offsets')
                if entity.text != row['text'][entity.start:entity.end]:
                    raise ValueError('changed source text')
                span = [len(row['text'][:entity.start].encode()), len(row['text'][:entity.end].encode()), config['mapping'][entity.label]]
                scored.append({'span': span, 'score': entity.score})
        except Exception as error:
            failure = type(error).__name__
            failures[failure] = failures.get(failure, 0) + 1
            scored = []
        import hashlib
        records.append({'id': row['id'], 'stratum': row['stratum'], 'gold': row['gold'],
                        'text_sha256': hashlib.sha256(row['text'].encode()).hexdigest(),
                        'predicted': sorted({tuple(s['span']) for s in scored}), 'scored': scored,
                        'abstained': failure is not None, 'candidate_failure': failure,
                        'seconds': time.monotonic() - began})
        if len(records) % 20 == 0:
            print(json.dumps({'completed': len(records), 'seconds': time.monotonic() - started}), flush=True)
    for path in paths:
        if digest(path) != hashes[str(path)]:
            raise ValueError('measurement changed')
    for name, path in files.items():
        if digest(path) != identities[name]['sha256']:
            raise ValueError('model changed')
    result = {'schema': 1, 'status': 'complete', 'candidate': 'gliner2-onnx-' + a.variant,
              'config': config, 'corpus': manifest, 'export_files': identities,
              'runtime_revision': SOURCE_REVISION, 'measurement_sha256': hashes,
              'runtime': ort.__version__, 'network': 'socket connections denied',
              'load_seconds': load_seconds, 'seconds': time.monotonic() - started,
              'peak_rss_kib': resource.getrusage(resource.RUSAGE_SELF).ru_maxrss,
              'candidate_failures': failures, 'records': records,
              'quality': score(records, manifest['scope']), 'independent_validation': False}
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    print(json.dumps({k: result[k] for k in ['candidate', 'seconds', 'peak_rss_kib', 'candidate_failures']}))


if __name__ == '__main__':
    main()
