import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const extension = path.join(root, '.output/chrome-mv3');
const profile = await mkdtemp(path.join(os.tmpdir(), 'dmz-shield-smoke-'));
const requests = [];
const errors = [];
let browser;

try {
  browser = await chromium.launchPersistentContext(profile, {
    headless: true,
    channel: 'chromium',
    viewport: { width: 380, height: 900 },
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
    args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`],
  });
  let worker = browser.serviceWorkers()[0];
  if (!worker) worker = await browser.waitForEvent('serviceworker', { timeout: 10000 });
  const id = new URL(worker.url()).host;
  const page = await browser.newPage();
  page.on('request', (request) => requests.push(request.url()));
  page.on('pageerror', (error) => errors.push(error.message));
  page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()); });
  await page.goto(`chrome-extension://${id}/sidepanel.html`);
  assert.equal(await page.locator('h1').textContent(), 'Stillingar');
  assert.equal(await page.locator('textarea').count(), 0);
  await page.waitForFunction(() => document.querySelector('#tab-status').textContent.includes('Opnaðu'));
  assert.equal(await page.locator('#reset').isDisabled(), true);
  for (const mode of ['light', 'dark']) {
    await page.locator('#theme').selectOption(mode);
    await page.reload();
    assert.equal(await page.locator('#theme').inputValue(), mode);
    assert.equal(await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme), mode);
    await page.screenshot({ path: `/tmp/dmz-settings-${mode}.png`, fullPage: true });
    assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  }
  await page.locator('#theme').selectOption('system');
  for (const colorScheme of ['light', 'dark']) {
    await page.emulateMedia({ colorScheme });
    assert.equal(await page.evaluate(() => getComputedStyle(document.documentElement).colorScheme), colorScheme);
  }
  for (const site of ['chatgpt', 'claude']) {
    const chat = await browser.newPage();
    const host = site === 'chatgpt' ? 'chatgpt.com' : 'claude.ai';
    const field = site === 'chatgpt' ? '<textarea id="prompt-textarea"></textarea>' : '<div class="ProseMirror" contenteditable="true"></div>';
    await chat.route(`https://${host}/**`, route => route.fulfill({ contentType: 'text/html', body: `<!doctype html><html><body>${field}</body></html>` }));
    await chat.goto(`https://${host}/`);
    await chat.bringToFront();
    const click = selector => page.locator(selector).evaluate(element => element.click());
    for (let attempt = 0; attempt < 100; attempt++) {
      await click('#refresh');
      if (!(await page.locator('#reset').isDisabled())) break;
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert.match(await page.locator('#tab-status').textContent(), /Shield er tilbúinn/);
    await click('#reset');
    assert.equal(await page.locator('#confirmation').isVisible(), true);
    await click('#cancel-reset');
    assert.equal(await page.locator('#confirmation').isVisible(), false);
    await click('#reset');
    await click('#confirm-reset');
    await page.waitForFunction(() => document.querySelector('#action-status').textContent.includes('hreinsuð'));
    await click('#reset');
    const other = await browser.newPage();
    await other.goto('about:blank');
    await other.bringToFront();
    await page.waitForFunction(() => document.querySelector('#confirmation').hidden && document.querySelector('#reset').disabled);
    await other.close();
    await chat.close();
  }
  assert.deepEqual(errors, []);
  assert.ok(requests.every((url) => url.startsWith(`chrome-extension://${id}/`)), `Unexpected request: ${requests.join(', ')}`);
  assert.ok(!requests.some(url => url.endsWith('.wasm')), 'settings must not create a separate detector session');
  console.log('Chromium settings theme, active-tab status, reset and tab-switch smoke test passed');
} finally {
  if (browser) await browser.close();
  await rm(profile, { recursive: true, force: true });
}
