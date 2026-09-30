import unittest
from tempfile import TemporaryDirectory
from pathlib import Path
from collect_web import canonical, extract, segments, Fetcher


class WebCollectionTests(unittest.TestCase):
    def test_prose_excludes_navigation_and_keeps_unicode(self):
        body = ('<meta property="og:title" content="Íslensk prófun"><main><nav>OUTSIDE</nav><h1>Íslensk prófun</h1><article><p>' +
                'Þetta er tilbúinn íslenskur texti um rannsóknir og samfélag. ' * 12 +
                '</p><footer>FOOTER</footer></article></main>').encode()
        text, selector, dates = extract(body)
        self.assertNotIn('OUTSIDE', text)
        self.assertNotIn('FOOTER', text)
        self.assertIn('Íslensk prófun', text)
        self.assertEqual(selector, 'article')

    def test_segments_are_exact_nonoverlapping_substrings(self):
        text = ('Örn er tilbúið dæmi. ' * 160 + '\n') * 4
        previous = 0
        for start, end, excerpt in segments(text):
            self.assertGreaterEqual(start, previous)
            self.assertEqual(text[start:end], excerpt)
            begin = len(text[:start].encode()); finish = len(text[:end].encode())
            self.assertEqual(text.encode()[begin:finish].decode(), excerpt)
            self.assertLessEqual(len(excerpt), 1800)
            previous = end

    def test_url_restrictions(self):
        self.assertEqual(canonical('https://example.com:443/a#b'), 'https://example.com/a')
        for url in ['http://example.com', 'https://user:pass@example.com', 'file:///tmp/a']:
            with self.assertRaises(ValueError): canonical(url)

    def test_robots_denial_prevents_article_request(self):
        with TemporaryDirectory() as d:
            fetcher = Fetcher(Path(d)); calls = []
            def request(url, delay=1):
                calls.append(url)
                return 200, {}, b'User-agent: *\nDisallow: /private\n'
            fetcher.request = request
            with self.assertRaisesRegex(ValueError, 'robots disallowed'):
                fetcher.get('https://example.com/private/article')
            self.assertEqual(calls, ['https://example.com/robots.txt'])

    def test_lowercase_content_type_is_accepted(self):
        with TemporaryDirectory() as d:
            fetcher = Fetcher(Path(d))
            fetcher.request = lambda url, delay=1: ((404, {}, b'') if url.endswith('robots.txt') else
                (200, {'content-type': 'text/html; charset=utf-8'}, b'<html>synthetic</html>'))
            url, headers, body = fetcher.get('https://example.com/article')
            self.assertEqual(body, b'<html>synthetic</html>')

    def test_cross_publisher_redirect_rejected(self):
        with TemporaryDirectory() as d:
            fetcher = Fetcher(Path(d))
            fetcher.request = lambda url, delay=1: ((404, {}, b'') if url.endswith('robots.txt') else
                (302, {'location': 'https://elsewhere.example/article'}, b''))
            with self.assertRaisesRegex(ValueError, 'cross-publisher'):
                fetcher.get('https://example.com/article')


if __name__ == '__main__': unittest.main()
