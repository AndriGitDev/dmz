import json
import unittest

from expand_web_sources import select, SPECS


class ExpansionTests(unittest.TestCase):
    def indexes(self):
        return {name: b'[]' for _, names, *_ in SPECS for name in names}

    def test_excludes_prior_urls_and_nonarticles_before_inference(self):
        indexes = self.indexes()
        indexes['hi'] = json.dumps(['https://www.hi.is/', 'https://www.hi.is/node/1',
                                    'https://www.hi.is/node/2#fragment', 'https://elsewhere.example/node/3']).encode()
        indexes['hi-page2'] = json.dumps(['https://www.hi.is/node/2']).encode()
        result = select(indexes, [dict(documents=[dict(url='https://www.hi.is/node/1')])])
        self.assertEqual(len(result['documents']), 1)
        self.assertEqual(result['documents'][0]['url'], 'https://www.hi.is/node/2')
        self.assertEqual(result['documents'][0]['discovery_keys'], ['hi', 'hi-page2'])
        self.assertEqual(result, select(indexes, [dict(documents=[dict(url='https://www.hi.is/node/1')])]))

    def test_bounded_and_empty_selection(self):
        indexes = self.indexes()
        with self.assertRaises(ValueError): select(indexes, [])
        indexes['hi'] = json.dumps([f'https://www.hi.is/node/{i}' for i in range(100)]).encode()
        self.assertEqual(len(select(indexes, [], 7)['documents']), 7)
        with self.assertRaises(ValueError): select(indexes, [], 1000)


if __name__ == '__main__': unittest.main()
