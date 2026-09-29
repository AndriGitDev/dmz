#!/usr/bin/env python3
"""Private vectors for full browser development replay, then aggregate scoring."""
import argparse
import json
from pathlib import Path
from evaluate import inputs,score,verify_tokenizer
from encoder import chunks,digest,DECODE
from ner import external,rows,tokens_and_labels,spans
from ensemble import merge


def compression_changes(records, baseline, crf):
    """Compare the same-runtime frozen reference at the unchanged thresholds."""
    result=dict(changed_records=0,added_missed_spans=0,recovered_spans=0,
                added_false_positives=0,removed_false_positives=0)
    for output,reference,primary in zip(records,baseline,crf,strict=True):
        if output['gold']!=reference['gold'] or output['gold']!=primary['gold']:
            raise ValueError('browser reference annotation mismatch')
        gold={tuple(span) for span in output['gold']}
        current=merge(primary['predictions'],output['predictions'],.75,0)
        frozen=merge(primary['predictions'],reference['predictions'],.75,0)
        result['changed_records']+=current!=frozen
        result['added_missed_spans']+=len((frozen-current)&gold)
        result['recovered_spans']+=len((current-frozen)&gold)
        result['added_false_positives']+=len((current-frozen)-gold)
        result['removed_false_positives']+=len((frozen-current)-gold)
    return result


def main():
    p=argparse.ArgumentParser(__doc__)
    p.add_argument('mode',choices=['prepare','score'])
    p.add_argument('--assets',type=Path,required=True)
    p.add_argument('--reference-dir',type=Path,required=True)
    p.add_argument('--general-input',type=Path,required=True)
    p.add_argument('--vectors',type=Path,required=True)
    p.add_argument('--predictions',type=Path)
    p.add_argument('--report',type=Path)
    p.add_argument('--model',default='int8',choices=['fp32','int8','int4','mixed'])
    p.add_argument('--selection',type=Path,default=Path(__file__).resolve().parents[1]/'results/error-audit-v2/selected-candidate.json')
    p.add_argument('--neural-prefix',default='neural-epoch-12-predictions')
    p.add_argument('--cache-output',type=Path,help='Private decoded browser spans for subsequent same-runtime comparison')
    p.add_argument('--browser-reference',type=Path,help='Private FP32 browser span cache, not a native substitute')
    a=p.parse_args()
    if a.report and a.report.exists():raise ValueError('refusing to overwrite browser report')
    if a.cache_output and external(a.cache_output).exists():raise ValueError('refusing to overwrite browser span cache')
    for key in ['assets','reference_dir','general_input','vectors']:
        setattr(a,key,external(getattr(a,key)))
    if a.mode=='prepare':
        if a.vectors.exists():raise ValueError('refusing to replace vectors')
        from transformers import AutoTokenizer
        verify_tokenizer(a.assets/'tokenizer')
        tokenizer=AutoTokenizer.from_pretrained(a.assets/'tokenizer',local_files_only=True,add_prefix_space=True)
        sets=[]
        for name,path in [('general',a.general_input),('domain',a.reference_dir/'domain-dev.jsonl')]:
            records=[]
            for row in rows(path,'dev'):
                words,offsets,labels=tokens_and_labels(row)
                pieces=[]
                for start,part in chunks(words,tokenizer):
                    feed=inputs([part],tokenizer)
                    pieces.append(dict(offset=start,words=len(part),feeds={k:dict(shape=list(v.shape),data=v.flatten().tolist()) for k,v in feed.items()}))
                records.append(dict(offsets=offsets,gold=[[s,e] for s,e,_,_ in spans(labels,offsets)],chunks=pieces))
            sets.append(dict(name=name,input_sha256=digest(path),records=records))
        a.vectors.write_text(json.dumps(dict(schema=1,model_sha256=digest(a.assets/(a.model+'.onnx')),sets=sets)))
        print(json.dumps({s['name']:len(s['records']) for s in sets}))
        return
    predictions=json.loads(external(a.predictions).read_text());vectors=json.loads(a.vectors.read_text())
    assert predictions['model_sha256']==vectors['model_sha256']==digest(a.assets/(a.model+'.onnx'))
    assert predictions['vectors_sha256']==digest(a.vectors)
    selected=json.loads(a.selection.read_text())
    assert selected['thresholds']==dict(crf=.75,neural=0)
    browser_reference=None
    if a.browser_reference:
        browser_reference=json.loads(external(a.browser_reference).read_text())
        if browser_reference['model_sha256']!=digest(a.assets/'fp32.onnx'):
            raise ValueError('browser reference is not this FP32 graph')
        for key in ['runtime','browser','threads','batch_size']:
            if browser_reference[key]!=predictions[key]:raise ValueError('browser runtime comparison mismatch')
    reports=[];decoded_sets=[]
    for dataset,predicted in zip(vectors['sets'],predictions['sets'],strict=True):
        name=dataset['name'];assert name==predicted['name']
        tag='dev' if name=='general' else 'domain'
        crf=json.loads((a.reference_dir/f'crf-predictions-{tag}.json').read_text())
        frozen=json.loads((a.reference_dir/f'{a.neural_prefix}-{tag}.json').read_text())
        native=json.loads((a.assets/f'{a.model}-{name}-batch1-predictions.json').read_text())
        assert crf['model_sha256']==selected['composition']['crf_sha256']
        assert frozen['model_sha256']==selected['composition']['neural_sha256']
        assert native['model_sha256']==predictions['model_sha256']
        for cache in [crf,frozen,native]:assert cache['input_sha256']==dataset['input_sha256']
        records=[];changed_frozen=changed_native=0
        added_misses=recovered=added_false_positives=removed_false_positives=0
        for i,(record,labels) in enumerate(zip(dataset['records'],predicted['labels'],strict=True)):
            assert len(labels)==len(record['offsets']) and all(x in (0,1,2) for x in labels)
            assert record['gold']==crf['records'][i]['gold']==frozen['records'][i]['gold']==native['records'][i]['gold']
            output=dict(gold=record['gold'],predictions=[[s,e,1.] for s,e,_,_ in spans([DECODE[x] for x in labels],record['offsets'])])
            records.append(output)
            merged=merge(crf['records'][i]['predictions'],output['predictions'],.75,0)
            baseline=merge(crf['records'][i]['predictions'],frozen['records'][i]['predictions'],.75,0)
            gold={tuple(span) for span in record['gold']}
            added_misses+=len((baseline-merged)&gold);recovered+=len((merged-baseline)&gold)
            added_false_positives+=len((merged-baseline)-gold);removed_false_positives+=len((baseline-merged)-gold)
            changed_frozen+=merged!=baseline
            changed_native+=merged!=merge(crf['records'][i]['predictions'],native['records'][i]['predictions'],.75,0)
        reports.append(dict(name=name,input_sha256=dataset['input_sha256'],rows=len(records),browser_replay_seconds=predicted['seconds'],neural=score(records),combined=score(records,crf['records']),changed_combined_from_frozen=changed_frozen,changed_combined_from_native_batch1=changed_native,versus_frozen=dict(added_missed_spans=added_misses,recovered_spans=recovered,added_false_positives=added_false_positives,removed_false_positives=removed_false_positives)))
        decoded_sets.append(dict(name=name,input_sha256=dataset['input_sha256'],records=records))
        if browser_reference:
            matching=[s for s in browser_reference['sets'] if s['name']==name]
            if len(matching)!=1 or matching[0]['input_sha256']!=dataset['input_sha256']:
                raise ValueError('browser reference input mismatch')
            reference_records=matching[0]['records']
            reports[-1]['versus_fp32_browser']=compression_changes(records,reference_records,crf['records'])
            reports[-1]['fp32_browser_combined']=score(reference_records,crf['records'])
            empty_crf=[dict(gold=r['gold'],predictions=[]) for r in records]
            reports[-1]['versus_fp32_browser_neural']=compression_changes(records,reference_records,empty_crf)
            reports[-1]['fp32_browser_neural']=score(reference_records)
    report={k:v for k,v in predictions.items() if k!='sets'}
    report.update(scope='Full fixed development replay through Chromium WASM with native pretokenization and cached CRF merge; not end-to-end Shield',independent_validation=False,sets=reports)
    if a.browser_reference:report['fp32_browser_reference_sha256']=digest(a.browser_reference)
    if a.cache_output:
        with external(a.cache_output).open('x') as f:
            json.dump(dict(model_sha256=predictions['model_sha256'],predictions_sha256=digest(a.predictions),
                runtime=predictions['runtime'],browser=predictions['browser'],threads=predictions['threads'],
                batch_size=predictions['batch_size'],sets=decoded_sets),f)
    a.report.write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))

if __name__=='__main__':main()
