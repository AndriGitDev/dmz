import assert from 'node:assert/strict';
import { cp, mkdtemp, rm, writeFile } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const profile = await mkdtemp(path.join(os.tmpdir(), 'dmz-chat-races-'));
const extension = path.join(root, '.output/chrome-mv3');
// The page deliberately changes its composer at two points in the send lifecycle.
// Every outgoing body is intercepted by Playwright; no fixture traffic leaves the test.
const html = `<!doctype html><html><head><meta charset="utf-8"></head><body>
<form id="chat"><textarea id="prompt-textarea"></textarea><button data-testid="send-button">Send</button></form>
<script>
window.sent = [];
window.raceTriggered = false;
const form = document.querySelector('form');
const button = document.querySelector('button');
document.querySelector('textarea').addEventListener('input', event => {
  if (window.mode === 'replace' && event.target.value.includes('⟦')) {
    const field = event.target;
    requestAnimationFrame(() => {
      const replacement = field.cloneNode();
      replacement.value = 'replacement@example.com';
      field.replaceWith(replacement);
      window.raceTriggered = true;
    });
  }
});
button.addEventListener('click', event => {
  if (window.mode === 'nested') {
    event.preventDefault();
    document.querySelector('textarea').value = 'nested@example.com';
    window.raceTriggered = true;
    form.requestSubmit();
  }
});
form.addEventListener('submit', event => {
  event.preventDefault();
  const text = document.querySelector('textarea').value;
  window.sent.push(text);
  void fetch('/__shield_test_send', { method: 'POST', body: text });
});
</script></body></html>`;
let browser;
try {
  browser = await chromium.launchPersistentContext(profile, {
    headless: true, channel: 'chromium',
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
    args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`],
  });
  for (const mode of ['normal', 'replace', 'nested']) {
    const page = await browser.newPage();
    const bodies = [];
    await page.route('https://chatgpt.com/**', route => route.fulfill({ contentType: 'text/html', body: html }));
    await page.route('https://chatgpt.com/__shield_test_send', route => {
      bodies.push(route.request().postData());
      return route.fulfill({ status: 204, body: '' });
    });
    await page.goto('https://chatgpt.com/');
    await page.locator('#dmz-shield-status').waitFor();
    await page.evaluate(value => { window.mode = value; }, mode);
    await page.locator('textarea').fill('source@example.com');
    // Register before clicking: the previous network-idle state can still be
    // true after the submit handler runs but before its fetch reaches routing.
    const sentResponse = mode === 'normal' ? page.waitForResponse(response =>
      response.url() === 'https://chatgpt.com/__shield_test_send' && response.request().method() === 'POST') : null;
    await page.locator('button').click();
    if (mode === 'normal') {
      await page.waitForFunction(() => window.sent.length === 1);
      await sentResponse;
      await page.waitForLoadState('networkidle');
      assert.equal(bodies.length, 1);
      assert.match(bodies[0], /^⟦EMAIL-[A-F0-9]{16}-\d+: XXXXX@XXXXX\.XXX⟧$/);
    } else {
      await page.waitForFunction(() => window.raceTriggered);
      await page.evaluate(() => new Promise(resolve => {
        let frames = 0;
        const next = () => ++frames === 4 ? resolve() : requestAnimationFrame(next);
        requestAnimationFrame(next);
      }));
      await page.waitForLoadState('networkidle');
      await page.waitForFunction(() => window.sent.length === 1);
      await page.waitForLoadState('networkidle');
      assert.deepEqual(bodies, [mode === 'replace' ? 'replacement@example.com' : 'nested@example.com'], `${mode}: current draft was not submitted`);
    }
    await page.close();
  }
  await browser.close();
  browser = undefined;
  const brokenExtension = path.join(profile, 'broken-extension');
  await cp(extension, brokenExtension, { recursive: true });
  await writeFile(path.join(brokenExtension, 'dmz.wasm'), new Uint8Array([0]));
  browser = await chromium.launchPersistentContext(path.join(profile, 'broken-profile'), {
    headless: true, channel: 'chromium',
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
    args: [`--disable-extensions-except=${brokenExtension}`, `--load-extension=${brokenExtension}`],
  });
  const failedPage = await browser.newPage();
  const failedBodies = [];
  await failedPage.route('https://chatgpt.com/**', route => route.fulfill({ contentType: 'text/html', body: html }));
  await failedPage.route('https://chatgpt.com/__shield_test_send', route => {
    failedBodies.push(route.request().postData());
    return route.fulfill({ status: 204, body: '' });
  });
  await failedPage.goto('https://chatgpt.com/');
  await failedPage.locator('#dmz-shield-status').waitFor({ timeout: 10000 });
  const accessibility = await (await browser.newCDPSession(failedPage)).send('Accessibility.getFullAXTree');
  assert.ok(accessibility.nodes.some(node => node.name?.value?.includes('DMZ-kjarni bilaður')), 'broken core had no visible error before sending');
  await failedPage.locator('textarea').fill('source@example.com');
  await failedPage.locator('button').click();
  await failedPage.waitForLoadState('networkidle');
  await failedPage.waitForFunction(() => window.sent.length === 1);
  await failedPage.waitForLoadState('networkidle');
  assert.deepEqual(failedBodies, ['source@example.com'], 'broken core blocked best-effort submission');
  console.log('DMZ network-body, composer-race and broken-core checks passed');
} finally {
  if (browser) await browser.close();
  await rm(profile, { recursive: true, force: true });
}
