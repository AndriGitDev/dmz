import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

from finetune import prepare, tensors, warm_start_record
from encoder import digest


class Encoded(dict):
    def word_ids(self):
        return [None, 0, 0, 1, None]


class FineTuneTests(unittest.TestCase):
    def test_warm_start_rejects_partial_tampered_or_different_training(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory); checkpoint=root/'epoch.safetensors'
            checkpoint.write_bytes(b'checkpoint'); source=root/'train.jsonl'; source.write_bytes(b'train')
            base=root/'base'; base.mkdir(); (base/'model.safetensors').write_bytes(b'base')
            report=dict(split='train',input_sha256=digest(source),
                base_weights_sha256=digest(base/'model.safetensors'),
                checkpoints=[dict(epoch=2,sha256=digest(checkpoint),complete_epoch=True)])
            manifest=root/'training.json'; manifest.write_text(json.dumps(report))
            self.assertEqual(warm_start_record(checkpoint,source,base)['source_epoch'],2)
            report['checkpoints'][0]['complete_epoch']=False; manifest.write_text(json.dumps(report))
            with self.assertRaisesRegex(ValueError,'complete checkpoint'):warm_start_record(checkpoint,source,base)
            report['checkpoints'][0]['complete_epoch']=True; manifest.write_text(json.dumps(report))
            source.write_bytes(b'different train')
            with self.assertRaisesRegex(ValueError,'input mismatch'):warm_start_record(checkpoint,source,base)
            source.write_bytes(b'train'); checkpoint.write_bytes(b'tampered')
            with self.assertRaisesRegex(ValueError,'complete checkpoint'):warm_start_record(checkpoint,source,base)

    @unittest.skipUnless(importlib.util.find_spec('torch'), 'optional encoder environment')
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
            row.update(original_text='Ása\n  Jóns',original_entities=[dict(start=0,end=12,kind='Person')])
            path.write_text(json.dumps(row)+'\n')
            records,jobs=prepare(path,'dev',tokenizer,text_mode='original')
            self.assertEqual(records[0]['offsets'],[(0,4),(7,12)])
            self.assertEqual(jobs[0]['labels'],[1,2])


if __name__=='__main__':unittest.main()
