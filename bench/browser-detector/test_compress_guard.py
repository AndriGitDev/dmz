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
                for recipe in ('int4','int8','mixed','int4b16','int4a32','hqq4','int8fp16'):
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

    def test_default_storage_metadata_restores_original_head_bytes(self):
        try:
            from onnx import helper, TensorProto
            from compress_frozen import preserve_head
        except ImportError:self.skipTest('optional ONNX dependencies required')
        tensor=helper.make_tensor('head.weight',TensorProto.FLOAT,[1],[1.25])
        original=tensor.SerializeToString()
        tensor.data_location=TensorProto.DEFAULT
        graph=helper.make_model(helper.make_graph([],'head',[],[],[tensor]))
        preserve_head(graph,{'head.weight':original})
        self.assertEqual(graph.graph.initializer[0].SerializeToString(),original)

    def test_head_value_location_and_missing_changes_are_rejected(self):
        try:
            from onnx import helper, TensorProto
            from compress_frozen import preserve_head
        except ImportError:self.skipTest('optional ONNX dependencies required')
        tensor=helper.make_tensor('head.weight',TensorProto.FLOAT,[1],[1.25])
        original=tensor.SerializeToString()
        for kind in ['value','location','missing']:
            with self.subTest(kind=kind):
                graph=helper.make_model(helper.make_graph([],'head',[],[],[tensor]))
                if kind=='value':graph.graph.initializer[0].float_data[0]=2.5
                elif kind=='location':graph.graph.initializer[0].data_location=TensorProto.EXTERNAL
                else:del graph.graph.initializer[:]
                with self.assertRaisesRegex(ValueError,'head initializer'):
                    preserve_head(graph,{'head.weight':original})


if __name__=='__main__':unittest.main()
