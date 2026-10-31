// Person-only regression replay; only the hash-pinned, already-used dev sources.
import {readFileSync,writeFileSync,realpathSync} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve,join,dirname} from 'node:path';
import {fileURLToPath} from 'node:url';
import vm from 'node:vm';
import assert from 'node:assert/strict';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../..');
const [corpusArg,wasmArg,outArg]=process.argv.slice(2);
if(!outArg)throw Error('Usage: replay_development_core.mjs PREPARED_DEV WASM PRIVATE_OUTPUT');
const corpus=realpathSync(corpusArg),out=resolve(outArg),privateRoot='/root/.local/share/dmz/experiments/';
if(!corpus.startsWith(privateRoot)||!realpathSync(dirname(out)).startsWith(privateRoot))throw Error('Private experiment paths required');
const hash=b=>createHash('sha256').update(b).digest('hex');
const manifest=JSON.parse(readFileSync(join(corpus,'manifest.json')));
assert.equal(manifest.inputs.general.source_sha256,'ddf057b278e1a143da7d1667872cf0c5d66d7a94e63aeebc57fd8b1753f3a2a0');
assert.equal(manifest.inputs.domain.source_sha256,'6246db44f8ba0b2a0f33228c6c6deac2c4937fe442b2e17351292baf3cbd6b87');
assert.deepEqual(manifest.scope,['name_demo']);
const raw=readFileSync(join(corpus,'records.jsonl'));assert.equal(hash(raw),manifest.records_sha256);
const rows=raw.toString().trim().split('\n').map(JSON.parse);assert.equal(rows.length,5838);
assert.equal(new Set(rows.map(r=>r.id)).size,5838);
const runtime=readFileSync(join(root,'apps/dmz/public/wasm_exec.js'));
vm.runInThisContext(runtime.toString());
const wasm=readFileSync(wasmArg),go=new globalThis.Go();
const {instance}=await WebAssembly.instantiate(wasm,go.importObject);void go.run(instance);
for(let i=0;i<200&&typeof globalThis.dmzProcessBestEffort!=='function';i++)await new Promise(r=>setTimeout(r,10));
assert.equal(typeof globalThis.dmzProcessBestEffort,'function');
const records=[],began=performance.now();
for(const row of rows){
 assert.equal(hash(row.text),row.text_sha256);
 globalThis.dmzReset();
 const result=JSON.parse(globalThis.dmzProcessBestEffort(row.text,JSON.stringify(row.names)));
 assert.ok(result.allowed,'replay rejected cached input');
 assert.equal(globalThis.dmzRestore(result.transformed),row.text,'restoration failure');
 records.push({id:row.id,stratum:row.stratum,text_sha256:row.text_sha256,gold:row.gold,
  predicted:(result.detected_spans||[]).filter(s=>s.kind==='name_demo').map(s=>[s.start,s.end,s.kind]),
  abstained:!result.spans_available,exact_restoration:true});
 if(records.length%500===0)console.log(JSON.stringify({completed:records.length}));
}
assert.equal(hash(readFileSync(wasmArg)),hash(wasm));
writeFileSync(out,JSON.stringify({schema:1,status:'complete',corpus:manifest,wasm_sha256:hash(wasm),runtime_sha256:hash(runtime),
 measurement_sha256:hash(readFileSync(fileURLToPath(import.meta.url))),seconds:(performance.now()-began)/1000,
 execution:'Node WASM core with unchanged cached CRF/encoder names; Person-only reused development',
 independent_validation:false,records},null,2)+'\n',{flag:'wx'});
console.log(JSON.stringify({status:'complete',excerpts:records.length,seconds:(performance.now()-began)/1000}));
