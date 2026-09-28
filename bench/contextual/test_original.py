import unittest
from ner import tokens_and_labels, spans


class OriginalTextTests(unittest.TestCase):
    def test_unicode_whitespace_and_initial_punctuation(self):
        raw='Til:\tÁsa  Þ. Gervi\r\nSkrá: virk.'
        name='Ása  Þ. Gervi';start=len('Til:\t'.encode());end=start+len(name.encode())
        row=dict(original_text=raw,original_entities=[dict(kind='Person',start=start,end=end)])
        words,offsets,labels=tokens_and_labels(row,text_mode='original')
        self.assertEqual(words,['Til',':','Ása','Þ','.','Gervi','Skrá',':','virk','.'])
        self.assertEqual([(a,b) for a,b,_,_ in spans(labels,offsets)],[(start,end)])
        for word,(a,b) in zip(words,offsets):self.assertEqual(raw.encode()[a:b].decode(),word)
        with self.assertRaises(ValueError):tokens_and_labels(row,text_mode='unknown')


if __name__=='__main__':unittest.main()
