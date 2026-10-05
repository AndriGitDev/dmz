#!/usr/bin/env python3
"""Private, model-blind multi-entity review and explicit human adjudication.

Structural checks cannot prove reviewer identity, annotation correctness, or
sample independence. Null labels never become negatives automatically.
"""
import argparse
from collections import Counter
import json
import os
from pathlib import Path
import zipfile

from review import external, sha
from review_editor import render_editor

KINDS = ['Person', 'Address', 'Kennitala', 'Email', 'Phone', 'IBAN', 'Card', 'Secret', 'TenantTerm']
RUBRIC = Path(__file__).with_name('entity-annotation-rubric.md')


def jsonl(path):
    return [json.loads(line) for line in path.read_text().splitlines()]


def write_rows(path, rows):
    with path.open('x') as stream:
        for row in rows:
            stream.write(json.dumps(row, ensure_ascii=False, sort_keys=True)+'\n')


def prepare(sources, output, terms=None):
    output = external(output)
    if terms is None:
        terms = []
    if not isinstance(terms, list) or any(not isinstance(t, str) or not t.strip() for t in terms) or len(terms) > 32:
        raise ValueError('invalid exact tenant dictionary')
    if len(set(terms)) != len(terms):
        raise ValueError('duplicate tenant term')
    rows, provenance, seen = [], [], set()
    for source in sources:
        source = external(source)
        provenance.append(dict(path=str(source), sha256=sha(source.read_bytes())))
        for row in jsonl(source):
            key = (row['document_id'], row['page'])
            if key in seen or row.get('entities') is not None or row.get('reviewed') is not False:
                raise ValueError('requires unique unlabelled, unreviewed source excerpts')
            seen.add(key)
            if not isinstance(row.get('text'), str) or not row['text'] or sha(row['text'].encode()) != row['text_sha256']:
                raise ValueError('source text identity mismatch')
            rows.append(row)
    if not rows:
        raise ValueError('empty review source')
    output.mkdir(parents=True, exist_ok=False)
    (output/'texts').mkdir()
    rubric = RUBRIC.read_bytes(); (output/'rubric.md').write_bytes(rubric)
    index, reviews, decisions = [], [], []
    for i, row in enumerate(rows, 1):
        identifier = f'excerpt-{i:04d}'
        text_file = f'texts/{identifier}.txt'
        (output/text_file).write_bytes(row['text'].encode())
        index.append(dict(id=identifier, document_id=row['document_id'], page=row['page'],
                          text_file=text_file, text_sha256=row['text_sha256'],
                          source_url=row.get('source_url'), stratum=row.get('stratum', 'unspecified'),
                          cluster_sha256=row.get('cluster_sha256'), source_split=row.get('split'),
                          prior_overlap_12grams=row.get('overlapping_mim_12grams', 0)))
        base = dict(id=identifier, text_sha256=row['text_sha256'], entities=None,
                    reviewed=False, extraction_verified=False, exclusion_reason=None)
        reviews.append(base | dict(reviewer=None, issues=[]))
        decisions.append(base | dict(adjudicator=None, decision_note=None, overlap_resolution=None))
    write_rows(output/'index.jsonl', index)
    for name in ('reviewer-a.jsonl', 'reviewer-b.jsonl'):
        write_rows(output/name, reviews)
    write_rows(output/'adjudication.jsonl', decisions)
    (output/'tenant-terms.json').write_text(json.dumps(terms, ensure_ascii=False)+'\n')
    manifest = dict(schema=1, sources=provenance, scope=KINDS, excerpts=len(rows),
                    documents=len({r['document_id'] for r in rows}),
                    index_sha256=sha((output/'index.jsonl').read_bytes()), rubric_sha256=sha(rubric),
                    tenant_terms_sha256=sha((output/'tenant-terms.json').read_bytes()),
                    status='Two separate blind human reviews and third-person adjudication required',
                    human_review_complete=False, model_evaluated=False, independent_final_evaluation=False)
    (output/'manifest.json').write_text(json.dumps(manifest, indent=2)+'\n')
    return {key: manifest[key] for key in ('schema', 'scope', 'excerpts', 'documents', 'status', 'independent_final_evaluation')}


def load_packet(packet):
    packet = external(packet)
    manifest = json.loads((packet/'manifest.json').read_text())
    if manifest.get('schema') != 1 or manifest.get('scope') != KINDS:
        raise ValueError('unsupported annotation contract')
    if manifest.get('rubric_sha256') != sha(RUBRIC.read_bytes()):
        raise ValueError('packet uses a different rubric version')
    for name, field in [('index.jsonl', 'index_sha256'), ('rubric.md', 'rubric_sha256'), ('tenant-terms.json', 'tenant_terms_sha256')]:
        if sha((packet/name).read_bytes()) != manifest[field]:
            raise ValueError('packet contract changed')
    expected = {}
    for source in manifest['sources']:
        path = external(Path(source['path']))
        if sha(path.read_bytes()) != source['sha256']:
            raise ValueError('source changed')
        for row in jsonl(path):
            key = (row['document_id'], row['page'])
            if key in expected:
                raise ValueError('duplicate source excerpt')
            expected[key] = row['text_sha256']
    index, seen = {}, set()
    for row in jsonl(packet/'index.jsonl'):
        key = (row['document_id'], row['page'])
        path = (packet/row['text_file']).resolve()
        if path.parent != packet/'texts' or row['id'] in index or key in seen or key not in expected:
            raise ValueError('invalid index or text path')
        data = path.read_bytes(); data.decode('utf-8')
        if sha(data) != row['text_sha256'] or row['text_sha256'] != expected[key]:
            raise ValueError('text changed')
        seen.add(key); index[row['id']] = (row, data)
    if seen != set(expected) or len(index) != manifest['excerpts']:
        raise ValueError('incomplete packet')
    return manifest, index


def validate_entities(values, data):
    if not isinstance(values, list):
        raise ValueError('null labels are not a reviewed negative')
    result = []
    for entity in values:
        if not isinstance(entity, dict):
            raise ValueError('invalid entity record')
        a, b, kind = entity.get('start'), entity.get('end'), entity.get('kind')
        if type(a) is not int or type(b) is not int or not 0 <= a < b <= len(data) or kind not in KINDS:
            raise ValueError('invalid entity span or type')
        data[:a].decode('utf-8'); value = data[a:b].decode('utf-8'); data[b:].decode('utf-8')
        if not value.strip():
            raise ValueError('empty entity value')
        result.append((a, b, kind))
    result.sort()
    if any(a[1] > b[0] for a, b in zip(result, result[1:])):
        raise ValueError('overlapping or duplicate annotation; adjudicate one semantic type')
    return result


def submissions(path, index, role):
    result = {}
    for row in jsonl(path):
        identifier = row.get('id')
        if identifier in result or identifier not in index:
            raise ValueError('duplicate or unknown review excerpt')
        expected, data = index[identifier]
        if row.get('text_sha256') != expected['text_sha256']:
            raise ValueError('review text identity mismatch')
        person = row.get(role)
        if not isinstance(person, str) or not person.strip() or row.get('reviewed') is not True:
            raise ValueError('incomplete human review')
        excluded = row.get('exclusion_reason')
        if excluded is not None and (not isinstance(excluded, str) or not excluded.strip()):
            raise ValueError('invalid exclusion decision')
        if excluded:
            if row.get('entities') is not None:
                raise ValueError('excluded excerpt must not supply negative labels')
            entities = None
        else:
            if row.get('extraction_verified') is not True:
                raise ValueError('extraction review required')
            entities = validate_entities(row.get('entities'), data)
        result[identifier] = row | dict(validated_entities=entities)
    if set(result) != set(index):
        raise ValueError('incomplete review coverage')
    return result


def compare(packet):
    packet = external(packet)
    _, index = load_packet(packet)
    a, b = [submissions(packet/name, index, 'reviewer') for name in ('reviewer-a.jsonl', 'reviewer-b.jsonl')]
    counts = Counter(excerpts=len(index), exact_agreements=0, disagreements=0, exclusion_disagreements=0)
    for identifier in index:
        left, right = a[identifier], b[identifier]
        if left['reviewer'].strip().casefold() == right['reviewer'].strip().casefold():
            raise ValueError('two distinct reviewer identities required')
        agreement = left['validated_entities'] == right['validated_entities']
        if bool(left.get('exclusion_reason')) != bool(right.get('exclusion_reason')):
            counts['exclusion_disagreements'] += 1
            agreement = False
        counts['exact_agreements' if agreement else 'disagreements'] += 1
    return dict(counts) | dict(human_identity_not_independently_verified=True, independent_final_evaluation=False)


def package_reviewer(packet, slot, output):
    packet, output = external(packet), external(output)
    if slot not in ('a', 'b'):
        raise ValueError('choose reviewer a or b')
    manifest, index = load_packet(packet)
    name = f'reviewer-{slot}.jsonl'
    # Export only a blank form. Completed submissions must not accidentally be
    # disclosed to the other reviewer under a misleading "blind" archive name.
    rows = jsonl(packet/name)
    if len(rows) != len(index) or {row.get('id') for row in rows} != set(index):
        raise ValueError('incomplete blank form')
    for row in rows:
        expected = dict(id=row['id'], text_sha256=index[row['id']][0]['text_sha256'],
                        entities=None, reviewed=False, extraction_verified=False,
                        exclusion_reason=None, reviewer=None, issues=[])
        if row != expected:
            raise ValueError('blind export requires a blank review form')
    editor = render_editor(packet, slot, manifest, index)
    with os.fdopen(os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600), 'wb') as output_stream:
        with zipfile.ZipFile(output_stream, 'w', compression=zipfile.ZIP_DEFLATED) as archive:
            for filename in ('rubric.md', 'index.jsonl', 'tenant-terms.json', name):
                archive.write(packet/filename, filename)
            for source, _ in index.values():
                archive.write(packet/source['text_file'], source['text_file'])
            archive.writestr('review.html', editor)
            archive.writestr('packet.json', json.dumps(dict(scope=KINDS, reviewer_slot=slot, editor_sha256=sha(editor),
                packet_manifest_sha256=sha((packet/'manifest.json').read_bytes()),
                rubric_sha256=manifest['rubric_sha256'], excerpts=len(index)), indent=2)+'\n')
            archive.writestr('README.txt',
                'Private blind human review packet. Read rubric.md first.\n'
                'Open review.html in a local Chromium/Brave browser for offline text-selection annotation.\n'
                'Download progress explicitly to preserve unfinished work; no automatic browser storage is used.\n'
                'Export the completed reviewer JSONL form only after every excerpt is reviewed or explicitly excluded.\n'
                'Complete only the reviewer form included here; do not seek model predictions or the other reviewer\'s labels.\n'
                'index.jsonl contains source URLs and text identities. Ask the packet owner for saved source HTML when needed for extraction review.\n'
                'Keep text unchanged. Null means unreviewed; [] is a fully reviewed negative.\n'
                'Return the completed reviewer form privately to the packet owner for separate adjudication.\n')
    return dict(excerpts=len(index), reviewer_slot=slot, archive_sha256=sha(output.read_bytes()),
                model_predictions_included=False, other_reviewer_submission_included=False, editor_sha256=sha(editor))


def freeze(packet, output):
    packet, output = external(packet), external(output)
    comparison = compare(packet)
    manifest, index = load_packet(packet)
    a, b = [submissions(packet/name, index, 'reviewer') for name in ('reviewer-a.jsonl', 'reviewer-b.jsonl')]
    decisions = submissions(packet/'adjudication.jsonl', index, 'adjudicator')
    terms = json.loads((packet/'tenant-terms.json').read_text())
    records, counts, exclusions = [], Counter(), 0
    for identifier, (source, data) in index.items():
        decision = decisions[identifier]
        reviewers = {a[identifier]['reviewer'].strip().casefold(), b[identifier]['reviewer'].strip().casefold()}
        if decision['adjudicator'].strip().casefold() in reviewers:
            raise ValueError('a distinct adjudicator is required')
        if not isinstance(decision.get('decision_note'), str) or not decision['decision_note'].strip():
            raise ValueError('explicit adjudication decision required')
        if decision.get('exclusion_reason'):
            exclusions += 1; continue
        resolution = decision.get('overlap_resolution')
        if source['prior_overlap_12grams'] and (not isinstance(resolution, str) or not resolution.strip()):
            raise ValueError('unresolved prior-text overlap')
        entities = []
        for start, end, kind in decision['validated_entities']:
            if kind == 'TenantTerm' and data[start:end].decode() not in terms:
                raise ValueError('tenant annotation is not an exact configured term')
            counts[kind] += 1
            entities.append(dict(start=start, end=end, kind=kind))
        records.append(source | dict(text=data.decode(), entities=entities, reviewed=True,
                                     review_status='structurally complete adjudicated review', eligible_for_final=False))
    if not records:
        raise ValueError('all excerpts excluded')
    output.mkdir(parents=True, exist_ok=False)
    write_rows(output/'gold.jsonl', records)
    report = dict(schema=1, scope=KINDS, included_excerpts=len(records), excluded_excerpts=exclusions,
                  entities=dict(counts), comparison=comparison, gold_sha256=sha((output/'gold.jsonl').read_bytes()),
                  packet_manifest_sha256=sha((packet/'manifest.json').read_bytes()),
                  review_hashes={name: sha((packet/name).read_bytes()) for name in ('reviewer-a.jsonl','reviewer-b.jsonl','adjudication.jsonl')},
                  rubric_sha256=manifest['rubric_sha256'], tenant_terms_sha256=manifest['tenant_terms_sha256'],
                  review_structure_complete=True, independent_final_evaluation=False,
                  limitations='Structural evidence only; reviewer independence, correctness, source rights and final sampling acceptance require separate evidence.')
    (output/'manifest.json').write_text(json.dumps(report, indent=2)+'\n')
    return report


if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__); commands=parser.add_subparsers(dest='command',required=True)
    p=commands.add_parser('prepare');p.add_argument('--source',type=Path,action='append',required=True);p.add_argument('--output',type=Path,required=True);p.add_argument('--terms',type=Path)
    p=commands.add_parser('compare');p.add_argument('--packet',type=Path,required=True)
    p=commands.add_parser('freeze');p.add_argument('--packet',type=Path,required=True);p.add_argument('--output',type=Path,required=True)
    p=commands.add_parser('package');p.add_argument('--packet',type=Path,required=True);p.add_argument('--slot',choices=['a','b'],required=True);p.add_argument('--output',type=Path,required=True)
    args=parser.parse_args()
    if args.command=='prepare': result=prepare(args.source,args.output,json.loads(external(args.terms).read_text()) if args.terms else None)
    elif args.command=='compare': result=compare(args.packet)
    elif args.command=='freeze': result=freeze(args.packet,args.output)
    else: result=package_reviewer(args.packet,args.slot,args.output)
    print(json.dumps(result,indent=2))
