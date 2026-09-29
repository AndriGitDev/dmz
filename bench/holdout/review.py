#!/usr/bin/env python3
"""Prepare/validate blind local annotation packets; never generate model labels."""
import argparse
import hashlib
import json
from pathlib import Path
import sys
sys.path.insert(0, str(Path(__file__).resolve().parents[1]/'contextual'))
from ner import external


def sha(data):
    return hashlib.sha256(data).hexdigest()


def prepare(sources, output):
    output=external(output); output.mkdir(parents=True,exist_ok=False)
    manifest=[]; count=0; documents=set(); seen=set()
    with (output/'annotations.jsonl').open('x') as labels:
        for source in sources:
            source=external(source)
            manifest.append(dict(path=str(source),sha256=sha(source.read_bytes())))
            for line in source.read_text().splitlines():
                row=json.loads(line)
                key=(row['document_id'],row['page'])
                if key in seen:raise ValueError('duplicate document/page')
                seen.add(key)
                if row['entities'] is not None or row['reviewed']:
                    raise ValueError('blind packet requires unlabelled source')
                data=row['text'].encode('utf-8')
                if sha(data)!=row['text_sha256']:raise ValueError('source text checksum mismatch')
                count+=1;documents.add(row['document_id'])
                name=f'page-{count:04d}.txt';(output/name).write_bytes(data)
                labels.write(json.dumps(dict(document_id=row['document_id'],page=row['page'],
                    text_file=name,text_sha256=row['text_sha256'],entities=None,reviewed=False,
                    reviewer=None,second_reviewer=None,extraction_verified=False,
                    exclusion_reason=None,overlap_resolution=None,
                    overlapping_mim_12grams=row['overlapping_mim_12grams'],eligible_for_final=False))+'\n')
    report=dict(schema=1,pages=count,documents=len(documents),sources=manifest,
        annotations_sha256=sha((output/'annotations.jsonl').read_bytes()),
        status='blind unlabelled annotation pilot; no model inference; not final evaluation')
    (output/'manifest.json').write_text(json.dumps(report,indent=2)+'\n')
    (output/'README.md').write_text('''# Blind annotation pilot

No model predictions are included. These pages are a collection pilot, not an
approved final evaluation sample. Source PDFs/HTML remain in each source directory.
Keep this entire package private and outside Git.

For each row in annotations.jsonl, compare text_file against its source PDF page
or the article at its recorded source URL (saved HTML is also available as text).
Record extraction_verified only after checking reading order, tables and broken
words. Exclude unusable pages with exclusion_reason; never silently edit text.
Mark every Person mention, following the MIM Person policy: include single names,
inflected names, initials and foreign names; distinguish organizations and places.
Use entities [{"start": 0, "end": 4, "kind": "Person"}] with exact UTF-8 byte
offsets into the unchanged text file. End offsets are exclusive. Null means
unlabelled; [] means a human reviewed the entire page and found no person names.
Record reviewer and reviewed=true. A second person should independently review
boundaries on a predeclared sample; record second_reviewer after adjudication.
Resolve overlap flags in overlap_resolution. Marking a flag resolved does not
establish document independence or rule out pretraining exposure.

Use review.py validate --packet PATH for structural validation. Passing that check
does not establish annotation correctness, licence approval or final eligibility.
Do not turn eligible_for_final on: a separate document-disjoint sampling and
approval protocol is required. Do not run candidate models on this pilot to choose
thresholds and later call it independent evidence.
''')
    return {k:v for k,v in report.items() if k!='sources'}


def validate(packet):
    packet=external(packet); report=dict(pages=0,reviewed=0,excluded=0,names=0)
    manifest=json.loads((packet/'manifest.json').read_text())
    expected={}
    for source in manifest['sources']:
        path=external(Path(source['path']))
        if sha(path.read_bytes())!=source['sha256']:raise ValueError('source changed')
        for line in path.read_text().splitlines():
            row=json.loads(line);expected[(row['document_id'],row['page'])]=row['text_sha256']
    seen=set()
    for line in (packet/'annotations.jsonl').read_text().splitlines():
        row=json.loads(line);key=(row['document_id'],row['page']);report['pages']+=1
        if key in seen or key not in expected:raise ValueError('duplicate or unknown page')
        seen.add(key)
        path=(packet/row['text_file']).resolve()
        if path.parent!=packet:raise ValueError('invalid text path')
        data=path.read_bytes()
        if sha(data)!=row['text_sha256'] or row['text_sha256']!=expected[key]:raise ValueError('text changed')
        if row.get('eligible_for_final'):raise ValueError('pilot is not approved final evaluation')
        if row.get('exclusion_reason'):
            if not row.get('reviewer'):raise ValueError('exclusion needs reviewer')
            report['excluded']+=1;continue
        if row.get('reviewed') is not True or not row.get('reviewer') or row.get('extraction_verified') is not True:
            raise ValueError('incomplete human/extraction review')
        if not isinstance(row.get('entities'),list):raise ValueError('null is not a negative label')
        if row['overlapping_mim_12grams'] and not row.get('overlap_resolution'):
            raise ValueError('unresolved overlap')
        previous=0
        for entity in sorted(row['entities'],key=lambda e:e['start']):
            start,end=entity['start'],entity['end']
            if type(start)!=int or type(end)!=int or not 0<=start<end<=len(data) or start<previous or entity['kind']!='Person':
                raise ValueError('invalid/overlapping Person span')
            data[:start].decode('utf-8');data[start:end].decode('utf-8');data[end:].decode('utf-8')
            previous=end;report['names']+=1
        report['reviewed']+=1
    if seen!=set(expected):raise ValueError('missing review pages')
    report['independent_final_evaluation']=False
    return report


if __name__=='__main__':
    p=argparse.ArgumentParser(__doc__);s=p.add_subparsers(dest='command',required=True)
    a=s.add_parser('prepare');a.add_argument('--source',type=Path,nargs='+',required=True);a.add_argument('--output',type=Path,required=True)
    v=s.add_parser('validate');v.add_argument('--packet',type=Path,required=True)
    args=p.parse_args()
    print(json.dumps(prepare(args.source,args.output) if args.command=='prepare' else validate(args.packet),indent=2))
