import assert from 'node:assert/strict';
import { mkdtemp, rm } from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const extension = process.env.DMZ_TEST_EXTENSION || path.join(root, '.output/chrome-mv3');
const profile = await mkdtemp(path.join(os.tmpdir(), 'dmz-composer-'));
const browser = await chromium.launchPersistentContext(profile, {
  headless: true, channel: 'chromium',
  args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`],
});
try {
  for (const origin of ['https://chatgpt.com', 'https://claude.ai']) {
    for (const mode of ['single-line', 'paragraphs', 'trailing-break', 'reject', 'rollback']) {
      for (const action of ['click', 'enter', 'submit']) {
        // Disabling contenteditable on focus also disables native Enter in the
        // fixture itself; exercise insertion rejection via its active button/form.
        if (mode === 'reject' && action === 'enter') continue;
        const page = await browser.newPage();
        const bodies = [];
        await page.route(`${origin}/**`, route => route.fulfill({ contentType: 'text/html; charset=utf-8', body: `
          <!doctype html><meta charset="utf-8">
          <style>.ProseMirror {white-space:pre-wrap} p {margin:0}</style>
          <form><div class="ProseMirror" contenteditable="true" role="textbox" data-composer-markdown=""><p></p></div>
          <button data-testid="send-button">Send</button></form>
          <script>
          const field = document.querySelector('[contenteditable]');
          const form = document.querySelector('form');
          window.mode = ${JSON.stringify(mode)};
          window.sent = [];
          window.model = '';
          const read = () => [...field.children].map(p => p.textContent).join('\\n');
          field.addEventListener('focus', () => { if (window.mode === 'reject') field.contentEditable = 'false'; });
          field.addEventListener('input', () => {
            window.model = read();
            if (!window.model.includes('⟦')) return;
            if (window.mode === 'rollback') {
              field.replaceChildren(Object.assign(document.createElement('p'), {textContent: window.original}));
              window.model = window.original;
            } else if (window.mode === 'trailing-break') {
              queueMicrotask(() => {
                const br = document.createElement('br');
                br.className = 'ProseMirror-trailingBreak';
                field.lastElementChild.append(br);
                window.model = read();
              });
            }
          });
          field.addEventListener('keydown', e => {
            if(e.key === 'Enter') {e.preventDefault();form.requestSubmit();}
          });
          form.addEventListener('submit', e => {
            e.preventDefault(); window.sent.push(window.model);
            void fetch('/__send', {method:'POST',body:window.model});
          });
          </script>` }));
        await page.route(`${origin}/__send`, route => {
          bodies.push(route.request().postData());
          return route.fulfill({ status: 204, body: '' });
        });
        await page.goto(`${origin}/new`);
        await page.locator('#dmz-status').waitFor();
        const input = mode === 'paragraphs' || mode === 'trailing-break'
          ? 'Please email demo@example.com\n\nThanks' : 'demo@example.com';
        await page.locator('[contenteditable]').evaluate((field, input) => {
          field.replaceChildren(...input.split('\n').map(line => {
            const p = document.createElement('p');
            if (line) p.textContent = line; else p.append(document.createElement('br'));
            return p;
          }));
          window.original = input;
          field.dispatchEvent(new InputEvent('input', { bubbles: true }));
        }, input);
        const response = page.waitForResponse(`${origin}/__send`, { timeout: 10000 });
        if (action === 'click') await page.locator('button').click();
        else if (action === 'enter') await page.locator('[contenteditable]').press('Enter');
        else await page.locator('form').evaluate(form => form.requestSubmit());
        await response.catch(error => { throw new Error(`${origin} ${mode} ${action}: ${error.message}`); });
        assert.equal(bodies.length, 1, `${origin} ${mode} ${action}: duplicate or missing submission`);
        if (mode === 'single-line' || mode === 'paragraphs' || mode === 'trailing-break') {
          assert.ok(!bodies[0].includes('demo@example.com'), `${mode}: PII leaked`);
          assert.equal(bodies[0].replace(/⟦EMAIL-[^⟧]+⟧/g, 'demo@example.com'), input, 'logical paragraph text changed');
        } else {
          assert.equal(bodies[0], input, 'failed update blocked or overwrote the current draft');
          const ax = await (await browser.newCDPSession(page)).send('Accessibility.getFullAXTree');
          assert.ok(ax.nodes.some(n => n.name?.value?.includes('Viðvörun:')), 'fallback had no visible warning');
        }
        await page.close();
      }
    }
  }
  console.log('28 Chrome composer cases passed: single-line/paragraphs, async reconciliation, failed updates, both sites and all send actions');
} finally {
  await browser.close();
  await rm(profile, { recursive: true, force: true });
}
