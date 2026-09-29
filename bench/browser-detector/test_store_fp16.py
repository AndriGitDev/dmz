import importlib.util
from pathlib import Path
import unittest

available = all(importlib.util.find_spec(name) for name in ('numpy', 'onnx', 'onnxruntime'))


@unittest.skipUnless(available, 'optional ONNX numerical dependencies required')
class StorageTest(unittest.TestCase):
    def setUp(self):
        spec = importlib.util.spec_from_file_location('store_fp16', Path(__file__).with_name('store_fp16.py'))
        self.module = importlib.util.module_from_spec(spec); spec.loader.exec_module(self.module)

    def graph(self, shared=False, value=1.123456):
        import numpy as np
        from onnx import TensorProto, helper, numpy_helper
        weights = [numpy_helper.from_array(np.array([[value]], dtype=np.float32), 'weight')]
        if not shared:
            weights.append(numpy_helper.from_array(np.array([[2.234567]], dtype=np.float32), 'head_weight'))
        nodes = [helper.make_node('MatMul', ['x', 'weight'], ['hidden'], name='/encoder/dense/MatMul'),
                 helper.make_node('MatMul', ['hidden', 'weight' if shared else 'head_weight'], ['y'], name='/head/dense/MatMul')]
        g = helper.make_graph(nodes, 'test', [helper.make_tensor_value_info('x', TensorProto.FLOAT, [1, 1])],
                              [helper.make_tensor_value_info('y', TensorProto.FLOAT, [1, 1])], weights)
        return helper.make_model(g, opset_imports=[helper.make_opsetid('', 17)], ir_version=10)

    def test_stored_rounding_with_fp32_compute_and_unchanged_head(self):
        import numpy as np
        import onnx
        import onnxruntime as ort
        graph = self.graph(); head = graph.graph.initializer[1].SerializeToString()
        self.assertEqual(self.module.convert(graph), dict(MatMul=1, Gather=0))
        self.assertEqual(graph.graph.initializer[1].SerializeToString(), head)
        cast = graph.graph.node[0]
        self.assertEqual(cast.op_type, 'Cast')
        self.assertEqual(onnx.helper.get_attribute_value(cast.attribute[0]), onnx.TensorProto.FLOAT)
        onnx.checker.check_model(graph)
        options = ort.SessionOptions(); options.intra_op_num_threads = 1; options.inter_op_num_threads = 1
        actual = ort.InferenceSession(graph.SerializeToString(), sess_options=options).run(None, {'x': np.array([[3.]], dtype=np.float32)})[0]
        expected = np.float32(3) * np.float32(np.float16(1.123456)) * np.float32(2.234567)
        np.testing.assert_allclose(actual, expected, rtol=1e-6)

    def test_shared_head_constants_are_preserved(self):
        graph = self.graph(shared=True); before = graph.SerializeToString()
        self.assertEqual(self.module.convert(graph), dict(MatMul=0, Gather=0))
        self.assertEqual(graph.SerializeToString(), before)

    def test_overflow_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'finite FP16'):
            self.module.convert(self.graph(value=70000))


if __name__ == '__main__':
    unittest.main()
