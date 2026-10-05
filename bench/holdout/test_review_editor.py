import base64
import hashlib
import json
from pathlib import Path
import re
from tempfile import TemporaryDirectory
import unittest
import zipfile

from entity_review import prepare, package_reviewer, sha


class ReviewEditorTests(unittest.TestCase):
    def test_editor_is_self_contained_hash_bound_and_escapes_source_markup(self):
        with TemporaryDirectory() as directory:
            root=Path(directory); source=root/'source.jsonl'; packet=root/'packet'
            text='🙂 Ása Demo\r\n</script><img src="https://example.invalid/private">'
            source.write_text(json.dumps(dict(document_id='synthetic',page=1,text=text,text_sha256=sha(text.encode()),entities=None,reviewed=False))+'\n')
            prepare([source],packet)
            report=package_reviewer(packet,'a',root/'a.zip')
            self.assertEqual((root/'a.zip').stat().st_mode & 0o077,0)
            with zipfile.ZipFile(root/'a.zip') as bundle:
                html=bundle.read('review.html').decode()
                metadata=json.loads(bundle.read('packet.json'))
                self.assertEqual(metadata['editor_sha256'],sha(html.encode()))
                self.assertEqual(report['editor_sha256'],metadata['editor_sha256'])
                self.assertNotIn('reviewer-b.jsonl',bundle.namelist())
                self.assertNotIn('adjudication.jsonl',bundle.namelist())
                payload=json.loads(re.search(r'<script id="packet" type="application/json">(.*?)</script>',html,re.S).group(1))
                self.assertEqual(payload['texts']['excerpt-0001'],text)
                self.assertNotIn('<img src=',html)
                self.assertNotIn(str(root),html)
                self.assertEqual(payload['slot'],'a')
                self.assertIsNone(json.loads(payload['form_jsonl'])['entities'])
                script=re.search(r'<script>(.*?)</script>',html,re.S).group(1)
                style=re.search(r'<style>(.*?)</style>',html,re.S).group(1)
                for kind,content in [('script',script),('style',style)]:
                    digest=base64.b64encode(hashlib.sha256(content.encode()).digest()).decode()
                    self.assertIn(f"{kind}-src 'sha256-{digest}'",html)
                self.assertIn("connect-src 'none'",html)
                self.assertNotIn('<script src=',html)
                self.assertNotIn('localStorage',script)
                self.assertNotIn('indexedDB',script)


if __name__=='__main__':unittest.main()
