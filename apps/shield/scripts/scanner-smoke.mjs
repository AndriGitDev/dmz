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
  const frames = p.frames().filter((f) => f.url().includes('restored.html'));
  assert.equal(frames.length, 1);
  await frames[0].waitForFunction(() => document.body.textContent.includes('Anna'));
  assert.ok(!(await p.locator('main').textContent()).includes('Anna'));
  metrics.cases.push({ case: 'isolated-restoration', passed: true });
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
  // A second tab cannot queue or bypass a busy model.
  await fill(p, 'Peter kom á fundinn. '.repeat(100));
  await fill(second, 'Anna mætti aftur.');
  await status(second, 'Sending stöðvuð');
  assert.equal(await second.evaluate(() => window.sent.length), 1);
  await p.waitForFunction(() => window.sent.length === 2, null, { timeout: 130000 });
  metrics.cases.push({ case: 'busy-model-blocks-competing-tab', passed: true });
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
  // Killing the offscreen scanner during work must not pass original text.
  await fill(p, 'Peter kom á fundinn. '.repeat(200));
  await sw.evaluate(() => chrome.offscreen.closeDocument());
  await status(p, 'Sending stöðvuð');
  assert.equal(await p.evaluate(() => window.sent.length), 2);
  metrics.cases.push({ case: 'scanner-termination-blocks-send', passed: true });
  await close();
  // A corrupt packaged model is an error, never a fallback to the old detector.
  const model = join(extension, 'scanner/model.onnx'),
    original = await readFile(model);
  try {
    await writeFile(model, 'corrupt');
    await launch();
    const q = await page();
    await fill(q, 'Anna mætti.');
    await status(q, 'Sending stöðvuð');
    assert.equal(await q.evaluate(() => window.sent.length), 0);
    metrics.cases.push({ case: 'corrupt-model-blocks-send', passed: true });
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
      await fill(q, 'Anna mætti.');
      await status(q, 'Sending stöðvuð', 135000);
      assert.equal(await q.evaluate(() => window.sent.length), 0);
      const elapsed = Date.now() - start;
      assert.ok(elapsed >= 119000 && elapsed < 135000);
      metrics.cases.push({
        case: 'unresponsive-worker-deadline-blocks-send',
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
