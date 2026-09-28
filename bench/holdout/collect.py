#!/usr/bin/env python3
"""Bounded public PDF collection for LOCAL annotation preparation, not training.
Source text never enters Git or stdout. No model predictions or inferred labels.
Install pypdf==6.1.1 in the local research environment.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import io
import json
from pathlib import Path
import re
from urllib.request import Request, urlopen
from pypdf import PdfReader


def sha(data):
    return hashlib.sha256(data).hexdigest()


def normalized(text):
    return re.findall(r'\w+|[^\w\s]', text.casefold())


def shingles(text):
    tokens = normalized(text)
    return {sha(' '.join(tokens[i:i+12]).encode()) for i in range(max(0, len(tokens)-11))}


def main():
    parser = argparse.ArgumentParser(__doc__)
    parser.add_argument('--sources', type=Path, default=Path(__file__).with_name('sources.json'))
    parser.add_argument('--only', nargs='+', help='Collect only these source IDs')
    parser.add_argument('--mim', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    repo = Path(__file__).resolve().parents[2]
    output = args.output.resolve()
    if output == repo or repo in output.parents:
        raise ValueError('source text must remain outside Git')
    output.mkdir(parents=True, exist_ok=False)
    known = set()
    mim_hashes = {}
    for split in ['train', 'dev', 'test']:
        source = args.mim / f'{split}.jsonl'
        mim_hashes[split] = sha(source.read_bytes())
        for line in source.open():
            known.update(shingles(json.loads(line)['text']))
    results = []
    with (output/'review.jsonl').open('w') as review:
        for source in json.loads(args.sources.read_text()):
            if args.only and source['id'] not in args.only:
                continue
            entry = dict(source)
            try:
                request = Request(source['url'], headers={'User-Agent':'DMZ-evaluation-research/0.1', 'Accept':'application/pdf'})
                with urlopen(request, timeout=45) as response:
                    data = response.read(20_000_001)
                    entry['resolved_url'] = response.url
                if len(data) > 20_000_000 or not data.startswith(b'%PDF'):
                    raise ValueError('not a bounded PDF')
                (output/(source['id']+'.pdf')).write_bytes(data)
                reader = PdfReader(io.BytesIO(data))
                pages = [page.extract_text() or '' for page in reader.pages]
                (output/(source['id']+'.pages.json')).write_text(json.dumps(pages, ensure_ascii=False))
                candidates = [(sha((source['id']+str(i)).encode()), i, text) for i, text in enumerate(pages) if len(text.strip()) >= 200]
                chosen = sorted(candidates)[:12]
                matches = 0
                for _, i, text in chosen:
                    overlap = len(shingles(text) & known)
                    matches += overlap
                    review.write(json.dumps(dict(document_id=source['id'],page=i+1,text=text,text_sha256=sha(text.encode()),entities=None,reviewed=False,overlapping_mim_12grams=overlap,eligible_for_final=False),ensure_ascii=False)+'\n')
                entry.update(sha256=sha(data),bytes=len(data),pages=len(pages),review_pages=len(chosen),overlapping_mim_12grams=matches)
            except Exception as error:
                # No response body or extracted text in committed metadata.
                entry['error'] = type(error).__name__
                if hasattr(error, 'code'): entry['http_status'] = error.code
            results.append(entry)
    report = dict(schema=1,collected_at=datetime.now(timezone.utc).isoformat(),state='unlabelled candidate pilot; not independent evaluation',mim_hashes=mim_hashes,sampling='up to 12 pages >=200 characters per document, sorted by SHA256(document ID + zero-based page); chosen before model inference',overlap='casefolded punctuation-tokenized exact 12-grams against all prepared MIM splits; does not exclude semantic, short phrase or pretraining overlap',documents=results,human_review_complete=False,model_evaluated=False)
    (output/'manifest.json').write_text(json.dumps(report,indent=2)+'\n')
    print(json.dumps(report,indent=2))


if __name__ == '__main__':
    main()
