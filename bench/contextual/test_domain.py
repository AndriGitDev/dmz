"""Validate the frozen synthetic probe's two UTF-8 annotation representations."""
import json
from pathlib import Path
import unittest
from ner import tokens_and_labels


class DomainTests(unittest.TestCase):
    def test_annotations_and_negative_counts(self):
        path = Path(__file__).resolve().parents[1]/'corpus/domain/legal-financial-v1.jsonl'
        rows = [json.loads(line) for line in path.read_text().splitlines()]
        self.assertEqual(len(rows), 40)
        self.assertEqual(len({r['id'] for r in rows}), 40)
        self.assertEqual(sum(not r['entities'] for r in rows), 10)
        self.assertEqual(sum(len(r['entities']) for r in rows), 37)
        for row in rows:
            self.assertEqual(row['split'], 'test')
            tokens_and_labels(row)
            self.assertEqual(len(row['entities']), len(row['original_entities']))
            for normalized, original in zip(row['entities'], row['original_entities']):
                a = row['text'].encode()[normalized['start']:normalized['end']].decode()
                b = row['original_text'].encode()[original['start']:original['end']].decode()
                self.assertEqual(a.replace(' ', ''), b.replace(' ', ''))
                self.assertEqual(original['kind'], 'Person')
                self.assertTrue(b)


if __name__ == '__main__': unittest.main()
