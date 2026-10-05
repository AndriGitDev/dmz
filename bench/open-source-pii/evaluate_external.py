"""Offline candidate guard for one pinned public synthetic English projection."""
import argparse,hashlib,json,resource,time
from pathlib import Path
from evaluate import ROOT,private_output,digest,offline,runner
from quality import score
TEST_SHA='6b70350932be463af6b53fc79bee5e53be2e66cc4133ae9bd0ff672d2237ad29'

def main():
 p=argparse.ArgumentParser();p.add_argument('--candidate',required=True,choices=['presidio','bert','gliner','gliner2','opf']);p.add_argument('--models',type=Path,required=True);p.add_argument('--corpus',type=Path,required=True);p.add_argument('--output',required=True);p.add_argument('--max-seconds',type=int,default=18000);a=p.parse_args()
 out=private_output(a.output);partial=private_output(str(out)+'.partial.jsonl');manifest=json.loads((a.corpus/'manifest.json').read_text())
 if manifest['source']['revision']!='e06eb1499ca8d54470f085021cd8e54f9efac7fd' or manifest['sets']['test']['sha256']!=TEST_SHA or digest(a.corpus/'test.jsonl')!=TEST_SHA:raise ValueError('wrong public projection')
 rows=[json.loads(line) for line in (a.corpus/'test.jsonl').read_text().splitlines()];config=json.loads((ROOT/'bench/open-source-pii/candidates.json').read_text())['candidates'][a.candidate]
 model_dir=a.models/config['model'].replace('/','--');identity={'model':config['model']}
 if a.candidate!='presidio':
  identity=json.loads((model_dir/'download-manifest.json').read_text());assert identity['revision']==config['revision']
  for name,entry in identity['files'].items():
   if digest(model_dir/name)!=entry['sha256']:raise ValueError('changed model')
 paths=[Path(__file__),ROOT/'bench/open-source-pii/evaluate.py',ROOT/'bench/open-source-pii/candidates.json',ROOT/'bench/contextual/quality.py'];sources={str(p.relative_to(ROOT)):digest(p) for p in paths}
 offline();start=time.monotonic();predict=runner(a.candidate,config,model_dir);load_seconds=time.monotonic()-start;records=[];status='complete';failures={};excluded={}
 import torch
 with partial.open('x') as log,torch.inference_mode():
  for row in rows:
   if time.monotonic()-start>a.max_seconds:status='time_limit';break
   if hashlib.sha256(row['text'].encode()).hexdigest()!=row['text_sha256']:raise ValueError('changed row')
   failure=None;predicted=[];scored=[];began=time.monotonic()
   try:
    for entity in predict(row['text']):
     kind=config['mapping'].get(entity['label'])
     if kind not in manifest['scope']:excluded[entity['label']]=excluded.get(entity['label'],0)+1;continue
     s,e=int(entity['start']),int(entity['end']);text=row['text']
     if not 0<=s<e<=len(text):raise ValueError('invalid candidate offsets')
     span=[len(text[:s].encode()),len(text[:e].encode()),kind]
     if span not in predicted:predicted.append(span)
     scored.append({'span':span,'score':float(entity['score'])})
   except Exception as error:
    # Do not print exception messages; upstream exceptions can include input.
    failure=type(error).__name__;failures[failure]=failures.get(failure,0)+1;predicted=[];scored=[]
   record={k:row[k] for k in ['id','stratum','text_sha256','gold']}
   record.update(predicted=predicted,scored=scored,abstained=failure is not None,candidate_failure=failure,seconds=time.monotonic()-began)
   records.append(record);log.write(json.dumps(record)+'\n');log.flush()
   if len(records)%100==0:print(json.dumps({'candidate':a.candidate,'completed':len(records),'total':len(rows),'seconds':time.monotonic()-start}),flush=True)
 for path in paths:
  if digest(path)!=sources[str(path.relative_to(ROOT))]:raise ValueError('measurement changed')
 result={'schema':1,'status':status,'candidate':a.candidate,'config':config,'model':identity,'corpus':manifest,'measurement_sha256':sources,'network':'socket connections denied for loading and inference','independent_validation':False,'scope':'projected source labels on public synthetic English; annotation errors/training overlap unknown','load_seconds':load_seconds,'seconds':time.monotonic()-start,'peak_rss_kib':resource.getrusage(resource.RUSAGE_SELF).ru_maxrss,'evaluated_excerpts':len(records),'expected_excerpts':len(rows),'candidate_failures':failures,'excluded_labels':excluded,'partial_sha256':digest(partial)}
 if status=='complete':result['quality']=score(records,manifest['scope'])
 with out.open('x') as f:json.dump(result,f,indent=2)
 print(json.dumps({k:result[k] for k in ['status','candidate','evaluated_excerpts','seconds','candidate_failures','peak_rss_kib']}),flush=True)
if __name__=='__main__':main()
