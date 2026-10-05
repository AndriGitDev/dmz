from copy import deepcopy
import unittest

from compare_quality import compare, TRAINING_INPUT, TRAINING_LABELS, TRAINING_SETTINGS
from quality import score


def result(misses):
    rows = [dict(gold=[(0, 1, 'Person')], predicted=[(0, 1, 'Person')]) for _ in range(100)]
    for row in rows[:misses]:
        row['predicted'] = []
    return score(rows, ['Person'])


class ComparisonTests(unittest.TestCase):
    def fixture(self):
        reference = dict(frozen=True, thresholds=dict(crf=.75, neural=0),
                         encoder=dict(base_weights_sha256='base'),
                         composition=dict(neural_sha256='frozen', crf_sha256='crf'),
                         evaluation_inputs=dict(general='general-source', domain='domain-source'))
        training = dict(label_mode='entities', split='train', base_weights_sha256='base',
                        schema=1, input_sha256=TRAINING_INPUT, rows=46196,
                        labels=list(TRAINING_LABELS), settings=dict(TRAINING_SETTINGS),
                        initial_head_sha256=None, independent_final_evaluation=False,
                        warm_start=dict(checkpoint_sha256='frozen'),
                        checkpoints=[dict(sha256='candidate', epoch=1, complete_epoch=True)])
        baselines, candidate = {}, {}
        for dataset in ('general', 'domain'):
            mode = 'canonical' if dataset == 'general' else 'original'
            baselines[dataset] = result(4 if dataset == 'general' else 2) | dict(
                input_sha256=dataset+'-source', text_mode=mode, thresholds=reference['thresholds'])
            candidate[dataset] = dict(model_sha256='candidate', label_mode='entities', split='dev',
                input_sha256=dataset+'-source', text_mode=mode, crf_sha256='crf',
                crf_threshold=.75, neural_threshold=0, independent_final_evaluation=False,
                rows=100, quality=dict(combined=result(1)))
        return training, [candidate], baselines, reference

    def test_qualifying_candidate_is_research_only(self):
        report = compare(*self.fixture())
        self.assertEqual(report['selected_research_checkpoint'], 'candidate')
        self.assertTrue(report['candidates'][0]['both_strata_point_targets_met'])
        self.assertFalse(report['independent_final_evaluation'])
        self.assertFalse(report['browser_promotion_approved'])

    def test_stratum_accuracy_or_precision_regression_rejects(self):
        args = self.fixture()
        args[1][0]['domain']['quality']['combined'] = result(3)
        report = compare(*args)
        self.assertIsNone(report['selected_research_checkpoint'])
        self.assertIn('domain accuracy below frozen reference', report['candidates'][0]['failures'])
        args = self.fixture()
        rows = [dict(gold=[(0, 1, 'Person')], predicted=[(0, 1, 'Person')]) for _ in range(100)]
        rows[0]['predicted'].append((2, 3, 'Person'))
        args[1][0]['domain']['quality']['combined'] = score(rows, ['Person'])
        report = compare(*args)
        self.assertIn('domain precision below frozen reference', report['candidates'][0]['failures'])

    def test_equal_recall_is_not_improvement(self):
        args = self.fixture()
        args[1][0]['general']['quality']['combined'] = result(4)
        self.assertIsNone(compare(*args)['selected_research_checkpoint'])

    def test_changed_training_recipe_cannot_enter_frozen_comparison(self):
        changes = [
            lambda a: a[0].update(input_sha256='different'),
            lambda a: a[0].update(rows=100),
            lambda a: a[0].update(labels=['O', 'B', 'I']),
            lambda a: a[0].update(initial_head_sha256='different'),
            lambda a: a[0].update(independent_final_evaluation=True),
            lambda a: a[0].update(settings=dict(seed=45)),
            lambda a: a[0]['settings'].update(encoder_lr=1e-5),
            lambda a: a[0]['settings'].update(max_steps=100),
            lambda a: a[0]['settings'].update(probe=0),
            lambda a: a[0]['checkpoints'][0].update(epoch=3),
            lambda a: a[0]['checkpoints'].append(dict(sha256='duplicate-epoch',epoch=1,complete_epoch=True)),
        ]
        for change in changes:
            with self.subTest(change=change):
                args = self.fixture(); change(args)
                with self.assertRaises(ValueError): compare(*args)

    def test_missing_incomplete_mismatched_or_changed_population_rejected(self):
        changes = [
            lambda a: a[0]['checkpoints'][0].update(complete_epoch=False),
            lambda a: a[0]['checkpoints'].append(dict(sha256='missing',epoch=2,complete_epoch=True)),
            lambda a: a[1][0].pop('domain'),
            lambda a: a[1][0]['domain'].update(model_sha256='other'),
            lambda a: a[1][0]['general'].update(text_mode='original'),
            lambda a: a[1][0]['general'].update(crf_threshold=.8),
            lambda a: a[1][0]['general'].update(crf_sha256='different'),
            lambda a: a[1][0]['general'].update(input_sha256='different'),
            lambda a: a[1][0]['general'].update(rows=99),
            lambda a: a[1][0]['general']['quality']['combined']['overall'].update(positive_excerpts=99),
            lambda a: a[1][0]['general']['quality']['combined']['overall'].update(accuracy=float('nan')),
            lambda a: a[1][0]['general']['quality']['combined']['overall'].update(accuracy=1),
            lambda a: a[1][0]['general']['quality']['combined'].update(annotation_scope=['Person','Address']),
            lambda a: a[0]['warm_start'].update(checkpoint_sha256='different'),
        ]
        for change in changes:
            with self.subTest(change=change):
                args=self.fixture(); change(args)
                with self.assertRaises(ValueError): compare(*args)

    def test_compare_all_complete_epochs_and_tie_prefers_earlier(self):
        args = self.fixture()
        args[0]['checkpoints'].append(dict(sha256='second',epoch=2,complete_epoch=True))
        second=deepcopy(args[1][0])
        for report in second.values(): report['model_sha256']='second'
        args[1].insert(0, second)
        self.assertEqual(compare(*args)['selected_research_checkpoint'],'candidate')
        for report in second.values(): report['quality']['combined']=result(0)
        self.assertEqual(compare(*args)['selected_research_checkpoint'],'second')


if __name__ == '__main__':
    unittest.main()
