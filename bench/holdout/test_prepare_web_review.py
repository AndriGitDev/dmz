import json
from pathlib import Path
import subprocess
import sys
from tempfile import TemporaryDirectory
import unittest
from collect import sha


class WebPacketTests(unittest.TestCase):
    def run_packet(self, mutation=None):
        with TemporaryDirectory() as tmp:
            root = Path(tmp); collection = root / 'collection'; collection.mkdir()
            text = 'Þetta er tilbúinn íslenskur prófunartexti. ' * 8
            row = dict(document_id='fabricated', page=1, text=text, text_sha256=sha(text.encode()),
                       document_byte_start=0, document_byte_end=len(text.encode()), entities=None, reviewed=False,
                       stratum='general', genre='fabricated', source_url='https://example.com/test',
                       overlapping_mim_12grams=0, eligible_for_final=False)
            manifest = dict(model_evaluated=False, training_use=False, documents=[dict(id='fabricated',
                text_sha256=sha(text.encode()), publisher='fabricated', overlapping_reference_12grams=0)])
            if mutation: mutation(row, manifest)
            (collection / 'fabricated.txt').write_text(text)
            data = json.dumps(row) + '\n'; (collection / 'review.jsonl').write_text(data)
            manifest['review_sha256'] = sha(data.encode())
            (collection / 'manifest.json').write_text(json.dumps(manifest))
            result = subprocess.run([sys.executable, str(Path(__file__).with_name('prepare_web_review.py')),
                '--collection', str(collection), '--output', str(root / 'packet')], capture_output=True)
            if result.returncode == 0:
                packet = json.loads((root / 'packet/annotation-packet/annotations.jsonl').read_text())
                self.assertIsNone(packet['entities']); self.assertFalse(packet['reviewed'])
            return result.returncode

    def test_blind_packet_keeps_null_labels(self):
        self.assertEqual(self.run_packet(), 0)

    def test_model_exposure_rejected(self):
        self.assertNotEqual(self.run_packet(lambda row, m: m.update(model_evaluated=True)), 0)

    def test_changed_excerpt_boundaries_rejected(self):
        self.assertNotEqual(self.run_packet(lambda row, m: row.update(document_byte_start=1)), 0)


if __name__ == '__main__': unittest.main()
