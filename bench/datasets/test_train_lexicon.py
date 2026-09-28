import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from train_lexicon import train

class LexiconTrainingTests(unittest.TestCase):
    def test_training_only_and_ambiguity_filter(self):
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp)
            source,addresses,out=root/'train.jsonl',root/'addresses.json',root/'pack.json'
            addresses.write_text(json.dumps(['Gervi  Slóð','Gervigata','Gervigata 99']))
            person=lambda t: dict(split='train',text=t,entities=[dict(kind='Person',start=0,end=len(t.encode()))])
            rows=[person('Mira Demo'),person('Mira Demo'),person('Gervi'),dict(split='train',text='Gervi',entities=[])]
            source.write_text(''.join(json.dumps(r)+'\n' for r in rows))
            with contextlib.redirect_stdout(io.StringIO()):
                train(source,addresses,out,minimum=1)
            pack=json.loads(out.read_text())
            self.assertEqual(pack['names'],['Mira Demo'])
            self.assertEqual(pack['addresses'],['Gervi Slóð','Gervigata'])
            source.write_text(json.dumps(dict(split='test',text='Do not learn this',entities=[]))+'\n')
            with self.assertRaises(ValueError):
                train(source,addresses,out)

class BoundaryTrainingTests(unittest.TestCase):
    def test_sentence_end_counts_and_component_evidence(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source, addresses, out = root/'train.jsonl', root/'addresses.json', root/'pack.json'
            addresses.write_text('[]')
            def row(text, value):
                start = text.encode().index(value.encode())
                return dict(split='train', text=text, entities=[dict(kind='Person', start=start, end=start+len(value.encode()))])
            rows = [row('Mira', 'Mira'), row('Mira', 'Mira'), row('Prófa Gervi', 'Prófa Gervi'), row('Prófa Gervi', 'Prófa Gervi'), dict(split='train', text='Gervi', entities=[])]
            source.write_text(''.join(json.dumps(r)+'\n' for r in rows))
            with contextlib.redirect_stdout(io.StringIO()):
                train(source, addresses, out, minimum=2, part_minimum=2, part_purity=.95)
            pack = json.loads(out.read_text())
            # The longer candidate must not multiply the last single token's denominator.
            self.assertEqual(pack['names'], ['Mira', 'Prófa Gervi'])
            # Components can be learned inside full names without becoming standalone detections.
            self.assertEqual(pack['name_parts'], ['Mira', 'Prófa'])

class SeedTrainingTests(unittest.TestCase):
    def test_seeds_count_components_and_exclude_ambiguous_words(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            source, addresses, out = root/'train.jsonl', root/'addresses.json', root/'pack.json'
            addresses.write_text('[]')
            rows = [dict(split='train', text='Mira Gervi', entities=[dict(kind='Person',start=0,end=len('Mira Gervi'.encode()))]) for _ in range(3)]
            rows.append(dict(split='train', text='Gervi', entities=[]))
            source.write_text(''.join(json.dumps(r)+'\n' for r in rows))
            with contextlib.redirect_stdout(io.StringIO()):
                train(source, addresses, out, seed_minimum=3, seed_purity=.99)
            pack = json.loads(out.read_text())
            self.assertNotIn('Mira', pack['names'])
            self.assertEqual(pack['name_seeds'], ['Mira'])

if __name__=='__main__': unittest.main()
