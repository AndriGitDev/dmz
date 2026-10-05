"""Deterministic expensive-model sample; selection never reads predictions."""
import hashlib
import json
from pathlib import Path

SEED = 'dmz-opf-budget-20261005-v1'
COUNT = 512


def selected_ids(rows, excluded):
    return sorted((r['id'] for r in rows if r['id'] not in excluded),
                  key=lambda key: (hashlib.sha256((SEED + '\0' + key).encode()).hexdigest(), key))[:COUNT]


def load_selection(path, rows, test_sha):
    raw = Path(path).read_bytes()
    data = json.loads(raw)
    identifiers = {r['id'] for r in rows}
    excluded = set(data['excluded_previously_inferred_ids'])
    if data['schema'] != 1 or data['seed'] != SEED or data['count'] != COUNT or data['corpus_test_sha256'] != test_sha or not excluded.issubset(identifiers):
        raise ValueError('wrong fixed selection')
    if data['ids'] != selected_ids(rows, excluded) or len(data['ids']) != COUNT:
        raise ValueError('selection was not the declared deterministic sample')
    by_id = {r['id']: r for r in rows}
    selected = [by_id[key] for key in data['ids']]
    if {r['id']: r['text_sha256'] for r in selected} != data['text_sha256']:
        raise ValueError('selection text changed')
    return selected, data | {'manifest_sha256': hashlib.sha256(raw).hexdigest()}
