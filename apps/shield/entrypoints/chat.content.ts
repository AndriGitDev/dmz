import '../public/wasm_exec.js';

type Finding = { kind: string; confidence: 'high' | 'uncertain'; action: string; reason: string };
type Result = { allowed: boolean; transformed?: string; findings: Finding[]; error?: string };
type Core = {
  Go: new () => { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };
  dmzProcess?: (input: string) => string;
  dmzRestore?: (input: string) => string;
};

const site = location.hostname === 'chatgpt.com' ? 'chatgpt' : 'claude';
const composers = site === 'chatgpt'
  ? ['#prompt-textarea', 'textarea[data-testid="prompt-textarea"]']
  : ['[data-testid="chat-input"] [contenteditable="true"]', '[contenteditable="true"][data-placeholder]'];
const sendButtons = site === 'chatgpt'
  ? ['button[data-testid="send-button"]']
  : ['button[aria-label="Send message"]', 'button[data-testid="send-button"]'];
const assistantMessages = site === 'chatgpt'
  ? '[data-message-author-role="assistant"]'
  : '[data-testid="assistant-message"]';

export default defineContentScript({
  matches: ['https://chatgpt.com/*', 'https://claude.ai/*'],
  runAt: 'document_start',
  main() {
    let ready = false;
    let processing = false;
    let replaying = false;
    let message = 'Shield hleður staðbundnum kjarna…';
    let toast: HTMLElement | undefined;
    let composerAvailable: boolean | undefined;
    const restoredMessages = new WeakMap<Element, { host: HTMLElement; output: HTMLElement; source: string }>();
    let scanQueued = false;

    function scanResponses() {
      scanQueued = false;
      if (!ready) return;
      const available = composer() !== null;
      if (composerAvailable !== available) {
        composerAvailable = available;
        inform(available ? 'Shield virkur fyrir texta á þessari síðu.' : 'Shield finnur ekki studdan textareit. Ekki senda gögn.');
      }
      const core = globalThis as unknown as Core;
      for (const response of document.querySelectorAll(assistantMessages)) {
        const source = response.textContent ?? '';
        const existing = restoredMessages.get(response);
        if (!source.includes('⟦')) {
          existing?.host.remove();
          continue;
        }
        const restored = core.dmzRestore?.(source);
        if (!restored || restored === source) {
          existing?.host.remove();
          continue;
        }
        let entry = existing;
        if (!entry) {
          const host = document.createElement('div');
          host.setAttribute('data-dmz-restored', 'true');
          host.style.cssText = 'display:block;margin:12px 0';
          const root = host.attachShadow({ mode: 'closed' });
          const output = document.createElement('div');
          output.setAttribute('role', 'note');
          output.style.cssText = 'white-space:pre-wrap;padding:12px 16px;border:1px solid #ef4444;border-radius:8px;background:#18181b;color:#fff;font:14px/1.5 system-ui';
          root.append(output);
          response.after(host);
          entry = { host, output, source: '' };
          restoredMessages.set(response, entry);
        }
        if (!entry.host.isConnected) response.after(entry.host);
        if (entry.source !== source) {
          entry.output.textContent = `Shield · endurheimt aðeins á þessu tæki\n${restored}`;
          entry.source = source;
        }
      }
    }

    function queueResponseScan() {
      if (scanQueued) return;
      scanQueued = true;
      requestAnimationFrame(scanResponses);
    }

    function inform(text: string) {
      message = text;
      if (!document.documentElement) return;
      if (!toast) {
        const host = document.createElement('div');
        host.id = 'dmz-shield-status';
        host.style.cssText = 'position:fixed;z-index:2147483647;right:16px;bottom:16px';
        const root = host.attachShadow({ mode: 'closed' });
        toast = document.createElement('div');
        toast.setAttribute('role', 'status');
        toast.style.cssText = 'max-width:360px;padding:12px 16px;border:1px solid #ef4444;border-radius:8px;background:#18181b;color:#fff;font:14px/1.4 system-ui;box-shadow:0 4px 18px #0008';
        root.append(toast);
        document.documentElement.append(host);
      }
      toast.textContent = text;
    }

    function composer(): HTMLElement | null {
      for (const selector of composers) {
        const found = document.querySelector<HTMLElement>(selector);
        if (found && (found instanceof HTMLTextAreaElement || found.isContentEditable)) return found;
      }
      return null;
    }

    function read(element: HTMLElement): string {
      return element instanceof HTMLTextAreaElement ? element.value : element.innerText;
    }

    function write(element: HTMLElement, value: string): boolean {
      if (element instanceof HTMLTextAreaElement) {
        const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, 'value')?.set;
        setter?.call(element, value);
        element.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: value }));
      } else {
        element.focus();
        const range = document.createRange();
        range.selectNodeContents(element);
        const selection = window.getSelection();
        selection?.removeAllRanges();
        selection?.addRange(range);
        document.execCommand('insertText', false, value);
        element.dispatchEvent(new InputEvent('input', { bubbles: true, inputType: 'insertText', data: value }));
      }
      return read(element) === value;
    }

    function cancel(event: Event) {
      event.preventDefault();
      event.stopImmediatePropagation();
    }

    async function gate(event: Event, replay: () => void) {
      if (replaying) return;
      if (processing) { cancel(event); return; }
      const field = composer();
      if (!field || !read(field).trim()) {
        cancel(event);
        inform('Shield fann ekki studdan textareit. Sending stöðvuð.');
        return;
      }
      cancel(event);
      if (!ready) {
        inform(message);
        return;
      }
      processing = true;
      try {
        const input = read(field);
        const core = globalThis as unknown as Core;
        const result = JSON.parse(core.dmzProcess!(input)) as Result;
        if (result.error || !result.allowed || result.transformed === undefined) {
          inform('Shield stöðvaði sendingu: óviss eða óstudd gögn. Breyttu textanum eða notaðu handvirka hliðarspjaldið.');
          return;
        }
        if (!write(field, result.transformed)) {
          inform('Shield gat ekki uppfært textareitinn. Sending stöðvuð.');
          return;
        }
        await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
        if (read(field) !== result.transformed) {
          inform('Textareitur breyttist eftir möskun. Sending stöðvuð.');
          return;
        }
        replaying = true;
        replay();
        inform(result.findings.length ? 'Shield maskaði auðkenni áður en sent var.' : 'Shield yfirfór textann áður en sent var.');
      } catch {
        inform('Shield gat ekki yfirfarið textann. Sending stöðvuð.');
      } finally {
        replaying = false;
        processing = false;
      }
    }

    document.addEventListener('submit', (event) => {
      if (!(event.target instanceof HTMLFormElement)) return;
      const form = event.target;
      if (!form.querySelector('textarea,[contenteditable="true"]')) return;
      const field = composer();
      if (!field || !form.contains(field)) {
        cancel(event);
        inform('Óþekktur textareitur á studdri síðu. Sending stöðvuð.');
        return;
      }
      void gate(event, () => form.requestSubmit());
    }, true);

    document.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;
      const target = event.target;
      if (!(target instanceof Element) || !target.closest('textarea,[contenteditable="true"]')) return;
      const field = composer();
      if (!field || !field.contains(target)) {
        cancel(event);
        inform('Óþekktur textareitur á studdri síðu. Sending stöðvuð.');
        return;
      }
      void gate(event, () => field.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })));
    }, true);

    document.addEventListener('click', (event) => {
      const target = event.target;
      if (!(target instanceof Element)) return;
      const button = target.closest('button');
      if (!button) return;
      const known = sendButtons.some((selector) => button.matches(selector));
      const sendLike = button.matches('[aria-label*="send" i]') || (button.type === 'submit' && !!button.closest('form')?.querySelector('textarea,[contenteditable="true"]'));
      if (!known && !sendLike) return;
      void gate(event, () => button.click());
    }, true);

    const core = globalThis as unknown as Core;
    void (async () => {
      const response = await fetch(chrome.runtime.getURL('dmz.wasm'));
      if (!response.ok) throw new Error('WASM unavailable');
      const go = new core.Go();
      const { instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
      void go.run(instance);
      for (let i = 0; i < 100 && typeof core.dmzProcess !== 'function'; i++) {
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
      if (typeof core.dmzProcess !== 'function') throw new Error('WASM did not start');
      ready = true;
      message = 'Shield tilbúinn á þessari síðu.';
      new MutationObserver(queueResponseScan).observe(document.documentElement, { childList: true, characterData: true, subtree: true });
      queueResponseScan();
    })().catch(() => { message = 'Shield-kjarni bilaður. Sending stöðvuð.'; });
  },
});
