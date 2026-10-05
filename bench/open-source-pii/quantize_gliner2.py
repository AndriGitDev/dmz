"""Resource-bounded, offline size trial for the predeclared GLiNER2 export."""
import argparse,json,os,threading,time
from pathlib import Path
from evaluate import private_output,digest,offline

def main():
 p=argparse.ArgumentParser();p.add_argument('--export',type=Path,required=True);p.add_argument('--output',type=Path,required=True);a=p.parse_args();a.output=private_output(str(a.output));a.output.mkdir();offline()
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
