// Local-origin Chromium experiment, not an extension integration or remote service.
import { readFile, writeFile, realpath, stat } from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { resolve, join, dirname, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from '../../apps/shield/node_modules/playwright/index.mjs';
const root = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const [exportArg, quantArg, packageArg, variant, corpusArg, outArg] = process.argv.slice(2);
if (!outArg || !['encoder-q8', 'all-q8'].includes(variant)) throw Error('Usage: gliner2-browser-probe.mjs EXPORT QUANTIZED TOKENIZER_PACKAGE encoder-q8|all-q8 SYNTHETIC_CORPUS PRIVATE_OUTPUT');
const out = resolve(outArg), corpus = await realpath(corpusArg);
if (!relative(root, await realpath(dirname(out))).startsWith('../') || !corpus.startsWith(join(root, 'bench/corpus/'))) throw Error('Invalid paths');
try { await stat(out); throw Error('Output exists'); } catch (error) { if (error.code !== 'ENOENT') throw error; }
const hash = b => createHash('sha256').update(b).digest('hex');
async function digest(path) { const h = createHash('sha256'); for await (const b of createReadStream(path)) h.update(b); return h.digest('hex'); }
const manifest = JSON.parse(await readFile(join(corpus, 'manifest.json'))), rows = [];
for (const [name, entry] of Object.entries(manifest.sets)) {
  const raw = await readFile(join(corpus, name)); if (hash(raw) !== entry.sha256) throw Error('Corpus changed');
  for (const row of raw.toString().trim().split('\n').map(JSON.parse)) {
    const gold = row.entities.map(e => {
      const position = row.text.indexOf(e.value);
      if (position < 0 || row.text.indexOf(e.value, position + 1) >= 0) throw Error('Ambiguous gold');
      return [Buffer.byteLength(row.text.slice(0, position)), Buffer.byteLength(row.text.slice(0, position + e.value.length)), e.kind];
    });
    rows.push({ ...row, gold });
  }
}
const exported = await realpath(exportArg), quantized = await realpath(quantArg), pkg = await realpath(packageArg);
const quant = JSON.parse(await readFile(join(quantized, 'manifest.json')));
const packageInfo = JSON.parse(await readFile(join(pkg, 'package.json')));
if (packageInfo.name !== '@huggingface/tokenizers' || packageInfo.version !== '0.2.0') throw Error('Wrong tokenizer');
const ortDir = join(root, 'bench/browser-detector/node_modules/onnxruntime-web');
if (JSON.parse(await readFile(join(ortDir, 'package.json'))).version !== '1.23.2') throw Error('Wrong ORT');
const routes = new Map([
  ['/adapter.mjs', join(root, 'bench/open-source-pii/gliner2-browser.mjs')],
  ['/tokenizers.mjs', join(pkg, 'dist/tokenizers.mjs')],
  ['/tokenizer.json', join(exported, 'tokenizer.json')],
  ['/tokenizer_config.json', join(exported, 'tokenizer_config.json')],
  ['/gliner2_config.json', join(exported, 'gliner2_config.json')],
]);
for (const name of ['ort.wasm.min.mjs', 'ort-wasm-simd-threaded.mjs', 'ort-wasm-simd-threaded.wasm']) routes.set('/ort/' + name, join(ortDir, 'dist', name));
for (const name of ['encoder', 'span_rep', 'count_embed']) {
  const original = join(exported, 'onnx', name + '.onnx'), entry = quant.files[name];
  if (await digest(original) !== entry.source_sha256) throw Error('Wrong original export');
  const path = variant === 'all-q8' || name === 'encoder' ? join(quantized, name + '.onnx') : original;
  if (path !== original && await digest(path) !== entry.sha256) throw Error('Changed quantized export');
  routes.set('/models/' + name + '.onnx', path);
}
const identities = {};
for (const [route, path] of routes) identities[route] = { sha256: await digest(path), bytes: (await stat(path)).size };
const sourceHash = await digest(fileURLToPath(import.meta.url));
const server = createServer((req, res) => {
  res.setHeader('Cross-Origin-Opener-Policy', 'same-origin');
  res.setHeader('Cross-Origin-Embedder-Policy', 'require-corp');
  if (req.method !== 'GET') { res.writeHead(405).end(); return; }
  const pathname = new URL(req.url, 'http://localhost').pathname;
  if (pathname === '/') { res.setHeader('Content-Type', 'text/html'); res.end('<!doctype html><meta charset="utf-8"><title>Local model experiment</title>'); return; }
  const path = routes.get(pathname);
  if (!path) { res.writeHead(404).end(); return; }
  res.setHeader('Content-Type', pathname.endsWith('.mjs') ? 'text/javascript' : pathname.endsWith('.json') ? 'application/json' : pathname.endsWith('.wasm') ? 'application/wasm' : 'application/octet-stream');
  const stream = createReadStream(path); stream.on('error', () => res.destroy()); stream.pipe(res);
});
await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
const origin = `http://127.0.0.1:${server.address().port}`;
let browser, external = 0;
const deadline = setTimeout(() => { console.error('Local browser trial exceeded 15-minute bound'); void browser?.close(); }, 15 * 60 * 1000);
const began = performance.now(), records = [];
try {
  browser = await chromium.launch({ headless: true, channel: 'chromium' });
  const context = await browser.newContext();
  await context.route('**/*', route => {
    if (new URL(route.request().url()).origin === origin) return route.continue();
    external++; return route.abort();
  });
  const page = await context.newPage();
  await page.goto(origin);
  await page.evaluate(async () => {
    const ort = await import('/ort/ort.wasm.min.mjs');
    const { Tokenizer } = await import('/tokenizers.mjs');
    const { createRuntime } = await import('/adapter.mjs');
    ort.env.wasm.numThreads = 1; ort.env.wasm.proxy = false; ort.env.wasm.wasmPaths = '/ort/';
    const read = async name => (await fetch('/' + name)).json();
    const model = await read('tokenizer.json');
    if (model.model.type !== 'Unigram') throw Error('unexpected_tokenizer');
    const tokenizer = new Tokenizer(model, await read('tokenizer_config.json'));
    globalThis.__localNER = await createRuntime(ort, tokenizer, await read('gliner2_config.json'), {
      encoder: '/models/encoder.onnx', span_rep: '/models/span_rep.onnx', count_embed: '/models/count_embed.onnx',
    });
  });
  const loadSeconds = (performance.now() - began) / 1000;
  const failures = {};
  for (const row of rows) {
    const result = await page.evaluate(async text => {
      const start = performance.now();
      try { return { scored: await globalThis.__localNER.predict(text), failure: null, ms: performance.now() - start }; }
      catch (error) {
        const safe = ['unsupported_input', 'unsupported_tokenization', 'invalid_offsets', 'invalid_score', 'too_many_spans', 'unexpected_model_output', 'unexpected_model_shape'];
        return { scored: [], failure: safe.includes(error.message) ? error.message : 'inference_error', ms: performance.now() - start };
      }
    }, row.text);
    if (result.failure) failures[result.failure] = (failures[result.failure] || 0) + 1;
    records.push({ id: row.id, stratum: row.stratum, text_sha256: hash(row.text), gold: row.gold,
      predicted: result.scored.map(r => r.span), scored: result.scored, abstained: result.failure !== null,
      candidate_failure: result.failure, seconds: result.ms / 1000 });
    if (records.length % 20 === 0) console.log(JSON.stringify({ completed: records.length }));
  }
  await page.evaluate(() => globalThis.__localNER.close());
  if (external) throw Error('External request attempted');
  for (const [route, path] of routes) if (await digest(path) !== identities[route].sha256) throw Error('Asset changed');
  if (await digest(fileURLToPath(import.meta.url)) !== sourceHash) throw Error('Measurement changed');
  await writeFile(out, JSON.stringify({ schema: 1, status: 'complete', candidate: 'gliner2-browser-' + variant,
    corpus: manifest, measurement_sha256: sourceHash, assets: identities, external_requests: external,
    browser_version: browser.version(), runtime: 'ORT Web 1.23.2 WASM single thread',
    execution: 'local-origin Chromium experiment, not packaged extension', independent_validation: false,
    load_seconds: loadSeconds, seconds: (performance.now() - began) / 1000, candidate_failures: failures, records }, null, 2) + '\n', { flag: 'wx' });
  console.log(JSON.stringify({ status: 'complete', excerpts: records.length, failures, load_seconds: loadSeconds }));
} finally {
  clearTimeout(deadline);
  if (browser) await browser.close();
  await new Promise(resolve => server.close(resolve));
}
