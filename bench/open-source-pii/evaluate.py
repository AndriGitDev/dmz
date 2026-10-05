"""Offline candidate inference on committed synthetic fixtures, private output only."""
from __future__ import annotations
import argparse
import hashlib
import importlib.metadata
import json
import os
from pathlib import Path
import socket
import sys
import time
from collections import Counter

ROOT=Path(__file__).resolve().parents[2]
sys.path.insert(0,str(ROOT/'bench/contextual'))
from quality import score

def digest(path):
    h=hashlib.sha256()
    with Path(path).open('rb') as f:
        while b:=f.read(1024*1024):h.update(b)
    return h.hexdigest()

def load_corpus(directory):
    directory=Path(directory).resolve()
    if not directory.is_relative_to(ROOT/'bench/corpus'):raise ValueError('synthetic corpus only')
    manifest=json.loads((directory/'manifest.json').read_text())
    rows=[]
    for name,info in manifest['sets'].items():
        p=directory/name
        if digest(p)!=info['sha256']:raise ValueError('corpus hash mismatch')
        for row in map(json.loads,p.read_text().splitlines()):
            if not isinstance(row['entities'],list):raise ValueError('complete labels required')
            gold=[]
            for entity in row['entities']:
                value=entity['value'];text=row['text']
                if text.count(value)!=1:raise ValueError('non-unique literal annotation')
                start=text.index(value)
                gold.append([len(text[:start].encode()),len(text[:start+len(value)].encode()),entity['kind']])
            rows.append(row|{'gold':gold})
    if len({r['id'] for r in rows})!=len(rows):raise ValueError('duplicate identifiers')
    return rows,manifest

def private_output(path):
    path=Path(path).resolve()
    if path.is_relative_to(ROOT) or not path.parent.is_dir() or path.exists():
        raise ValueError('new private output required')
    return path

def offline():
    os.environ.update(HF_HUB_OFFLINE='1',TRANSFORMERS_OFFLINE='1',HF_HUB_DISABLE_TELEMETRY='1',TOKENIZERS_PARALLELISM='false')
    def deny(*a,**kw):raise RuntimeError('network disabled during inference')
    socket.create_connection=deny
    socket.socket.connect=deny
    socket.socket.connect_ex=deny


def runner(name,config,model_dir):
    import torch
    torch.set_num_threads(2);torch.set_num_interop_threads(1)
    mapping=config['mapping']
    if name=='presidio':
        from presidio_analyzer import AnalyzerEngine
        from presidio_analyzer.nlp_engine import NlpEngineProvider
        engine=NlpEngineProvider(nlp_configuration={'nlp_engine_name':'spacy','models':[{'lang_code':'en','model_name':'en_core_web_lg'}]}).create_engine()
        model=AnalyzerEngine(nlp_engine=engine,supported_languages=['en'])
        def predict(text):
            return [{'start':r.start,'end':r.end,'label':r.entity_type,'score':r.score} for r in model.analyze(text=text,language='en',entities=list(mapping),score_threshold=config['threshold'])]
    elif name=='gliner':
        from gliner import GLiNER
        import onnxruntime as ort
        opts=ort.SessionOptions();opts.intra_op_num_threads=2;opts.inter_op_num_threads=1
        model=GLiNER.from_pretrained(str(model_dir),local_files_only=True,load_onnx_model=True,onnx_model_file='onnx/model.onnx',runtime_options={'session_options':opts})
        def predict(text):return model.predict_entities(text,list(mapping),threshold=config['threshold'],flat_ner=True)
    elif name=='gliner2':
        from gliner2 import GLiNER2
        model=GLiNER2.from_pretrained(str(model_dir),local_files_only=True).eval()
        def predict(text):
            result=model.extract_entities(text,list(mapping),threshold=config['threshold'],include_confidence=True,include_spans=True)
            return [{'start':r['start'],'end':r['end'],'label':label,'score':r.get('confidence',r.get('score',1.0))} for label,values in result['entities'].items() for r in values]
    elif name=='bert':
        from transformers import AutoTokenizer,AutoModelForTokenClassification,pipeline
        tok=AutoTokenizer.from_pretrained(str(model_dir),local_files_only=True)
        model=AutoModelForTokenClassification.from_pretrained(str(model_dir),local_files_only=True).eval()
        pipe=pipeline('token-classification',model=model,tokenizer=tok,aggregation_strategy=config['aggregation'],device=-1)
        def predict(text):return [{'start':r['start'],'end':r['end'],'label':r['entity_group'],'score':float(r['score'])} for r in pipe(text)]
    elif name=='opf':
        from opf import OPF
        model=OPF(model=str(model_dir/'original'),device='cpu',context_window_length=config['context_window_length'],decode_mode=config['decode'])
        model.get_prediction_components()
        def predict(text):
            result=model.redact(text)
            if result.text!=text or result.warning or result.summary.get('decoded_mismatch'):
                raise ValueError('Privacy Filter changed the original text')
            return [{'start':r.start,'end':r.end,'label':r.label,'score':1.0} for r in result.detected_spans]
    else:raise ValueError('unknown candidate')
    return predict


def main():
    parser=argparse.ArgumentParser()
    parser.add_argument('--candidate',required=True,choices=['presidio','gliner','gliner2','bert','opf'])
    parser.add_argument('--models',type=Path,required=True)
    parser.add_argument('--output',required=True)
    parser.add_argument('--corpus',type=Path,default=ROOT/'bench/corpus/open-source-pii-v1')
    args=parser.parse_args()
    out=private_output(args.output)
    registry=ROOT/'bench/open-source-pii/candidates.json'
    settings=json.loads(registry.read_text());config=settings['candidates'][args.candidate]
    rows,manifest=load_corpus(args.corpus)
    model_dir=args.models/config['model'].replace('/','--')
    identity={}
    if args.candidate!='presidio':
        identity=json.loads((model_dir/'download-manifest.json').read_text())
        if identity['revision']!=config['revision']:raise ValueError('wrong revision')
        for name,entry in identity['files'].items():
            if digest(model_dir/name)!=entry['sha256']:raise ValueError('model asset changed')
    config_hash=digest(registry);source_hash=digest(__file__)
    offline();start=time.monotonic()
    predict=runner(args.candidate,config,model_dir)
    load_seconds=time.monotonic()-start
    records=[];excluded=Counter();times=[]
    import torch
    with torch.inference_mode():
        for index,row in enumerate(rows):
            t=time.monotonic();entities=predict(row['text']);times.append(time.monotonic()-t)
            predicted=[];scored=[]
            for entity in entities:
                label=entity['label'];kind=config['mapping'].get(label)
                if kind is None:excluded[label]+=1;continue
                a,b=int(entity['start']),int(entity['end']);text=row['text']
                if not 0<=a<b<=len(text):raise ValueError('invalid model offsets')
                byte_a,byte_b=len(text[:a].encode()),len(text[:b].encode())
                span=[byte_a,byte_b,kind]
                if span not in predicted:predicted.append(span)
                scored.append({'span':span,'score':float(entity['score'])})
            records.append({'id':row['id'],'text_sha256':hashlib.sha256(row['text'].encode()).hexdigest(),'stratum':row['stratum'],'gold':row['gold'],'predicted':predicted,'scored':scored})
            if (index+1)%20==0:print(json.dumps({'candidate':args.candidate,'completed':index+1}),flush=True)
    if digest(registry)!=config_hash or digest(__file__)!=source_hash:raise ValueError('measurement changed')
    report={'schema':1,'candidate':args.candidate,'config':config,'config_sha256':config_hash,'measurement_sha256':source_hash,'model':identity,'corpus':manifest,'status':'complete','independent_validation':False,'quality':score(records,settings['scope']),'excluded_labels':dict(excluded),'load_seconds':load_seconds,'inference_seconds':sum(times),'per_excerpt_seconds':times,'records':records,'network':'socket connections denied throughout loading and inference','versions':{p:importlib.metadata.version(p) for p in ['torch','transformers','gliner','gliner2','presidio-analyzer','spacy','onnxruntime','numpy']}}
    with out.open('x') as f:json.dump(report,f,indent=2)
    print(json.dumps({'candidate':args.candidate,'quality':report['quality']['overall'],'load_seconds':load_seconds,'inference_seconds':sum(times)}),flush=True)
if __name__=='__main__':main()
