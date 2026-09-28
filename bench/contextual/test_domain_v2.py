import hashlib
import json
from pathlib import Path
import unittest
from domain import generate
from ner import tokens_and_labels


class DomainV2Tests(unittest.TestCase):
    def test_reproducible_split_and_exact_annotations(self):
        root=Path(__file__).resolve().parents[1]/'corpus/domain/legal-financial-v2'
        manifest=json.loads((root/'manifest.json').read_text())
        rows=generate();self.assertEqual(len(rows),1120)
        inventories={}
        for split in ('train','dev'):
            selected=[r for r in rows if r['split']==split]
            file=root/(split+'.jsonl')
            self.assertEqual([json.loads(line) for line in file.read_text().splitlines()],selected)
            self.assertEqual(hashlib.sha256(file.read_bytes()).hexdigest(),manifest['splits'][split]['sha256'])
            seen=set();names=set()
            for row in selected:
                self.assertNotIn(row['text'],seen);seen.add(row['text'])
                self.assertEqual(row['review_status'],'unreviewed-synthetic')
                tokens_and_labels(row,'entities')
                for e,o in zip(row['entities'],row['original_entities']):
                    a=row['text'].encode()[e['start']:e['end']].decode()
                    b=row['original_text'].encode()[o['start']:o['end']].decode()
                    self.assertEqual(a.replace(' ',''),b.replace(' ',''))
                    if e['kind']=='Person':names.add(a.lower())
            inventories[split]=(seen,names,{r['template_family'] for r in selected})
        for train,dev in zip(inventories['train'],inventories['dev']):self.assertFalse(train & dev)


if __name__=='__main__':unittest.main()
