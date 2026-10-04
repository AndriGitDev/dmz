import unittest
import json
from pathlib import Path
from tempfile import TemporaryDirectory

from split_review_pool import clusters, freeze, sha


class ClusterSplitTests(unittest.TestCase):
    def fixture(self, root):
        source = root/'source'; source.mkdir()
        text = 'Þetta er alveg tilbúið dæmi um gögn sem eru geymd í einkamöppu fyrir ólokna mannlega yfirferð.'
        data = text.encode()
        (source/'sample.txt').write_bytes(data)
        row = dict(document_id='sample', page=1, text=text, text_sha256=sha(data),
                   document_byte_start=0, document_byte_end=len(data), entities=None,
                   reviewed=False, stratum='general')
        review = (json.dumps(row)+'\n').encode(); (source/'review.jsonl').write_bytes(review)
        manifest = dict(model_evaluated=False, training_use=False, review_sha256=sha(review),
                        documents=[dict(id='sample', text_sha256=sha(data), overlapping_reference_12grams=0)])
        (source/'manifest.json').write_text(json.dumps(manifest))
        return source

    def test_frozen_review_is_unlabelled_and_not_training_data(self):
        with TemporaryDirectory() as directory:
            root = Path(directory); source = self.fixture(root); output = root/'output'
            report = freeze(source, output)
            self.assertFalse(report['independent_final_evaluation'])
            self.assertFalse(report['training_use'])
            self.assertEqual(sum(s['documents'] for s in report['splits'].values()), 1)
            row = json.loads(next(output.glob('*-review.jsonl')).read_text())
            self.assertIsNone(row['entities'])
            self.assertTrue(row['split'].endswith('-review'))
            self.assertNotIn('Þetta', json.dumps(report, ensure_ascii=False))

    def test_changed_or_model_exposed_collection_rejected(self):
        for change in ('text', 'model'):
            with self.subTest(change=change), TemporaryDirectory() as directory:
                root = Path(directory); source = self.fixture(root)
                if change == 'text': (source/'sample.txt').write_text('changed')
                else:
                    path=source/'manifest.json'; manifest=json.loads(path.read_text())
                    manifest['model_evaluated']=True; path.write_text(json.dumps(manifest))
                with self.assertRaises(ValueError): freeze(source, root/'output')

    def test_transitive_related_articles_share_one_split(self):
        documents = dict(a={'x', 'y'}, b={'y', 'z'}, c={'z', 'w'}, d={'separate'})
        result = clusters(documents, .5)
        self.assertEqual(result['a'], result['b'])
        self.assertEqual(result['b'], result['c'])
        self.assertNotEqual(result['a']['cluster_sha256'], result['d']['cluster_sha256'])

    def test_order_independent_and_bounded_configuration(self):
        documents = dict(a={'x'}, b={'y'}, c={'z'})
        self.assertEqual(clusters(documents), clusters(dict(reversed(list(documents.items())))))
        with self.assertRaises(ValueError): clusters(documents, 0)


if __name__ == '__main__': unittest.main()
