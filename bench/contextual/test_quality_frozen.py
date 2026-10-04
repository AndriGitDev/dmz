import json
from pathlib import Path
import tempfile
import unittest

from ner import sha
from quality_frozen import evaluate


class FrozenQualityTests(unittest.TestCase):
    def fixture(self, root):
        paths = [root / name for name in ('source.jsonl', 'crf.json', 'neural.json', 'selection.json')]
        source, crf, neural, selection = paths
        source.write_text(json.dumps(dict(split='dev', text='Ása kom .',
                                         entities=[dict(start=0, end=4, kind='Person')])) + '\n')
        cache = dict(split='dev', input_sha256=sha(source), text_mode='canonical',
                     records=[dict(gold=[[0, 4]], predictions=[[0, 4, 1.]])])
        for path, model in ((crf, 'crf-identity'), (neural, 'neural-identity')):
            path.write_text(json.dumps(cache | dict(model_sha256=model)))
        selection.write_text(json.dumps(dict(frozen=True, evaluation_inputs=dict(general=sha(source)),
                                            composition=dict(crf_sha256='crf-identity', neural_sha256='neural-identity'),
                                            thresholds=dict(crf=.75, neural=0))))
        return paths

    def test_verified_frozen_cache(self):
        with tempfile.TemporaryDirectory() as directory:
            paths = self.fixture(Path(directory))
            report = evaluate(*paths, 'general')
            self.assertEqual(report['overall']['accuracy'], 1)
            self.assertTrue(report['overall']['point_targets_met'])
            self.assertFalse(report['independent_validation'])
            self.assertNotIn('Ása', json.dumps(report, ensure_ascii=False))

    def test_corrupt_or_misaligned_cache_fails(self):
        mutations = [
            lambda c: c.update(input_sha256='different'),
            lambda c: c.update(split='test'),
            lambda c: c.update(model_sha256='different'),
            lambda c: c.update(text_mode='original'),
            lambda c: c.update(records=[]),
            lambda c: c['records'][0].update(gold=[]),
            lambda c: c['records'][0].update(predictions=[[1, 4, 1.]]),
            lambda c: c['records'][0].update(predictions=[[0, 4, float('nan')]]),
            lambda c: c['records'][0].update(predictions=[[0, 4, 1.], [0, 4, 1.]]),
        ]
        for mutate in mutations:
            with self.subTest(mutation=mutate), tempfile.TemporaryDirectory() as directory:
                paths = self.fixture(Path(directory))
                cache = json.loads(paths[1].read_text()); mutate(cache)
                paths[1].write_text(json.dumps(cache))
                with self.assertRaises((ValueError, UnicodeDecodeError)):
                    evaluate(*paths, 'general')


if __name__ == '__main__':
    unittest.main()
