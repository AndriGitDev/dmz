import json
from pathlib import Path
import tempfile
import unittest
from audit_composite import audit
from encoder import digest


class CompositeAuditTests(unittest.TestCase):
    def test_overlap_priority_is_not_misreported_as_threshold_loss(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);source=root/'dev.jsonl';training=root/'train.jsonl'
            row=dict(split='dev',text='Ása kom',entities=[dict(start=0,end=4,kind='Person')])
            source.write_text(json.dumps(row)+'\n');row['split']='train';training.write_text(json.dumps(row)+'\n')
            def cache(path,predictions):
                path.write_text(json.dumps(dict(input_sha256=digest(source),model_sha256='fabricated',split='dev',
                    text_mode='canonical',records=[dict(gold=[[0,4]],predictions=predictions)])))
            crf=root/'crf.json';neural=root/'neural.json'
            cache(crf,[[0,8,.9]]);cache(neural,[[0,4,.9]])
            result=audit(source,training,crf,neural)
            self.assertEqual(result['false_negatives'],{'correct_neural_span_blocked_by_crf_overlap':1})
            self.assertEqual(result['false_positives'],{'boundary_or_merged_name':1})
            cache(crf,[[0,4,.5]]);cache(neural,[])
            self.assertEqual(audit(source,training,crf,neural)['false_negatives'],{'exact_crf_span_below_threshold':1})


if __name__=='__main__':unittest.main()
