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
  await page.locator('#mask').waitFor({ state: 'visible' });
  try {
    await page.waitForFunction(() => !document.querySelector('#mask').disabled, null, { timeout: 10000 });
  } catch (error) {
    throw new Error(`Core did not start. Status: ${await page.locator('#status').textContent()}; errors: ${errors.join(' | ')}`, { cause: error });
  }

  const original = 'Arna Jónsdóttir, 140182-8858, demo@example.com';
  await page.locator('#prompt').fill(original);
  await page.locator('#mask').click();
  const outbound = await page.locator('#masked').inputValue();
  assert.match(outbound, /⟦KT-[A-F0-9]{16}-\d+: XXXXX-XXXX⟧/);
  assert.match(outbound, /⟦NAME-[A-F0-9]{16}-\d+: XXXXX XXXXX⟧/);
  assert.match(outbound, /⟦EMAIL-[A-F0-9]{16}-\d+: XXXXX@XXXXX\.XXX⟧/);
  assert.ok(!outbound.includes('140182-8858'));
  assert.ok(!outbound.includes('demo@example.com'));
  if (process.env.DMZ_SCREENSHOT) await page.screenshot({ path: process.env.DMZ_SCREENSHOT, fullPage: true });

  await page.locator('#answer').fill(outbound);
  await page.locator('#restore').click();
  assert.equal(await page.locator('#restored').inputValue(), original);

  await page.locator('#prompt').fill('sk-test_key_000000000000');
  await page.locator('#mask').click();
  assert.equal(await page.locator('#masked').inputValue(), '');
  assert.equal(await page.locator('#copy').isDisabled(), true);

  await page.locator('#reset').click();
  assert.equal(await page.locator('#prompt').inputValue(), '');
  await page.locator('#prompt').fill('140182-8868');
  await page.locator('#mask').click();
  assert.notEqual(await page.locator('#masked').inputValue(), outbound);
  await page.locator('#answer').fill(outbound);
  await page.locator('#restore').click();
  assert.equal(await page.locator('#restored').inputValue(), outbound, 'old markers restored in a new session');
  assert.ok(requests.every((url) => url.startsWith(`chrome-extension://${id}/`)), `Unexpected request: ${requests.join(', ')}`);
  console.log('Chromium extension mask/block/restore smoke test passed');
} finally {
  if (browser) await browser.close();
  await rm(profile, { recursive: true, force: true });
}
