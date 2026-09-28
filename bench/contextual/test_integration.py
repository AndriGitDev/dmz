"""Optional end-to-end check: only fabricated text, no licensed corpus needed."""
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from ner import evaluate, train


@unittest.skipUnless(importlib.util.find_spec('pycrfsuite'), 'install contextual requirements for integration test')
class IntegrationTests(unittest.TestCase):
    def test_train_evaluate_and_reject_modified_model(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source, model = root/'train.jsonl', root/'tiny.crfsuite'
            rows = [dict(split='train', text='Ása Gervi skrifar', entities=[dict(kind='Person',start=0,end=len('Ása Gervi'.encode()))]),
                    dict(split='train', text='Skjal er tilbúið', entities=[])] * 8
            source.write_text(''.join(json.dumps(r)+'\n' for r in rows))
            metadata = train(source, model, 20, .1, .1, version=2)
            self.assertEqual(metadata['training_sentences'], 16)
            with self.assertRaises(ValueError): train(source, model, 20, .1, .1)
            for row in rows: row['split'] = 'dev'
            dev = root/'dev.jsonl'
            dev.write_text(''.join(json.dumps(r)+'\n' for r in rows))
            report = evaluate(dev, model, 'dev', [0])
            self.assertEqual(report['sentences'], 16)
            self.assertEqual(report['scores'][0]['tp'], 8)
            self.assertEqual(report['scores'][0]['fp'], 0)
            metadata['model_sha256'] = 'incorrect'
            model.with_suffix('.metadata.json').write_text(json.dumps(metadata))
            with self.assertRaises(ValueError): evaluate(dev, model, 'dev', [0])
            with self.assertRaises(ValueError): train(dev, root/'wrong.crfsuite', 20, .1, .1)


if __name__ == '__main__': unittest.main()
