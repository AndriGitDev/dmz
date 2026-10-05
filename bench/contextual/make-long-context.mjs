// Construct synthetic chunk-boundary controls without any model inference.
import { readFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import assert from 'node:assert/strict';
import { Tokenizer, wordsFromText } from '../../apps/shield/scanner/text.mjs';
const [tokenizerPath] = process.argv.slice(2);
if (!tokenizerPath) throw Error('Pass the pinned private tokenizer.json');
const root = fileURLToPath(new URL('../../', import.meta.url));
const bytes = await readFile(tokenizerPath), sha = value => createHash('sha256').update(value).digest('hex');
assert.equal(sha(bytes), '0305755b907b7a616fed8aa81aa0d12fe45a0916111988189b8646cc066fe20a');
const tokenizer = new Tokenizer(JSON.parse(bytes));
// This offline cache contains only this generator's fabricated words, never prompts.
const pieces = new Map(), encode = tokenizer.encodeWord.bind(tokenizer);
tokenizer.encodeWord = word => {
  if (!pieces.has(word)) pieces.set(word, encode(word));
  return pieces.get(word);
};
const names = ['Hildur Sýnisteinsdóttir', 'Rósa Prófvaldsdóttir', 'Björn Gervivaldsson',
  'Valur Prófvaldsson', 'Þórdís Sýnivaldsdóttir', 'Margrét Gervimarsdóttir', 'Jamie Testford', 'Robin Exampleton'];
const filler = 'Þetta er tilbúinn texti um almenna skráningu og afgreiðslu. ';
const intros = ['', 'Athugið. ', 'Til upplýsinga. ', 'Skráning stendur yfir. ', 'Þetta er dæmi. '];
const sets = Object.fromEntries(['short','crossing','interior','nfd'].map(k => [k, []]));
const metadata = [];
function inspect(text, name) {
  const { words, offsets } = wordsFromText(text), chunks = tokenizer.chunks(words);
  const result = { chars: text.length, bytes: Buffer.byteLength(text), words: words.length,
    chunk_word_offsets: chunks.map(c => c.offset), chunk_subwords: chunks.map(c => c.ids.length) };
  assert.ok(text.length <= 16000 && words.length <= 2048);
  if (name) {
    const charStart = text.indexOf(name); assert.ok(charStart >= 0 && text.indexOf(name, charStart + 1) === -1);
    const start = Buffer.byteLength(text.slice(0, charStart)), end = start + Buffer.byteLength(name);
    const a = offsets.findIndex(o => o[0] === start), last = offsets.findIndex(o => o[1] === end);
    assert.ok(a >= 0 && last >= a);
    Object.assign(result, { gold_start: start, gold_end: end, name_word_start: a, name_word_end: last + 1,
      crossing_cuts: chunks.map(c => c.offset).filter(c => a < c && c <= last),
      interior_distance: Math.min(...chunks.slice(1).map(c => Math.min(Math.abs(c.offset - a), Math.abs(c.offset - last - 1)))) });
  }
  return { metadata: result, words };
}
function add(group, index, text, name, extra = {}) {
  const id = `long-context-${group}-${index}`;
  const checked = inspect(text, name);
  sets[group].push({ id, text, entities: name ? [{ kind: 'name_demo', value: name }] : [] });
  const meta = { id, group, ...checked.metadata, ...extra };
  if (!Number.isFinite(meta.interior_distance)) delete meta.interior_distance;
  metadata.push(meta); return checked;
}
for (const [index, name] of names.entries()) {
  const clause = `Umsækjandi er ${name}. Beiðni bíður yfirferðar. `;
  add('short', index + 1, clause.trimEnd(), name);
  let chosen;
  for (const intro of intros) {
    for (let before = 0; before <= 24; before++) {
      const text = intro + filler.repeat(before) + clause + filler.repeat(24 - before);
      const checked = inspect(text, name);
      if (checked.metadata.crossing_cuts.length) { chosen = { intro, before, text, checked }; break; }
    }
    if (chosen) break;
  }
  assert.ok(chosen, 'No crossing layout for a declared synthetic name');
  const candidates = [];
  for (let before = 0; before <= 24; before++) {
    const text = chosen.intro + filler.repeat(before) + clause + filler.repeat(24 - before);
    const checked = inspect(text, name);
    if (!checked.metadata.crossing_cuts.length && checked.metadata.interior_distance >= 4)
      candidates.push({ before, text });
  }
  candidates.sort((a,b) => Math.abs(a.before-chosen.before)-Math.abs(b.before-chosen.before) || a.before-b.before);
  assert.ok(candidates.length, 'No paired interior control');
  const cross = add('crossing', index + 1, chosen.text, name, { filler_before: chosen.before });
  add('interior', index + 1, candidates[0].text, name, { filler_before: candidates[0].before });
  const nfd = add('nfd', index + 1, chosen.text.normalize('NFD'), name.normalize('NFD'), { filler_before: chosen.before });
  assert.deepEqual(nfd.words, cross.words);
  assert.deepEqual(nfd.metadata.chunk_word_offsets, cross.metadata.chunk_word_offsets);
}
add('short', 'negative', filler.trimEnd(), null);
for (const group of ['crossing','interior','nfd']) add(group, 'negative', group === 'nfd' ? filler.repeat(24).normalize('NFD') : filler.repeat(24), null);
const corpus = {};
for (const [group, rows] of Object.entries(sets)) {
  const name = `long-context-${group}-v3.jsonl`, content = rows.map(r => JSON.stringify(r)).join('\n') + '\n';
  await writeFile(resolve(root, 'bench/corpus', name), content, { flag: 'wx' });
  corpus[name] = sha(content);
}
await writeFile(resolve(root, 'bench/gates/long-context-v3.sha256'), Object.entries(corpus).map(([name,hash]) => `${hash}  bench/corpus/${name}\n`).join(''), { flag: 'wx' });
const report = { schema: 1, scope: 'Mechanical construction of reused synthetic long-prompt controls; no inference',
  tokenizer_sha256: sha(bytes), source_sha256: {}, corpus_sha256: corpus, records: metadata, independent_validation: false };
for (const name of ['bench/contextual/make-long-context.mjs','bench/contextual/long-context-protocol.md','apps/shield/scanner/text.mjs'])
  report.source_sha256[name] = sha(await readFile(resolve(root,name)));
await writeFile(resolve(root, 'bench/results/quality-99-v1/long-context-construction.json'), JSON.stringify(report,null,2)+'\n', { flag: 'wx' });
console.log(JSON.stringify({ corpora: Object.keys(corpus).length, cases: metadata.length,
  crossing_cases: metadata.filter(r => r.crossing_cuts?.length).length }));
