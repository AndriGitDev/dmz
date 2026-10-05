import json
from pathlib import Path
from tempfile import TemporaryDirectory
import unittest
import zipfile

from entity_review import prepare, compare, freeze, load_packet, validate_entities, sha, jsonl, package_reviewer


class EntityReviewTests(unittest.TestCase):
    def test_reviewer_export_is_blind_and_refuses_completed_form(self):
        with TemporaryDirectory() as directory:
            root=Path(directory); _,packet=self.fixture(root)
            report=package_reviewer(packet,'a',root/'a.zip')
            self.assertFalse(report['model_predictions_included'])
            with zipfile.ZipFile(root/'a.zip') as archive:
                self.assertIn('reviewer-a.jsonl',archive.namelist())
                self.assertNotIn('reviewer-b.jsonl',archive.namelist())
                self.assertNotIn('adjudication.jsonl',archive.namelist())
            form=packet/'reviewer-a.jsonl'; row=jsonl(form)[0]; row['predictions']=[]
            form.write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(ValueError,'blank review'):
                package_reviewer(packet,'a',root/'unexpected.zip')
            self.complete(packet)
            with self.assertRaisesRegex(ValueError,'blank review'):
                package_reviewer(packet,'b',root/'b.zip')

    def fixture(self, root, overlap=0):
        text = 'Ása býr við Gervigata 99.'
        source = root/'source.jsonl'; packet = root/'packet'
        source.write_text(json.dumps(dict(document_id='fabricated', page=1, text=text,
            text_sha256=sha(text.encode()), entities=None, reviewed=False, overlapping_mim_12grams=overlap,
            stratum='general'))+'\n')
        prepare([source], packet)
        return source, packet

    def complete(self, packet):
        for name, role, person in [('reviewer-a.jsonl','reviewer','a'), ('reviewer-b.jsonl','reviewer','b'),
                                   ('adjudication.jsonl','adjudicator','c')]:
            path=packet/name; row=jsonl(path)[0]
            row.update(reviewed=True, extraction_verified=True, entities=[dict(start=0,end=4,kind='Person')])
            row[role]=person
            if role=='adjudicator': row['decision_note']='Synthetic fixture only; all types reviewed.'
            path.write_text(json.dumps(row)+'\n')

    def test_incomplete_packet_never_becomes_negative_gold(self):
        with TemporaryDirectory() as directory:
            root=Path(directory); source,packet=self.fixture(root)
            self.assertEqual(load_packet(packet)[0]['excerpts'],1)
            with self.assertRaisesRegex(ValueError,'incomplete human'):
                compare(packet)
            self.assertFalse((root/'gold').exists())
            self.complete(packet)
            self.assertEqual(compare(packet)['exact_agreements'],1)
            report=freeze(packet,root/'gold')
            self.assertEqual(report['entities'],{'Person':1})
            self.assertFalse(report['independent_final_evaluation'])
            self.assertNotIn('Ása',json.dumps(report,ensure_ascii=False))
            self.assertFalse(jsonl(root/'gold/gold.jsonl')[0]['eligible_for_final'])
            with self.assertRaises(FileExistsError): freeze(packet,root/'gold')

    def test_two_reviewers_and_distinct_adjudicator_required(self):
        with TemporaryDirectory() as directory:
            root=Path(directory); _,packet=self.fixture(root); self.complete(packet)
            path=packet/'reviewer-b.jsonl'; row=jsonl(path)[0]; row['reviewer']=' A '
            path.write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(ValueError,'distinct reviewer'): compare(packet)
            self.complete(packet)
            path=packet/'adjudication.jsonl'; row=jsonl(path)[0]; row['adjudicator']='b'
            path.write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(ValueError,'distinct adjudicator'): freeze(packet,root/'gold')

    def test_disagreement_requires_explicit_adjudication(self):
        with TemporaryDirectory() as directory:
            root=Path(directory); _,packet=self.fixture(root); self.complete(packet)
            path=packet/'reviewer-b.jsonl'; row=jsonl(path)[0]; row['entities']=[]
            path.write_text(json.dumps(row)+'\n')
            self.assertEqual(compare(packet)['disagreements'],1)
            path=packet/'adjudication.jsonl'; row=jsonl(path)[0]; row['decision_note']=None
            path.write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(ValueError,'explicit adjudication'): freeze(packet,root/'gold')

    def test_byte_bounds_null_overlap_and_scope(self):
        data='Ása Demo'.encode()
        for entities in [None, [None], [dict(start=True,end=4,kind='Person')],
                         [dict(start=1,end=4,kind='Person')], [dict(start=0,end=99,kind='Person')],
                         [dict(start=0,end=4,kind='Unknown')],
                         [dict(start=0,end=4,kind='Person'),dict(start=0,end=4,kind='Person')]]:
            with self.subTest(entities=entities), self.assertRaises((ValueError,UnicodeDecodeError)):
                validate_entities(entities,data)
        self.assertEqual(validate_entities([],data),[])

    def test_exclusion_is_not_a_negative_label(self):
        with TemporaryDirectory() as directory:
            root=Path(directory); _,packet=self.fixture(root); self.complete(packet)
            path=packet/'reviewer-a.jsonl'; row=jsonl(path)[0]; row['exclusion_reason']='Unusable extraction'
            path.write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(ValueError,'negative labels'): compare(packet)
            row['entities']=None;path.write_text(json.dumps(row)+'\n')
            self.assertEqual(compare(packet)['exclusion_disagreements'],1)

    def test_source_text_and_rubric_tampering_rejected(self):
        for what in ('source','text','rubric','index'):
            with self.subTest(what=what), TemporaryDirectory() as directory:
                root=Path(directory); source,packet=self.fixture(root)
                path={'source':source,'text':packet/'texts/excerpt-0001.txt','rubric':packet/'rubric.md','index':packet/'index.jsonl'}[what]
                path.write_text('changed')
                with self.assertRaises(ValueError):load_packet(packet)

    def test_unconfigured_tenant_label_and_missing_review_rejected(self):
        with TemporaryDirectory() as directory:
            root=Path(directory); _,packet=self.fixture(root); self.complete(packet)
            path=packet/'adjudication.jsonl';row=jsonl(path)[0];row['entities'][0]['kind']='TenantTerm'
            path.write_text(json.dumps(row)+'\n')
            with self.assertRaisesRegex(ValueError,'configured term'):freeze(packet,root/'gold')
            (packet/'reviewer-a.jsonl').write_text('')
            with self.assertRaisesRegex(ValueError,'coverage'):compare(packet)

    def test_prior_overlap_requires_written_resolution(self):
        with TemporaryDirectory() as directory:
            root=Path(directory); _,packet=self.fixture(root, overlap=1); self.complete(packet)
            path=packet/'adjudication.jsonl'; row=jsonl(path)[0]
            for resolution in (None, True, [], '   '):
                row['overlap_resolution']=resolution; path.write_text(json.dumps(row)+'\n')
                with self.assertRaisesRegex(ValueError,'unresolved prior-text overlap'):
                    freeze(packet,root/'gold')
            row['overlap_resolution']='Synthetic test of a recorded decision; not evidence of independence.'
            path.write_text(json.dumps(row)+'\n')
            self.assertFalse(freeze(packet,root/'gold')['independent_final_evaluation'])


if __name__=='__main__':unittest.main()
