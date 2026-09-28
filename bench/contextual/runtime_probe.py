#!/usr/bin/env python3
"""Measure local CPU encoder + pooling + selected head; fabricated inputs only.
Not a browser benchmark or a quality evaluation. No embedding cache is used.
"""
import argparse
import json
import platform
import resource
import statistics
import time
from pathlib import Path
from encoder import classifier, chunks, digest


def main():
    parser = argparse.ArgumentParser(__doc__)
    parser.add_argument('--base', type=Path, required=True)
    parser.add_argument('--head', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    start = time.perf_counter()
    import torch
    from transformers import AutoModel, AutoTokenizer
    from safetensors.torch import load_file
    torch.set_num_threads(3)
    tokenizer = AutoTokenizer.from_pretrained(args.base, local_files_only=True, add_prefix_space=True, trust_remote_code=False)
    model = AutoModel.from_pretrained(args.base, local_files_only=True, use_safetensors=True, add_pooling_layer=False, trust_remote_code=False).eval()
    head = classifier(768, 128)
    head.load_state_dict(load_file(str(args.head)))
    head.eval()
    load_seconds = time.perf_counter() - start
    def infer(words):
        count = 0
        with torch.inference_mode():
            for _, part in chunks(words, tokenizer):
                batch = tokenizer([part], is_split_into_words=True, return_tensors='pt')
                hidden = model(**batch).last_hidden_state[0]
                ids = batch.word_ids(0)
                pooled = torch.stack([hidden[[i for i, w in enumerate(ids) if w == word]].mean(0) for word in range(len(part))])
                # Match the float16 cache round-trip used for head training.
                torch.softmax(head(pooled.half().float()), dim=-1).argmax(-1).tolist()
                count += 1
        return count
    phrase = 'Tilbúið próf Arna Jónsdóttir sendi samning til yfirferðar og greiðsla verður bókuð á morgun .'.split()
    results = []
    for size in [32, 128, 512]:
        words = (phrase * (size // len(phrase) + 1))[:size]
        cold_start = time.perf_counter(); chunk_count = infer(words)
        first = time.perf_counter() - cold_start
        timings = []
        for _ in range(5):
            start = time.perf_counter(); infer(words); timings.append(time.perf_counter() - start)
        results.append(dict(words=size, chunks=chunk_count, first_seconds=first, warm_median_seconds=statistics.median(timings), warm_max_seconds=max(timings), repeats=5))
    output = dict(schema=1, scope='Python CPU encoder + mean pooling + BIO head; excludes CRF, browser runtime and DOM; fabricated repeated text', platform=platform.platform(), torch=torch.__version__, threads=3, encoder_bytes=(args.base/'model.safetensors').stat().st_size, encoder_sha256=digest(args.base/'model.safetensors'), head_sha256=digest(args.head), encoder_parameters=sum(p.numel() for p in model.parameters()), load_seconds=load_seconds, peak_process_rss_kib=resource.getrusage(resource.RUSAGE_SELF).ru_maxrss, measurements=results, browser_validated=False)
    args.output.write_text(json.dumps(output, indent=2)+'\n')
    print(json.dumps(output, indent=2))


if __name__ == '__main__':
    main()
