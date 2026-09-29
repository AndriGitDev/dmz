import unittest


class QuantizedRepairTests(unittest.TestCase):
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
