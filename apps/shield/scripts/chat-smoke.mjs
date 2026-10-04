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
    window.frameMessages = [];
    window.addEventListener('message', event => window.frameMessages.push(event.data));
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

// DevTools can operate extension controls in closed roots without adding a
// production test bridge that the host page could invoke.
async function clickShieldControl(page, attribute) {
  const cdp = await browser.newCDPSession(page);
  try {
    const { root: dom } = await cdp.send('DOM.getDocument', { depth: -1, pierce: true });
    function find(node) {
      if (node.attributes?.includes(attribute)) return node;
      for (const child of [...(node.children ?? []), ...(node.shadowRoots ?? [])]) {
        const found = find(child);
        if (found) return found;
      }
    }
    const control = find(dom);
    assert.ok(control, `Shield control missing: ${attribute}`);
    const { object } = await cdp.send('DOM.resolveNode', { backendNodeId: control.backendNodeId });
    await cdp.send('Runtime.callFunctionOn', { objectId: object.objectId, functionDeclaration: 'function() { this.click(); }' });
    await cdp.send('Runtime.releaseObject', { objectId: object.objectId });
  } finally { await cdp.detach(); }
}

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
  const restoredFrame = page.frameLocator('[data-dmz-restored="true"]');
  await restoredFrame.getByText('Arna Jónsdóttir, 140182-8858', { exact: true }).waitFor();
  assert.equal(await page.locator('[data-dmz-restored="true"]').evaluate(element => element.contentDocument), null, 'page can read restored frame document');
  const selectedValue = await page.evaluate(() => {
    window.getSelection()?.removeAllRanges();
    window.find('140182-8858', false, false, true, false, true, false);
    const selected = window.getSelection()?.toString() ?? '';
    window.getSelection()?.removeAllRanges();
    return selected;
  });
  assert.ok(!selectedValue.includes('140182-8858'), 'page selection exposed restored value');
  assert.ok(!(await page.evaluate(() => JSON.stringify(window.frameMessages))).includes('140182-8858'), 'frame sent restored text back to the host page');
  await page.locator('[data-message-author-role="assistant"]').first().evaluate(element => element.remove());
  await page.locator('[data-dmz-restored="true"]').waitFor({ state: 'detached' });

  const secretPage = await browser.newPage();
  await secretPage.route('https://chatgpt.com/**', route => route.fulfill({ contentType: 'text/html', body: html }));
  await secretPage.goto('https://chatgpt.com/');
  await secretPage.locator('#dmz-shield-status').waitFor();
  await secretPage.locator('#prompt-textarea').fill('sk-test_key_000000000000');
  await secretPage.locator('[data-testid="send-button"]').click();
  await secretPage.waitForFunction(() => window.sent.length === 1);
  assert.ok(!(await secretPage.evaluate(() => window.sent[0])).includes('sk-test'));
  assert.match(await secretPage.evaluate(() => window.sent[0]), /⟦VALUE-/);
  await secretPage.close();
  await page.locator('#prompt-textarea').fill('demo@example.com');
  await page.locator('#prompt-textarea').press('Enter');
  await page.waitForFunction(() => window.sent.length === 2, null, { timeout: 10000 });
  const emailMarker = await page.evaluate(() => window.sent[1]);
  assert.match(emailMarker, /⟦EMAIL-[A-F0-9]{16}-\d+: XXXXX@XXXXX\.XXX⟧/);
  await page.locator('#prompt-textarea').fill('demo@example.com');
  await page.locator('[data-testid="send-button"]').click();
  await page.waitForFunction(() => window.sent.length === 3, null, { timeout: 10000 });
  assert.equal(await page.evaluate(() => window.sent[2]), emailMarker, 'same-tab marker changed across turns');
  // A retry or pasted current-session marker must not mask its placeholder again.
  await page.locator('#prompt-textarea').fill(emailMarker);
  await page.locator('[data-testid="send-button"]').click();
  await page.waitForFunction(() => window.sent.length === 4);
  assert.equal(await page.evaluate(() => window.sent[3]), emailMarker, 'resubmission nested the marker');
  await page.locator('[data-dmz-restored="true"]').last().waitFor();
  await page.frameLocator('[data-dmz-restored="true"]').last().getByText('demo@example.com', { exact: true }).waitFor();
  await page.locator('#prompt-textarea').evaluate(element => element.removeAttribute('id'));
  await page.locator('textarea').fill('demo@example.com');
  await page.locator('[data-testid="send-button"]').click();
  assert.equal(await page.evaluate(() => window.sent.length), 4, 'unknown composer was submitted');
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
  await claude.frameLocator('[data-dmz-restored="true"]').getByText('demo@example.com', { exact: true }).waitFor();
  // The live reply contains an autolink inside a paragraph. Its wrapper can
  // also contain a hidden duplicate and action/timestamp text.
  await claude.locator('[data-testid="assistant-message"]').evaluate(reply => {
    const marker = reply.textContent;
    reply.replaceChildren();
    const announcement = document.createElement('div');
    announcement.className = 'sr-only';
    announcement.textContent = 'Claude responded: ' + marker;
    const paragraph = document.createElement('p');
    paragraph.dir = 'ltr';
    const [prefix, suffix] = marker.split('XXXXX@XXXXX.XXX');
    const link = document.createElement('a');
    link.href = 'mailto:XXXXX@XXXXX.XXX';
    link.textContent = 'XXXXX@XXXXX.XXX';
    paragraph.append(prefix, link, suffix);
    const actions = document.createElement('div');
    actions.dataset.cds = 'MessageActions';
    actions.textContent = '\ue056\ue0e4\ue0fb\ue0f9\ue11d3 minutes ago';
    reply.append(announcement, paragraph, actions);
  });
  await claude.waitForTimeout(150);
  assert.equal(await claude.frameLocator('[data-dmz-restored="true"]').locator('#restored-text').textContent(), 'demo@example.com', 'reply included wrapper metadata or duplicated its link');
  // Legitimate repeated paragraphs and streamed updates must survive extraction.
  await claude.locator('[data-testid="assistant-message"] p').evaluate(p => p.after(p.cloneNode(true)));
  await claude.waitForTimeout(150);
  assert.equal(await claude.frameLocator('[data-dmz-restored="true"]').locator('#restored-text').innerText(), 'demo@example.com\n\ndemo@example.com');
  const oldMarker = await claude.evaluate(() => window.sent[0]);
  await clickShieldControl(claude, 'data-dmz-reset');
  await claude.locator('[data-dmz-restored="true"]').waitFor({ state: 'detached' });
  assert.equal(await claude.locator('[data-dmz-original]').count(), 0, 'reset left the site reply hidden');
  assert.ok(await claude.locator('[data-testid="assistant-message"] p').first().isVisible(), 'reset failed to reveal masked answer');
  await claude.locator('[contenteditable]').fill('demo@example.com');
  await claude.locator('button[aria-label="Send message"]').click();
  await claude.waitForFunction(() => window.sent.length === 2);
  assert.notEqual(await claude.evaluate(() => window.sent[1]), oldMarker, 'reset reused the old session namespace');
  await claude.locator('[data-dmz-restored="true"]').waitFor();
  assert.equal(await claude.locator('[data-dmz-restored="true"]').count(), 1, 'old reply restored after reset');
  // Reported new-chat layouts: ChatGPT has no ID; Claude's placeholder is a child.
  for (const [origin, editor] of [
    ['https://chatgpt.com', '<div contenteditable="true" role="textbox" class="ProseMirror" data-composer-markdown="" aria-label="Ask ChatGPT"><p data-empty-paragraph="true" data-placeholder="Ask ChatGPT" class="placeholder"><br class="ProseMirror-trailingBreak"></p></div>'],
    ['https://claude.ai', '<div class="ProseMirror" contenteditable="true"><p data-placeholder="How can I help you today?" class="is-empty is-editor-empty"><br class="ProseMirror-trailingBreak"></p></div>'],
  ]) {
    const modern = await browser.newPage();
    const fixture = `<!doctype html><form>${editor}<button data-testid="send-button">Send</button></form>
      <div id="replies"></div>
      <script>
        window.sent = [];
        const field = document.querySelector('[contenteditable]');
        let model = '';
        field.addEventListener('input', () => { model = field.innerText; });
        field.addEventListener('keydown', event => {
          if (event.key === 'Enter') { event.preventDefault(); document.querySelector('form').requestSubmit(); }
        });
        document.querySelector('form').addEventListener('submit', event => {
          event.preventDefault(); window.sent.push(model);
          const response = document.createElement('div');
          response.setAttribute('${origin.includes('chatgpt') ? 'data-markdown-text-style' : 'data-testid'}', 'assistant-message');
          const p = document.createElement('p');
          p.style.cssText = 'font-size:18px;color:rgb(40, 50, 60);margin:0';
          const [prefix, suffix = ''] = model.split('XXXXX@XXXXX.XXX');
          const link = document.createElement('a');
          link.href = 'mailto:XXXXX@XXXXX.XXX';
          const span = document.createElement('span');
          span.textContent = 'XXXXX@XXXXX.XXX';
          const icon = document.createElement('span');
          icon.setAttribute('aria-hidden', 'true');
          icon.textContent = 'icon';
          link.append(span, icon);
          p.append(prefix, link, suffix);
          response.append(p);
          document.querySelector('#replies').append(response);
        });
      </script>`;
    await modern.route(`${origin}/**`, route => route.fulfill({ contentType: 'text/html', body: fixture }));
    await modern.goto(`${origin}/new`);
    const field = modern.locator('[contenteditable]');
    for (const action of ['click', 'enter']) {
      await field.fill('demo@example.com');
      if (action === 'click') await modern.locator('button').click();
      else await field.press('Enter');
      await modern.waitForFunction(count => window.sent.length === count, action === 'click' ? 1 : 2);
    }
    const sent = await modern.evaluate(() => window.sent);
    assert.ok(sent.every(text => text.includes('⟦EMAIL-') && !text.includes('demo@example.com')), `${origin}: unmasked new-chat submission`);
    await modern.frameLocator('[data-dmz-restored]').first().getByText('demo@example.com', { exact: true }).waitFor();
    await modern.waitForFunction(() => document.querySelectorAll('[data-dmz-original="hidden"]').length === 2);
    assert.equal(await modern.locator('#replies p').first().isVisible(), false, 'masked reply was not replaced visually');
    const inline = modern.frameLocator('[data-dmz-restored]').first();
    assert.equal(await inline.locator('strong').count(), 0, 'extra Shield heading still shown');
    assert.equal(await inline.locator('p').evaluate(p => getComputedStyle(p).fontSize), '18px', 'reply typography lost');
    for (const [mode, background, foreground] of [
      ['dark', 'rgb(21, 21, 21)', 'rgb(240, 239, 236)'],
      ['light', 'rgb(250, 250, 250)', 'rgb(32, 32, 32)'],
    ]) {
      await modern.evaluate(({ mode, background, foreground }) => {
        document.documentElement.dataset.mode = mode;
        document.documentElement.style.colorScheme = mode;
        document.body.style.backgroundColor = background;
        for (const p of document.querySelectorAll('#replies p')) p.style.color = foreground;
      }, { mode, background, foreground });
      await inline.locator(`html[style*="${background}"]`).waitFor({ state: 'attached' });
      assert.equal(await inline.locator('html').evaluate(element => getComputedStyle(element).colorScheme), mode);
      assert.equal(await inline.locator('p').evaluate(element => getComputedStyle(element).color), foreground);
      assert.equal(await modern.locator('[data-dmz-restored]').first().evaluate(element => getComputedStyle(element).colorScheme), mode);
      if (process.env.SHIELD_SCREENSHOTS) {
        await modern.locator('[data-dmz-restored]').first().screenshot({ path: path.join(process.env.SHIELD_SCREENSHOTS, `shield-${new URL(origin).hostname}-${mode}.png`) });
      }
    }
    // A background-only theme update must refresh an unchanged restored reply.
    await modern.evaluate(() => { document.body.style.backgroundColor = 'rgb(230, 230, 230)'; });
    await inline.locator('html[style*="rgb(230, 230, 230)"]').waitFor({ state: 'attached' });
    await modern.evaluate(() => {
      const style = document.createElement('style');
      style.textContent = '@media (prefers-color-scheme: dark) { body { background-color: rgb(30, 30, 30) !important; } #replies p { color: rgb(240, 240, 240) !important; } }';
      document.head.append(style);
    });
    await modern.emulateMedia({ colorScheme: 'dark' });
    await inline.locator('html[style*="rgb(30, 30, 30)"]').waitFor({ state: 'attached' });
    await modern.emulateMedia({ colorScheme: 'light' });
    await inline.locator('html[style*="rgb(230, 230, 230)"]').waitFor({ state: 'attached' });
    assert.ok(!(await modern.locator('#replies').textContent()).includes('demo@example.com'), 'restored value written into host DOM');
    assert.equal(await modern.locator('[data-dmz-restored]').first().evaluate(frame => frame.contentDocument), null);
    await modern.locator('#replies > div').first().evaluate(response => {
      const marker = window.sent[0];
      const heading = document.createElement('h2');
      heading.textContent = 'Synthetic reply';
      const p = document.createElement('p');
      const strong = document.createElement('strong');
      strong.textContent = marker;
      p.append(strong);
      const list = document.createElement('ul');
      const item = document.createElement('li');
      item.textContent = 'Fabricated example';
      list.append(item);
      const pre = document.createElement('pre');
      const code = document.createElement('code');
      code.textContent = 'example = ' + marker;
      pre.append(code);
      response.replaceChildren(heading, p, list, pre);
    });
    await inline.locator('strong').getByText('demo@example.com', { exact: true }).waitFor();
    assert.equal(await inline.locator('h2').textContent(), 'Synthetic reply');
    assert.equal(await inline.locator('li').textContent(), 'Fabricated example');
    assert.equal(await inline.locator('pre code').textContent(), 'example = demo@example.com');
    assert.equal(await inline.locator('#restored-text').locator('a,script,img,iframe').count(), 0, 'host resources copied into restored frame');
    await field.fill('sk-test_key_000000000000');
    await modern.locator('button').click();
    await modern.waitForFunction(() => window.sent.length === 3);
    assert.ok(!(await modern.evaluate(() => window.sent[2])).includes('sk-test'));
    await field.evaluate(element => element.after(element.cloneNode(true)));
    await modern.locator('button').click();
    assert.equal(await modern.evaluate(() => window.sent.length), 3, 'ambiguous editor submitted');
    await modern.locator('[contenteditable]').last().evaluate(element => { element.hidden = true; });
    await modern.locator('[contenteditable]').first().fill('demo@example.com');
    await modern.locator('button').click();
    await modern.waitForFunction(() => window.sent.length === 4);
    await modern.close();
  }
  // Canonically equal identifiers with different original spellings must not
  // corrupt each other's restoration within one tab or across repeated turns.
  for (const [origin, fixture, editor, send] of [
    ['https://chatgpt.com', html, '#prompt-textarea', '[data-testid="send-button"]'],
    ['https://claude.ai', claudeHtml, '[contenteditable]', 'button[aria-label="Send message"]'],
  ]) {
    const variants = await browser.newPage();
    variants.on('pageerror', error => errors.push(error.message));
    await variants.route(`${origin}/**`, route => route.fulfill({ contentType: 'text/html', body: fixture }));
    await variants.goto(`${origin}/new`);
    const forms = ['140182-8858 próf@example.invalid', '140182\u00a08858 próf@EXAMPLE.INVALID'.normalize('NFD')];
    for (const [index, input] of [forms[0], forms[1], forms[0]].entries()) {
      await variants.locator(editor).fill(input);
      await variants.locator(send).click();
      await variants.waitForFunction(count => window.sent.length === count, index + 1);
      await variants.frameLocator('[data-dmz-restored="true"]').nth(index)
        .getByText(input, { exact: true }).waitFor();
      assert.equal(await variants.frameLocator('[data-dmz-restored="true"]').nth(index)
        .locator('#restored-text').textContent(), input, 'original formatting changed during restoration');
    }
    const sent = await variants.evaluate(() => window.sent);
    assert.notEqual(sent[0], sent[1], 'different original forms shared their markers');
    assert.equal(sent[0], sent[2], 'same exact form lost its stable markers');
    assert.ok(sent.every(text => !text.includes('140182') && !text.includes('próf') && !text.includes('pro\u0301f')));
    assert.ok(!(await variants.locator('#responses').textContent()).includes('140182'), 'restored identifier reached host DOM');
    assert.equal(await variants.locator('[data-dmz-restored="true"]').last().evaluate(frame => frame.contentDocument), null);
    await variants.close();
  }
  assert.deepEqual(errors, []);
  console.log('ChatGPT and Claude synthetic interception, block and restore smoke tests passed');
} finally {
  if (browser) await browser.close();
  await rm(profile, { recursive: true, force: true });
}
