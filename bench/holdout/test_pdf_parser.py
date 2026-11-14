import io
from pathlib import Path
import subprocess
import sys
import unittest

from pypdf import PdfReader, PdfWriter


class PdfParserTests(unittest.TestCase):
    def make_pdf(self):
        writer = PdfWriter()
        writer.add_blank_page(width=72, height=72)
        writer.add_metadata({'/Title': 'Tilbúin PDF prófun'})
        output = io.BytesIO()
        writer.write(output)
        return output.getvalue()

    def test_synthetic_pdf_preserves_page_and_unicode_metadata(self):
        reader = PdfReader(io.BytesIO(self.make_pdf()))
        self.assertEqual(len(reader.pages), 1)
        self.assertEqual(reader.metadata.title, 'Tilbúin PDF prófun')
        self.assertEqual(reader.pages[0].extract_text(), '')

    def test_missing_xref_trailer_terminates(self):
        # A subprocess bound prevents a parser regression from hanging the suite.
        truncated = self.make_pdf().split(b'trailer', 1)[0] + b'%%EOF\n'
        code = 'import io,sys; from pypdf import PdfReader; PdfReader(io.BytesIO(sys.stdin.buffer.read()))'
        result = subprocess.run([sys.executable, '-c', code], input=truncated,
                                capture_output=True, timeout=5)
        self.assertNotEqual(result.returncode, 0)


if __name__ == '__main__':
    unittest.main()
