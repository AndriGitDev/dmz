import unittest
import test_compare_quality as fixtures
from auxiliary_person import SETTINGS, INITIAL_SHA
from compare_auxiliary import compare
from finetune import ENTITY_DECODE

class AuxiliaryComparisonTests(unittest.TestCase):
 def fixture(self):
  training,candidates,baselines,reference=fixtures.ComparisonTests().fixture()
  training.update(label_mode='person',labels=['O','B','I'],auxiliary_labels=ENTITY_DECODE,
   chunks=46236,settings=dict(SETTINGS),probe=False,primary_initialization_exact=True,
   warm_start=dict(checkpoint_sha256=INITIAL_SHA))
  training['checkpoints'][0].update(optimizer_steps=1445,tokens_seen=824869,auxiliary_sha256='auxiliary')
  reference['composition']['neural_sha256']=INITIAL_SHA
  for r in candidates[0].values():r['label_mode']='person'
  return training,candidates[0],baselines,reference
 def test_pass_is_only_research_and_targets_separate(self):
  result=compare(*self.fixture())
  self.assertTrue(result['research_gate_passed']);self.assertTrue(result['both_strata_point_targets_met'])
  self.assertFalse(result['browser_promotion_approved']);self.assertFalse(result['independent_final_evaluation'])
  args=self.fixture();args[1]['general']['quality']['combined']=fixtures.result(3)
  result=compare(*args)
  self.assertTrue(result['research_gate_passed']);self.assertFalse(result['both_strata_point_targets_met'])
 def test_equal_recall_and_precision_loss_fail(self):
  args=self.fixture();args[1]['general']['quality']['combined']=fixtures.result(4)
  self.assertIn('lower-stratum recall did not improve',compare(*args)['failures'])
  args=self.fixture();args[1]['domain']['quality']['combined']=fixtures.result(3)
  self.assertIn('domain accuracy below frozen reference',compare(*args)['failures'])
 def test_recipe_population_and_incomplete_trials_rejected(self):
  for mutation in [
   lambda a:a[0]['settings'].update(auxiliary_weight=.5),
   lambda a:a[0]['settings'].update(epochs=True),
   lambda a:a[0].update(probe=True),
   lambda a:a[0].update(primary_initialization_exact=False),
   lambda a:a[0]['checkpoints'][0].update(complete_epoch=False),
   lambda a:a[0]['checkpoints'][0].update(tokens_seen=1),
   lambda a:a[0]['checkpoints'].append(dict(a[0]['checkpoints'][0])),
   lambda a:a[1].pop('domain'),
   lambda a:a[1]['general'].update(crf_threshold=.6),
   lambda a:a[1]['domain'].update(label_mode='entities'),
   lambda a:a[1]['general'].update(rows=99),
   lambda a:a[1]['domain'].update(model_sha256='unknown'),
  ]:
   args=self.fixture();mutation(args)
   with self.assertRaises(ValueError):compare(*args)

if __name__=='__main__':unittest.main()
