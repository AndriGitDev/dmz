import assert from 'node:assert/strict';
import { mkdtemp, rm, readFile, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
const root = fileURLToPath(new URL('../', import.meta.url)),
  extension = join(root, '.output/chrome-mv3');
const html = `<!doctype html><html><body><form><textarea id="prompt-textarea"></textarea><button data-testid="send-button">Send</button></form><main></main><script>
window.sent=[];document.querySelector('form').onsubmit=e=>{e.preventDefault();const value=document.querySelector('textarea').value;window.sent.push(value);const reply=document.createElement('article');reply.dataset.messageAuthorRole='assistant';reply.textContent=value;document.querySelector('main').append(reply);};
</script></body></html>`;
let browser, profile;
const traffic = [],
  metrics = {
    scope: 'Synthetic extension workflow checks; not independent detector accuracy',
    cases: [],
  };
async function launch() {
  profile = await mkdtemp(join(tmpdir(), 'dmz-trained-'));
  browser = await chromium.launchPersistentContext(profile, {
    headless: true,
    channel: 'chromium',
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
    args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`],
  });
  browser.on('request', (r) => {
    if (!r.url().startsWith('chrome-extension://')) traffic.push(r.url());
  });
  await browser.route('https://chatgpt.com/**', (r) =>
    r.fulfill({ contentType: 'text/html', body: html }),
  );
  let sw = browser.serviceWorkers()[0];
  if (!sw) sw = await browser.waitForEvent('serviceworker');
  return sw;
}
async function close() {
  if (browser) await browser.close();
  if (profile) await rm(profile, { recursive: true, force: true });
  browser = undefined;
}
async function status(p, fragment, timeout = 130000) {
  const cdp = await browser.newCDPSession(p);
  const start = Date.now();
  try {
    while (Date.now() - start < timeout) {
      const tree = await cdp.send('Accessibility.getFullAXTree');
      if (tree.nodes.some((n) => n.name?.value?.includes(fragment))) return;
      await new Promise((r) => setTimeout(r, 100));
    }
    throw Error('Status not reached: ' + fragment);
  } finally {
    await cdp.detach();
  }
}
async function page() {
  const p = await browser.newPage();
  await p.goto('https://chatgpt.com/');
  await status(p, 'Shield virkur');
  return p;
}
async function fill(p, text) {
  await p.locator('textarea').fill(text);
  await p.locator('button').click();
}
try {
  const sw = await launch(),
    p = await page();
  const start = Date.now();
  await fill(p, 'Anna ræddi við Peter í gær.');
  await p.waitForFunction(() => window.sent.length === 1, null, { timeout: 130000 });
  const sent = await p.evaluate(() => window.sent[0]);
  assert.ok(!sent.includes('Anna') && !sent.includes('Peter'));
  assert.match(sent, /⟦NAME-/);
  metrics.cases.push({
    case: 'actual-name-masking',
    passed: true,
    firstScanMs: Date.now() - start,
  });
  await p.locator('iframe[data-dmz-restored]').waitFor();
  await p.frameLocator('iframe[data-dmz-restored]').getByText('Anna ræddi við Peter í gær.', { exact: true }).waitFor();
  assert.ok(!(await p.locator('main').textContent()).includes('Anna'));
  metrics.cases.push({ case: 'isolated-restoration', passed: true });
  // Names embedded in an email must keep the full email mask, not block or
  // replace only the local part. Use a separate page to keep race counts stable.
  const emailPage = await page();
  const emailInput = 'Skrifaðu 10 setningar sem innihalda eftirfarandi: anna@example.com, peter@example.org and 1401828858';
  await fill(emailPage, emailInput);
  await emailPage.waitForFunction(() => window.sent.length === 1, null, { timeout: 130000 });
  const emailSent = await emailPage.evaluate(() => window.sent[0]);
  for (const value of ['anna', 'peter', 'example.com', 'example.org', '1401828858']) assert.ok(!emailSent.includes(value));
  assert.equal((emailSent.match(/⟦EMAIL-/g) || []).length, 2);
  assert.match(emailSent, /⟦KT-/);
  await emailPage.locator('iframe[data-dmz-restored]').waitFor();
  await emailPage.frameLocator('iframe[data-dmz-restored]').getByText(emailInput, { exact: true }).waitFor();
  await emailPage.close();
  metrics.cases.push({ case: 'names-inside-emails-mask-and-restore', passed: true });
  // Editing the draft during asynchronous inference cannot be overwritten or sent.
  await fill(p, 'Peter kom á fundinn.');
  await p.locator('textarea').fill('Ný útgáfa textans.');
  await status(p, 'breyttist');
  assert.equal(await p.evaluate(() => window.sent.length), 1);
  assert.equal(await p.locator('textarea').inputValue(), 'Ný útgáfa textans.');
  metrics.cases.push({ case: 'edit-during-inference', passed: true });
  const second = await page();
  await fill(second, 'Anna mætti.');
  await second.waitForFunction(() => window.sent.length === 1, null, { timeout: 130000 });
  const contexts = await sw.evaluate(() =>
    chrome.runtime.getContexts({ contextTypes: ['OFFSCREEN_DOCUMENT'] }),
  );
  assert.equal(contexts.length, 1);
  assert.notEqual(
    (await second.evaluate(() => window.sent[0])).match(/NAME-([A-F0-9]+)/)[1],
    sent.match(/NAME-([A-F0-9]+)/)[1],
  );
  metrics.cases.push({ case: 'one-model-separate-tab-mappings', passed: true });
  // A competing tab uses basic masking with a warning instead of queueing.
  await fill(p, 'Peter kom á fundinn. '.repeat(100));
  await fill(second, 'Anna mætti aftur.');
  await status(second, 'Sent með grunnmöskun');
  assert.equal(await second.evaluate(() => window.sent.length), 2);
  await p.waitForFunction(() => window.sent.length === 2, null, { timeout: 130000 });
  metrics.cases.push({ case: 'busy-model-warns-and-uses-basic-masking', passed: true });
  // A reset during inference invalidates the pending result.
  const settings = await browser.newPage();
  await settings.goto(new URL('sidepanel.html', sw.url()).href);
  await p.bringToFront();
  const tabId = await sw.evaluate(
    async () => (await chrome.tabs.query({ active: true, currentWindow: true }))[0]?.id,
  );
  assert.ok(tabId !== undefined);
  await fill(p, 'Peter kom á fundinn. '.repeat(100));
  const reset = await settings.evaluate(
    (id) => chrome.tabs.sendMessage(id, { type: 'dmz:settings-reset' }),
    tabId,
  );
  assert.ok(reset.ok);
  await status(p, 'breyttist');
  assert.equal(await p.evaluate(() => window.sent.length), 2);
  await settings.close();
  metrics.cases.push({ case: 'reset-during-inference', passed: true });
  // Scanner termination still applies the basic detector before sending.
  await fill(p, 'Peter kom á fundinn. '.repeat(200) + 'fallback@example.com');
  await sw.evaluate(() => chrome.offscreen.closeDocument());
  await status(p, 'Sent með grunnmöskun');
  assert.equal(await p.evaluate(() => window.sent.length), 3);
  assert.ok(!(await p.evaluate(() => window.sent[2])).includes('fallback@example.com'));
  metrics.cases.push({ case: 'scanner-termination-warns-and-masks-email', passed: true });
  await close();
  // A corrupt model falls back to basic masking with a visible warning.
  const model = join(extension, 'scanner/model.onnx'),
    original = await readFile(model);
  try {
    await writeFile(model, 'corrupt');
    await launch();
    const q = await page();
    await fill(q, 'Anna mætti. fallback@example.com sk-test_key_000000000000');
    await status(q, 'Sent með grunnmöskun');
    assert.equal(await q.evaluate(() => window.sent.length), 1);
    assert.ok(!(await q.evaluate(() => window.sent[0])).includes('fallback@example.com'));
    assert.ok(!(await q.evaluate(() => window.sent[0])).includes('sk-test'));
    metrics.cases.push({ case: 'corrupt-model-warns-and-masks-identifiers', passed: true });
  } finally {
    await close();
    await writeFile(model, original);
  }
  if (process.env.DMZ_SCANNER_DEADLINE_TEST === '1') {
    const file = join(extension, 'scanner/worker.mjs'),
      originalWorker = await readFile(file);
    try {
      await writeFile(file, 'self.onmessage = () => {};');
      await launch();
      const q = await page();
      const start = Date.now();
      await fill(q, 'Anna mætti. fallback@example.com sk-test_key_000000000000');
      await status(q, 'Sent með grunnmöskun', 135000);
      assert.equal(await q.evaluate(() => window.sent.length), 1);
      assert.ok(!(await q.evaluate(() => window.sent[0])).includes('fallback@example.com'));
      const elapsed = Date.now() - start;
      assert.ok(elapsed >= 119000 && elapsed < 135000);
      metrics.cases.push({
        case: 'unresponsive-worker-deadline-falls-back',
        passed: true,
        elapsedMs: elapsed,
      });
    } finally {
      await close();
      await writeFile(file, originalWorker);
    }
  }
  assert.ok(
    traffic.every((url) => url === 'https://chatgpt.com/'),
    'Unexpected network traffic',
  );
  metrics.externalScannerRequests = 0;
  console.log(JSON.stringify(metrics, null, 2));
  if (process.env.DMZ_SCANNER_REPORT)
    await writeFile(
      resolve(process.env.DMZ_SCANNER_REPORT),
      JSON.stringify(metrics, null, 2) + '\n',
      { flag: 'wx' },
    );
} finally {
  await close();
}
