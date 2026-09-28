import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec=importlib.util.spec_from_file_location('review',Path(__file__).with_name('review.py'))
review=importlib.util.module_from_spec(spec);spec.loader.exec_module(review)


class ReviewTests(unittest.TestCase):
    def test_null_review_integrity_and_utf8(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);source=root/'source.jsonl';packet=root/'packet'
            text='Ása kom.'
            source.write_text(json.dumps(dict(document_id='fabricated',page=1,text=text,
                text_sha256=review.sha(text.encode()),entities=None,reviewed=False,
                overlapping_mim_12grams=0))+'\n')
            self.assertEqual(review.prepare([source],packet)['pages'],1)
            with self.assertRaisesRegex(ValueError,'incomplete'):
                review.validate(packet)
            path=packet/'annotations.jsonl';row=json.loads(path.read_text())
            row.update(reviewed=True,reviewer='test-only',extraction_verified=True)
            path.write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(ValueError,'null'):
                review.validate(packet)
            row['entities']=[dict(start=0,end=4,kind='Person')]
            path.write_text(json.dumps(row)+'\n')
            self.assertEqual(review.validate(packet)['names'],1)
            row['entities'][0]['start']=1;path.write_text(json.dumps(row)+'\n')
            with self.assertRaises(UnicodeDecodeError):review.validate(packet)
            row['entities']=[];path.write_text(json.dumps(row)+'\n')
            self.assertEqual(review.validate(packet)['names'],0)
            (packet/row['text_file']).write_text('changed')
            with self.assertRaisesRegex(ValueError,'changed'):review.validate(packet)


if __name__=='__main__':unittest.main()
