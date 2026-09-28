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

if __name__=='__main__': unittest.main()
