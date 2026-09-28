import unittest
from gate import check

class GateTests(unittest.TestCase):
    def score(self,tp,fp,fn):return dict(tp=tp,fp=fp,fn=fn,precision=tp/(tp+fp),recall=tp/(tp+fn))
    def test_both_metrics_must_pass(self):
        self.assertFalse(check(self.score(1008,47,534))['target_met'])
        self.assertFalse(check(self.score(955,100,45))['target_met'])
        self.assertTrue(check(self.score(955,45,45))['target_met'])
    def test_empty_or_misreported_results_fail(self):
        for s in [dict(tp=0,fp=0,fn=0),dict(tp=1,fp=1,fn=1,precision=1.,recall=1.)]:
            with self.assertRaises(ValueError):check(s)

if __name__=='__main__':unittest.main()
