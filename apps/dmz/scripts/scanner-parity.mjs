import assert from 'node:assert/strict';
import { readFileSync, createReadStream } from 'node:fs';
import { createInterface } from 'node:readline';
import { Tokenizer, wordsFromText, features, crfPredict } from '../scanner/text.mjs';
const [assets, fixtures] = process.argv.slice(2);
const tokenizer = new Tokenizer(JSON.parse(readFileSync(assets + '/tokenizer.json')));
const crf = JSON.parse(readFileSync(assets + '/crf.json'));
let records = 0,
  words = 0,
  maxMarginalError = 0;
for await (const line of createInterface({
  input: createReadStream(fixtures),
  crlfDelay: Infinity,
})) {
  const r = JSON.parse(line);
  if (r.text) {
    const actual = wordsFromText(r.text);
    assert.deepEqual(actual.words, r.words);
    assert.deepEqual(actual.offsets, r.offsets);
  }
  assert.deepEqual(features(r.words), r.features, 'CRF features');
  const prediction = crfPredict(r.words, crf);
  assert.deepEqual(prediction.labels, r.labels, 'CRF decode');
  prediction.confidence.forEach((v, i) => {
    const e = Math.abs(v - r.confidence[i]);
    maxMarginalError = Math.max(maxMarginalError, e);
    assert.ok(e < 0.0001, 'CRF marginal error');
    assert.equal(v >= 0.75, r.confidence[i] >= 0.75, 'threshold mismatch');
  });
  const chunks = tokenizer.chunks(r.words);
  assert.equal(chunks.length, r.chunks.length);
  chunks.forEach((c, i) => {
    assert.equal(c.offset, r.chunks[i].offset);
    assert.deepEqual(c.ids, r.chunks[i].ids, 'subword IDs');
    const ids = r.chunks[i].word_ids;
    for (let w = 0; w < c.words; w++) {
      const count = ids.filter((x) => x === w).length;
      for (let j = 0; j < ids.length; j++)
        assert.equal(
          c.pooling[w * ids.length + j],
          Math.fround(ids[j] === w ? 1 / count : 0),
          'pooling',
        );
    }
  });
  records++;
  words += r.words.length;
}
console.log(JSON.stringify({ records, words, maxMarginalError, allChecksPassed: true }));
