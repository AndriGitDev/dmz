import copy
import unittest
from ensemble import merge,evaluate


class EnsembleTests(unittest.TestCase):
    def test_overlap_priority_threshold_and_touching_spans(self):
        p=[[0,4,.8],[20,25,.4]];n=[[0,8,.99],[4,8,.9],[20,25,.9]]
        self.assertEqual(merge(p,n,.65,.65),{(0,4),(4,8),(20,25)})
        self.assertEqual(merge(p,n,.9,.95),{(0,8)})

    def test_identity_and_gold_agreement(self):
        p=dict(split='dev',input_sha256='synthetic',text_mode='canonical',model_sha256='a',records=[dict(gold=[[0,4]],predictions=[[0,4,.9]])])
        n=copy.deepcopy(p);n['model_sha256']='b'
        result=evaluate(p,n)
        self.assertEqual(result['models'][0]['report']['scores'][0]['tp'],1)
        n['records'][0]['gold']=[[0,5]]
        with self.assertRaises(ValueError):evaluate(p,n)
        n=copy.deepcopy(p);n['split']='test'
        with self.assertRaises(ValueError):evaluate(p,n)


if __name__=='__main__':unittest.main()
