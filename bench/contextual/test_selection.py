import unittest
from selection import select


def report(identity,model,tp,fp,fn,split='dev'):
    return dict(split=split,input_sha256=identity,model_sha256=model,scores={'0.65':dict(tp=tp,fp=fp,fn=fn,precision=tp/(tp+fp),recall=tp/(tp+fn))})


class SelectionTests(unittest.TestCase):
    def test_both_datasets_must_qualify_and_worst_recall_wins(self):
        a=('unbalanced',report('g','a',98,2,2),report('d','a',50,1,50))
        b=('balanced',report('g','b',90,2,10),report('d','b',90,2,10))
        c=('poor_precision',report('g','c',99,1,1),report('d','c',99,9,1))
        self.assertEqual(select([a,b,c])['selected']['candidate'],'balanced')
        self.assertFalse(select([c])['selected'])
        self.assertFalse(select([b])['selected']['target_met'])

    def test_mixed_inputs_models_and_test_split_rejected(self):
        a=('a',report('g','a',90,2,10),report('d','a',90,2,10))
        with self.assertRaises(ValueError):select([a,('b',report('other','b',90,2,10),report('d','b',90,2,10))])
        with self.assertRaises(ValueError):select([('a',a[1],report('d','different',90,2,10))])
        with self.assertRaises(ValueError):select([('a',report('g','a',90,2,10,split='test'),a[2])])

    def test_candidate_threshold_grid_cannot_change(self):
        a=('a',report('g','a',90,2,10),report('d','a',90,2,10))
        b=('b',report('g','b',90,2,10),report('d','b',90,2,10))
        for r in b[1:]:r['scores']['0.7']=r['scores'].pop('0.65')
        with self.assertRaises(ValueError):select([a,b])
        for r in b[1:]:r['scores']['nan']=r['scores'].pop('0.7')
        with self.assertRaises(ValueError):select([b])


if __name__=='__main__':unittest.main()
