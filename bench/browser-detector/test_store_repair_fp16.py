import copy
import unittest
from store_repair_fp16 import require_quality_preservation


class QualityGateTest(unittest.TestCase):
    def setUp(self):
        score = dict(tp=97, fp=3, fn=3, precision=.97, recall=.97)
        self.reference = {name: dict(score) for name in ['general', 'domain']}
        self.report = dict(model_sha256='parent', text_mode='canonical', independent_validation=False,
                           runtime='onnxruntime-web test', browser='test', externalRequests=[], sets=[dict(name=name, combined=dict(score),
                           fp32_browser_combined=dict(score)) for name in self.reference])

    def test_matching_quality_is_eligible(self):
        require_quality_preservation(self.report, 'parent', self.reference)

    def test_recall_loss_above_target_still_rejects(self):
        self.report['sets'][0]['combined'] = dict(tp=96, fp=2, fn=4, precision=96/98, recall=.96)
        with self.assertRaisesRegex(ValueError, 'does not preserve'):
            require_quality_preservation(self.report, 'parent', self.reference)

    def test_wrong_parent_and_missing_domain_reject(self):
        with self.assertRaisesRegex(ValueError, 'parent graph'):
            require_quality_preservation(self.report, 'other', self.reference)
        duplicate = copy.deepcopy(self.report)
        duplicate['sets'][1]['name'] = 'general'
        with self.assertRaisesRegex(ValueError, 'both development sets'):
            require_quality_preservation(duplicate, 'parent', self.reference)

    def test_omitted_gold_spans_reject(self):
        self.report['sets'][0]['combined'] = dict(tp=97, fp=3, fn=2, precision=.97, recall=97/99)
        with self.assertRaisesRegex(ValueError, 'reference gold spans'):
            require_quality_preservation(self.report, 'parent', self.reference)


if __name__ == '__main__':
    unittest.main()
