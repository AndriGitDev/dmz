// SPDX-License-Identifier: GPL-3.0-only
// Development asset generation only; no remote files, fonts or runtime deps.
import {createRequire} from 'node:module';
import {readFile} from 'node:fs/promises';
import {resolve} from 'node:path';
import {fileURLToPath} from 'node:url';
const {chromium}=createRequire(import.meta.url)(resolve(process.env.DMZ_TEST_PLAYWRIGHT || fileURLToPath(new URL('../apps/dmz/node_modules/playwright',import.meta.url))));
const svg=await readFile(new URL('../assets/icons/dmz.svg',import.meta.url),'utf8');
const browser=await chromium.launch({headless:true,...(process.env.DMZ_TEST_BROWSER?{executablePath:process.env.DMZ_TEST_BROWSER}:{})});
try {
 const page=await browser.newPage({deviceScaleFactor:1});
 await page.route('**/*',r=>r.abort());
 for(const size of [16,32,48,64,128,256,512,1024]) {
  await page.setViewportSize({width:size,height:size});
  await page.setContent(`<style>html,body{margin:0;background:transparent}svg{display:block;width:100vw;height:100vh}</style>${svg}`);
  await page.screenshot({path:new URL(`../assets/icons/png/${size}.png`,import.meta.url).pathname,omitBackground:true});
 }
 await page.setViewportSize({width:36,height:36});
 await page.setContent(`<style>html,body{margin:0;background:transparent}svg{display:block;width:100vw;height:100vh}svg>rect:nth-of-type(1),svg>rect:nth-of-type(2){display:none}svg>path,svg>rect:last-of-type{fill:black}</style>${svg}`);
 await page.locator('svg').evaluate(svg => svg.setAttribute('viewBox','180 180 640 640'));
 await page.screenshot({path:new URL('../assets/icons/MenuBar.png',import.meta.url).pathname,omitBackground:true});
} finally {await browser.close();}
