// Isolate a core change using verified, unchanged contextual-name predictions.
// This is cached composition, not a rerun of the neural model in Chromium.
import { readFileSync, writeFileSync, realpathSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { resolve, relative, dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import vm from 'node:vm';
import assert from 'node:assert/strict';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const [corpusArg, referenceArg, wasmArg, outArg] = process.argv.slice(2);
if (!outArg) throw Error('Usage: replay_core.mjs CORPUS PRIVATE_REFERENCE WASM PRIVATE_OUTPUT');
const corpus = realpathSync(corpusArg), out = resolve(outArg);
if (!relative(root, realpathSync(dirname(out))).startsWith('../')) throw Error('Private output required');
const hash = b => createHash('sha256').update(b).digest('hex');
const manifest = JSON.parse(readFileSync(join(corpus, 'manifest.json')));
const referenceBytes = readFileSync(referenceArg), reference = JSON.parse(referenceBytes);
assert.equal(reference.status, 'complete');
assert.equal(reference.external_requests, 0);
assert.deepEqual(reference.corpus, manifest);
const byID = new Map(reference.records.map(r => [r.id, r]));
assert.equal(byID.size, reference.records.length);
const rows = [];
if (corpus.startsWith(join(root, 'bench/corpus/'))) {
  for (const [name, entry] of Object.entries(manifest.sets)) {
    const raw = readFileSync(join(corpus, name)); assert.equal(hash(raw), entry.sha256);
    rows.push(...raw.toString().trim().split('\n').map(JSON.parse));
  }
} else {
  assert.equal(manifest.source.revision, 'e06eb1499ca8d54470f085021cd8e54f9efac7fd');
  const raw = readFileSync(join(corpus, 'test.jsonl'));
  assert.equal(hash(raw), '6b70350932be463af6b53fc79bee5e53be2e66cc4133ae9bd0ff672d2237ad29');
  rows.push(...raw.toString().trim().split('\n').map(JSON.parse));
}
assert.equal(new Set(rows.map(r => r.id)).size, byID.size);
const runtime = readFileSync(join(root, 'apps/shield/public/wasm_exec.js'));
vm.runInThisContext(runtime.toString());
const wasm = readFileSync(wasmArg), go = new globalThis.Go();
const { instance } = await WebAssembly.instantiate(wasm, go.importObject);
void go.run(instance);
for (let i = 0; i < 200 && typeof globalThis.dmzProcessBestEffort !== 'function'; i++) await new Promise(r => setTimeout(r, 10));
assert.equal(typeof globalThis.dmzProcessBestEffort, 'function');
const times = [], records = [], began = performance.now();
for (const row of rows) {
  const original = byID.get(row.id); assert.ok(original);
  assert.equal(original.text_sha256, hash(row.text));
  assert.equal(original.stratum, row.stratum);
  const record = { id: row.id, text_sha256: original.text_sha256, stratum: row.stratum,
    names: original.names, scanner_code: original.scanner_code };
  for (const mode of ['basic', 'trained']) {
    globalThis.dmzReset(); assert.ok(JSON.parse(globalThis.dmzSetTenantTerms('["Verkefni Gervi"]')).ok);
    const start = performance.now();
    const result = JSON.parse(globalThis.dmzProcessBestEffort(row.text, JSON.stringify(mode === 'trained' ? original.names : [])));
    times.push(performance.now() - start);
    if (!result.allowed || globalThis.dmzRestore(result.transformed) !== row.text) throw Error('Restoration failed');
    record[mode] = { predicted: (result.detected_spans || []).map(s => [s.start, s.end, s.kind]),
      abstained: !result.spans_available, exact_restoration: true, altered: row.text !== result.transformed };
  }
  records.push(record);
}
assert.equal(hash(readFileSync(wasmArg)), hash(wasm));
times.sort((a, b) => a - b);
const result = { schema: 1, status: 'complete', corpus: manifest, external_requests: 0,
  execution: 'Node WASM with unchanged cached browser name spans; no neural inference rerun',
  reference_sha256: hash(referenceBytes), wasm_sha256: hash(wasm), runtime_sha256: hash(runtime),
  measurement_sha256: hash(readFileSync(fileURLToPath(import.meta.url))),
  seconds: (performance.now() - began) / 1000, p50_ms: times[Math.floor(times.length * .5)],
  p95_ms: times[Math.floor(times.length * .95)], max_ms: times.at(-1), records };
writeFileSync(out, JSON.stringify(result, null, 2) + '\n', { flag: 'wx' });
console.log(JSON.stringify({ excerpts: records.length, p50_ms: result.p50_ms, p95_ms: result.p95_ms, max_ms: result.max_ms }));
