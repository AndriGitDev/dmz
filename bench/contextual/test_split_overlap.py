import unittest
import json
import tempfile
from pathlib import Path
from split_overlap import audit,sequence_key,load

class SplitOverlapTests(unittest.TestCase):
 def test_normalization_and_case_are_separate(self):
  result=audit([(('Ása',),('B-Person',))],{'dev':[(('A\u0301sa',),('B-Person',)),(('ása',),('O',))]})['comparisons']
  self.assertEqual(result['exact_tokens']['strata']['dev']['overlapping_rows'],0)
  self.assertEqual(result['nfc_tokens']['strata']['dev']['overlapping_rows'],1)
  folded=result['nfc_casefold_tokens']['strata']['dev']
  self.assertEqual(folded['overlapping_rows'],2);self.assertEqual(folded['overlapping_person_positive_rows'],1)
  self.assertEqual(folded['overlap_label_sequence_not_seen_in_training'],1)
 def test_duplicates_and_multiple_labelings_not_conflated(self):
  same=(('Example',),('B-Organization',));other=(('Example',),('B-Miscellaneous',))
  result=audit([same,same,other],{'dev':[]})['comparisons']['exact_tokens']
  self.assertEqual(result['unique_training_sequences'],1)
  self.assertEqual(result['training_duplicate_sequences'],1)
  self.assertEqual(result['training_sequences_with_multiple_full_labelings'],1)
 def test_short_common_sequences_and_strata_remain_separate(self):
  short=(('one',),('O',));long=(tuple('abcde'),('O',)*5)
  result=audit([short,long],{'general':[short,long],'domain':[]})['comparisons']['exact_tokens']['strata']
  self.assertEqual(result['general']['overlapping_rows_at_least_five_tokens'],1)
  self.assertEqual(result['domain']['overlapping_rows'],0)
 def test_loader_projects_other_annotated_types_to_background(self):
  with tempfile.TemporaryDirectory() as directory:
   path=Path(directory)/'dev.jsonl'
   path.write_text(json.dumps(dict(split='dev',text='2026',entities=[dict(start=0,end=4,kind='Date')]))+'\n')
   data,digest=load(path,'dev')
   self.assertEqual(data,[(('2026',),('O',))]);self.assertEqual(len(digest),64)
   with self.assertRaises(ValueError):load(path,'train')
 def test_ignored_type_still_requires_aligned_boundaries(self):
  with tempfile.TemporaryDirectory() as directory:
   path=Path(directory)/'dev.jsonl'
   path.write_text(json.dumps(dict(split='dev',text='2026',entities=[dict(start=1,end=4,kind='Date')]))+'\n')
   with self.assertRaises(ValueError):load(path,'dev')
 def test_invalid_sequences_and_modes_rejected(self):
  for row in [((),()),(('x',),()),(('x',),('unknown',))]:
   with self.assertRaises(ValueError):audit([row],{})
  with self.assertRaises(ValueError):sequence_key(['x'],'loose')

if __name__=='__main__':unittest.main()
