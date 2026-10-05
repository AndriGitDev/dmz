// Differential runtime check on the pinned upstream examples/generated digits.
import {readFileSync,writeFileSync,realpathSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve,join,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
import vm from 'node:vm';
import assert from 'node:assert/strict';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../..');
const [fixturesArg,manifestArg,wasmArg,outArg]=process.argv.slice(2);
if(!outArg)throw Error('Usage: phone-world-wasm.mjs PRIVATE_FIXTURES MANIFEST WASM PRIVATE_OUTPUT');
const out=resolve(outArg),privateRoot='/root/.local/share/dmz/experiments/';
if(!realpathSync(fixturesArg).startsWith(privateRoot)||!realpathSync(dirname(out)).startsWith(privateRoot))throw Error('Private paths required');
const hash=b=>createHash('sha256').update(b).digest('hex');
const manifest=JSON.parse(readFileSync(manifestArg)),raw=readFileSync(fixturesArg);
assert.equal(manifest.source,'nyaruka/phonenumbers/v2 v2.0.14, libphonenumber metadata v9.0.40');
assert.equal(hash(raw),manifest.fixture_sha256);assert.equal(manifest.counts.cases,49245);
assert.equal(manifest.counts.unexpected_miss||0,0);assert.equal(manifest.counts.unexpected_false_positive||0,0);
const rows=raw.toString().trim().split('\n').map(JSON.parse);assert.equal(rows.length,49245);
const runtime=readFileSync(join(root,'apps/shield/public/wasm_exec.js'));
vm.runInThisContext(runtime.toString());
const wasm=readFileSync(wasmArg),go=new globalThis.Go();
const {instance}=await WebAssembly.instantiate(wasm,go.importObject);void go.run(instance);
for(let i=0;i<200&&typeof globalThis.dmzProcessBestEffort!=='function';i++)await new Promise(r=>setTimeout(r,10));
assert.equal(typeof globalThis.dmzProcessBestEffort,'function');
let completed=0;const began=performance.now(),times=[];
for(const row of rows){
 assert.match(row.input,/^Tel: \+[0-9]+$/);
 // These constrained inputs cannot contain issued markers. Reusing a bounded
 // session also tests restoration across different canonical numeric values.
 if(completed%128===0)globalThis.dmzReset();
 const start=performance.now(),result=JSON.parse(globalThis.dmzProcessBestEffort(row.input,'[]'));
 times.push(performance.now()-start);
 assert.ok(result.allowed&&result.spans_available,`case ${completed} rejected`);
 assert.equal(globalThis.dmzRestore(result.transformed),row.input,'restoration failure');
 assert.deepEqual((result.detected_spans||[]).map(s=>[s.start,s.end,s.kind]),row.predicted,`case ${completed} differs`);
 if(++completed%1000===0)console.log(JSON.stringify({completed}));
}
assert.equal(hash(readFileSync(wasmArg)),hash(wasm));times.sort((a,b)=>a-b);
const result={schema:1,status:'complete',cases:completed,source:manifest.source,fixture_sha256:hash(raw),
 wasm_sha256:hash(wasm),runtime_sha256:hash(runtime),measurement_sha256:hash(readFileSync(fileURLToPath(import.meta.url))),
 seconds:(performance.now()-began)/1000,cases_per_session:128,p50_ms:times[Math.floor(times.length*.5)],p95_ms:times[Math.floor(times.length*.95)],max_ms:times.at(-1),
 exact_restoration:true,native_span_mismatches:0,scope:'Canonical phone-rule runtime parity, not PII accuracy or independent field evidence'};
writeFileSync(out,JSON.stringify(result,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify(result));
