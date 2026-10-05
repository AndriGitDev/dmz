// Compare original-byte core results with a pinned private native capture.
import {readFileSync,writeFileSync,realpathSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve,relative,dirname,join} from 'node:path';
import {fileURLToPath} from 'node:url';
import vm from 'node:vm';
import assert from 'node:assert/strict';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../..');
const [corpusArg,nativeArg,outArg,wasmArg]=process.argv.slice(2);
if(!outArg)throw Error('Usage: wasm-core-probe.mjs SYNTHETIC_DIR PRIVATE_NATIVE_JSONL PRIVATE_OUTPUT [WASM]');
const corpus=realpathSync(corpusArg),out=resolve(outArg);
if(!corpus.startsWith(join(root,'bench/corpus/'))||!relative(root,realpathSync(dirname(out))).startsWith('../'))throw Error('Invalid paths');
const hash=b=>createHash('sha256').update(b).digest('hex');
const manifest=JSON.parse(readFileSync(join(corpus,'manifest.json')));
const reference=new Map(readFileSync(nativeArg,'utf8').trim().split('\n').map(JSON.parse).map(r=>[r.id,r]));
const publicDir=join(root,'apps/shield/public');
vm.runInThisContext(readFileSync(join(publicDir,'wasm_exec.js'),'utf8'));
const go=new globalThis.Go(),bytes=readFileSync(wasmArg||join(publicDir,'dmz.wasm'));
const {instance}=await WebAssembly.instantiate(bytes,go.importObject);void go.run(instance);
for(let i=0;i<200&&typeof globalThis.dmzProcessBestEffort!=='function';i++)await new Promise(r=>setTimeout(r,10));
assert.equal(typeof globalThis.dmzProcessBestEffort,'function');
const records=[],times=[];
for(const [name,entry]of Object.entries(manifest.sets)){
 const raw=readFileSync(join(corpus,name));assert.equal(hash(raw),entry.sha256);
 for(const row of raw.toString().trim().split('\n').map(JSON.parse)){
  const ref=reference.get(row.id);assert.equal(ref.text_sha256,hash(row.text));
  globalThis.dmzReset();assert.ok(JSON.parse(globalThis.dmzSetTenantTerms('["Verkefni Gervi"]')).ok);
  const start=performance.now();const result=JSON.parse(globalThis.dmzProcessBestEffort(row.text,'[]'));times.push(performance.now()-start);
  assert.ok(result.allowed);assert.equal(globalThis.dmzRestore(result.transformed),row.text);
  const predicted=(result.detected_spans||[]).map(s=>[s.start,s.end,s.kind]);
  assert.deepEqual(predicted,ref.predicted,row.id);assert.equal(!result.spans_available,ref.abstained,row.id);
  records.push({id:row.id,text_sha256:ref.text_sha256,predicted,exact_restoration:true});
 }
}
assert.equal(records.length,reference.size);times.sort((a,b)=>a-b);
const result={schema:1,status:'complete',wasm_sha256:hash(bytes),runtime_sha256:hash(readFileSync(join(publicDir,'wasm_exec.js'))),corpus:manifest,native_sha256:hash(readFileSync(nativeArg)),excerpts:records.length,p50_ms:times[Math.floor(times.length*.5)],p95_ms:times[Math.floor(times.length*.95)],max_ms:times.at(-1),records};
writeFileSync(out,JSON.stringify(result,null,2)+'\n',{flag:'wx'});console.log(JSON.stringify({excerpts:result.excerpts,p50_ms:result.p50_ms,p95_ms:result.p95_ms,max_ms:result.max_ms}));
