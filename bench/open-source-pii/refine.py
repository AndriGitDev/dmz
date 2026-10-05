"""Replay the bounded, documented calibration family; never infer on blind data."""
import argparse,json,re,unicodedata,hashlib
from pathlib import Path
from evaluate import ROOT,load_corpus,private_output,digest
from compare import add_nonoverlapping
from quality import score

HONORIFIC=re.compile(r'^(?:Mr|Mrs|Ms|Miss|Dr|Prof|Sir|Dame)\.?[ \t]+',re.I)
def compact_name_boundaries(text,items):
 data=text.encode();names=[];others=[]
 for item in sorted(items,key=lambda x:x['span']):
  start,end,kind=item['span']
  if kind!='name_demo':others.append(item);continue
  chars=len(data[:end].decode())
  while chars<len(text) and unicodedata.category(text[chars]).startswith('M'):chars+=1
  end=len(text[:chars].encode())
  if names and start>=names[-1]['span'][1] and re.fullmatch(rb'[ \t]+',data[names[-1]['span'][1]:start]):
   names[-1]['span'][1]=end;names[-1]['score']=min(names[-1]['score'],item['score'])
  else:names.append({'span':[start,end,kind],'score':item['score']})
 result=[]
 for item in names:
  s,e,k=item['span'];value=data[s:e].decode();title=HONORIFIC.match(value)
  if title:s+=len(title[0].encode())
  if s<e:result.append({'span':[s,e,k],'score':item['score']})
 return result+others

def main():
 p=argparse.ArgumentParser();p.add_argument('--directory',type=Path,required=True);p.add_argument('--output',required=True);a=p.parse_args()
 out=private_output(a.output);rows,manifest=load_corpus(ROOT/'bench/corpus/open-source-pii-v1');basepath=a.directory/'frozen-browser.json';base=json.loads(basepath.read_text());base={r['id']:r for r in base['records']};runs={};hashes={}
 for name in ['presidio','bert','gliner','gliner2','opf','gravitee-fp32','gravitee-int8']:
  path=a.directory/(name+'-initial.json');d=json.loads(path.read_text());assert d['corpus']==manifest and d['status']=='complete';hashes[name]=digest(path)
  predicted={r['id']:r for r in d['records']}
  assert set(predicted)==set(base)
  thresholds=[0] if name=='opf' else [0,.9,.99]
  for adaptation in (['raw','name-boundaries'] if name.startswith('gravitee') else ['raw']):
   for scope in ['all','name_demo','address','phone']:
    for threshold in thresholds:
     records=[]
     for row in rows:
      b=base[row['id']]['trained'];cached=predicted[row['id']];assert cached['text_sha256']==hashlib.sha256(row['text'].encode()).hexdigest()
      items=cached['scored']
      if adaptation=='name-boundaries':items=compact_name_boundaries(row['text'],items)
      items=[i for i in items if i['score']>=threshold and (scope=='all' or i['span'][2]==scope)]
      result=[] if b['abstained'] else add_nonoverlapping(b['predicted'],items)
      records.append({'gold':row['gold'],'predicted':result,'stratum':row['stratum'],'abstained':b['abstained']})
     key=f'{name}/{adaptation}/{scope}/{threshold}'
     runs[key]=score(records,manifest['scope'])
 result={'schema':1,'independent_validation':False,'scope':'calibration replay only, all predeclared variants retained','corpus':manifest,'reference_sha256':digest(basepath),'candidate_hashes':hashes,'measurement_sha256':digest(Path(__file__)),'runs':runs}
 with out.open('x') as f:json.dump(result,f,indent=2)
 for key,run in runs.items():
  strata=run['strata'];ex=[strata[s]['exact_excerpts'] for s in ['english-general','english-work','mixed-is-en']]
  if ex[0]>=25 and ex[1]>=27 and ex[2]>=22 and ex!=[25,27,22]:print(key,ex,'TP/FP/FN',run['overall']['tp'],run['overall']['fp'],run['overall']['fn'])
if __name__=='__main__':main()
