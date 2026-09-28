import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
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
    await page.locator('button').click();
    if (mode === 'normal') {
      await page.waitForFunction(() => window.sent.length === 1);
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
      assert.equal(await page.evaluate(() => window.sent.length), 0, `${mode}: changed composer reached submit handler`);
      assert.deepEqual(bodies, [], `${mode}: changed composer reached network`);
    }
    await page.close();
  }
  console.log('Shield network-body and composer-race checks passed');
} finally {
  if (browser) await browser.close();
  await rm(profile, { recursive: true, force: true });
}
