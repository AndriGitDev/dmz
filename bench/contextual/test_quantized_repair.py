import unittest


class QuantizedRepairTests(unittest.TestCase):
    def test_repair_rejects_nontraining_input_and_changed_checkpoint(self):
        import hashlib
        import json
        from pathlib import Path
        import tempfile
        from quantized_repair import identities
        def sha(path):return hashlib.sha256(path.read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);base=root/'base';assets=root/'assets';run=root/'run'
            for path in [base,assets,run]:path.mkdir()
            checkpoint=run/'epoch-2.safetensors';checkpoint.write_bytes(b'frozen checkpoint')
            (base/'model.safetensors').write_bytes(b'base')
            (assets/'int4.onnx').write_bytes(b'quantized graph')
            (assets/'fp32.onnx').write_bytes(b'frozen FP32 graph')
            train=root/'train.jsonl';train.write_bytes(b'training-only records')
            dev=root/'dev.jsonl';dev.write_bytes(b'evaluation-only records')
            manifest=run/'training.json';manifest.write_text(json.dumps(dict(split='train',input_sha256=sha(train))))
            selected=root/'selection.json';selected.write_text(json.dumps(dict(frozen=True,thresholds=dict(crf=.75,neural=0),
                candidate=dict(model_sha256=sha(checkpoint)),encoder=dict(base_weights_sha256=sha(base/'model.safetensors'),tokenizer_files={}),
                provenance=dict(training_sha256=sha(manifest)))))
            (assets/'export.json').write_text(json.dumps(dict(selection_sha256=sha(selected),model_sha256=sha(assets/'fp32.onnx'))))
            (assets/'int4-compression.json').write_text(json.dumps(dict(selection_sha256=sha(selected),recipe='int4',reference_sha256=sha(assets/'fp32.onnx'),sha256=sha(assets/'int4.onnx'))))
            self.assertEqual(identities(assets,selected,checkpoint,base,train)['input_sha256'],sha(train))
            with self.assertRaisesRegex(ValueError,'original training mixture'):identities(assets,selected,checkpoint,base,dev)
            checkpoint.write_bytes(b'changed checkpoint')
            with self.assertRaisesRegex(ValueError,'model identity mismatch'):identities(assets,selected,checkpoint,base,train)

    def test_packed_decoder_matches_real_ort_kernel(self):
        try:
            import numpy as np
            import onnx
            import onnxruntime as ort
            from onnxruntime.quantization.matmul_nbits_quantizer import MatMulNBitsQuantizer
            from quantized_repair import dequantized_weight
        except ImportError:self.skipTest('optional encoder/ONNX environment required')
        weight=np.random.default_rng(44).normal(size=(32,3)).astype(np.float32)
        graph=onnx.helper.make_graph([onnx.helper.make_node('MatMul',['x','w'],['y'],name='/linear/MatMul')],
            'probe',[onnx.helper.make_tensor_value_info('x',onnx.TensorProto.FLOAT,[32,32])],
            [onnx.helper.make_tensor_value_info('y',onnx.TensorProto.FLOAT,[32,3])],[onnx.numpy_helper.from_array(weight,'w')])
        model=onnx.helper.make_model(graph,opset_imports=[onnx.helper.make_opsetid('',17)]);model.ir_version=10
        for symmetric in [True,False]:
            quantizer=MatMulNBitsQuantizer(onnx.ModelProto.FromString(model.SerializeToString()),block_size=32,is_symmetric=symmetric,accuracy_level=1)
            quantizer.process();quant=quantizer.model.model
            node=next(n for n in quant.graph.node if n.op_type=='MatMulNBits')
            decoded=dequantized_weight(node,{v.name:v for v in quant.graph.initializer})
            options=ort.SessionOptions();options.intra_op_num_threads=1
            session=ort.InferenceSession(quant.SerializeToString(),sess_options=options,providers=['CPUExecutionProvider'])
            actual=session.run(None,{'x':np.eye(32,dtype=np.float32)})[0]
            np.testing.assert_allclose(decoded.T,actual,rtol=1e-6,atol=1e-6)


if __name__=='__main__':unittest.main()
