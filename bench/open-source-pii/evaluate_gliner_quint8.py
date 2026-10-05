"""Published GLiNER QUInt8 graph, unchanged labels/threshold, offline diagnostics."""
import argparse
import hashlib
import json
import resource
import time
from pathlib import Path

from evaluate import ROOT, digest, load_corpus, offline, private_output
from quality import score


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--model', type=Path, required=True)
    p.add_argument('--quantized', type=Path, required=True)
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    rows, manifest = load_corpus(a.corpus)
    config = json.loads((ROOT / 'bench/open-source-pii/candidates.json').read_text())['candidates']['gliner']
    identity = json.loads((a.model / 'download-manifest.json').read_text())
    if identity['revision'] != '61726e0ad791dcab3e29339bbec3ad42ded65641':
        raise ValueError('wrong base model')
    for name, entry in identity['files'].items():
        if digest(a.model / name) != entry['sha256']:
            raise ValueError('changed model asset')
    graph = a.quantized / 'model.onnx'
    graph_hash = digest(graph)
    if graph_hash != '0514c8fd86d0513ce5351a3267f132b57d5bcd8f99a90d43cde1228092881d19':
        raise ValueError('wrong published quantized graph')
    watched = [Path(__file__), ROOT / 'bench/open-source-pii/evaluate.py',
               ROOT / 'bench/open-source-pii/candidates.json', ROOT / 'bench/contextual/quality.py']
    hashes = {str(x.relative_to(ROOT)): digest(x) for x in watched}
    offline()
    import torch
    import onnxruntime as ort
    from gliner import GLiNER
    torch.set_num_threads(2)
    torch.set_num_interop_threads(1)
    opts = ort.SessionOptions()
    opts.intra_op_num_threads, opts.inter_op_num_threads = 2, 1
    started = time.monotonic()
    model = GLiNER.from_pretrained(str(a.model), local_files_only=True, load_onnx_model=True,
                                 onnx_model_file=str(graph.resolve()), runtime_options={'session_options': opts})
    load_seconds = time.monotonic() - started
    records = []
    failures = 0
    with torch.inference_mode():
        for row in rows:
            predicted, scored, failure = [], [], None
            began = time.monotonic()
            try:
                entities = model.predict_entities(row['text'], list(config['mapping']),
                                                  threshold=config['threshold'], flat_ner=True)
                for entity in entities:
                    start, end = int(entity['start']), int(entity['end'])
                    if not 0 <= start < end <= len(row['text']):
                        raise ValueError('invalid candidate offsets')
                    kind = config['mapping'][entity['label']]
                    span = [len(row['text'][:start].encode()), len(row['text'][:end].encode()), kind]
                    if span not in predicted:
                        predicted.append(span)
                    scored.append({'span': span, 'score': float(entity['score'])})
            except Exception as error:
                failure = type(error).__name__
                predicted, scored = [], []
                failures += 1
            records.append({'id': row['id'], 'stratum': row['stratum'], 'gold': row['gold'],
                            'text_sha256': hashlib.sha256(row['text'].encode()).hexdigest(),
                            'predicted': predicted, 'scored': scored, 'abstained': failure is not None,
                            'candidate_failure': failure, 'seconds': time.monotonic() - began})
    if any(digest(x) != hashes[str(x.relative_to(ROOT))] for x in watched):
        raise ValueError('measurement changed')
    result = {'schema': 1, 'status': 'complete', 'candidate': 'gliner-published-quint8',
              'config': config | {'runtime': 'published-onnx-quint8'}, 'corpus': manifest,
              'model': identity, 'graph_sha256': graph_hash, 'graph_bytes': graph.stat().st_size,
              'measurement_sha256': hashes, 'network': 'socket connections denied',
              'independent_validation': False, 'load_seconds': load_seconds,
              'seconds': time.monotonic() - started, 'peak_rss_kib': resource.getrusage(resource.RUSAGE_SELF).ru_maxrss,
              'candidate_failures': failures, 'records': records, 'quality': score(records, manifest['scope'])}
    with out.open('x') as stream:
        json.dump(result, stream, indent=2)
    print(json.dumps({k: result[k] for k in ['candidate', 'seconds', 'peak_rss_kib', 'candidate_failures']}))
    print(json.dumps(result['quality']['overall']))


if __name__ == '__main__':
    main()
