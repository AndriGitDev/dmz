import copy
import unittest
from entity_veto import typed_spans, evaluate_veto, LABELS


class EntityVetoTests(unittest.TestCase):
    def test_complete_bio_types_and_orphan_i(self):
        labels=['I-Person','I-Person','B-Organization','I-Location','O','B-Miscellaneous']
        self.assertEqual(typed_spans(labels,[(0,2),(3,5),(6,8),(9,11),(12,14),(15,17)],[.9,.8,.7,.6,.5,.4]),
                         [[0,5,'Person',.8],[6,8,'Organization',.7],[9,11,'Location',.6],[15,17,'Miscellaneous',.4]])
    def test_invalid_tokens(self):
        for label,offset,confidence in [('O-Foo',(0,1),.5),('B-Unknown',(0,1),.5),('B-Person',(1,1),.5),('O',(0,1),float('nan'))]:
            with self.assertRaises(ValueError): typed_spans([label],[offset],[confidence])
        with self.assertRaises(ValueError): typed_spans(['O'],[],[.5])

    def caches(self):
        identity=dict(input_sha256='source',split='dev',text_mode='canonical')
        gold=[[0,4],[20,24]]
        crf=dict(**identity,model_sha256='crf',records=[dict(gold=gold,predictions=[[0,4,.9],[10,14,.8],[20,24,.9]])])
        person=dict(**identity,model_sha256='nine',records=[dict(gold=gold,predictions=[[0,4,.8]])])
        typed=dict(**identity,schema=1,label_mode='entities',labels=LABELS,model_sha256='nine',records=[dict(gold=gold,
            predictions=[[0,4,'Person',.8],[10,14,'Organization',.6],[20,24,'Location',.9]])])
        return crf,person,typed

    def test_exact_veto_counts_false_positive_and_harm(self):
        report=evaluate_veto(*self.caches())
        self.assertEqual(report['diagnostics'],dict(vetoed_crf=2,vetoed_gold_person=1,vetoed_false_person=1))
        self.assertEqual({k:report['quality']['overall'][k] for k in ['tp','fp','fn']},dict(tp=1,fp=0,fn=1))
        self.assertEqual(report['original_quality']['overall']['tp'],2)
    def test_partial_other_span_does_not_veto(self):
        crf,person,typed=self.caches()
        typed['records'][0]['predictions'][1][1]=13
        typed['records'][0]['predictions'].pop()
        result=evaluate_veto(crf,person,typed)
        self.assertEqual(result['diagnostics']['vetoed_crf'],0)
        self.assertEqual(result['quality'],result['original_quality'])
    def test_identity_and_person_parity_are_mandatory(self):
        for mutate in [lambda t:t.update(model_sha256='different'),lambda t:t.update(text_mode='original'),
                       lambda t:t['records'][0]['predictions'][0].__setitem__(3,.7),
                       lambda t:t['records'][0]['predictions'][1].__setitem__(0,2)]:
            crf,person,typed=self.caches();mutate(typed)
            with self.assertRaises(ValueError):evaluate_veto(crf,person,typed)
        crf,person,typed=self.caches();typed['records'].append(copy.deepcopy(typed['records'][0]))
        with self.assertRaises(ValueError):evaluate_veto(crf,person,typed)

if __name__ == '__main__': unittest.main()
