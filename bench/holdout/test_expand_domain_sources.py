import json
import unittest

from expand_domain_sources import domain_sources


class DomainSourcesTest(unittest.TestCase):
    def test_frozen_selection_excludes_prior_and_index_links(self):
        urls = [f'https://yskn.is/urskurdir/skoda-urskurd/?nr={i}' for i in range(60)]
        indexes = {'yskn-2026': json.dumps(urls+['https://yskn.is/urskurdir/?year=2025',
                       'https://yskn.is/urskurdir/skoda-urskurd/?nr=90&other=1',
                       'https://yskn.is/urskurdir/skoda-urskurd/?nr=invalid']).encode(),
                   'yskn-2025': json.dumps(list(reversed(urls))).encode(),
                   'islandssjodir-news': json.dumps(['https://islandssjodir.is/frett/demo/',
                       'https://islandssjodir.is/frettir/']).encode()}
        prior = [{'documents': [{'url': urls[0], 'resolved_url': urls[1]}]}]
        result = domain_sources(indexes, prior)
        self.assertEqual(len(result['documents']), 41)
        self.assertEqual(len({d['url'] for d in result['documents']}), 41)
        self.assertFalse(set(urls[:2]) & {d['url'] for d in result['documents']})
        self.assertEqual({d['stratum'] for d in result['documents']}, {'legal', 'financial'})
        self.assertEqual(result, domain_sources(indexes, prior))
        self.assertTrue(all(d['discovery_keys'] == ['yskn-2025', 'yskn-2026']
                            for d in result['documents'] if d['publisher'] == 'yskn'))


if __name__ == '__main__':
    unittest.main()
