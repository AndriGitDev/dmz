"""Resource-bounded, offline size trial for the predeclared GLiNER2 export."""
import argparse,json,os,threading,time
from pathlib import Path
from evaluate import private_output,digest,offline

def bounded_arrays():
 """Bound temporary arrays in ORT's elementwise UINT8 conversion, not its math."""
 import numpy as np
 from onnx import TensorProto
 from onnxruntime.quantization import quant_utils,base_quantizer
 original=quant_utils.quantize_nparray
 chunk_size=4*1024*1024
 def convert(qtype,arr,scale,zero_point,low=None,high=None,*,chunk=chunk_size):
  if qtype!=TensorProto.UINT8 or np.size(scale)!=1 or np.size(zero_point)!=1 or arr.size<=chunk:
   return original(qtype,arr,scale,zero_point,low,high)
  flat=arr.reshape(-1);result=np.empty(flat.shape,dtype=np.uint8)
  for start in range(0,len(flat),chunk):
   result[start:start+chunk]=original(qtype,flat[start:start+chunk],scale,zero_point,low,high)
  return result.reshape(arr.shape)
 # Compare the exact primitive across chunk boundaries, clipping and half ties.
 rng=np.random.default_rng(412)
 checks=0
 for arr in [rng.normal(size=(101,103)).astype(np.float32),np.arange(-1024,1024,dtype=np.float32)/2]:
  for scale in [np.float32(.01),np.float32(.5),np.float32(1.)]:
   for zero in [np.uint8(0),np.uint8(128),np.uint8(255)]:
    for low,high in [(None,None),(8,247)]:
     expected=original(TensorProto.UINT8,arr,scale,zero,low,high)
     actual=convert(TensorProto.UINT8,arr,scale,zero,low,high,chunk=127)
     if not np.array_equal(expected,actual):raise ValueError('chunked conversion differs')
     checks+=1
 quant_utils.quantize_nparray=convert
 base_quantizer.quantize_nparray=convert
 return {'chunk_elements':chunk_size,'primitive_parity_checks':checks,'scope':'scalar-scale UINT8 conversion only; original ORT operation per chunk'}

def main():
 p=argparse.ArgumentParser();p.add_argument('--export',type=Path,required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--bounded-arrays',action='store_true');a=p.parse_args();a.output=private_output(str(a.output));a.output.mkdir();offline()
 stop=threading.Event()
 def memory_guard():
  while not stop.wait(.25):
   available=next(int(line.split()[1]) for line in Path('/proc/meminfo').read_text().splitlines() if line.startswith('MemAvailable:'))
   if available<900*1024:
    print('Quantization stopped: available host memory below 900 MiB',flush=True);os._exit(75)
 threading.Thread(target=memory_guard,daemon=True).start()
 import onnxruntime as ort
 from onnxruntime.quantization import quantize_dynamic,QuantType
 result={'schema':1,'onnxruntime':ort.__version__,'quantization':{'weight_type':'QUInt8','per_channel':True,'op_types':['MatMul','Gemm','Gather']},'files':{}}
 if a.bounded_arrays:result['bounded_arrays']=bounded_arrays()
 result['measurement_sha256']=digest(Path(__file__))
 try:
  for component in ['encoder','classifier','span_rep','count_embed']:
   src=a.export/'onnx'/(component+'.onnx');dst=a.output/(component+'.onnx');before=digest(src);start=time.monotonic()
   quantize_dynamic(str(src),str(dst),op_types_to_quantize=['MatMul','Gemm','Gather'],per_channel=True,weight_type=QuantType.QUInt8)
   if digest(src)!=before:raise ValueError('source changed')
   result['files'][component]={'source_sha256':before,'source_bytes':src.stat().st_size,'sha256':digest(dst),'bytes':dst.stat().st_size,'seconds':time.monotonic()-start}
   print(json.dumps({component:result['files'][component]}),flush=True)
  (a.output/'manifest.json').write_text(json.dumps(result,indent=2)+'\n')
 finally:stop.set()
if __name__=='__main__':main()
