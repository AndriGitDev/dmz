import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from encoder import chunks, digest


class ChunkTests(unittest.TestCase):
    def test_word_boundaries_and_no_silent_truncation(self):
        tokenizer=lambda words,**kwargs: {'input_ids':list(range(2+sum(len(w) for w in words)))}
        words=['ab','cde','f','ghij']
        parts=list(chunks(words,tokenizer,limit=7))
        self.assertEqual([w for _,part in parts for w in part],words)
        for start,part in parts:self.assertEqual(words[start:start+len(part)],part)
        with self.assertRaises(ValueError):list(chunks(['abcdefgh'],tokenizer,limit=7))


@unittest.skipUnless(all(importlib.util.find_spec(m) for m in ['numpy','torch','safetensors']), 'optional encoder environment')
class EncoderIntegrationTests(unittest.TestCase):
    def test_private_cache_training_evaluation_and_integrity(self):
        import numpy as np
        from encoder import cache, train, evaluate
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            for split in ['train','dev']:
                d=root/split;d.mkdir()
                np.save(d/'x.npy',np.eye(3,dtype=np.float16).repeat(8,axis=0))
                np.save(d/'y.npy',np.arange(3,dtype=np.int8).repeat(8))
                (d/'records.jsonl').write_text(json.dumps(dict(start=0,end=24,offsets=[[i*2,i*2+1] for i in range(24)]))+'\n')
                meta=dict(split=split,text_mode='canonical',rows=1,tokens=24,hidden_size=3,input_sha256='synthetic-'+split,base_weights_sha256='synthetic-base',pooling='synthetic',files={f:digest(d/f) for f in ['x.npy','y.npy','records.jsonl']})
                (d/'metadata.json').write_text(json.dumps(meta))
            train(root/'train',root/'head',epochs=2,hidden=0,threads=1)
            result=evaluate(root/'dev',root/'head',1,[0],threads=1)
            self.assertEqual(result['sentences'],1)
            # BIO B labels each start a new span; final B plus following I is one.
            self.assertEqual(result['scores'][0]['tp']+result['scores'][0]['fn'],8)
            from ensemble import neural_cache
            single=neural_cache(root/'dev',root/'head',1)
            averaged=neural_cache(root/'dev',root/'head',[1,2])
            self.assertEqual(single['records'][0]['gold'],averaged['records'][0]['gold'])
            self.assertEqual(averaged['composition']['epochs'],[1,2])
            self.assertNotEqual(single['model_sha256'],averaged['model_sha256'])
            with self.assertRaises(ValueError):neural_cache(root/'dev',root/'head',[1,1])
            with self.assertRaises(ValueError):cache(root/'dev','train')
            with (root/'dev/y.npy').open('ab') as f:f.write(b'corrupt')
            with self.assertRaises(ValueError):cache(root/'dev','dev')


if __name__=='__main__':unittest.main()
