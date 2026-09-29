import importlib.util
import json
from pathlib import Path
import tempfile
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

    @unittest.skipUnless(all(importlib.util.find_spec(name) for name in ['numpy','onnxruntime']), 'optional browser encoder environment')
    def test_changed_or_added_tokenizer_file_is_rejected(self):
        from evaluate import verify_tokenizer
        from encoder import digest
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);tokenizer=root/'tokenizer';tokenizer.mkdir()
            vocab=tokenizer/'vocab.json';vocab.write_text('{}')
            (root/'export.json').write_text(json.dumps(dict(tokenizer_files={'vocab.json':digest(vocab)})))
            verify_tokenizer(tokenizer)
            added=tokenizer/'added_tokens.json';added.write_text('{}')
            with self.assertRaisesRegex(ValueError,'file set'):verify_tokenizer(tokenizer)
            added.unlink();vocab.write_text('{"changed":1}')
            with self.assertRaisesRegex(ValueError,'integrity'):verify_tokenizer(tokenizer)


if __name__=='__main__':unittest.main()
