from collections import Counter
import unittest
from audit import characteristics, error_categories
from ner import spans, tokens_and_labels


class AuditTests(unittest.TestCase):
    def test_exclusive_error_reasons_and_boundary_double_penalty(self):
        gold={(0,10),(20,25),(30,40)}
        candidates={(0,5):.9,(20,25):.3,(50,60):.9,(70,75):.9}
        selected={s for s,c in candidates.items() if c>=.65}
        fn,fp=error_categories(gold,selected,candidates,{(50,60):'Organization'})
        self.assertEqual(fn,{(0,10):'boundary_or_merged_name',(20,25):'removed_by_confidence_threshold',(30,40):'no_name_prediction'})
        self.assertEqual(fp,{(0,5):'boundary_or_merged_name',(50,60):'other_entity_Organization',(70,75):'unannotated_as_entity'})

    def test_inventory_groups_do_not_invent_case_analysis(self):
        inventory=(Counter({'ása gervi':1}),Counter({'ása':1,'gervi':1}),Counter({'ása':1,'gervi':4}))
        self.assertIn('full_name_seen_in_training',characteristics(['Ása','Gervi'],inventory))
        groups=characteristics(['Ókunn'],inventory)
        self.assertIn('unseen_word',groups)
        self.assertIn('single_token',groups)
        self.assertNotIn('oblique_patronymic_shape',groups)

    def test_typed_labels_and_person_projection(self):
        row=dict(text='Ása Gervi hjá Sýniverki',entities=[dict(kind='Person',start=0,end=len('Ása Gervi'.encode())),dict(kind='Organization',start=len('Ása Gervi hjá '.encode()),end=len('Ása Gervi hjá Sýniverki'.encode()))])
        _,offsets,labels=tokens_and_labels(row,'entities')
        self.assertEqual(labels,['B-Person','I-Person','O','B-Organization'])
        self.assertEqual(spans(labels,offsets),[(0,len('Ása Gervi'.encode()),0,2)])
        with self.assertRaises(ValueError):spans(['B-Unknown'],[(0,5)])
        with self.assertRaises(ValueError):tokens_and_labels(row,'unknown')


if __name__=='__main__':unittest.main()
