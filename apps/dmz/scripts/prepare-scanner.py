#!/usr/bin/env python3
"""Prepare pinned, private scanner assets outside Git; no downloads or training."""
import argparse, hashlib, json, shutil
from pathlib import Path
import pycrfsuite

p=argparse.ArgumentParser(__doc__)
p.add_argument('--experiment',type=Path,required=True)
p.add_argument('--base',type=Path,required=True)
p.add_argument('--crf',type=Path,required=True)
p.add_argument('--output',type=Path,required=True)
a=p.parse_args()
repo=Path(__file__).resolve().parents[3]
if a.output.resolve().is_relative_to(repo):raise ValueError('Assets must stay outside checkout')
if a.output.exists():raise ValueError('Refusing to overwrite assets')
def sha(path):
 h=hashlib.sha256()
 with path.open('rb') as f:
  for b in iter(lambda:f.read(1024*1024),b''):h.update(b)
 return h.hexdigest()
selection=json.loads((repo/'bench/results/full-precision-v1/selected-candidate.json').read_text())
assert sha(a.crf)==selection['composition']['crf_sha256']
model=a.experiment/'browser-frozen/fp16storage.onnx'
assert sha(model)=='e5acd596fde1873c6f4a5620db9c5417f11a59f3ad70cd96423614dd667b4690'
tokenizer=a.experiment/'browser-frozen/tokenizer/tokenizer.json'
# tokenizer.json is derived by the pinned native tokenizer. Verify the pinned source files.
for name,digest in selection['encoder']['tokenizer_files'].items():
 assert sha(a.base/name)==digest,(name,'tokenizer checksum')
tokenizer_data=json.loads(tokenizer.read_text())
assert tokenizer_data['model']['vocab']==json.loads((a.base/'vocab.json').read_text())
merges=[line.split() for line in (a.base/'merges.txt').read_text().splitlines() if line and not line.startswith('#')]
assert tokenizer_data['model']['merges']==merges
metadata=json.loads(a.crf.with_suffix('.metadata.json').read_text())
assert metadata['feature_version']==3 and metadata['label_mode']=='person'
tagger=pycrfsuite.Tagger();tagger.open(str(a.crf));info=tagger.info()
labels=list(info.labels);assert set(labels)=={'O','B','I'}
state={}
for (feature,label),weight in info.state_features.items():state.setdefault(feature,[]).append([labels.index(label),weight])
transitions=[[info.transitions.get((i,j),0.) for j in labels] for i in labels]
a.output.mkdir(parents=True)
shutil.copyfile(model,a.output/'model.onnx');shutil.copyfile(tokenizer,a.output/'tokenizer.json')
(a.output/'crf.json').write_text(json.dumps(dict(labels=labels,state=state,transitions=transitions),ensure_ascii=False,separators=(',',':')))
files={name:dict(sha256=sha(a.output/name),bytes=(a.output/name).stat().st_size) for name in ['model.onnx','tokenizer.json','crf.json']}
(a.output/'manifest.json').write_text(json.dumps(dict(schema=1,model='fp16storage',files=files,crf_source_sha256=sha(a.crf),crf_export_precision='python-crfsuite info six decimal weights; parity checked separately',thresholds=selection['thresholds']),indent=2)+'\n')
print(json.dumps(dict(output=str(a.output),files=files)))
