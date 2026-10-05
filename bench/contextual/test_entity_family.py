import unittest
from entity_family import study, compose
import test_entity_veto as fixtures
from quality import score


class FamilyTests(unittest.TestCase):
    def test_neural_priority_preserves_exact_proposal(self):
        c,p,t=fixtures.EntityVetoTests().caches()
        c['records'][0]['predictions']=[[0,5,.9]]
        p['records'][0]['gold']=[[0,4]];c['records'][0]['gold']=[[0,4]];t['records'][0]['gold']=[[0,4]]
        t['records'][0]['predictions']=[[0,4,'Person',.8]]
        self.assertEqual(compose(c,p,t,.75,'crf')['quality']['overall']['tp'],0)
        self.assertEqual(compose(c,p,t,.75,'neural')['quality']['overall']['tp'],1)
    def test_complete_family_and_no_automatic_promotion(self):
        caches=fixtures.EntityVetoTests().caches()
        baseline=score([dict(gold=[(0,4,'Person'),(20,24,'Person')],predicted=[(0,4,'Person'),(20,24,'Person')])],['Person'])
        result=study([dict(epoch=e,model_sha256='nine',caches=dict(general=caches,domain=caches)) for e in [1,2]],dict(general=baseline,domain=baseline))
        self.assertEqual(result['configurations'],36)
        self.assertEqual(len({r['id'] for r in result['results']}),36)
        self.assertEqual(result['decision'],'retain frozen reference')
        self.assertFalse(result['browser_promotion_approved'])
        with self.assertRaises(ValueError):study([],dict(general=baseline,domain=baseline))
    def test_fixed_tie_break(self):
        c,p,t=fixtures.EntityVetoTests().caches()
        c['records'][0]['predictions']=[[0,4,.9],[20,24,.9]]
        t['records'][0]['predictions']=[[0,4,'Person',.8]]
        baseline=score([dict(gold=[(0,4,'Person'),(20,24,'Person')],predicted=[(0,4,'Person')])],['Person'])
        result=study([dict(epoch=e,model_sha256='nine',caches=dict(general=(c,p,t),domain=(c,p,t))) for e in [1,2]],dict(general=baseline,domain=baseline))
        chosen=next(r for r in result['results'] if r['id']==result['selected_research_configuration'])
        self.assertEqual(chosen['configuration']['epoch'],1)
        self.assertEqual(chosen['configuration']['crf_threshold'],.75)
        self.assertEqual(chosen['configuration']['priority'],'crf')
    def test_reject_undeclared_configuration(self):
        with self.assertRaises(ValueError):compose(*fixtures.EntityVetoTests().caches(),.73,'crf')
        with self.assertRaises(ValueError):compose(*fixtures.EntityVetoTests().caches(),.75,'blend')

if __name__=='__main__':unittest.main()
