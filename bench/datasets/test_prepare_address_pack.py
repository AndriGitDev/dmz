import json
from pathlib import Path
import tempfile
import unittest

from prepare_address_pack import prepare


class AddressPackTests(unittest.TestCase):
    def test_address_only_pack_filters_numbered_records_and_normalizes(self):
        with tempfile.TemporaryDirectory() as directory:
            source, output = Path(directory) / 'forms.json', Path(directory) / 'pack.json'
            source.write_text(json.dumps(['Gerviborg', 'Gerviborg', 'Gervi  Slóð',
                                         'Gerviborg 99', '', 'invalid_secret']))
            report = prepare(source, output)
            self.assertEqual(report['addresses'], 2)
            self.assertEqual(json.loads(output.read_text()),
                             dict(schema=1, names=[], addresses=['Gervi Slóð', 'Gerviborg']))

    def test_empty_pack_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / 'forms.json'
            source.write_text('[]')
            with self.assertRaises(ValueError):
                prepare(source, Path(directory) / 'pack.json')
