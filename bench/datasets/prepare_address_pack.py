#!/usr/bin/env python3
"""Prepare an address-only Shield lexicon from locally approved HMS forms."""
import argparse
import hashlib
import json
from pathlib import Path
import re


def prepare(source, output):
    repo = Path(__file__).resolve().parents[2]
    if output.resolve() == repo or repo in output.resolve().parents:
        raise ValueError('generated data must stay outside the repository')
    raw = source.read_bytes()
    forms = json.loads(raw)
    if not isinstance(forms, list) or not all(isinstance(s, str) for s in forms):
        raise ValueError('expected an address-form array')
    addresses = sorted({' '.join(s.split()) for s in forms
                        if 1 <= len(s.split()) <= 8
                        and all(re.fullmatch(r"[^\W\d_]+(?:[-’'][^\W\d_]+)*", t)
                                for t in s.split())
                        and len(' '.join(s.split()).encode()) <= 256})
    if not addresses or len(addresses) > 50000:
        raise ValueError('invalid address-form count')
    pack = dict(schema=1, names=[], addresses=addresses)
    data = (json.dumps(pack, ensure_ascii=False, separators=(',', ':')) + '\n').encode()
    if len(data) > 4 * 1024 * 1024:
        raise ValueError('address pack exceeds size limit')
    output.write_bytes(data)
    # Only aggregate provenance is printed; no source forms or complete addresses.
    return dict(addresses=len(addresses), source_sha256=hashlib.sha256(raw).hexdigest(),
                pack_sha256=hashlib.sha256(data).hexdigest())


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--addresses', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    print(json.dumps(prepare(args.addresses, args.output)))
