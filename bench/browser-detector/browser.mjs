import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { createReadStream, readdirSync, readFileSync } from 'node:fs';
import { readFile, writeFile, stat } from 'node:fs/promises';
import { dirname, resolve, join, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
const root = dirname(fileURLToPath(import.meta.url));
const assets = process.env.DMZ_MODEL_ASSETS;
const reportPath = process.env.DMZ_BROWSER_REPORT;
if (!assets || !reportPath) throw new Error('Set DMZ_MODEL_ASSETS and DMZ_BROWSER_REPORT');
try { await stat(reportPath); throw new Error('Refusing to replace benchmark report'); }
catch(error) { if(error.code!=='ENOENT')throw error; }
const fixtureBytes = await readFile(process.env.DMZ_BROWSER_FIXTURES || join(assets, 'fixtures.json'));
const fixtures = JSON.parse(fixtureBytes.toString('utf8'));
for (const [model, expected] of Object.entries(fixtures.models)) {
  assert.ok(['fp32', 'int8', 'int4', 'mixed', 'int4b16', 'int4a32', 'hqq4', 'repair1', 'repair2'].includes(model));
  const hash = createHash('sha256');
  for await (const chunk of createReadStream(join(assets, model + '.onnx'))) hash.update(chunk);
  assert.equal(hash.digest('hex'), expected, 'Model differs from native fixture reference');
}
const runtime = join(root, 'node_modules/onnxruntime-web/dist');
let isolate = false;
let server;
let browserServer;
let browser;
let peakRss = 0;
const cases = process.env.DMZ_BENCH_CASES
  ? process.env.DMZ_BENCH_CASES.split(',').map(value => { const [model, threads] = value.split(':'); return [model, Number(threads)]; })
  : Object.keys(fixtures.models).flatMap(model=>[[model,1],[model,3]]);
for (const [model, threads] of cases) assert.ok(Object.hasOwn(fixtures.models,model) && [1,3].includes(threads));
const multiModel=process.env.DMZ_MULTI_MODEL || (Object.hasOwn(fixtures.models,'int8')?'int8':Object.keys(fixtures.models).at(-1));
assert.ok(Object.hasOwn(fixtures.models,multiModel));
const traffic = [];
const failures = [];
const result = { schema: 1, fixtures_sha256: createHash('sha256').update(fixtureBytes).digest('hex'), scope: 'Chromium CPU WASM encoder/pooling/head on fabricated pretokenized inputs; excludes tokenizer, CRF, masking and Shield integration', models: fixtures.models, sessions: [], failures, memory: 'Linux summed RSS of Chromium process tree sampled every 200ms; shared pages may be counted more than once, not unique physical memory', externalRequests: [] };
function treeRss(rootPid) {
  const rows = [];
  for (const pid of readdirSync('/proc').filter(n => /^\d+$/.test(n))) {
    try {
      const status = readFileSync(`/proc/${pid}/status`, 'utf8');
      rows.push({ pid: Number(pid), parent: Number(status.match(/^PPid:\s+(\d+)/m)?.[1]), rss: Number(status.match(/^VmRSS:\s+(\d+)/m)?.[1] || 0) });
    } catch { /* Process exited during sample. */ }
  }
  const ids = new Set([rootPid]);
  for (let count = -1; count !== ids.size;) { count = ids.size; for (const row of rows) if (ids.has(row.parent)) ids.add(row.pid); }
  return rows.filter(r => ids.has(r.pid)).reduce((s, r) => s + r.rss, 0);
}
function median(values) { return [...values].sort((a,b) => a-b)[Math.floor(values.length/2)]; }
function parity(outputs, testCase, model) {
  assert.equal(outputs.length, testCase.chunks.length);
  let changed = 0, maximum = 0;
  outputs.forEach((values, chunk) => {
    const expected = testCase.chunks[chunk].expected[model];
    assert.equal(values.length, expected.logits.length);
    for (let i = 0; i < values.length; i++) { assert.ok(Number.isFinite(values[i])); maximum = Math.max(maximum, Math.abs(values[i] - expected.logits[i])); }
    for (let i = 0; i < values.length; i += 3) {
      const label = values.slice(i,i+3).indexOf(Math.max(...values.slice(i,i+3)));
      if (label !== expected.labels[i/3]) changed++;
    }
  });
  return { changedWordLabels: changed, maxAbsoluteLogitError: maximum };
}
async function guardedContext(origin) {
  const context = await browser.newContext();
  await context.route('**/*', route => {
    const url = route.request().url(); traffic.push(url);
    if (url.startsWith(origin + '/')) return route.continue();
    result.externalRequests.push(url); return route.abort();
  });
  return context;
}
async function pageFor(context, origin) {
  const page = await context.newPage();
  await page.goto(origin);
  await page.waitForFunction(() => typeof window.makeClient === 'function');
  await page.evaluate(() => window.makeClient());
  return page;
}
async function infer(page, testCase, deadline = 60000) {
  return page.evaluate(async ({ chunks, deadline }) => {
    const started = performance.now();
    const result = await window.client.request('infer', { chunks }, deadline);
    return { ...result, wallMs: performance.now() - started };
  }, { chunks: testCase.chunks, deadline });
}
try {
  server = createServer(async (request, response) => {
    const path = new URL(request.url, 'http://localhost').pathname;
    const headers = { 'Cache-Control': 'no-store' };
    if (isolate) Object.assign(headers, { 'Cross-Origin-Opener-Policy': 'same-origin', 'Cross-Origin-Embedder-Policy': 'require-corp' });
    if (path === '/') { response.writeHead(200, { ...headers, 'Content-Type': 'text/html' }).end('<!doctype html><html><head><title>Private detector benchmark</title></head><body><p>Local detector benchmark. No prompt is sent.</p><script type="module" src="/client.mjs"></script></body></html>'); return; }
    if (path === '/crash.mjs') { response.writeHead(200, { ...headers, 'Content-Type': 'text/javascript' }).end('throw new Error("Deliberate worker crash")'); return; }
    if (path === '/corrupt.onnx') { response.writeHead(200, headers).end('invalid model'); return; }
    let file;
    if (['/worker.mjs','/client.mjs'].includes(path)) file = join(root, path.slice(1));
    else if (['/fp32.onnx','/int8.onnx','/int4.onnx','/mixed.onnx','/int4b16.onnx','/int4a32.onnx','/hqq4.onnx','/repair1.onnx','/repair2.onnx'].includes(path)) file = join(assets, path.slice(1));
    else if (/^\/runtime\/[a-z0-9.-]+$/.test(path)) file = join(runtime, path.slice('/runtime/'.length));
    if (!file) { response.writeHead(404, headers).end(); return; }
    try {
      const info = await stat(file);
      response.writeHead(200, { ...headers, 'Content-Length': info.size, 'Content-Type': extname(file) === '.mjs' ? 'text/javascript' : extname(file) === '.wasm' ? 'application/wasm' : 'application/octet-stream' });
      const stream = createReadStream(file); response.on('close', () => stream.destroy()); stream.pipe(response);
    } catch { response.writeHead(404, headers).end(); }
  });
  await new Promise(r => server.listen(0, '127.0.0.1', r));
  const origin = `http://127.0.0.1:${server.address().port}`;
  browserServer = await chromium.launchServer({ headless: true, host: '127.0.0.1', ...(process.env.DMZ_BROWSER_EXECUTABLE ? { executablePath: process.env.DMZ_BROWSER_EXECUTABLE } : {}) });
  browser = await chromium.connect(browserServer.wsEndpoint());
  result.browser = browser.version(); result.runtime = 'onnxruntime-web 1.23.2';
  result.hardwareConcurrency = undefined;
  const sampler = setInterval(() => { peakRss = Math.max(peakRss, treeRss(browserServer.process().pid)); }, 200);
  try {
    for (const [model, threads] of cases) {
      isolate = threads > 1;
      const context = await guardedContext(origin);
      const page = await pageFor(context, origin);
      result.hardwareConcurrency = await page.evaluate(() => navigator.hardwareConcurrency);
      const baselineRss = treeRss(browserServer.process().pid); peakRss = baselineRss;
      const record = { model, threads, baselineTreeRssKiB: baselineRss, timings: [] };
      result.sessions.push(record);
      try {
        const start = Date.now();
        record.load = await page.evaluate(({ model, threads }) => window.client.request('load', { url: '/' + model + '.onnx', threads }, 120000), { model, threads });
        record.load.wallMs = Date.now() - start;
        for (const testCase of fixtures.cases) {
          const first = await infer(page, testCase);
          const warm = [];
          let maxError = parity(first.outputs,testCase,model).maxAbsoluteLogitError;
          let changed = parity(first.outputs,testCase,model).changedWordLabels;
          for (let i=0;i<5;i++) {
            const run = await infer(page,testCase); warm.push(run.wallMs);
            const match = parity(run.outputs,testCase,model); maxError = Math.max(maxError,match.maxAbsoluteLogitError); changed += match.changedWordLabels;
          }
          record.timings.push({ words:testCase.words,chunks:testCase.chunks.length,firstMs:first.wallMs,warmMedianMs:median(warm),warmMaxMs:Math.max(...warm),repeats:5,changedWordLabels:changed,maxAbsoluteLogitError:maxError });
        }
        record.loadedTreeRssKiB = treeRss(browserServer.process().pid);
        // Deliberately malformed input must reject, never return a success.
        const malformed = await page.evaluate(async () => { try { await window.client.request('infer',{ chunks:[{ feeds:{} }] }); return false; } catch { return true; } });
        assert.ok(malformed); failures.push({model,threads,test:'invalid-input-rejected',passed:malformed});
        // A deadline terminates a busy worker and rejects further calls.
        const deadline = await page.evaluate(async chunks => {
          let rejected = false; try { await window.client.request('infer',{ chunks },1); } catch { rejected=true; }
          let closed=false;try { await window.client.request('infer',{ chunks }); } catch { closed=true; }
          return { rejected, closed, pending:window.client.pending.size };
        }, fixtures.cases.at(-1).chunks);
        assert.deepEqual(deadline,{rejected:true,closed:true,pending:0});failures.push({model,threads,test:'deadline-terminates-worker',passed:true});
      } catch (error) { record.error=String(error.message).slice(0,1200); }
      record.peakTreeRssKiB=peakRss;
      await context.close();
      console.log(JSON.stringify(record));
    }
    if (process.env.DMZ_BENCH_SINGLE_ONLY !== '1') {
    // Three independent tab workers measure the naive per-tab memory cost.
    isolate=false;
    const context=await guardedContext(origin);
    const pages=[];
    for(let i=0;i<3;i++)pages.push(await pageFor(context,origin));
    peakRss=treeRss(browserServer.process().pid);
    result.multiTab={tabs:3,model:multiModel,threadsPerTab:1,baselineTreeRssKiB:peakRss};
    try {
      const started=Date.now();
      await Promise.all(pages.map(page=>page.evaluate(model=>window.client.request('load',{url:'/'+model+'.onnx',threads:1},120000),multiModel)));
      result.multiTab.loadWallMs=Date.now()-started;
      result.multiTab.loadedTreeRssKiB=treeRss(browserServer.process().pid);
      const runs=await Promise.all(pages.map(page=>infer(page,fixtures.cases.at(-1))));
      result.multiTab.inferenceWallMs=runs.map(r=>r.wallMs);
      result.multiTab.parity=runs.map(r=>parity(r.outputs,fixtures.cases.at(-1),multiModel));
      const killed=await pages[0].evaluate(async chunks=>{const pending=window.client.request('infer',{chunks});window.client.close();try{await pending;return false;}catch{return window.client.pending.size===0;}},fixtures.cases.at(-1).chunks);
      assert.ok(killed);failures.push({test:'worker-termination-rejects-pending',passed:killed});
    } catch(error){result.multiTab.error=String(error.message).slice(0,1200);}
    result.multiTab.peakTreeRssKiB=peakRss;
    await context.close();
    }
    isolate=false;
    // Corrupt model and inference before ready reject.
    const broken=await guardedContext(origin);const page=await pageFor(broken,origin);
    for (const type of ['not-ready','corrupt-model']) {
      const rejected=await page.evaluate(async type=>{try { await window.client.request(type==='not-ready'?'infer':'load',type==='not-ready'?{chunks:[]}:{url:'/corrupt.onnx',threads:1});return false;}catch{return true;}},type);
      assert.ok(rejected);failures.push({test:type,passed:rejected});
    }
    const crashed = await page.evaluate(async () => {
      const client = new window.DetectorClient('/crash.mjs');
      try { await client.request('load', {}, 5000); return false; }
      catch (error) { return error.message === 'Worker error' && client.closed && client.pending.size === 0; }
    });
    assert.ok(crashed); failures.push({test:'worker-crash-rejects-pending',passed:crashed});
    await broken.close();
    assert.deepEqual(result.externalRequests,[]);
  } finally { clearInterval(sampler); }
} catch(error) { result.fatal=String(error.stack);process.exitCode=1; }
finally {
  if(browser)await browser.close();if(browserServer)await browserServer.close();if(server)await new Promise(r=>server.close(r));
  result.requests=traffic.length;
  result.loadScope='Fresh worker/context with no-store HTTP, same browser process; not a fresh OS or browser cache cold start';
  result.allRunCasesSucceeded=!result.fatal && !result.sessions.some(session=>session.error) && !result.multiTab?.error;
  if(!result.allRunCasesSucceeded)process.exitCode=1;
  await writeFile(reportPath,JSON.stringify(result,null,2)+'\n',{flag:'wx'});
  console.log(JSON.stringify({report:reportPath,fatal:result.fatal,sessions:result.sessions.length}));
}
