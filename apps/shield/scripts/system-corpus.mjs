// Synthetic corpus replay through packaged Chromium scanner AND masking core.
// Reports aggregate complete-identifier coverage, never text or predictions.
import { readFile, writeFile, mkdtemp, rm, realpath, stat, readdir } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve, relative, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
import { annotationSpans, restoredMaskSpans, coverage, markers } from './coverage.mjs';
import { aggregateQuality, typedRecord } from './typed-quality.mjs';

const [outputArg, ...sourceArgs] = process.argv.slice(2);
if (!outputArg || !sourceArgs.length) throw Error('Usage: system-corpus.mjs PRIVATE_OUTPUT SYNTHETIC_CORPUS...');
const root = fileURLToPath(new URL('../', import.meta.url));
const repo = resolve(root, '../..'), corpusRoot = await realpath(join(repo, 'bench/corpus'));
const extension = join(root, '.output/chrome-mv3');
const output = resolve(outputArg), outputRelative = relative(repo, output);
if (outputRelative !== '..' && !outputRelative.startsWith('../')) throw Error('Output must stay outside repository');
const parentRelative = relative(repo, await realpath(dirname(output)));
if (parentRelative !== '..' && !parentRelative.startsWith('../')) throw Error('Output parent resolves inside repository');
try { await stat(output); throw Error('Refusing to overwrite an existing evaluation'); }
catch (error) { if (error.code !== 'ENOENT') throw error; }
const digest = bytes => createHash('sha256').update(bytes).digest('hex');
async function fileDigest(path) {
  const hash = createHash('sha256');
  for await (const bytes of createReadStream(path)) hash.update(bytes);
  return hash.digest('hex');
}
async function artifactFiles(relativePath = '', files = {}) {
  const entries = await readdir(join(extension, relativePath), { withFileTypes: true });
  entries.sort((a, b) => a.name.localeCompare(b.name, 'en'));
  for (const entry of entries) {
    const name = relativePath ? `${relativePath}/${entry.name}` : entry.name;
    if (name === 'BUILD.json') continue;
    if (entry.isDirectory()) await artifactFiles(name, files);
    else if (entry.isFile()) files[name] = await fileDigest(join(extension, name));
    else throw Error('Packaged artifact contains an unsupported file entry');
  }
  return files;
}
const packagedFiles = await artifactFiles();
const measurementFiles = {};
for (const name of ['apps/shield/scripts/system-corpus.mjs', 'apps/shield/scripts/coverage.mjs',
  'apps/shield/scripts/typed-quality.mjs', 'bench/contextual/quality.py']) {
  measurementFiles[name] = await fileDigest(join(repo, name));
}
const corpus = [];
for (const path of sourceArgs) {
  const source = await realpath(path), rel = relative(corpusRoot, source);
  if (rel.startsWith('../') || rel === '..' || !rel.endsWith('.jsonl'))
    throw Error('This harness accepts only the committed synthetic corpus directory');
  const bytes = await readFile(source), rows = bytes.toString().trim().split('\n').map(JSON.parse);
  const ids = new Set();
  for (const row of rows) {
    if (typeof row.id !== 'string' || !row.id || ids.has(row.id)) throw Error('Missing or duplicate sample ID');
    ids.add(row.id);
    annotationSpans(row);
  }
  corpus.push({ source: rel, sha256: digest(bytes), rows });
}
const manifestBytes = await readFile(join(extension, 'manifest.json'));
const manifest = JSON.parse(manifestBytes);
if (!manifest.permissions?.includes('offscreen')) throw Error('Requires trained preview');
const report = {
  schema: 4,
  scope: 'Selected/reused synthetic corpora through packaged Chromium scanner and Go WASM best-effort masking; no live-site submission UI',
  independent_validation: false,
  metric: 'Full UTF-8 identifier interval covered by issued restorable markers; partial masking is not success',
  typed_metric: 'Exact final entity span/type set per excerpt; unavailable typed results count as abstentions and incorrect excerpts',
  not_measured: 'Representative field performance, real-site interception, scanner warning UI',
  build: { manifest_sha256: digest(manifestBytes), version_name: manifest.version_name,
    wasm_sha256: digest(await readFile(join(extension, 'dmz.wasm'))),
    detector_data: JSON.parse(await readFile(join(extension, 'detector-data.json'))),
    scanner_manifest_sha256: digest(await readFile(join(extension, 'scanner/manifest.json'))) },
  packaged_files_sha256: packagedFiles, measurement_files_sha256: measurementFiles,
  runtime: { node: process.version },
  external_requests: 0, sets: [], status: 'running',
};
const profile = await mkdtemp(join(tmpdir(), 'dmz-system-corpus-'));
let browser;
function blank() {
  return { excerpts: 0, allowed: 0, blocked: 0, negative_excerpts: 0, altered_negative_excerpts: 0,
    fully_masked: 0, partially_masked: 0, unmasked: 0, positive_excerpts_completely_masked: 0,
    positive_excerpts: 0, exact_restorations: 0, scanner_fallbacks: 0, scanner_fallback_reasons: {}, blocked_identifiers: 0, per_type: {} };
}
try {
  browser = await chromium.launchPersistentContext(profile, { headless: true, channel: 'chromium',
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
    args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`] });
  browser.on('request', request => { if (!request.url().startsWith('chrome-extension://')) report.external_requests++; });
  await browser.route(/^https?:/, route => route.abort());
  let worker = browser.serviceWorkers()[0];
  if (!worker) worker = await browser.waitForEvent('serviceworker');
  // URL.origin is "null" for chrome-extension URLs in Node.
  const extensionOrigin = worker.url().slice(0, worker.url().lastIndexOf('/'));
  const core = await browser.newPage();
  await core.goto(extensionOrigin + '/sidepanel.html');
  report.runtime.user_agent = await core.evaluate(() => navigator.userAgent);
  const devtools = await browser.newCDPSession(core);
  report.runtime.chromium = await devtools.send('Browser.getVersion');
  await devtools.detach();
  await core.addScriptTag({ url: extensionOrigin + '/wasm_exec.js' });
  await core.evaluate(async () => {
    const go = new globalThis.Go();
    const bytes = await (await fetch(chrome.runtime.getURL('dmz.wasm'))).arrayBuffer();
    const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
    void go.run(instance);
  });
  await core.waitForFunction(() => typeof globalThis.dmzProcessBestEffort === 'function');
  await worker.evaluate(() => chrome.offscreen.createDocument({ url: 'scanner.html',
    reasons: [chrome.offscreen.Reason.WORKERS], justification: 'Local synthetic system evaluation' }));
  const started = Date.now();
  for (const source of corpus) {
    const result = { source: source.source, sha256: source.sha256, basic: blank(), trained: blank() };
    const typed = { basic: [], trained: [] };
    for (const row of source.rows) {
      const gold = annotationSpans(row);
      const scanned = await worker.evaluate(text => chrome.runtime.sendMessage({ type: 'dmz:scanner-run', text }), row.text);
      for (const mode of ['basic', 'trained']) {
        const totals = result[mode], fallback = mode === 'trained' && !!scanned.error;
        const names = mode === 'trained' && !fallback ? scanned.spans : [];
        const processed = await core.evaluate(({ text, names }) => {
          globalThis.dmzReset();
          const configured = JSON.parse(globalThis.dmzSetTenantTerms(JSON.stringify(['Verkefni Gervi'])));
          if (!configured.ok) throw Error('Unable to configure synthetic terms');
          const result = JSON.parse(globalThis.dmzProcessBestEffort(text, JSON.stringify(names)));
          return { result, restored: result.allowed ? globalThis.dmzRestore(result.transformed) : null };
        }, { text: row.text, names });
        typed[mode].push(typedRecord(row.text, gold, processed.result));
        totals.excerpts++; totals.scanner_fallbacks += Number(fallback);
        if (fallback) {
          const code = ['unsupported_input', 'asset_failure', 'inference_failure', 'busy',
            'worker_unavailable', 'worker_message_error', 'timeout'].includes(scanned.code) ? scanned.code : 'unknown';
          totals.scanner_fallback_reasons[code] = (totals.scanner_fallback_reasons[code] || 0) + 1;
        }
        totals.negative_excerpts += Number(!gold.length); totals.positive_excerpts += Number(!!gold.length);
        if (!processed.result.allowed) { totals.blocked++; totals.blocked_identifiers += gold.length; continue; }
        totals.allowed++;
        if (processed.restored !== row.text) throw Error('Non-exact restoration prevents trustworthy interval accounting');
        totals.exact_restorations++;
        const values = markers(processed.result.transformed);
        const restored = await core.evaluate(values => values.map(value => globalThis.dmzRestore(value)), values);
        const masks = restoredMaskSpans(row.text, processed.result.transformed, restored);
        const measured = coverage(gold, masks);
        for (const key of ['fully_masked', 'partially_masked', 'unmasked']) totals[key] += measured[key];
        for (const [kind, counts] of Object.entries(measured.per_type)) {
          const bucket = totals.per_type[kind] ||= { fully_masked: 0, partially_masked: 0, unmasked: 0 };
          for (const key of Object.keys(bucket)) bucket[key] += counts[key];
        }
        if (gold.length && measured.fully_masked === gold.length) totals.positive_excerpts_completely_masked++;
        if (!gold.length && processed.result.transformed !== row.text) totals.altered_negative_excerpts++;
      }
    }
    for (const mode of ['basic', 'trained']) {
      const totals = result[mode], n = totals.fully_masked + totals.partially_masked + totals.unmasked;
      totals.complete_identifier_coverage_in_allowed_text = n ? totals.fully_masked / n : null;
      totals.typed_quality = aggregateQuality(typed[mode]);
    }
    report.sets.push(result);
    console.log(JSON.stringify({ completed_corpus: source.source, excerpts: source.rows.length }));
  }
  report.seconds = (Date.now() - started) / 1000;
  if (report.external_requests) throw Error('Unexpected network requests');
  if (JSON.stringify(await artifactFiles()) !== JSON.stringify(packagedFiles))
    throw Error('Packaged artifact changed during evaluation');
  for (const [name, hash] of Object.entries(measurementFiles)) {
    if (await fileDigest(join(repo, name)) !== hash) throw Error('Measurement code changed during evaluation');
  }
  report.status = 'complete';
} finally {
  if (browser) await browser.close();
  await rm(profile, { recursive: true, force: true });
  await writeFile(output, JSON.stringify(report, null, 2) + '\n', { flag: 'wx' });
}
console.log(JSON.stringify({ status: report.status, external_requests: report.external_requests, sets: report.sets.length }));
