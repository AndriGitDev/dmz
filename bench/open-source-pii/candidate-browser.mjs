// Capture a checksum-pinned trained candidate on synthetic probes or one hash-pinned
// public synthetic English projection. Never accepts arbitrary private corpora.
// No texts, transformed values, or session mappings are persisted.
import {readFile,writeFile,mkdtemp,rm,realpath,stat} from 'node:fs/promises';
import {createReadStream} from 'node:fs';
import {createHash} from 'node:crypto';
import {resolve,join,relative,dirname} from 'node:path';
import {tmpdir} from 'node:os';
import {fileURLToPath} from 'node:url';
import {chromium} from '../../apps/shield/node_modules/playwright/index.mjs';
import {annotationSpans,restoredMaskSpans,coverage,markers} from '../../apps/shield/scripts/coverage.mjs';
const root=resolve(dirname(fileURLToPath(import.meta.url)),'../..');
const [outArg,corpusArg,extensionArg,buildHash]=process.argv.slice(2);
if(!outArg||!corpusArg||!extensionArg||!/^[a-f0-9]{64}$/.test(buildHash||''))throw Error('Usage: candidate-browser.mjs PRIVATE_OUTPUT CORPUS EXTENSION BUILD_SHA256');
const out=resolve(outArg),parent=await realpath(dirname(out));
if(!relative(root,parent).startsWith('../'))throw Error('Output must be private');
try {await stat(out);throw Error('Output already exists');}catch(e){if(e.code!=='ENOENT')throw e;}
const corpus=await realpath(corpusArg);
const committed=corpus.startsWith(join(root,'bench/corpus/'));
const digest=b=>createHash('sha256').update(b).digest('hex');
async function fileDigest(path){const h=createHash('sha256');for await(const b of createReadStream(path))h.update(b);return h.digest('hex');}
const manifest=JSON.parse(await readFile(join(corpus,'manifest.json'),'utf8'));
const rows=[];
if(committed){
 for(const [name,entry]of Object.entries(manifest.sets)){
  const bytes=await readFile(join(corpus,name));if(digest(bytes)!==entry.sha256)throw Error('Corpus changed');
  for(const row of bytes.toString().trim().split('\n').map(JSON.parse)){annotationSpans(row);rows.push(row);}
 }
}else{
 if(manifest.source?.revision!=='e06eb1499ca8d54470f085021cd8e54f9efac7fd'||manifest.sets?.test?.sha256!=='6b70350932be463af6b53fc79bee5e53be2e66cc4133ae9bd0ff672d2237ad29')throw Error('Only the pinned public English projection is permitted');
 const bytes=await readFile(join(corpus,'test.jsonl'));if(digest(bytes)!==manifest.sets.test.sha256)throw Error('External source changed');
 for(const row of bytes.toString().trim().split('\n').map(JSON.parse)){
  const source=Buffer.from(row.text);if(source.toString()!==row.text||digest(row.text)!==row.text_sha256)throw Error('Invalid projected text');
  let previous=0;
  for(const [start,end,kind]of row.gold){
   if(!Number.isInteger(start)||!Number.isInteger(end)||start<previous||end<=start||end>source.length||!manifest.scope.includes(kind)||Buffer.from(source.subarray(start,end).toString()).compare(source.subarray(start,end))!==0)throw Error('Invalid projected gold');
   previous=end;
  }
  rows.push(row);
 }
}
const goldFor=row=>committed?annotationSpans(row):row.gold.map(([start,end,kind])=>({start,end,kind}));
const extension=extensionArg?await realpath(extensionArg):join(root,'apps/shield/.output/chrome-mv3');
const buildBytes=await readFile(join(extension,'BUILD.json'));
if(digest(buildBytes)!==buildHash)throw Error('Candidate BUILD checksum differs');
const build=JSON.parse(buildBytes);
if(build.scannerProfile!=='trained-fp16storage'||build.sha256['scanner/model.onnx']!=='e5acd596fde1873c6f4a5620db9c5417f11a59f3ad70cd96423614dd667b4690')throw Error('Requires pinned trained model');
if(build.sha256['scanner/tokenizer.json']!=='0305755b907b7a616fed8aa81aa0d12fe45a0916111988189b8646cc066fe20a'||build.sha256['scanner/crf.json']!=='b777808dee5c9eecc5596912aca46c5f65692acd51b0e632b9c55e4bdf0f0419')throw Error('Requires pinned preprocessing and CRF');
const extensionManifest=JSON.parse(await readFile(join(extension,'manifest.json')));
if(extensionManifest.version_name!==`${extensionManifest.version} preview ${build.sourceCommit}`)throw Error('Candidate build/source mismatch');
async function checkAssets(){for(const[name,sha]of Object.entries(build.sha256)){if(await fileDigest(join(extension,name))!==sha)throw Error('Candidate asset changed: '+name);}}
await checkAssets();
const sourceHash=await fileDigest(fileURLToPath(import.meta.url));
const profile=await mkdtemp(join(tmpdir(),'dmz-oss-reference-'));
let browser;let requests=0;const records=[];const started=Date.now();
try{
 browser=await chromium.launchPersistentContext(profile,{headless:true,channel:'chromium',args:[`--disable-extensions-except=${extension}`,`--load-extension=${extension}`]});
 browser.on('request',r=>{if(!r.url().startsWith('chrome-extension://'))requests++;});
 await browser.route(/^https?:/,r=>r.abort());
 const worker=browser.serviceWorkers()[0]||await browser.waitForEvent('serviceworker');
 const origin=worker.url().slice(0,worker.url().lastIndexOf('/'));
 const core=await browser.newPage();await core.goto(origin+'/sidepanel.html');
 await core.addScriptTag({url:origin+'/wasm_exec.js'});
 await core.evaluate(async()=>{const go=new globalThis.Go();const b=await(await fetch(chrome.runtime.getURL('dmz.wasm'))).arrayBuffer();const {instance}=await WebAssembly.instantiate(b,go.importObject);void go.run(instance);});
 await core.waitForFunction(()=>typeof globalThis.dmzProcessBestEffort==='function');
 await worker.evaluate(()=>{globalThis.__probeKeepalive=setInterval(()=>chrome.runtime.getPlatformInfo(),15000);});
 await worker.evaluate(()=>chrome.offscreen.createDocument({url:'scanner.html',reasons:[chrome.offscreen.Reason.WORKERS],justification:'Local synthetic integration comparison'}));
 for(const row of rows){
  const t=Date.now();const scan=await worker.evaluate(text=>chrome.runtime.sendMessage({type:'dmz:scanner-run',text}),row.text);
  if((scan?.error&&(committed||scan.code!=='unsupported_input'))||(!scan?.error&&!Array.isArray(scan?.spans)))throw Error('Candidate trained scanner failed; refuse a basic-only result');
  const record={id:row.id,text_sha256:digest(row.text),stratum:row.stratum,names:scan.spans||[],scanner_code:scan.code||null,scan_seconds:(Date.now()-t)/1000};
  for(const mode of ['basic','trained']){
   const names=mode==='trained'&&!scan.error?scan.spans:[];
   const result=await core.evaluate(({text,names})=>{globalThis.dmzReset();const t=JSON.parse(globalThis.dmzSetTenantTerms(JSON.stringify(['Verkefni Gervi'])));if(!t.ok)throw Error('terms failed');const r=JSON.parse(globalThis.dmzProcessBestEffort(text,JSON.stringify(names)));return {result:r,restored:r.allowed?globalThis.dmzRestore(r.transformed):null};},{text:row.text,names});
   if(!result.result.allowed||result.restored!==row.text)throw Error('Candidate restoration failure');
   const ms=markers(result.result.transformed);
   const restored=await core.evaluate(ms=>ms.map(m=>globalThis.dmzRestore(m)),ms);
   record[mode]={predicted:(result.result.detected_spans||[]).map(s=>[s.start,s.end,s.kind]),abstained:!result.result.spans_available,coverage:coverage(goldFor(row),restoredMaskSpans(row.text,result.result.transformed,restored)),exact_restoration:true,altered:row.text!==result.result.transformed};
  }
  records.push(record);if(records.length%(committed?20:100)===0)console.log(JSON.stringify({completed:records.length}));
 }
 await checkAssets();if(await fileDigest(fileURLToPath(import.meta.url))!==sourceHash)throw Error('Measurement changed');
 if(requests)throw Error('External request observed');
 await writeFile(out,JSON.stringify({schema:1,status:'complete',execution:'Fresh trained Chromium inference and WASM masking/restoration',build_manifest_sha256:buildHash,build,corpus:manifest,measurement_sha256:sourceHash,external_requests:requests,seconds:(Date.now()-started)/1000,records},null,2)+'\n',{flag:'wx'});
 console.log(JSON.stringify({status:'complete',excerpts:records.length,external_requests:requests}));
}finally{if(browser)await browser.close();await rm(profile,{recursive:true,force:true});}
