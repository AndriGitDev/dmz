import unittest
from priority import compare


class PriorityTests(unittest.TestCase):
    def test_overlap_changes_but_thresholds_stay_fixed(self):
        shared=dict(split='dev',input_sha256='input',text_mode='canonical')
        gold=[[0,8],[20,24]]
        crf=dict(**shared,model_sha256='crf',records=[dict(gold=gold,predictions=[[0,3,.9],[20,24,.8],[30,35,.7]])])
        neural=dict(**shared,model_sha256='neural',records=[dict(gold=gold,predictions=[[0,8,.1]])])
        result=compare(crf,neural)
        self.assertEqual([result['scores'][k] for k in ('tp','fp','fn')],[2,0,0])
        self.assertEqual(result['versus_crf_priority']['recovered_spans'],1)
        self.assertEqual(result['versus_crf_priority']['removed_false_positives'],1)
        neural['input_sha256']='different'
        with self.assertRaisesRegex(ValueError,'identity'):compare(crf,neural)


if __name__ == '__main__':unittest.main()
