import sys
from pathlib import Path
import unittest
sys.path.insert(0,str(Path(__file__).resolve().parents[1]/'contextual'))


class RepairExportTests(unittest.TestCase):
    def test_zero_corrections_preserve_output_and_head_replacement_takes_effect(self):
        try:
            import numpy as np
            import onnx
            import onnxruntime as ort
            import torch
            from export_repair import attach
        except ImportError:self.skipTest('optional encoder/ONNX environment required')
        helper=onnx.helper;array=onnx.numpy_helper.from_array;rng=np.random.default_rng(7)
        nodes=[];initializers=[];state={}
        for i in range(72):
            name=f'/encoder/layer.{i}/MatMul_Q4';prefix=f'encoder.layer.{i}'
            initializers.extend([array(np.full((3,1,16),0x97,dtype=np.uint8),f'w{i}'),array(np.full((3,1),.1,dtype=np.float32),f's{i}')])
            nodes.append(helper.make_node('MatMulNBits',['x',f'w{i}',f's{i}'],[f'v{i}'],name=name,domain='com.microsoft',K=32,N=3,bits=4,block_size=32,accuracy_level=1))
            state[prefix+'.adapter_a.weight']=torch.from_numpy(rng.normal(size=(16,32)).astype(np.float32))
            state[prefix+'.adapter_b.weight']=torch.zeros(3,16)
        head={
            'head.0.weight':np.ones(3,dtype=np.float32),'head.0.bias':np.zeros(3,dtype=np.float32),
            'head.1.weight':rng.normal(size=(4,3)).astype(np.float32),'head.1.bias':rng.normal(size=4).astype(np.float32),
            'head.4.weight':rng.normal(size=(3,4)).astype(np.float32),'head.4.bias':rng.normal(size=3).astype(np.float32)}
        for key,value in head.items():
            state[key]=torch.from_numpy(value.copy())
            initializers.append(array(value.T.copy() if key in ('head.1.weight','head.4.weight') else value,key))
        nodes.extend([
            helper.make_node('LayerNormalization',['v71','head.0.weight','head.0.bias'],['n'],name='/head/head.0/LayerNormalization',axis=-1),
            helper.make_node('MatMul',['n','head.1.weight'],['a'],name='/head/head.1/MatMul'),
            helper.make_node('Add',['a','head.1.bias'],['b'],name='/head/head.1/Add'),
            helper.make_node('MatMul',['b','head.4.weight'],['c'],name='/head/head.4/MatMul'),
            helper.make_node('Add',['c','head.4.bias'],['y'],name='/head/head.4/Add')])
        graph=helper.make_graph(nodes,'repair-test',[helper.make_tensor_value_info('x',onnx.TensorProto.FLOAT,[1,32])],[helper.make_tensor_value_info('y',onnx.TensorProto.FLOAT,[1,3])],initializers)
        model=helper.make_model(graph,opset_imports=[helper.make_opsetid('',17),helper.make_opsetid('com.microsoft',1)]);model.ir_version=10
        options=ort.SessionOptions();options.intra_op_num_threads=1;options.log_severity_level=3
        feed={'x':rng.normal(size=(1,32)).astype(np.float32)}
        def run(m):return ort.InferenceSession(m.SerializeToString(),sess_options=options,providers=['CPUExecutionProvider']).run(None,feed)[0]
        expected=run(model);original=model.SerializeToString();details=attach(model,state)
        self.assertFalse(details['backbone_requantized']);self.assertEqual(details['adapter_matrices'],144)
        onnx.checker.check_model(model);np.testing.assert_allclose(run(model),expected,rtol=1e-6,atol=1e-6)
        changed=onnx.ModelProto.FromString(original);state['head.4.bias']+=1;attach(changed,state)
        np.testing.assert_allclose(run(changed),expected+1,rtol=1e-6,atol=1e-6)


if __name__=='__main__':unittest.main()
