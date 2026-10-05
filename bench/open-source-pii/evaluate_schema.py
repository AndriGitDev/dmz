"""One predeclared GLiNER2 full_name schema trial. Native, offline, synthetic only."""
import argparse,json,time,hashlib
from pathlib import Path
from evaluate import ROOT,load_corpus,private_output,digest,offline,runner
from quality import score

def main():
 p=argparse.ArgumentParser();p.add_argument('--models',type=Path,required=True);p.add_argument('--corpus',type=Path,default=ROOT/'bench/corpus/open-source-pii-v1');p.add_argument('--output',required=True);a=p.parse_args();out=private_output(a.output)
 rows,manifest=load_corpus(a.corpus);registry=ROOT/'bench/open-source-pii/candidates.json';config=json.loads(registry.read_text())['candidates']['gliner2'];config['mapping']={'full_name':'name_demo'}|{k:v for k,v in config['mapping'].items() if k!='person'}
 model_dir=a.models/config['model'].replace('/','--');identity=json.loads((model_dir/'download-manifest.json').read_text());assert identity['revision']==config['revision']
 for name,entry in identity['files'].items():
  if digest(model_dir/name)!=entry['sha256']:raise ValueError('model changed')
 paths=[Path(__file__),registry,ROOT/'bench/open-source-pii/evaluate.py',ROOT/'bench/contextual/quality.py'];sources={str(p.relative_to(ROOT)):digest(p) for p in paths}
 offline();start=time.monotonic();predict=runner('gliner2',config,model_dir);load_seconds=time.monotonic()-start;records=[];excluded={}
 import torch
 with torch.inference_mode():
  for row in rows:
   predicted=[];scored=[]
   for e in predict(row['text']):
    kind=config['mapping'].get(e['label'])
    if not kind:excluded[e['label']]=excluded.get(e['label'],0)+1;continue
    s,t=int(e['start']),int(e['end']);text=row['text']
    if not 0<=s<t<=len(text):raise ValueError('invalid offsets')
    span=[len(text[:s].encode()),len(text[:t].encode()),kind]
    if span not in predicted:predicted.append(span)
    scored.append({'span':span,'score':float(e['score'])})
   records.append({'id':row['id'],'stratum':row['stratum'],'text_sha256':hashlib.sha256(row['text'].encode()).hexdigest(),'gold':row['gold'],'predicted':predicted,'scored':scored})
 for path in paths:
  if digest(path)!=sources[str(path.relative_to(ROOT))]:raise ValueError('measurement changed')
 result={'schema':1,'status':'complete','candidate':'gliner2-full-name','config':config,'model':identity,'corpus':manifest,'measurement_sha256':sources,'network':'socket connections denied for model loading/inference','load_seconds':load_seconds,'inference_seconds':time.monotonic()-start-load_seconds,'independent_validation':False,'excluded_labels':excluded,'quality':score(records,manifest['scope']),'records':records}
 with out.open('x') as f:json.dump(result,f,indent=2)
 print(json.dumps({k:result[k] for k in ['candidate','load_seconds','inference_seconds','quality']}),flush=True)
if __name__=='__main__':main()
