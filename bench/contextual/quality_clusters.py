#!/usr/bin/env python3
"""Descriptive, stratified whole-cluster bootstrap; never an acceptance claim."""
from collections import defaultdict
import hashlib
import json
from pathlib import Path
import platform
import random

from quality import score

REPLICATES = 10_000
SEED = 20261005
COUNTS = ('exact_excerpts', 'excerpts', 'tp', 'fp', 'fn')


def quantile(values, probability):
    """Linear interpolation between sorted order statistics (type 7)."""
    ordered = sorted(values)
    position = (len(ordered) - 1) * probability
    lower = int(position)
    upper = min(lower + 1, len(ordered) - 1)
    return ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)


def ratios(counts):
    exact, excerpts, tp, fp, fn = counts
    return dict(accuracy=exact / excerpts,
                recall=tp / (tp + fn) if tp + fn else None,
                precision=tp / (tp + fp) if tp + fp else None)


def interval(values, clusters):
    valid = [v for v in values if v is not None]
    status = ('insufficient_clusters' if clusters < 2 else
              'undefined_replicates' if len(valid) != len(values) else
              'degenerate' if min(valid) == max(valid) else 'available')
    return dict(status=status, valid_replicates=len(valid),
                undefined_replicates=len(values) - len(valid),
                lower=quantile(valid, .025) if status == 'available' else None,
                upper=quantile(valid, .975) if status == 'available' else None)


def report(records, kinds):
    records = list(records)
    point = score(records, kinds)
    documents, excerpts, membership = {}, set(), []
    groups = defaultdict(dict)
    for row in records:
        if any(not isinstance(row.get(key), str) or not row[key].strip()
               for key in ('excerpt_id', 'document_id', 'cluster_id', 'stratum')):
            raise ValueError('explicit excerpt, document, cluster and stratum identities required')
        excerpt, document, cluster, stratum = (row[k] for k in
                                             ('excerpt_id', 'document_id', 'cluster_id', 'stratum'))
        if excerpt in excerpts:
            raise ValueError('duplicate excerpt identity')
        excerpts.add(excerpt)
        if documents.setdefault(document, (cluster, stratum)) != (cluster, stratum):
            raise ValueError('document crosses clusters or strata')
        membership.append((excerpt, document, cluster, stratum))
        counts = score([row], kinds)['overall']
        aggregate = groups[stratum].setdefault(cluster, [0] * len(COUNTS))
        for i, key in enumerate(COUNTS):
            aggregate[i] += counts[key]
    cluster_strata = {}
    for stratum, clusters in groups.items():
        for cluster in clusters:
            if cluster_strata.setdefault(cluster, stratum) != stratum:
                raise ValueError('related cluster crosses strata')
    uncertainty = {}
    for stratum, clusters in sorted(groups.items()):
        # Identity ordering makes input row order irrelevant. Separate RNG streams
        # prevent another stratum from perturbing this stratum's resamples.
        units = [clusters[key] for key in sorted(clusters)]
        seed = hashlib.sha256(json.dumps([SEED, stratum]).encode()).digest()
        rng = random.Random(int.from_bytes(seed, 'big'))
        samples = {key: [] for key in ('accuracy', 'recall', 'precision')}
        for _ in range(REPLICATES):
            total = [0] * len(COUNTS)
            for _ in units:
                unit = units[rng.randrange(len(units))]
                for i, count in enumerate(unit):
                    total[i] += count
            for key, value in ratios(total).items():
                samples[key].append(value)
        uncertainty[stratum] = dict(
            clusters=len(units), documents=sum(s == stratum for _, s in documents.values()),
            excerpts=point['strata'][stratum]['excerpts'],
            metrics={key: interval(values, len(units)) for key, values in samples.items()})
    return dict(schema=1, point_estimates=point, strata=uncertainty,
                procedure=dict(name='stratified whole-cluster percentile bootstrap',
                               replicates=REPLICATES, seed=SEED, confidence_level=.95,
                               quantile='linear type 7', python=platform.python_version()),
                membership_sha256=hashlib.sha256(json.dumps(sorted(membership),
                    ensure_ascii=True, separators=(',', ':')).encode()).hexdigest(),
                independent_validation=False, population_guarantee=False,
                limitation='Descriptive approximate intervals only. Sampling design, complete independent labels, source rights and system acceptance require separate evidence.')


def main():
    import argparse
    from ner import external
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--input', type=Path, required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    raw = external(args.input).read_bytes()
    payload = json.loads(raw)
    result = report(payload['records'], payload['kinds'])
    result['input_sha256'] = hashlib.sha256(raw).hexdigest()
    result['implementation_sha256'] = {
        name: hashlib.sha256(Path(__file__).with_name(name).read_bytes()).hexdigest()
        for name in ('quality_clusters.py', 'quality.py')}
    # Never overwrite an existing experiment; report contains no entity values.
    import os
    fd = os.open(external(args.output), os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w') as output:
        json.dump(result, output, indent=2, allow_nan=False)
        output.write('\n')


if __name__ == '__main__':
    main()
