import unittest

from quality import score


class QualityTests(unittest.TestCase):
    def test_exact_excerpt_accuracy_is_not_precision_or_token_accuracy(self):
        name = (0, 4, 'Person')
        result = score([
            dict(gold=[name], predicted=[name], stratum='general'),
            dict(gold=[name], predicted=[(0, 3, 'Person')], stratum='general'),
            dict(gold=[], predicted=[], stratum='domain'),
            dict(gold=[], predicted=[name], stratum='domain'),
        ], ['Person'])
        overall = result['overall']
        self.assertEqual((overall['tp'], overall['fp'], overall['fn']), (1, 2, 1))
        self.assertEqual(overall['accuracy'], .5)
        self.assertEqual(overall['recall'], .5)
        self.assertAlmostEqual(overall['precision'], 1 / 3)
        self.assertEqual(overall['negative_excerpt_false_positive_rate'], .5)
        self.assertFalse(overall['point_targets_met'])
        self.assertEqual(result['strata']['domain']['recall'], None)

    def test_kind_mismatch_and_multiple_entities_fail_whole_excerpt(self):
        result = score([dict(gold=[(0, 4, 'Person'), (5, 9, 'Address')],
                             predicted=[(0, 4, 'Address'), (5, 9, 'Address')])],
                       ['Person', 'Address'])
        self.assertEqual(result['overall']['accuracy'], 0)
        self.assertEqual(result['overall']['recall'], .5)
        self.assertEqual(result['per_type']['Person']['fn'], 1)
        self.assertEqual(result['per_type']['Address']['fp'], 1)

    def test_no_positive_labels_cannot_pass_recall_target(self):
        result = score([dict(gold=[], predicted=[])], ['Person'])['overall']
        self.assertEqual(result['accuracy'], 1)
        self.assertIsNone(result['recall'])
        self.assertIsNone(result['precision'])
        self.assertFalse(result['point_targets_met'])

    def test_target_requires_both_metrics(self):
        rows = [dict(gold=[(0, 1, 'Person')], predicted=[(0, 1, 'Person')]) for _ in range(100)]
        rows[0]['predicted'] = []
        self.assertTrue(score(rows, ['Person'])['overall']['point_targets_met'])
        rows[1]['predicted'] = []
        self.assertFalse(score(rows, ['Person'])['overall']['point_targets_met'])

    def test_unavailable_typed_result_cannot_count_as_correct_negative(self):
        report = score([
            dict(gold=[], predicted=[], abstained=True),
            dict(gold=[(0, 1, 'Person')], predicted=[], abstained=True),
            dict(gold=[], predicted=[]),
        ], ['Person'])['overall']
        self.assertEqual(report['accuracy'], 1/3)
        self.assertEqual(report['recall'], 0)
        self.assertEqual(report['fn'], 1)
        self.assertEqual(report['abstained_excerpts'], 2)
        self.assertEqual(report['abstained_negative_excerpts'], 1)
        for row in [dict(gold=[], predicted=[], abstained='yes'),
                    dict(gold=[], predicted=[(0,1,'Person')], abstained=True)]:
            with self.assertRaises(ValueError): score([row], ['Person'])

    def test_reject_incomplete_invalid_duplicate_and_wrong_scope(self):
        for gold in (None, 'missing', [(0, 1, 'Person')] * 2,
                     [(True, 1, 'Person')], [(1, 1, 'Person')], [(0, 1, 'Address')]):
            with self.subTest(gold=gold), self.assertRaises(ValueError):
                score([dict(gold=gold, predicted=[])], ['Person'])
        with self.assertRaises(ValueError):
            score([], ['Person'])
        for minimum in (float('nan'), True, 0, 2):
            with self.assertRaises(ValueError):
                score([dict(gold=[], predicted=[])], ['Person'], minimum)


if __name__ == '__main__':
    unittest.main()
