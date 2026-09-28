import importlib.util
import unittest


class ComparisonTests(unittest.TestCase):
    @unittest.skipUnless(all(importlib.util.find_spec(name) for name in ['numpy','onnxruntime']), 'optional browser encoder environment')
    def test_boundary_regression_and_recovery_are_separate(self):
        from corpus import compression_changes
        gold=[[0,4],[10,14]]
        primary=[dict(gold=gold,predictions=[])]
        frozen=[dict(gold=gold,predictions=[[0,4,1.],[20,24,1.]])]
        compressed=[dict(gold=gold,predictions=[[0,3,1.],[10,14,1.]])]
        self.assertEqual(compression_changes(compressed,frozen,primary),dict(
            changed_records=1,added_missed_spans=1,recovered_spans=1,
            added_false_positives=1,removed_false_positives=1))
        with self.assertRaises(ValueError):compression_changes(compressed,[],primary)
        wrong=[dict(gold=[],predictions=[])]
        with self.assertRaisesRegex(ValueError,'annotation'):compression_changes(compressed,wrong,primary)


if __name__=='__main__':unittest.main()
