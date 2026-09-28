import contextlib
import io
import json
from pathlib import Path
import tempfile
import unittest
import zipfile

from prepare import prepare, sentence


class PreparationTests(unittest.TestCase):
    def test_utf8_span_and_split_stability(self):
        item, repairs = sentence(['Til', 'Prófrúnar', 'Gervidóttur', '.'], ['O', 'B-Person', 'I-Person', 'O'])
        entity = item['entities'][0]
        self.assertEqual(item['text'].encode()[entity['start']:entity['end']].decode(), 'Prófrúnar Gervidóttur')
        self.assertEqual(repairs, 0)
        other, _ = sentence(['Til', 'Prófrúnar', 'Gervidóttur', '.'], ['O'] * 4)
        self.assertEqual(item['split'], other['split'])

    def test_orphan_inside_tag_is_counted(self):
        item, repairs = sentence(['Gervi'], ['I-Person'])
        self.assertEqual(repairs, 1)
        self.assertEqual(item['entities'], [dict(kind='Person', start=0, end=5)])
        with self.assertRaises(ValueError):
            sentence(['Gervi'], ['B-Unknown'])

    def test_import_strips_address_identifiers_and_deduplicates(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            hms, mim, out = root / 'hms.csv', root / 'mim.zip', root / 'out'
            hms.write_text('HEITI_NF,HEITI_TGF,POSTNR,HUSNR,HNIT\nPrófgata,Prófgötu,999,99,DO-NOT-COPY\n')
            text = 'Prófrún B-Person\nGervidóttir I-Person\n\n'
            with zipfile.ZipFile(mim, 'w') as z:
                z.writestr('fixture.txt', text + text + 'Broken O merged O\n\n')
            stdout = io.StringIO()
            with contextlib.redirect_stdout(stdout):
                prepare(hms, mim, out)
            report = json.loads(stdout.getvalue())
            self.assertEqual(report['mim']['duplicate_sentences_removed'], 1)
            self.assertEqual(len(report['mim']['quarantined_sentences']), 1)
            self.assertEqual(sum(report['mim']['sentences'].values()), 1)
            self.assertNotIn('Prófrún', stdout.getvalue())
            self.assertNotIn('DO-NOT-COPY', (out / 'address-forms.json').read_text())
            self.assertEqual(json.loads((out / 'address-forms.json').read_text()), ['Prófgata', 'Prófgötu'])
            with self.assertRaises(FileExistsError):
                prepare(hms, mim, out)


if __name__ == '__main__':
    unittest.main()
