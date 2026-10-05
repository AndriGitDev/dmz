import copy
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

from quality_clusters import interval, quantile, ratios, report


def row(excerpt, cluster, correct=True, stratum='general', document=None):
    entity = [0, 4, 'Person']
    return dict(excerpt_id=excerpt, document_id=document or cluster,
                cluster_id=cluster, stratum=stratum, gold=[entity],
                predicted=[entity] if correct else [])


class ClusterQualityTests(unittest.TestCase):
    def test_whole_clusters_and_pooled_counts_not_mean_percentages(self):
        rows = [row('one', 'a', False)] + [row(str(i), 'b') for i in range(9)]
        # Each replicate draws both clusters. The nine-row cluster must retain
        # all its weight: a mean of cluster percentages would incorrectly be .5.
        class Alternating:
            def __init__(self, seed): self.n = 0
            def randrange(self, n):
                self.n += 1
                assert n == 2
                return (self.n - 1) % 2
        with patch('quality_clusters.random.Random', Alternating), \
                patch('quality_clusters.interval', side_effect=lambda values, n: values[0]):
            result = report(rows, ['Person'])
        self.assertEqual(result['strata']['general']['metrics']['accuracy'], .9)
        self.assertEqual(result['strata']['general']['metrics']['recall'], .9)
        self.assertEqual(result['strata']['general']['documents'], 2)

    def test_repeated_draw_duplicates_entire_related_document_cluster(self):
        rows = [row('one', 'a', False), row('two', 'b', document='b1'),
                row('three', 'b', document='b2')]
        with patch('quality_clusters.random.Random') as rng, \
                patch('quality_clusters.interval', side_effect=lambda values, n: values[0]):
            rng.return_value.randrange.return_value = 1
            result = report(rows, ['Person'])
        self.assertEqual(result['strata']['general']['metrics']['accuracy'], 1)
        self.assertEqual(result['strata']['general']['documents'], 3)
        self.assertEqual(result['strata']['general']['clusters'], 2)
        self.assertEqual(result['point_estimates']['overall']['accuracy'], 2 / 3)

    def test_real_resamples_are_order_stable_and_stratum_independent(self):
        rows = [row('one', 'a'), row('two', 'b', False), row('three', 'c')]
        baseline = report(rows, ['Person'])
        self.assertEqual(baseline, report(list(reversed(rows)), ['Person']))
        extended = report(rows + [row('four', 'd', stratum='domain')], ['Person'])
        self.assertEqual(baseline['strata']['general'], extended['strata']['general'])
        self.assertNotEqual(baseline['membership_sha256'], extended['membership_sha256'])
        accuracy = baseline['strata']['general']['metrics']['accuracy']
        self.assertEqual(accuracy['status'], 'available')
        self.assertEqual((accuracy['lower'], accuracy['upper']), (0, 1))

    def test_abstention_and_undefined_denominators(self):
        rows = [dict(row('one', 'a'), gold=[], predicted=[], abstained=True),
                dict(row('two', 'b'), gold=[], predicted=[])]
        result = report(rows, ['Person'])
        self.assertEqual(result['point_estimates']['overall']['accuracy'], .5)
        self.assertEqual(result['point_estimates']['overall']['abstained_negative_excerpts'], 1)
        recall = result['strata']['general']['metrics']['recall']
        self.assertEqual(recall['status'], 'undefined_replicates')
        self.assertEqual(recall['undefined_replicates'], 10000)
        self.assertIsNone(recall['lower'])
        rows[0] = row('one', 'a')
        recall = report(rows, ['Person'])['strata']['general']['metrics']['recall']
        self.assertTrue(0 < recall['undefined_replicates'] < 10000)
        self.assertIsNone(recall['upper'])

    def test_no_perfect_population_interval_or_single_cluster_interval(self):
        for rows, expected in [([row('one', 'a')], 'insufficient_clusters'),
                               ([row('one', 'a'), row('two', 'b')], 'degenerate')]:
            result = report(rows, ['Person'])
            self.assertFalse(result['independent_validation'])
            self.assertFalse(result['population_guarantee'])
            accuracy = result['strata']['general']['metrics']['accuracy']
            self.assertEqual(accuracy['status'], expected)
            self.assertIsNone(accuracy['lower'])
            self.assertIsNone(accuracy['upper'])

    def test_reject_missing_identity_duplicates_crossed_groups_and_null_labels(self):
        original = [row('one', 'a'), row('two', 'b')]
        variants = []
        for key in ('excerpt_id', 'document_id', 'cluster_id', 'stratum'):
            for value in (None, '', ' ', 1):
                changed = copy.deepcopy(original)
                changed[1][key] = value
                variants.append(changed)
        variants += [original + [original[0]],
                     [original[0], dict(original[1], document_id='a')],
                     [original[0], dict(original[1], cluster_id='a', stratum='domain')],
                     [dict(original[0], gold=None)]]
        for rows in variants:
            with self.subTest(rows=rows), self.assertRaises(ValueError):
                report(rows, ['Person'])

    def test_quantiles_and_ratios(self):
        self.assertEqual(quantile([30, 0, 20, 10], .25), 7.5)
        self.assertEqual(quantile([30, 0, 20, 10], .975), 29.25)
        self.assertEqual(ratios([9, 10, 8, 2, 4]),
                         dict(accuracy=.9, recall=2/3, precision=.8))
        self.assertEqual(interval([None, 1], 2)['status'], 'undefined_replicates')

    def test_cli_aggregate_only_provenance_and_no_overwrite(self):
        with tempfile.TemporaryDirectory() as directory:
            source, output = (Path(directory) / name for name in ('input.json', 'output.json'))
            source.write_text(json.dumps(dict(kinds=['Person'], records=[
                dict(row('private-excerpt-a', 'private-cluster-a'), text='PRIVATE RAW TEXT'),
                row('private-excerpt-b', 'private-cluster-b', False)])))
            command = [sys.executable, str(Path(__file__).with_name('quality_clusters.py')),
                       '--input', str(source), '--output', str(output)]
            subprocess.run(command, check=True, capture_output=True)
            raw = output.read_text()
            self.assertNotIn('private-', raw)
            self.assertNotIn('PRIVATE RAW TEXT', raw)
            self.assertNotIn('predicted', raw)
            result = json.loads(raw)
            self.assertEqual(len(result['input_sha256']), 64)
            self.assertEqual(set(result['implementation_sha256']), {'quality.py', 'quality_clusters.py'})
            self.assertEqual(output.stat().st_mode & 0o777, 0o600)
            self.assertNotEqual(subprocess.run(command, capture_output=True).returncode, 0)
            self.assertEqual(output.read_text(), raw)


if __name__ == '__main__':
    unittest.main()
