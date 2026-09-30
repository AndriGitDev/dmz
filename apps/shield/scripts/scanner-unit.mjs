import assert from 'node:assert/strict';
import { wordsFromText, spans, mergeNames, features } from '../scanner/text.mjs';
assert.deepEqual(wordsFromText('Örn, Anna!').offsets, [
  [0, 4],
  [4, 5],
  [6, 10],
  [10, 11],
]);
assert.deepEqual(wordsFromText('Jean-Luc O’Neil').words, ['Jean-Luc', 'O’Neil']);
for (const input of ['x\u200by', 'a\u0301', '\ud800', 'a'.repeat(16001)])
  assert.throws(() => wordsFromText(input));
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
