#!/usr/bin/env python3
"""Native parity fixtures stay private, never in Git."""
import argparse,json,sys
from pathlib import Path
p=argparse.ArgumentParser();p.add_argument('--input',type=Path,required=True);p.add_argument('--crf',type=Path,required=True);p.add_argument('--tokenizer',type=Path,required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--original',action='store_true');a=p.parse_args()
sys.path.insert(0,str(Path(__file__).resolve().parents[3]/'bench/contextual'))
from ner import external,tokens_and_labels,features
from encoder import chunks
from transformers import AutoTokenizer
import pycrfsuite
external(a.output)
tokenizer=AutoTokenizer.from_pretrained(a.tokenizer,local_files_only=True,add_prefix_space=True)
tagger=pycrfsuite.Tagger();tagger.open(str(a.crf))
with a.output.open('x') as out:
 for line in a.input.open():
  row=json.loads(line)
  words,offsets,_=tokens_and_labels(row,text_mode='original' if a.original else 'canonical')
  fs=features(words,3);labels=tagger.tag(fs)
  encoded=[]
  for start,part in chunks(words,tokenizer):
   batch=tokenizer(part,is_split_into_words=True);encoded.append(dict(offset=start,ids=batch['input_ids'],word_ids=batch.word_ids()))
  out.write(json.dumps(dict(text=row.get('original_text') if a.original else None,words=words,offsets=offsets,features=fs,labels=labels,confidence=[tagger.marginal(label,i) for i,label in enumerate(labels)],chunks=encoded),ensure_ascii=False)+'\n')
