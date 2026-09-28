import json
from pathlib import Path
import tempfile
import unittest
from ner import external, features, rows, spans, tokens_and_labels

class ContextualTests(unittest.TestCase):
    def test_utf8_and_adjacent_names(self):
        text = 'Ása Gervi og Þóra'
        raw = text.encode()
        row = dict(text=text, entities=[dict(kind='Person',start=0,end=len('Ása Gervi'.encode())),dict(kind='Person',start=raw.index('Þóra'.encode()),end=len(raw))])
        tokens,offsets,labels=tokens_and_labels(row)
        self.assertEqual(labels,['B','I','O','B'])
        self.assertEqual([(a,b) for a,b,_,_ in spans(labels,offsets)],[(0,len('Ása Gervi'.encode())),(raw.index('Þóra'.encode()),len(raw))])
        self.assertEqual(len(features(tokens)),len(tokens))
        self.assertIn('0:suffix3=ása',features(tokens)[0])

    def test_partial_is_not_exact_and_b_starts_new_entity(self):
        offsets=[(0,4),(5,10)]
        gold={(a,b) for a,b,_,_ in spans(['B','I'],offsets)}
        partial={(a,b) for a,b,_,_ in spans(['B','O'],offsets)}
        self.assertEqual(len(gold & partial),0)
        self.assertEqual(len(gold-partial),1)
        self.assertEqual(len(partial-gold),1)
        self.assertEqual(len(spans(['B','B'],offsets)),2)
        self.assertEqual(spans(['I','O'],offsets),[(0,4,0,1)])

    def test_split_and_repository_boundaries(self):
        with tempfile.TemporaryDirectory() as tmp:
            p=Path(tmp)/'rows.jsonl'
            p.write_text(json.dumps(dict(split='dev',text='Gervi',entities=[]))+'\n')
            with self.assertRaises(ValueError):list(rows(p,'train'))
            self.assertEqual(len(list(rows(p,'dev'))),1)
        with self.assertRaises(ValueError):external(Path(__file__))
        with self.assertRaises(ValueError):tokens_and_labels(dict(text='Ása',entities=[dict(kind='Person',start=1,end=4)]))
        with self.assertRaises(ValueError):tokens_and_labels(dict(text='Mira  Gervi',entities=[]))

if __name__=='__main__':unittest.main()
