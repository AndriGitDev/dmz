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
  // An input-only rejection must preserve the healthy shared model worker.
  const inspector = await browser.newCDPSession(p);
  // This synthetic host creates no workers; Chromium exposes the extension
  // worker with an empty URL, so identify its sole dedicated-worker target.
  const workerIDs = async () => (await inspector.send('Target.getTargets')).targetInfos
    .filter(target => target.type === 'worker')
    .map(target => target.targetId).sort();
  const beforeRejectedInput = await workerIDs();
  assert.equal(beforeRejectedInput.length, 1, 'expected one warmed scanner worker');
  const rejected = await sw.evaluate(() => chrome.runtime.sendMessage({
    type: 'dmz:scanner-run', text: 'Gervi\u034ftexti',
  }));
  assert.equal(rejected.code, 'unsupported_input');
  assert.ok(rejected.error);
  assert.deepEqual(await workerIDs(), beforeRejectedInput, 'input rejection discarded the healthy model');
  const recovered = await sw.evaluate(() => chrome.runtime.sendMessage({
    type: 'dmz:scanner-run', text: 'Anna ræddi við Peter í gær.',
  }));
  assert.equal(recovered.error, undefined);
  assert.equal(recovered.spans.length, 2);
  assert.deepEqual(await workerIDs(), beforeRejectedInput, 'next scan reloaded the healthy model');
  await inspector.detach();
  metrics.cases.push({ case: 'unsupported-input-preserves-warm-model', passed: true });
  const decomposedPage = await page();
  const decomposed = 'Anna ræddi við Peter í gær.'.normalize('NFD');
  await fill(decomposedPage, decomposed);
  await decomposedPage.waitForFunction(() => window.sent.length === 1, null, { timeout: 130000 });
  const decomposedSent = await decomposedPage.evaluate(() => window.sent[0]);
  assert.ok(!decomposedSent.includes('Anna') && !decomposedSent.includes('Peter'));
  assert.equal((decomposedSent.match(/⟦NAME-/g) || []).length, 2, 'decomposed prose fell back or lost trained names');
  await decomposedPage.frameLocator('iframe[data-dmz-restored]').getByText(decomposed, { exact: true }).waitFor();
  assert.equal(await decomposedPage.frameLocator('iframe[data-dmz-restored]').locator('#restored-text').textContent(), decomposed);
  assert.ok(!(await decomposedPage.locator('main').textContent()).includes('Anna'));
  await decomposedPage.close();
  metrics.cases.push({ case: 'canonical-decomposed-trained-names-and-exact-restoration', passed: true });
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
  const keyPage = await page();
  const keyInput = 'Prófun: -----BEGIN PRIVATE KEY-----\nSYNTHETIC_NOT_A_KEY\n-----END PRIVATE KEY----- lokið.';
  await fill(keyPage, keyInput);
  await keyPage.waitForFunction(() => window.sent.length === 1, null, { timeout: 130000 });
  const keySent = await keyPage.evaluate(() => window.sent[0]);
  assert.ok(keySent.startsWith('Prófun: ') && keySent.endsWith(' lokið.'));
  assert.ok(!keySent.includes('SYNTHETIC') && !keySent.includes('PRIVATE KEY'));
  assert.equal((keySent.match(/⟦/g) || []).length, 1);
  await keyPage.frameLocator('iframe[data-dmz-restored]').getByText(keyInput, { exact: true }).waitFor();
  assert.equal(await keyPage.frameLocator('iframe[data-dmz-restored]').locator('#restored-text').textContent(), keyInput);
  await keyPage.close();
  metrics.cases.push({ case: 'complete-private-key-preserves-prose-and-restores', passed: true });
  const repeatPage = await page();
  const repeatInput = 'AuthoriZAtion:BeArer 0 0';
  await fill(repeatPage, repeatInput);
  await repeatPage.waitForFunction(() => window.sent.length === 1, null, { timeout: 130000 });
  const repeatSent = await repeatPage.evaluate(() => window.sent[0]);
  assert.match(repeatSent, /^AuthoriZAtion:BeArer ⟦[^⟦⟧]+⟧ 0$/);
  await repeatPage.frameLocator('iframe[data-dmz-restored]').getByText(repeatInput, { exact: true }).waitFor();
  await fill(repeatPage, repeatSent);
  await repeatPage.waitForFunction(() => window.sent.length === 2, null, { timeout: 130000 });
  assert.equal(await repeatPage.evaluate(() => window.sent[1]), repeatSent,
    'resubmitting a masked credential changed the following prose token');
  await repeatPage.close();
  metrics.cases.push({ case: 'issued-credential-marker-resubmission-is-stable', passed: true });
  if (process.env.DMZ_LEXICON_FILE) {
    // Local vocabulary plus a fabricated number; never persist source forms.
    const pack = JSON.parse(await readFile(process.env.DMZ_LEXICON_FILE, 'utf8'));
    const addresses = [`${pack.addresses[0]} 99b`, `${pack.addresses[0]} 97‑99`];
    const hyphenated = pack.addresses.find(value => value.includes('-'));
    if (hyphenated) addresses.push(`${hyphenated.replaceAll('-', '‐')} 97‐99`);
    for (const address of addresses) {
      const addressInput = `Skrifaðu 5 setningar: demo@example.com +354 555 0100 ${address}`;
      const addressPage = await page();
      await fill(addressPage, addressInput);
      await addressPage.waitForFunction(() => window.sent.length === 1, null, { timeout: 130000 });
      const outbound = await addressPage.evaluate(() => window.sent[0]);
      assert.ok(!outbound.includes(address), 'address remained in outbound text');
      assert.match(outbound, /^Skrifaðu 5 setningar: ⟦EMAIL-[^⟧]+⟧ ⟦PHONE-[^⟧]+⟧ ⟦VALUE-[^⟧]+⟧$/, 'partial address remained in outbound text');
      for (const kind of ['EMAIL', 'PHONE', 'VALUE']) assert.ok(outbound.includes(`⟦${kind}-`), `missing ${kind} mask`);
      await addressPage.locator('iframe[data-dmz-restored]').waitFor();
      const restored = await addressPage.frameLocator('iframe[data-dmz-restored]').locator('body').textContent();
      assert.ok(restored.includes(addressInput), 'mixed prompt did not restore');
      assert.ok(!(await addressPage.locator('main').textContent()).includes(address), 'address leaked into host DOM');
      await addressPage.close();
    }
    metrics.cases.push({ case: 'address-without-postcode-and-unicode-hyphens-mask-and-restore', passed: true });
  }
  // Editing the draft during asynchronous inference is preserved and submitted with a warning.
  await fill(p, 'Peter kom á fundinn.');
  await p.locator('textarea').fill('Ný útgáfa textans.');
  await status(p, 'breyttist');
  assert.equal(await p.evaluate(() => window.sent.length), 2);
  assert.equal(await p.evaluate(() => window.sent[1]), 'Ný útgáfa textans.');
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
  // A competing tab waits for the shared model and retains name masking.
  await fill(p, 'Peter kom á fundinn. '.repeat(100));
  await fill(second, 'Anna mætti aftur.');
  await second.waitForFunction(() => window.sent.length === 2, null, { timeout: 130000 });
  const queuedSent = await second.evaluate(() => window.sent[1]);
  assert.ok(!queuedSent.includes('Anna'));
  assert.match(queuedSent, /⟦NAME-/);
  await p.waitForFunction(() => window.sent.length === 3, null, { timeout: 130000 });
  metrics.cases.push({ case: 'competing-tab-queues-and-masks-name', passed: true });
  // Dispatch together to saturate admission while the real model is running.
  const batch = await sw.evaluate(() => Promise.all(Array.from({ length: 4 }, () =>
    chrome.runtime.sendMessage({ type: 'dmz:scanner-run', text: 'Anna ræddi við Peter. '.repeat(100) }))));
  assert.ok(batch.slice(0, 3).every(result => Array.isArray(result.spans) && result.spans.length > 0));
  assert.equal(batch[3].code, 'busy');
  metrics.cases.push({ case: 'one-active-two-waiting-overflow-is-busy', passed: true });
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
  assert.equal(await p.evaluate(() => window.sent.length), 4);
  await settings.close();
  metrics.cases.push({ case: 'reset-during-inference', passed: true });
  // Scanner termination still applies the basic detector before sending.
  await fill(p, 'Peter kom á fundinn. '.repeat(200) + 'fallback@example.com');
  await sw.evaluate(() => chrome.offscreen.closeDocument());
  await status(p, 'Sent með grunnmöskun');
  assert.equal(await p.evaluate(() => window.sent.length), 5);
  assert.ok(!(await p.evaluate(() => window.sent[4])).includes('fallback@example.com'));
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
      const q = await page(), queued = await page(), edited = await page(), overflow = await page();
      const start = Date.now();
      const draft = 'Anna mætti. fallback@example.com sk-test_key_000000000000';
      await fill(q, draft);
      await fill(queued, draft);
      await fill(edited, draft);
      await edited.locator('textarea').fill('Ný útgáfa textans.');
      await fill(overflow, draft);
      await status(overflow, 'Sent með grunnmöskun', 10000);
      assert.equal(await overflow.evaluate(() => window.sent.length), 1);
      assert.ok(!(await overflow.evaluate(() => window.sent[0])).includes('fallback@example.com'));
      await status(q, 'Sent með grunnmöskun', 135000);
      await status(queued, 'Sent með grunnmöskun', 10000);
      assert.equal(await queued.evaluate(() => window.sent.length), 1);
      assert.ok(!(await queued.evaluate(() => window.sent[0])).includes('fallback@example.com'));
      await status(edited, 'breyttist', 10000);
      assert.equal(await edited.evaluate(() => window.sent.length), 1);
      assert.equal(await edited.evaluate(() => window.sent[0]), 'Ný útgáfa textans.');
      assert.equal(await edited.locator('textarea').inputValue(), 'Ný útgáfa textans.');
      assert.equal(await q.evaluate(() => window.sent.length), 1);
      assert.ok(!(await q.evaluate(() => window.sent[0])).includes('fallback@example.com'));
      const elapsed = Date.now() - start;
      assert.ok(elapsed >= 119000 && elapsed < 135000);
      metrics.cases.push({
        case: 'queued-worker-deadline-overflow-fallback-and-edited-draft-stop',
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
