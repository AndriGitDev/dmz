// Accuracy replay only. Private pretokenized corpora and predictions stay outside Git.
import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { createServer } from 'node:http';
import { createReadStream } from 'node:fs';
import { readFile, writeFile, stat } from 'node:fs/promises';
import { dirname, join, extname, resolve, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
const root=dirname(fileURLToPath(import.meta.url));
const repo=resolve(root,'../..');
function external(value) {
  assert.ok(value,'Missing asset/vector/prediction path');
  const path=resolve(value), rel=relative(repo,path);
  assert.ok(rel==='..'||rel.startsWith('../'),'Private files must stay outside Git');
  return path;
}
const assets=external(process.env.DMZ_MODEL_ASSETS);
const vectorsPath=external(process.env.DMZ_BROWSER_CORPUS);
const output=external(process.env.DMZ_BROWSER_PREDICTIONS);
try { await stat(output); throw new Error('Refusing to overwrite predictions'); }
catch(error) { if(error.code!=='ENOENT')throw error; }
async function digest(path) {const hash=createHash('sha256');for await(const chunk of createReadStream(path))hash.update(chunk);return hash.digest('hex');}
const vectors=JSON.parse(await readFile(vectorsPath,'utf8'));
const model=process.env.DMZ_MODEL_NAME || 'int8';
assert.ok(['fp32','int8','int4','mixed','int4b16','int4a32','hqq4','repair1','repair2'].includes(model));
assert.equal(await digest(join(assets,model+'.onnx')),vectors.model_sha256);
const result={schema:1,model_sha256:vectors.model_sha256,vectors_sha256:await digest(vectorsPath),runtime:'onnxruntime-web 1.23.2',threads:3,batch_size:1,externalRequests:[],sets:[]};
const runtime=join(root,'node_modules/onnxruntime-web/dist');
const server=createServer(async(req,res)=>{
  const path=new URL(req.url,'http://localhost').pathname;
  const headers={'Cache-Control':'no-store','Cross-Origin-Opener-Policy':'same-origin','Cross-Origin-Embedder-Policy':'require-corp'};
  if(path==='/'){res.writeHead(200,{...headers,'Content-Type':'text/html'}).end('<!doctype html><script type="module" src="/client.mjs"></script>');return;}
  let file;
  if(['/worker.mjs','/client.mjs'].includes(path))file=join(root,path.slice(1));
  else if(path==='/'+model+'.onnx')file=join(assets,model+'.onnx');
  else if(/^\/runtime\/[a-z0-9.-]+$/.test(path))file=join(runtime,path.slice(9));
  if(!file){res.writeHead(404,headers).end();return;}
  try{const info=await stat(file);res.writeHead(200,{...headers,'Content-Length':info.size,'Content-Type':extname(file)==='.mjs'?'text/javascript':extname(file)==='.wasm'?'application/wasm':'application/octet-stream'});const stream=createReadStream(file);res.on('close',()=>stream.destroy());stream.pipe(res);}
  catch{res.writeHead(404,headers).end();}
});
let browser;
try {
  await new Promise(r=>server.listen(0,'127.0.0.1',r));
  const origin=`http://127.0.0.1:${server.address().port}`;
  browser=await chromium.launch({headless:true,...(process.env.DMZ_BROWSER_EXECUTABLE?{executablePath:process.env.DMZ_BROWSER_EXECUTABLE}:{})});result.browser=browser.version();
  const context=await browser.newContext();
  await context.route('**/*',route=>{const url=route.request().url();if(url.startsWith(origin+'/'))return route.continue();result.externalRequests.push(url);return route.abort();});
  const page=await context.newPage();await page.goto(origin);
  await page.waitForFunction(()=>typeof window.makeClient==='function');
  result.load=await page.evaluate(async model=>{window.makeClient();return window.client.request('load',{url:'/'+model+'.onnx',threads:3},120000);},model);
  assert.equal(result.load.isolated,true);
  for(const dataset of vectors.sets) {
    const run={name:dataset.name,labels:[]};const start=Date.now();
    for(const record of dataset.records) {
      const labels=await page.evaluate(async chunks=>{
        const {outputs}=await window.client.request('infer',{chunks},60000);
        if(outputs.length!==chunks.length)throw new Error('Missing chunk output');
        const labels=[];
        outputs.forEach((values,index)=>{
          if(values.length!==chunks[index].words*3||!values.every(Number.isFinite))throw new Error('Invalid logits');
          if(labels.length!==chunks[index].offset)throw new Error('Noncontiguous chunks');
          for(let i=0;i<values.length;i+=3){const logits=values.slice(i,i+3);labels.push(logits.indexOf(Math.max(...logits)));}
        });return labels;
      },record.chunks);
      assert.equal(labels.length,record.offsets.length);run.labels.push(labels);
      if(run.labels.length%256===0)console.log(JSON.stringify({set:run.name,records:run.labels.length,total:dataset.records.length,seconds:(Date.now()-start)/1000}));
    }
    run.seconds=(Date.now()-start)/1000;result.sets.push(run);
    console.log(JSON.stringify({set:run.name,complete:run.labels.length,seconds:run.seconds}));
  }
  assert.deepEqual(result.externalRequests,[]);
  await writeFile(output,JSON.stringify(result),{encoding:'utf8',flag:'wx'});
} finally {if(browser)await browser.close();await new Promise(r=>server.close(r));}
