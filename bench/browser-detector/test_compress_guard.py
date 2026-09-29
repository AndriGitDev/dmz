import hashlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest


class CompressionGuardTests(unittest.TestCase):
    def test_unfrozen_and_changed_references_rejected_before_compression(self):
        script=Path(__file__).with_name('compress_frozen.py')
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);selection=root/'selected.json';model=root/'fp32.onnx'
            model.write_bytes(b'fabricated graph placeholder')
            def run(expected):
                for recipe in ('int4','int8','mixed'):
                    result=subprocess.run([sys.executable,str(script),'--assets',str(root),
                        '--selection',str(selection),'--recipe',recipe],capture_output=True,text=True)
                    self.assertNotEqual(result.returncode,0)
                    self.assertIn(expected,result.stderr)
                    self.assertFalse((root/(recipe+'.onnx')).exists())
            selection.write_text(json.dumps(dict(frozen=False,thresholds=dict(crf=.75,neural=0))))
            (root/'export.json').write_text('{}')
            run('must be frozen')
            selection.write_text(json.dumps(dict(frozen=True,thresholds=dict(crf=.75,neural=0))))
            (root/'export.json').write_text(json.dumps(dict(selection_sha256='stale')))
            run('selection changed')
            (root/'export.json').write_text(json.dumps(dict(
                selection_sha256=hashlib.sha256(selection.read_bytes()).hexdigest(),model_sha256='stale')))
            run('export changed')
            self.assertEqual(model.read_bytes(),b'fabricated graph placeholder')


if __name__=='__main__':unittest.main()
