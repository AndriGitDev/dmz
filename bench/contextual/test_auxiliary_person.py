import unittest
try:
 import torch
except ImportError:
 torch=None
from auxiliary_person import person_targets, auxiliary_head

@unittest.skipIf(torch is None,'optional research PyTorch dependency')
class AuxiliaryTests(unittest.TestCase):
 def test_target_mapping_preserves_padding_and_input(self):
  source=torch.tensor([[-100,0,1,2,3,4,5,6,7,8]])
  self.assertEqual(person_targets(source).tolist(),[[-100,0,1,2,0,0,0,0,0,0]])
  self.assertEqual(source.tolist(),[[-100,0,1,2,3,4,5,6,7,8]])
  with self.assertRaises(ValueError):person_targets(torch.tensor([9]))
 def test_auxiliary_head_independent_and_primary_exact(self):
  torch.manual_seed(45)
  model=torch.nn.Module();model.head=torch.nn.Sequential(torch.nn.Linear(4,5),torch.nn.ReLU(),torch.nn.Linear(5,3))
  values=torch.randn(2,4);original=model.head(values).detach().clone()
  aux=auxiliary_head(model)
  self.assertTrue(torch.equal(original,model.head(values)))
  self.assertTrue(torch.equal(original,aux(values)[:,:3]))
  self.assertEqual(model.head[-1].out_features,3)
  self.assertFalse(set(map(id,model.head.parameters()))&set(map(id,aux.parameters())))
  with torch.no_grad():aux[-1].weight.add_(1)
  self.assertTrue(torch.equal(original,model.head(values)))
 def test_auxiliary_gradient_reaches_shared_representation_only(self):
  primary=torch.nn.Linear(4,3);auxiliary=torch.nn.Linear(4,9)
  hidden=torch.randn(2,4,requires_grad=True)
  target=torch.tensor([1,4]);other=torch.nn.functional.cross_entropy(auxiliary(hidden),target)
  other.backward()
  self.assertGreater(hidden.grad.abs().sum().item(),0)
  self.assertIsNone(primary.weight.grad)

if __name__=='__main__':unittest.main()
