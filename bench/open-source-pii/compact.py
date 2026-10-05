"""Pinned compact ONNX PII inference. All assets and inference stay local."""
import argparse,json,time,hashlib
from pathlib import Path
from evaluate import ROOT,digest,private_output,load_corpus,offline
from quality import score

CONFIG=ROOT/'bench/open-source-pii/compact-candidates.json'

def load(models,variant):
 import numpy as np
 import onnxruntime as ort
 from transformers import AutoTokenizer
 config=json.loads(CONFIG.read_text());path=models/config['model'].replace('/','--')
 identity=json.loads((path/'download-manifest.json').read_text())
 if identity['revision']!=config['revision']:raise ValueError('wrong revision')
 for name,entry in identity['files'].items():
  if digest(path/name)!=entry['sha256']:raise ValueError('changed model asset')
 opts=ort.SessionOptions();opts.intra_op_num_threads=1;opts.inter_op_num_threads=1
 session=ort.InferenceSession(str(path/config['variants'][variant]),sess_options=opts,providers=['CPUExecutionProvider'])
 tokenizer=AutoTokenizer.from_pretrained(path,local_files_only=True)
 labels=json.loads((path/'config.json').read_text())['id2label']
 def predict(text):
  encoded=tokenizer(text,return_tensors='np',return_offsets_mapping=True,return_overflowing_tokens=True,truncation=True,max_length=512,stride=64,padding=True)
  result={}
  for chunk,offsets in enumerate(encoded['offset_mapping']):
   feeds={i.name:encoded[i.name][chunk:chunk+1] for i in session.get_inputs()}
   logits=session.run(None,feeds)[0][0];ids=logits.argmax(-1)
   probs=np.exp(logits-logits.max(-1,keepdims=True));probs/=probs.sum(-1,keepdims=True)
   current=None
   def flush():
    nonlocal current
    if current:
     s,e,label,scores=current;key=(s,e,label);item={'start':s,'end':e,'label':label,'score':sum(scores)/len(scores)}
     if key not in result or result[key]['score']<item['score']:result[key]=item
    current=None
   for i,((s,e),label_id) in enumerate(zip(offsets,ids,strict=True)):
    s,e=int(s),int(e);tag=labels[str(int(label_id))]
    if s==e or tag=='O':flush();continue
    prefix,label=tag.split('-',1);confidence=float(probs[i,label_id])
    if current and prefix=='I' and current[2]==label:
     current[1]=e;current[3].append(confidence)
    else:
     flush();current=[s,e,label,[confidence]]
   flush()
  return list(result.values())
 return predict,config,identity

def main():
 p=argparse.ArgumentParser();p.add_argument('--variant',required=True,choices=['gravitee-fp32','gravitee-int8']);p.add_argument('--models',type=Path,required=True);p.add_argument('--output',required=True);p.add_argument('--corpus',type=Path,default=ROOT/'bench/corpus/open-source-pii-v1');a=p.parse_args()
 out=private_output(a.output);rows,manifest=load_corpus(a.corpus)
 paths=[Path(__file__),CONFIG,ROOT/'bench/contextual/quality.py',ROOT/'bench/open-source-pii/evaluate.py'];sources={str(p.relative_to(ROOT)):digest(p) for p in paths}
 offline();started=time.monotonic();predict,config,identity=load(a.models,a.variant);load_seconds=time.monotonic()-started;records=[];excluded={}
 for row in rows:
  raw=[];scored=[]
  for item in predict(row['text']):
   if item['label'] not in config['mapping']:excluded[item['label']]=excluded.get(item['label'],0)+1;continue
   s,e=item['start'],item['end'];text=row['text']
   if not 0<=s<e<=len(text):raise ValueError('bad span')
   span=[len(text[:s].encode()),len(text[:e].encode()),config['mapping'][item['label']]]
   if span not in raw:raw.append(span)
   scored.append({'span':span,'score':item['score']})
  records.append({'id':row['id'],'stratum':row['stratum'],'text_sha256':hashlib.sha256(row['text'].encode()).hexdigest(),'gold':row['gold'],'predicted':raw,'scored':scored})
 if any(digest(p)!=sources[str(p.relative_to(ROOT))] for p in paths):raise ValueError('measurement changed')
 result={'schema':1,'status':'complete','candidate':a.variant,'config':config,'model':identity,'corpus':manifest,'measurement_sha256':sources,'network':'socket connections denied for loading and inference','load_seconds':load_seconds,'inference_seconds':time.monotonic()-started-load_seconds,'excluded_labels':excluded,'independent_validation':False,'quality':score(records,manifest['scope']),'records':records}
 with out.open('x') as f:json.dump(result,f,indent=2)
 print(json.dumps({k:result[k] for k in ['candidate','quality','load_seconds','inference_seconds']}),flush=True)
if __name__=='__main__':main()
