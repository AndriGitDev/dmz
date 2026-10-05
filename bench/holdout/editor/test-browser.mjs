// Offline editor verification uses fabricated excerpts only.
import assert from 'node:assert/strict';
import {readFile, writeFile, mkdtemp, rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join, resolve} from 'node:path';
import {fileURLToPath, pathToFileURL} from 'node:url';
import {execFileSync} from 'node:child_process';
import {createRequire} from 'node:module';
const require = createRequire(new URL('../../../apps/shield/package.json',import.meta.url));
const {chromium} = require('playwright');
const repo = fileURLToPath(new URL('../../../',import.meta.url));
const directory = await mkdtemp(join(tmpdir(),'dmz-review-editor-'));
let browser;
const python = process.env.PYTHON_BIN || 'python3';
const addressValue = 'Gervislo\u0301ð 99';
const sourceText = `🙂 Ása Demo\r\nbýr á ${addressValue}.\r\n`;
const texts = [sourceText,'Ordinary fabricated excerpt. </script><img src="https://blocked.invalid/private"><script>globalThis.injected=true</script>', 'Fabricated extraction error 🧪.'];
try {
  execFileSync(python,['-c',`
import json,sys,zipfile
from pathlib import Path
sys.path.insert(0,'bench/holdout')
from entity_review import prepare,package_reviewer,sha
root=Path(sys.argv[1]);texts=json.loads(sys.argv[2]);source=root/'source.jsonl'
source.write_text(''.join(json.dumps(dict(document_id='synthetic-'+str(i),page=1,text=text,text_sha256=sha(text.encode()),entities=None,reviewed=False))+'\\n' for i,text in enumerate(texts)))
prepare([source],root/'packet',terms=['Gervislóð 99'])
package_reviewer(root/'packet','a',root/'a.zip')
with zipfile.ZipFile(root/'a.zip') as bundle:(root/'review.html').write_bytes(bundle.read('review.html'))
`,directory,JSON.stringify(texts)],{cwd:repo,stdio:'pipe'});
  browser = await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE || undefined});
  const context = await browser.newContext({acceptDownloads:true,viewport:{width:1280,height:1000}});
  let network = 0;
  await context.route(/^https?:/, route => {network++;return route.abort();});
  const page = await context.newPage();
  const url = pathToFileURL(join(directory,'review.html')).href;
  await page.goto(url);
  await page.locator('#workspace').waitFor({state:'visible'});
  assert.match(await page.locator('#packet-title').textContent(),/Offline packet verified/);
  assert.equal(await page.locator('#excerpt').textContent(),sourceText);
  assert.equal(await page.locator('#export').isDisabled(),true);
  await page.locator('#reviewer').fill('reviewer-synthetic-a');
  async function select(page,start,end) {
    await page.evaluate(({start,end}) => {
      const root=document.getElementById('excerpt'), walker=document.createTreeWalker(root,NodeFilter.SHOW_TEXT);
      const nodes=[];let node,position=0;
      while(node=walker.nextNode()){nodes.push({node,start:position,end:position+node.textContent.length});position+=node.textContent.length;}
      const first=nodes.find(part=>start>=part.start && start<part.end),last=nodes.find(part=>end>part.start && end<=part.end);
      const range=document.createRange();range.setStart(first.node,start-first.start);range.setEnd(last.node,end-last.start);
      const selection=window.getSelection();selection.removeAllRanges();selection.addRange(range);
    },{start,end});
  }
  await select(page,3,11);
  await page.locator('#kind').selectOption('TenantTerm');
  await page.locator('#add').click();
  assert.match(await page.locator('#message').textContent(),/configured term/);
  await page.locator('#kind').selectOption('Person');
  await page.locator('#add').click();
  assert.equal(await page.locator('#annotations li').count(),1);
  assert.equal(await page.locator('#excerpt').textContent(),sourceText);
  await select(page,3,11);await page.locator('#add').click();
  assert.match(await page.locator('#message').textContent(),/overlap/);
  const addressStart=sourceText.indexOf(addressValue);
  await select(page,addressStart,addressStart+addressValue.length);
  await page.locator('#kind').selectOption('Address');await page.locator('#add').click();
  assert.equal(await page.locator('#annotations li').count(),2);
  await page.locator('#reviewed').click();
  assert.match(await page.locator('#message').textContent(),/Confirm extraction/);
  await page.locator('#extraction').check();await page.locator('#all-types').check();await page.locator('#reviewed').click();
  assert.match(await page.locator('#progress').textContent(),/1 of 3 reviewed/);
  assert.equal(await page.locator('#add').isDisabled(),true);
  await page.locator('#excerpt-select').selectOption('2');
  await page.locator('details').filter({has:page.locator('#exclusion')}).locator('summary').click();
  await page.locator('#exclusion').fill('Unfinished extraction note');
  await page.locator('#previous').click();await page.locator('#next').click();
  assert.equal(await page.locator('#exclusion').inputValue(),'Unfinished extraction note');
  const saved=page.waitForEvent('download');await page.locator('#save-progress').click();
  const download=await saved, savedPath=join(directory,'progress.json');await download.saveAs(savedPath);
  const progress=JSON.parse(await readFile(savedPath,'utf8'));
  assert.deepEqual(progress.rows[0].entities[0],{start:5,end:14,kind:'Person'});
  assert.deepEqual(progress.rows[0].entities[1],{start:Buffer.byteLength(sourceText.slice(0,addressStart)),end:Buffer.byteLength(sourceText.slice(0,addressStart)+addressValue),kind:'Address'});
  assert.equal(progress.rows[1].entities,null);
  assert.equal(progress.draft_exclusions[2],'Unfinished extraction note');
  const wrong={...progress,slot:'b'};
  await page.locator('#load-progress').setInputFiles({name:'wrong.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(wrong))});
  await page.getByText('Progress belongs to a different packet or reviewer slot',{exact:true}).waitFor();
  assert.match(await page.locator('#progress').textContent(),/1 of 3 reviewed/);
  const resumed=await context.newPage();await resumed.goto(url);await resumed.locator('#workspace').waitFor({state:'visible'});
  assert.match(await resumed.locator('#progress').textContent(),/0 of 3 reviewed/);
  await resumed.locator('#load-progress').setInputFiles(savedPath);
  await resumed.getByText('Progress restored after packet, slot and annotation checks.',{exact:true}).waitFor();
  assert.match(await resumed.locator('#progress').textContent(),/1 of 3 reviewed/);
  await resumed.locator('#next').click();
  assert.equal(await resumed.locator('#excerpt').textContent(),texts[1]);
  assert.equal(await resumed.locator('#excerpt img').count(),0);
  assert.equal(await resumed.evaluate(()=>globalThis.injected),undefined);
  await resumed.locator('#extraction').check();await resumed.locator('#all-types').check();await resumed.locator('#reviewed').click();
  assert.equal(await resumed.locator('#export').isDisabled(),true);
  await resumed.locator('#next').click();
  await resumed.locator('details').filter({has:resumed.locator('#exclusion')}).locator('summary').click();
  assert.equal(await resumed.locator('#exclusion').inputValue(),'Unfinished extraction note');
  await resumed.locator('#exclusion').fill('Missing source line in fabricated fixture');await resumed.locator('#exclude').click();
  assert.equal(await resumed.locator('#export').isDisabled(),false);
  const exported=resumed.waitForEvent('download');await resumed.locator('#export').click();
  const form=await exported,formPath=join(directory,'reviewer-a.jsonl');await form.saveAs(formPath);
  const rows=(await readFile(formPath,'utf8')).trim().split('\n').map(JSON.parse);
  assert.equal(rows.length,3);assert.deepEqual(rows[1].entities,[]);assert.equal(rows[2].entities,null);assert.equal(rows[2].extraction_verified,false);
  execFileSync(python,['-c',`
import sys
from pathlib import Path
sys.path.insert(0,'bench/holdout')
from entity_review import load_packet,submissions
root=Path(sys.argv[1]);_,index=load_packet(root/'packet')
assert len(submissions(root/'reviewer-a.jsonl',index,'reviewer'))==3
`,directory],{cwd:repo,stdio:'pipe'});
  assert.equal(await resumed.evaluate(async()=>{try{await fetch('https://blocked.invalid/csp-test');return false;}catch{return true;}}),true);
  assert.equal(network,0,'Offline editor attempted a network request');
  if(process.env.DMZ_EDITOR_SCREENSHOT) {
    await resumed.locator('#excerpt-select').selectOption('0');
    await resumed.screenshot({path:resolve(process.env.DMZ_EDITOR_SCREENSHOT),fullPage:true});
  }
  console.log('Offline Chromium review: UTF-8 selection, overlaps, explicit negatives/exclusions, private progress resume, Python-compatible export and network/markup isolation passed');
} finally {if(browser)await browser.close();await rm(directory,{recursive:true,force:true});}
