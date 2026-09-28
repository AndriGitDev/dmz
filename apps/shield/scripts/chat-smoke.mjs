import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const extension = path.join(root, '.output/chrome-mv3');
const profile = await mkdtemp(path.join(os.tmpdir(), 'dmz-chat-smoke-'));
const html = `<!doctype html><html><body>
  <form id="chat"><textarea id="prompt-textarea"></textarea><button data-testid="send-button">Send</button></form>
  <input id="upload" type="file"><button aria-label="Attach files" id="attach">Attach</button>
  <main id="responses"></main>
  <script>
    window.sent = [];
    window.uploads = 0;
    document.querySelector('#upload').addEventListener('input', () => window.uploads++);
    document.querySelector('#upload').addEventListener('change', () => window.uploads++);
    document.querySelector('#attach').addEventListener('click', () => window.uploads++);
    document.addEventListener('drop', () => window.uploads++);
    document.addEventListener('paste', () => window.uploads++);
    window.model = '';
    document.querySelector('#prompt-textarea').addEventListener('input', event => { window.model = event.target.value; });
    document.querySelector('#prompt-textarea').addEventListener('keydown', event => {
      if (event.key === 'Enter' && !event.shiftKey) { event.preventDefault(); document.querySelector('#chat').requestSubmit(); }
    });
    document.querySelector('#chat').addEventListener('submit', event => {
      event.preventDefault();
      window.sent.push(window.model);
      const reply = document.createElement('article');
      reply.dataset.messageAuthorRole = 'assistant';
      reply.textContent = window.sent.at(-1);
      document.querySelector('#responses').append(reply);
    });
  </script>
</body></html>`;
const claudeHtml = `<!doctype html><html><body>
  <div data-testid="chat-input"><div contenteditable="true" data-placeholder="Message"></div></div>
  <button aria-label="Send message">Send</button>
  <main id="responses"></main>
  <script>
    window.sent = [];
    document.querySelector('button').addEventListener('click', () => {
      const text = document.querySelector('[contenteditable]').innerText;
      window.sent.push(text);
      const reply = document.createElement('article');
      reply.dataset.testid = 'assistant-message';
      reply.textContent = text;
      document.querySelector('#responses').append(reply);
    });
  </script>
</body></html>`;
let browser;

try {
  browser = await chromium.launchPersistentContext(profile, {
    headless: true,
    channel: 'chromium',
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
    args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`],
  });
  const page = await browser.newPage();
  const errors = [];
  page.on('pageerror', error => errors.push(error.message));
  page.on('console', message => { if (message.type() === 'error') errors.push(message.text()); });
  await page.route('https://chatgpt.com/**', route => route.fulfill({ status: 200, contentType: 'text/html', body: html }));
  await page.goto('https://chatgpt.com/');
  await page.locator('#prompt-textarea').fill('Arna Jónsdóttir, 140182-8858');
  await page.locator('[data-testid="send-button"]').click();
  try {
    await page.waitForFunction(() => window.sent.length === 1, null, { timeout: 15000 });
  } catch (error) {
    throw new Error(`No replay. field=${await page.locator('#prompt-textarea').inputValue()}; sent=${JSON.stringify(await page.evaluate(() => window.sent))}; status=${await page.locator('#dmz-shield-status').count()}; errors=${errors.join(' | ')}`, { cause: error });
  }
  const outbound = await page.evaluate(() => window.sent[0]);
  assert.match(outbound, /⟦KT-[A-F0-9]{16}-\d+: XXXXX-XXXX⟧/);
  assert.match(outbound, /⟦NAME-[A-F0-9]{16}-\d+: XXXXX XXXXX⟧/);
  assert.ok(!outbound.includes('140182-8858'));
  await page.locator('[data-dmz-restored="true"]').waitFor();
  assert.equal(await page.locator('[data-dmz-restored="true"]').evaluate(element => element.shadowRoot), null);
  const cdp = await browser.newCDPSession(page);
  const tree = await cdp.send('Accessibility.getFullAXTree');
  assert.ok(tree.nodes.some(node => node.name?.value?.includes('140182-8858')), 'restored value absent from local overlay');
  await page.locator('[data-message-author-role="assistant"]').first().evaluate(element => element.remove());
  await page.locator('[data-dmz-restored="true"]').waitFor({ state: 'detached' });

  await page.locator('#prompt-textarea').fill('sk-test_key_000000000000');
  await page.locator('[data-testid="send-button"]').click();
  assert.equal(await page.evaluate(() => window.sent.length), 1, 'blocked secret reached page submit handler');
  await page.locator('#prompt-textarea').fill('demo@example.com');
  await page.locator('#prompt-textarea').press('Enter');
  await page.waitForFunction(() => window.sent.length === 2, null, { timeout: 10000 });
  const emailMarker = await page.evaluate(() => window.sent[1]);
  assert.match(emailMarker, /⟦EMAIL-[A-F0-9]{16}-\d+: XXXXX@XXXXX\.XXX⟧/);
  await page.locator('#prompt-textarea').fill('demo@example.com');
  await page.locator('[data-testid="send-button"]').click();
  await page.waitForFunction(() => window.sent.length === 3, null, { timeout: 10000 });
  assert.equal(await page.evaluate(() => window.sent[2]), emailMarker, 'same-tab marker changed across turns');
  await page.locator('#prompt-textarea').evaluate(element => element.removeAttribute('id'));
  await page.locator('textarea').fill('demo@example.com');
  await page.locator('[data-testid="send-button"]').click();
  assert.equal(await page.evaluate(() => window.sent.length), 3, 'unknown composer was submitted');
  await page.locator('#upload').setInputFiles({ name: 'synthetic.txt', mimeType: 'text/plain', buffer: Buffer.from('fabricated') });
  assert.equal(await page.evaluate(() => window.uploads), 0, 'file selection reached page handler');
  assert.equal(await page.locator('#upload').evaluate(element => element.files.length), 0, 'selected file was retained');
  await page.locator('#attach').click();
  assert.equal(await page.evaluate(() => window.uploads), 0, 'attach action reached page handler');
  await page.locator('body').evaluate(element => {
    const transfer = new DataTransfer();
    transfer.items.add(new File(['fabricated'], 'synthetic.txt', { type: 'text/plain' }));
    element.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: transfer }));
  });
  assert.equal(await page.evaluate(() => window.uploads), 0, 'file drop reached page handler');
  await page.locator('textarea').evaluate(element => {
    const transfer = new DataTransfer();
    transfer.items.add(new File(['fabricated'], 'synthetic.txt', { type: 'text/plain' }));
    element.dispatchEvent(new ClipboardEvent('paste', { bubbles: true, cancelable: true, clipboardData: transfer }));
  });
  assert.equal(await page.evaluate(() => window.uploads), 0, 'file paste reached page handler');
  assert.match(await page.locator('#dmz-shield-status').getAttribute('style'), /position:\s*fixed/);

  const claude = await browser.newPage();
  claude.on('pageerror', error => errors.push(error.message));
  await claude.route('https://claude.ai/**', route => route.fulfill({ status: 200, contentType: 'text/html', body: claudeHtml }));
  await claude.goto('https://claude.ai/new');
  await claude.locator('[contenteditable]').fill('demo@example.com');
  await claude.locator('button[aria-label="Send message"]').click();
  await claude.waitForFunction(() => window.sent.length === 1, null, { timeout: 15000 });
  assert.match(await claude.evaluate(() => window.sent[0]), /⟦EMAIL-[A-F0-9]{16}-\d+: XXXXX@XXXXX\.XXX⟧/);
  await claude.locator('[data-dmz-restored="true"]').waitFor();
  const claudeTree = await (await browser.newCDPSession(claude)).send('Accessibility.getFullAXTree');
  assert.ok(claudeTree.nodes.some(node => node.name?.value?.includes('demo@example.com')), 'Claude restored value absent from local overlay');
  const oldMarker = await claude.evaluate(() => window.sent[0]);
  // DevTools can inspect closed roots for verification; the page itself cannot.
  const claudeCdp = await browser.newCDPSession(claude);
  const { root: dom } = await claudeCdp.send('DOM.getDocument', { depth: -1, pierce: true });
  function findReset(node) {
    if (node.attributes?.includes('data-dmz-reset')) return node;
    for (const child of [...(node.children ?? []), ...(node.shadowRoots ?? [])]) {
      const found = findReset(child);
      if (found) return found;
    }
  }
  const control = findReset(dom);
  assert.ok(control, 'tab reset control missing');
  const { object } = await claudeCdp.send('DOM.resolveNode', { backendNodeId: control.backendNodeId });
  await claudeCdp.send('Runtime.callFunctionOn', { objectId: object.objectId, functionDeclaration: 'function() { this.click(); }' });
  await claudeCdp.send('Runtime.releaseObject', { objectId: object.objectId });
  await claude.locator('[data-dmz-restored="true"]').waitFor({ state: 'detached' });
  await claude.locator('[contenteditable]').fill('demo@example.com');
  await claude.locator('button[aria-label="Send message"]').click();
  await claude.waitForFunction(() => window.sent.length === 2);
  assert.notEqual(await claude.evaluate(() => window.sent[1]), oldMarker, 'reset reused the old session namespace');
  await claude.locator('[data-dmz-restored="true"]').waitFor();
  assert.equal(await claude.locator('[data-dmz-restored="true"]').count(), 1, 'old reply restored after reset');
  assert.deepEqual(errors, []);
  console.log('ChatGPT and Claude synthetic interception, block and restore smoke tests passed');
} finally {
  if (browser) await browser.close();
  await rm(profile, { recursive: true, force: true });
}
