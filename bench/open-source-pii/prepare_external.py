"""Deterministic source-label projection, never a detector or label repair pass."""
import argparse,ast,collections,hashlib,json,re,unicodedata
from pathlib import Path
from evaluate import digest,private_output
MAP={'name':'name_demo','first_name':'name_demo','last_name':'name_demo','address':'address','street_address':'address','email':'email','phone_number':'phone','credit_card_number':'card','password':'secret','api_key':'secret'}
TITLE=re.compile(r'^(?:Mr|Mrs|Ms|Miss|Dr|Prof|Sir|Dame)\.?[ \t]+',re.I)
REV='e06eb1499ca8d54470f085021cd8e54f9efac7fd'

def convert(row):
 text=row['text'];spans=set()
 for annotation in ast.literal_eval(row['entities']):
  kinds={MAP[k] for k in annotation['types'] if k in MAP}
  if not kinds:continue
  if len(kinds)!=1:return None,'conflicting_mapped_types'
  kind=kinds.pop();value=annotation['entity'].strip()
  if kind=='name_demo':value=TITLE.sub('',value)
  if not value:return None,'empty_mapped_value'
  def word(c):return c.isalnum() or c=='_' or unicodedata.category(c).startswith('M')
  occurrences=[m for m in re.finditer(re.escape(value),text) if not (m.start()>0 and word(value[0]) and word(text[m.start()-1])) and not (m.end()<len(text) and word(value[-1]) and word(text[m.end()]))]
  if not occurrences:return None,'missing_mapped_value'
  for match in occurrences:spans.add((match.start(),match.end(),kind))
 ordered=sorted(spans);merged=[]
 for s,e,k in ordered:
  if merged and s<merged[-1][1]:
   ps,pe,pk=merged[-1]
   if k==pk and s>=ps and e<=pe:continue
   return None,'incompatible_projected_overlap'
  if merged and k==merged[-1][2]=='name_demo' and re.fullmatch(r'[ \t]+',text[merged[-1][1]:s]):merged[-1][1]=e
  else:merged.append([s,e,k])
 gold=[[len(text[:s].encode()),len(text[:e].encode()),k] for s,e,k in merged]
 return {'id':row['uid'],'stratum':row['domain'],'document_type':row['document_type'],'text':text,'text_sha256':hashlib.sha256(text.encode()).hexdigest(),'gold':gold},None

def main():
 p=argparse.ArgumentParser();p.add_argument('--source',type=Path,required=True);p.add_argument('--output-directory',type=Path,required=True);a=p.parse_args()
 a.output_directory=private_output(str(a.output_directory));a.output_directory.mkdir(exist_ok=False)
 identity=json.loads((a.source/'download-manifest.json').read_text());assert identity['revision']==REV
 for name,info in identity['files'].items():
  if digest(a.source/name)!=info['sha256']:raise ValueError('source changed')
 import pyarrow.parquet as pq
 manifest={'schema':1,'source':identity,'independent_validation':False,'training_overlap':'unknown; public source may have been used by candidates','scope':sorted(set(MAP.values())),'conversion_sha256':digest(Path(__file__)),'sets':{}}
 for split in ['train','validation','test']:
  rows=pq.read_table(a.source/'data'/f'{split}-00000-of-00001.parquet').to_pylist();converted=[];rejected=collections.Counter();domains=collections.Counter()
  for row in rows:
   result,error=convert(row)
   if error:rejected[error]+=1
   else:converted.append(result);domains[result['stratum']]+=1
  # Stable source order; no quality-dependent sampling.
  target=a.output_directory/(split+'.jsonl')
  with target.open('x') as f:
   for row in converted:f.write(json.dumps(row,ensure_ascii=False)+'\n')
  manifest['sets'][split]={'sha256':digest(target),'source_rows':len(rows),'retained_rows':len(converted),'rejected':dict(rejected),'domains':dict(domains),'entities':sum(len(r['gold']) for r in converted)}
 (a.output_directory/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
 print(json.dumps({s:{k:v for k,v in d.items() if k!='domains'} for s,d in manifest['sets'].items()},indent=2))
if __name__=='__main__':main()
