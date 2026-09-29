import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
from average_finetuned import average
from encoder import digest


@unittest.skipUnless(all(importlib.util.find_spec(name) for name in ['torch','safetensors']), 'optional encoder environment')
class AverageGuardTests(unittest.TestCase):
    def test_partial_or_tampered_checkpoint_cannot_enter_average(self):
        import torch
        from safetensors.torch import save_file
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);run=root/'run';run.mkdir();records=[]
            for epoch in (1,2):
                path=run/f'epoch-{epoch}.safetensors';save_file({'weight':torch.ones(2)},str(path))
                records.append(dict(epoch=epoch,file=path.name,sha256=digest(path),complete_epoch=epoch==1))
            meta=run/'training.json';meta.write_text(json.dumps(dict(checkpoints=records)))
            with self.assertRaisesRegex(ValueError,'complete epochs'):average(run,root/'partial')
            self.assertFalse((root/'partial').exists())
            records[1]['complete_epoch']=True;records[1]['sha256']='tampered'
            meta.write_text(json.dumps(dict(checkpoints=records)))
            with self.assertRaisesRegex(ValueError,'integrity'):average(run,root/'tampered')
            self.assertFalse((root/'tampered').exists())


if __name__=='__main__':unittest.main()
