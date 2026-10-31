import assert from 'node:assert/strict';
import { wordsFromText, spans, mergeNames, features } from '../scanner/text.mjs';
assert.deepEqual(wordsFromText('Örn, Anna!').offsets, [
  [0, 4],
  [4, 5],
  [6, 10],
  [10, 11],
]);
assert.deepEqual(wordsFromText('Jean-Luc O’Neil').words, ['Jean-Luc', 'O’Neil']);
for (const input of ['x\u200by', 'x\u0301', '\u0301a', '\ud800', 'a'.repeat(16001)])
  assert.throws(() => wordsFromText(input));
assert.deepEqual(wordsFromText('A\u0301sa, O\u0308rn!'), {
  words: ['Ása', ',', 'Örn', '!'], offsets: [[0, 5], [5, 6], [7, 12], [12, 13]],
});
for (const letter of 'ÁÉÍÓÚÝÖáéíóúýöÅǺậḗ각') {
  const word = letter.normalize('NFD') + 'sa';
  const text = '😀 ' + word + ', demo.';
  const actual = wordsFromText(text);
  assert.deepEqual(actual.words, ['😀', letter + 'sa', ',', 'demo', '.']);
  assert.deepEqual(actual.offsets[1], [5, 5 + Buffer.byteLength(word)]);
  for (const [i, [start, end]] of actual.offsets.entries())
    assert.equal(Buffer.from(text).subarray(start, end).toString().normalize('NFC'), actual.words[i]);
}
assert.deepEqual(wordsFromText('Ångström').words, ['Ångström']);
assert.deepEqual(wordsFromText('ﬃ １２').words, ['ﬃ', '１２'], 'NFC must not become compatibility folding');
const offsets = [
  [0, 4],
  [5, 9],
  [10, 15],
];
assert.deepEqual(spans(['I', 'I', 'B'], offsets), [
  { start: 0, end: 9 },
  { start: 10, end: 15 },
]);
assert.deepEqual(spans(['B', 'I', 'O'], offsets, [0.9, 0.74, 0.99]), []);
assert.deepEqual(
  mergeNames(
    [{ start: 0, end: 9 }],
    [
      { start: 5, end: 15 },
      { start: 20, end: 24 },
    ],
  ),
  [
    { start: 0, end: 9 },
    { start: 20, end: 24 },
  ],
);
assert.ok(features(['J', '.'])[0].includes('0:initial_before_dot=True'));
console.log('UTF-8 bounds, unsupported input, BIO spans and composition checks passed');
