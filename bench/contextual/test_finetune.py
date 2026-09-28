import json
from pathlib import Path
import tempfile
import unittest

from finetune import prepare, tensors


class Encoded(dict):
    def word_ids(self):
        return [None, 0, 0, 1, None]


class FineTuneTests(unittest.TestCase):
    def test_pooling_padding_and_masked_targets(self):
        import torch
        jobs=[dict(ids=[0,4,5,6,2],word_ids=[None,0,0,1,None],words=2,labels=[1,2]),
              dict(ids=[0,7,2],word_ids=[None,0,None],words=1,labels=[0])]
        ids,mask,pool,target=tensors(jobs,1)
        self.assertEqual(ids.tolist()[1],[0,7,2,1,1])
        self.assertEqual(mask.tolist()[1],[1,1,1,0,0])
        self.assertEqual(target.tolist(),[[1,2],[0,-100]])
        hidden=torch.arange(10,dtype=torch.float32).reshape(2,5,1)
        self.assertEqual((pool@hidden).squeeze(-1).tolist(),[[1.5,3.0],[6.0,0.0]])

    def test_training_rejects_development_and_preserves_utf8(self):
        row=dict(split='dev',text='Ása Jóns',entities=[dict(start=0,end=10,kind='Person')])
        tokenizer=lambda words,**kwargs:Encoded(input_ids=[0,4,5,6,2])
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'sample.jsonl';path.write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(ValueError,'split'):
                prepare(path,'train',tokenizer)
            records,jobs=prepare(path,'dev',tokenizer)
            self.assertEqual(records[0]['offsets'],[(0,4),(5,10)])
            self.assertEqual(jobs[0]['labels'],[1,2])


if __name__=='__main__':unittest.main()
