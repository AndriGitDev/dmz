"""Freeze phone-focused synthetic probes before implementing the OSS adapter.

NANPA 555-01xx and Ofcom London 020 7946 0xxx are fictional ranges.
Icelandic values are existing synthetic DMZ fixtures. No calls are made.
"""
import make_corpus
from pathlib import Path
import hashlib,json,re
cases={
'international':[
('us-space','Call {{phone|+1 202-555-0107}} tomorrow.'),
('us-compact','Telephone: {{phone|+12025550114}}.'),
('us-parens','Contact {{phone|+1 (202) 555-0138}}.'),
('us-dots','Telephone: {{phone|+1.202.555.0148}}'),
('us-extension','Call {{phone|+1 202-555-0162 ext. 45}}.'),
('uk-spaces','Call {{phone|+44 20 7946 0123}}.'),
('uk-compact','Telephone: {{phone|+442079460456}}'),
('uk-parens','Reach us at {{phone|+44 (20) 7946 0789}}.'),
('is','Sími {{phone|+354 555 1234}}.'),
('is-local','Síminn er {{phone|555-1234}}.'),
('is-dialling','Sími {{phone|00354 555 1234}}.'),
('unicode-prefix','Þú getur hringt í {{phone|+1 202-555-0107}}.'),
('nfd-prefix','Jose\u0301: {{phone|+44 20 7946 0123}}.'),
('json','{"phone":"{{phone|+12025550185}}"}'),
('two','Call {{phone|+1 202-555-0107}} or {{phone|+44 20 7946 0123}}.'),
('newline','Telephone:\n{{phone|+1 202 555 0194}}'),
('unicode-space','Phone: {{phone|+1\u00a0202\u00a0555\u00a00107}}.'),
('parenthesized','Call ({{phone|+1 202-555-0107}}).'),
],
'domestic':[
('us-parentheses','Phone: {{phone|(202) 555-0114}}.'),
('us-dots','Contact number: {{phone|202.555.0138}}.'),
('uk-local','Phone: {{phone|020 7946 0123}}.'),
('is-local-space','Phone: {{phone|555 1234}}.'),
],
'negative-and-overlap':[
('counts','There are 202 items, 555 pages and 107 comments.'),
('equation','Compute +1 + 202 + 555 + 107.'),
('date','Review date: 2026-10-05.'),
('price','The price increased by +1202.55 dollars.'),
('version','Version 1.202.555.0107 is a software build identifier.'),
('long','Batch 120255501071234567890 has not finished.'),
('prefixed','Reference ABC12025550107XYZ is a synthetic identifier.'),
('short','Country code +44 and extension 123 are separate fields.'),
('email','Email {{email|+12025550107@example.invalid}}.'),
('card','Test card: {{card|4111 1111 1111 1111}}.'),
('iban','Test IBAN: {{iban|IS140159260076545510730339}}.'),
('secret','password="{{secret|+12025550107AbC9zX8qW7}}"'),
('placeholder','Phone: [PHONE].'),
('roles','The customer will call the doctor tomorrow.'),
]}
root=Path(__file__).resolve().parents[1]/'corpus/international-phone-v1';root.mkdir(exist_ok=True)
manifest={'schema':1,'independent_validation':False,'authorship':'synthetic authored before adapter; domestic forms deliberately retained as out-of-initial-scope diagnostic','scope':['name_demo','address','phone','email','iban','card','kennitala','secret','tenant_dictionary'],'sets':{}}
for stratum,examples in cases.items():
 rows=[]
 for key,marked in examples:
  entities=[]
  def repl(m):entities.append({'kind':m[1],'value':m[2]});return m[2]
  text=re.sub(r'\{\{([a-z_]+)\|(.*?)\}\}',repl,marked,flags=re.S)
  assert all(text.count(e['value'])==1 for e in entities)
  rows.append({'id':stratum+'-'+key,'stratum':stratum,'family':key,'text':text,'entities':entities})
 data=''.join(json.dumps(r,ensure_ascii=False)+'\n' for r in rows).encode();p=root/(stratum+'.jsonl')
 if p.exists() and p.read_bytes()!=data:raise ValueError('frozen corpus changed')
 p.write_bytes(data);manifest['sets'][p.name]={'sha256':hashlib.sha256(data).hexdigest(),'excerpts':len(rows),'positive_excerpts':sum(bool(r['entities']) for r in rows),'entities':sum(len(r['entities']) for r in rows)}
(root/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps(manifest,indent=2))
