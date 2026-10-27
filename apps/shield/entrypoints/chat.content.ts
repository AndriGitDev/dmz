import '../public/wasm_exec.js';
import { trainedScanner, scanNames } from '../utils/scanner';
import { restoredTags, restoredStyles, type RestoredNode } from '../utils/restored-model';

type Finding = { kind: string; confidence: 'high' | 'uncertain'; action: string; reason: string };
type Result = { allowed: boolean; transformed?: string; findings: Finding[]; error?: string };
type Core = {
  Go: new () => { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };
  dmzProcessBestEffort?: (input: string, spans: string) => string;
  dmzProcess?: (input: string) => string;
  dmzProcessWithNames?: (input: string, spans: string) => string;
  dmzRestore?: (input: string) => string;
  dmzReset?: () => boolean;
};

const site = location.hostname === 'chatgpt.com' ? 'chatgpt' : 'claude';
const composers = site === 'chatgpt'
  ? ['#prompt-textarea', 'textarea[data-testid="prompt-textarea"]', '[contenteditable="true"][data-composer-markdown][role="textbox"]']
  : ['[data-testid="chat-input"] [contenteditable="true"]', '[contenteditable="true"][data-placeholder]', '.ProseMirror[contenteditable="true"]'];
const sendButtons = site === 'chatgpt'
  ? ['button[data-testid="send-button"]']
  : ['button[aria-label="Send message"]', 'button[data-testid="send-button"]'];
const assistantMessages = site === 'chatgpt'
  ? '[data-message-author-role="assistant"],[data-markdown-text-style="assistant-message"]'
  : '[data-testid="assistant-message"]';

export default defineContentScript({
  matches: ['https://chatgpt.com/*', 'https://claude.ai/*'],
  runAt: 'document_start',
  main() {
    let ready = false;
    let processing = false;
    let replaying = false;
    let message = 'DMZ hleður staðbundnum kjarna…';
    let toast: HTMLElement | undefined;
    let resetButton: HTMLButtonElement | undefined;
    let reviewButton: HTMLButtonElement | undefined;
    let sessionEpoch = 0;
    let composerAvailable: boolean | undefined;
    const extensionOrigin = chrome.runtime.getURL('/').replace(/\/$/, '');
    type Surface = { background: string; colorScheme: 'dark' | 'light' };
    type RestoredEntry = { host: HTMLIFrameElement; source: string; restored: string; nodes: RestoredNode[]; surface: Surface; signature: string; nonce: string; loaded: boolean; originals: Element[]; presented: boolean };
    const restoredMessages = new Map<Element, RestoredEntry>();
    let scanQueued = false;

    const surfaceCanvas = document.createElement('canvas');
    surfaceCanvas.width = surfaceCanvas.height = 1;
    const surfaceContext = surfaceCanvas.getContext('2d', { willReadFrequently: true })!;
    function replySurface(original: Element): Surface {
      const scheme = getComputedStyle(original).colorScheme;
      const dark = scheme.includes('dark') && (!scheme.includes('light') || matchMedia('(prefers-color-scheme: dark)').matches);
      // Composite ancestor background colours, including translucent layers.
      // An explicit opaque canvas avoids cross-origin iframe colour-scheme
      // mismatches turning a transparent dark reply into a white rectangle.
      surfaceContext.fillStyle = dark ? '#121212' : '#ffffff';
      surfaceContext.fillRect(0, 0, 1, 1);
      const ancestors: Element[] = [];
      for (let node: Element | null = original; node; node = node.parentElement) ancestors.push(node);
      for (const node of ancestors.reverse()) {
        surfaceContext.fillStyle = getComputedStyle(node).backgroundColor;
        surfaceContext.fillRect(0, 0, 1, 1);
      }
      const [r = 255, g = 255, b = 255] = surfaceContext.getImageData(0, 0, 1, 1).data;
      return { background: `rgb(${r}, ${g}, ${b})`, colorScheme: 0.2126 * r + 0.7152 * g + 0.0722 * b < 128 ? 'dark' : 'light' };
    }

    function displayRestored(entry: RestoredEntry) {
      if (!entry.loaded || !entry.host.isConnected) return;
      const original = entry.originals[0];
      const style = original ? getComputedStyle(original) : undefined;
      const typography = style ? Object.fromEntries(['color', 'font-family', 'font-size', 'font-weight', 'font-style', 'line-height', 'letter-spacing', 'text-align'].map(key => [key, style.getPropertyValue(key)])) : {};
      entry.host.contentWindow?.postMessage({ type: 'dmz:restored', nonce: entry.nonce, text: entry.restored, nodes: entry.nodes, typography, surface: entry.surface }, extensionOrigin);
    }

    const originalStyle = document.createElement('style');
    originalStyle.textContent = '[data-dmz-original="hidden"] { display:none !important; }';

    function removeRestored(entry: RestoredEntry) {
      entry.restored = '';
      entry.nodes = [];
      for (const original of entry.originals) original.removeAttribute('data-dmz-original');
      entry.host.remove();
    }

    function responseContent(response: Element): { source: string; originals: Element[]; nodes: RestoredNode[] } {
      const excluded = 'button,[role="button"],nav,time,svg,script,style,textarea,input,iframe,[hidden],[aria-hidden="true"],.sr-only,[data-cds="MessageActions"]';
      function visible(element: Element): boolean {
        for (let current: Element | null = element; current; current = current.parentElement) {
          if (current.matches(excluded)) return false;
          const style = getComputedStyle(current);
          if ((style.display === 'none' && current.getAttribute('data-dmz-original') !== 'hidden') || style.visibility === 'hidden') return false;
          if (current === response) break;
        }
        return true;
      }
      function text(node: Node): string {
        if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? '';
        if (!(node instanceof Element) || !visible(node)) return '';
        if (node.tagName === 'BR') return '\n';
        const value = [...node.childNodes].map(text).join('');
        return /^(P|DIV|PRE|LI|TR|BLOCKQUOTE|H[1-6])$/.test(node.tagName) ? `${value}\n` : value;
      }
      // Read semantic answer blocks, not the whole message wrapper. Wrappers
      // may include accessibility duplicates, action glyphs and timestamps.
      const blocks = [...response.querySelectorAll('p,pre,ul,ol,table,blockquote,h1,h2,h3,h4,h5,h6')]
        .filter(visible);
      const roots = blocks.filter(block => !blocks.some(other => other !== block && other.contains(block)));
      function structure(node: Node): RestoredNode[] {
        if (node.nodeType === Node.TEXT_NODE) return [{ text: node.textContent ?? '' }];
        if (!(node instanceof Element) || !visible(node)) return [];
        const tag = node.tagName.toLowerCase();
        const computed = getComputedStyle(node);
        return [{ tag: restoredTags.has(tag) ? tag : 'span',
          style: Object.fromEntries(restoredStyles.map(key => [key, computed.getPropertyValue(key)])),
          children: [...node.childNodes].flatMap(structure) }];
      }
      const originals = roots.length ? roots : [response];
      return { source: (roots.length ? roots.map(text).join('\n') : text(response)).trim(), originals, nodes: originals.flatMap(structure) };
    }

    function restoreNodes(nodes: RestoredNode[]): void {
      const leaves: { node: { text: string }; start: number; end: number }[] = [];
      let flat = '';
      function collect(node: RestoredNode) {
        if ('text' in node) {
          leaves.push({ node, start: flat.length, end: flat.length + node.text.length });
          flat += node.text;
        } else {
          if (node.tag === 'br') flat += '\n';
          node.children.forEach(collect);
          if (['p', 'div', 'pre', 'li', 'tr', 'blockquote'].includes(node.tag)) flat += '\n';
        }
      }
      nodes.forEach(collect);
      // Restore complete markers even when the site splits one across spans
      // and an autolink. Apply edits backwards to preserve leaf offsets.
      for (const match of [...flat.matchAll(/⟦[^⟦⟧]*⟧/g)].reverse()) {
        const replacement = (globalThis as unknown as Core).dmzRestore?.(match[0]);
        if (!replacement || replacement === match[0]) continue;
        const start = match.index!;
        const end = start + match[0].length;
        let first = true;
        for (const leaf of leaves) {
          if (leaf.end <= start || leaf.start >= end) continue;
          const from = Math.max(0, start - leaf.start);
          const to = Math.min(leaf.end, end) - leaf.start;
          leaf.node.text = leaf.node.text.slice(0, from) + (first ? replacement : '') + leaf.node.text.slice(to);
          first = false;
        }
      }
    }

    // Only dimensions cross back into the host page; restored content never does.
    window.addEventListener('message', (event) => {
      if (event.origin !== extensionOrigin || event.data?.type !== 'dmz:restored-height') return;
      for (const entry of restoredMessages.values()) {
        if (event.source === entry.host.contentWindow && event.data.nonce === entry.nonce
          && Number.isFinite(event.data.height)) {
          entry.host.style.height = `${Math.max(1, Math.min(100000, event.data.height))}px`;
          entry.presented = true;
          for (const original of entry.originals) original.setAttribute('data-dmz-original', 'hidden');
          break;
        }
      }
    });

    function scanResponses() {
      scanQueued = false;
      if (!ready) return;
      if (!originalStyle.isConnected) (document.head ?? document.documentElement).append(originalStyle);
      const responses = [...document.querySelectorAll(assistantMessages)].filter(response => !response.querySelector(assistantMessages));
      for (const [response, entry] of restoredMessages) {
        if (!responses.includes(response)) {
          removeRestored(entry);
          restoredMessages.delete(response);
        }
      }
      const available = composer() !== null;
      if (composerAvailable !== available) {
        composerAvailable = available;
        inform(available ? 'DMZ virkur fyrir texta á þessari síðu.' : 'Viðvörun: DMZ finnur ekki studdan textareit. Sending heldur áfram án DMZ-möskunar.');
      }
      const core = globalThis as unknown as Core;
      for (const response of responses) {
        const { source, originals, nodes } = responseContent(response);
        const existing = restoredMessages.get(response);
        if (!source.includes('⟦')) {
          if (existing) removeRestored(existing);
          restoredMessages.delete(response);
          continue;
        }
        const restored = core.dmzRestore?.(source);
        if (!restored || restored === source) {
          if (existing) removeRestored(existing);
          restoredMessages.delete(response);
          continue;
        }
        let entry = existing;
        const surface = replySurface(originals[0] ?? response);
        if (!entry) {
          const host = document.createElement('iframe');
          host.setAttribute('data-dmz-restored', 'true');
          host.title = 'DMZ · endurheimt aðeins á þessu tæki';
          host.referrerPolicy = 'no-referrer';
          host.style.cssText = 'display:block;width:100%;height:1px;margin:0;border:0;background:transparent';
          entry = { host, source: '', restored: '', nodes: [], surface, signature: '', nonce: crypto.randomUUID(), loaded: false, originals, presented: false };
          restoredMessages.set(response, entry);
          const frameEntry = entry;
          host.addEventListener('load', () => {
            frameEntry.loaded = true;
            if (restoredMessages.get(response) === frameEntry) displayRestored(frameEntry);
          });
          host.src = `${chrome.runtime.getURL('restored.html')}#${entry.nonce}`;
          (originals[0] ?? response).before(host);
        }
        for (const original of entry.originals) if (!originals.includes(original)) original.removeAttribute('data-dmz-original');
        entry.originals = originals;
        if (entry.presented) for (const original of originals) original.setAttribute('data-dmz-original', 'hidden');
        if (!entry.host.isConnected) (originals[0] ?? response).before(entry.host);
        entry.surface = surface;
        if (entry.host.style.colorScheme !== surface.colorScheme) entry.host.style.colorScheme = surface.colorScheme;
        const signature = JSON.stringify({ nodes, surface });
        if (entry.source !== source || entry.signature !== signature) {
          entry.restored = restored;
          entry.source = source;
          entry.signature = signature;
          restoreNodes(nodes);
          entry.nodes = nodes;
          displayRestored(entry);
        }
      }
    }

    function queueResponseScan() {
      if (scanQueued) return;
      scanQueued = true;
      requestAnimationFrame(scanResponses);
    }

    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', queueResponseScan);

    function resetSession(): boolean {
      if (!ready) return false;
      sessionEpoch++;
      for (const entry of restoredMessages.values()) {
        removeRestored(entry);
      }
      restoredMessages.clear();
      try {
        if (!(globalThis as unknown as Core).dmzReset?.()) throw new Error('Reset failed');
        inform('DMZ-lota hreinsuð. Eldri merkimiðar verða ekki endurheimtir.');
        return true;
      } catch { coreFailed(); return false; }
    }

    chrome.runtime.onMessage.addListener((request, sender, respond) => {
      // Only the extension settings page may inspect status or reset this tab.
      if (sender.id !== chrome.runtime.id || sender.url !== chrome.runtime.getURL('sidepanel.html')) return;
      if (request?.type === 'dmz:settings-status') {
        respond({ ready, composer: Boolean(composer()), site });
      } else if (request?.type === 'dmz:settings-reset') {
        respond({ ok: resetSession() });
      }
    });

    function inform(text: string, review = false) {
      message = text;
      if (!document.documentElement) return;
      if (!toast) {
        const host = document.createElement('div');
        host.id = 'dmz-shield-status';
        host.style.cssText = 'position:fixed;z-index:2147483647;right:16px;bottom:16px';
        const root = host.attachShadow({ mode: 'closed' });
        const panel = document.createElement('div');
        panel.style.cssText = 'max-width:360px;padding:12px 16px;border:1px solid #ef4444;border-radius:8px;background:#18181b;color:#fff;font:14px/1.4 system-ui;box-shadow:0 4px 18px #0008';
        toast = document.createElement('div');
        toast.setAttribute('role', 'status');
        resetButton = document.createElement('button');
        resetButton.type = 'button';
        resetButton.setAttribute('data-dmz-reset', 'true');
        resetButton.textContent = 'Hreinsa DMZ-lotu';
        resetButton.title = 'Eyðir tengitöflu og endurheimtum svörum í þessum flipa.';
        resetButton.style.cssText = 'margin-top:8px;padding:6px 10px;background:#27272a;color:#fff;border:1px solid #71717a;border-radius:4px;cursor:pointer';
        resetButton.disabled = !ready;
        resetButton.addEventListener('click', resetSession);
        reviewButton = document.createElement('button');
        reviewButton.type = 'button';
        reviewButton.setAttribute('data-dmz-review', 'true');
        reviewButton.textContent = 'Breyta texta';
        reviewButton.style.cssText = resetButton.style.cssText;
        reviewButton.addEventListener('click', () => composer()?.focus());
        panel.append(toast, reviewButton, resetButton);
        root.append(panel);
        document.documentElement.append(host);
      }
      toast.textContent = text;
      if (resetButton) resetButton.disabled = !ready;
      if (reviewButton) reviewButton.hidden = !review;
    }

    function composer(): HTMLElement | null {
      const candidates = new Set<HTMLElement>();
      for (const selector of composers) {
        for (const found of document.querySelectorAll<HTMLElement>(selector)) {
          if (found.getClientRects().length && getComputedStyle(found).visibility === 'visible'
            && ((found instanceof HTMLTextAreaElement && !found.disabled && !found.readOnly) || found.isContentEditable)) candidates.add(found);
        }
      }
      // Never guess which draft will be submitted when several editors match.
      return candidates.size === 1 ? [...candidates][0] ?? null : null;
    }

    function read(element: HTMLElement): string {
      if (element instanceof HTMLTextAreaElement) return element.value;
      // innerText adds layout-dependent blank lines between editor paragraphs.
      // Read logical editor lines instead, including empty paragraphs, without
      // treating ProseMirror's caret-only trailing break as prompt text.
      const block = /^(P|DIV|LI|H[1-6]|PRE|BLOCKQUOTE)$/;
      function text(node: Node): string {
        if (node.nodeType === Node.TEXT_NODE) return node.textContent ?? '';
        if (!(node instanceof HTMLElement)) return '';
        if (node.tagName === 'BR') return node.classList.contains('ProseMirror-trailingBreak') ? '' : '\n';
        const children = [...node.childNodes];
        if ((node === element || block.test(node.tagName)) && children.length === 1
          && children[0] instanceof HTMLBRElement) return '';
        let result = '';
        let previousBlock = false;
        children.forEach((child, index) => {
          const currentBlock = child instanceof HTMLElement && block.test(child.tagName);
          if (index && (previousBlock || currentBlock)) result += '\n';
          result += text(child);
          previousBlock = currentBlock;
        });
        return result;
      }
      return text(element);
    }

    function write(element: HTMLElement, value: string): boolean {
      if (read(element) === value) return true;
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
        // Native insertion already emits input; a second synthetic event can
        // make a framework process the replacement twice.
        if (!document.execCommand('insertText', false, value)) return false;
      }
      // Rich editors reconcile their document after the native input event.
      // The gate verifies the exact logical text after that reconciliation.
      return true;
    }

    function cancel(event: Event) {
      event.preventDefault();
      event.stopImmediatePropagation();
    }

    async function gate(event: Event, replay: () => void, action: HTMLElement) {
      if (replaying) return;
      if (processing) { cancel(event); return; }
      const field = composer();
      if (!field || !read(field).trim()) {
        inform('Viðvörun: DMZ fann ekki studdan textareit. Sent án DMZ-möskunar.');
        return;
      }
      if (!ready) {
        inform('Viðvörun: DMZ-kjarninn er ekki tiltækur. Sent án DMZ-möskunar.');
        return;
      }
      cancel(event);
      processing = true;
      try {
        const url = location.href;
        const epoch = sessionEpoch;
        const input = read(field);
        const core = globalThis as unknown as Core;
        let result = JSON.parse(core.dmzProcessBestEffort!(input, '[]')) as Result;
        let basicOnly = false;
        if (result.error) throw new Error('Core processing failed');
        if (!result.allowed) { inform('Viðvörun: DMZ gat ekki maskað þennan texta. Sending heldur áfram.'); return; }
        if (trainedScanner) {
          inform('DMZ yfirfer nöfn á tækinu… Fyrsta keyrsla getur tekið lengri tíma.');
          try {
            const names = await scanNames(input);
            if (sessionEpoch !== epoch || location.href !== url || !ready || !field.isConnected
              || !action.isConnected || composer() !== field || read(field) !== input) throw new Error('Draft changed');
            const enhanced = JSON.parse(core.dmzProcessBestEffort!(input, JSON.stringify(names))) as Result;
            if (enhanced.error || !enhanced.allowed || enhanced.transformed === undefined) throw new Error('Invalid trained result');
            result = enhanced;
          } catch {
            basicOnly = true;
          }
          // Async inference must not overwrite edits, a new route or a reset session.
          if (sessionEpoch !== epoch || location.href !== url || !ready || !field.isConnected
            || !action.isConnected || composer() !== field || read(field) !== input) {
            inform('Viðvörun: Textareitur breyttist við yfirferð. Sent með núverandi texta; möskun getur verið ófullkomin.'); return;
          }
        }
        if (result.error) throw new Error('Core processing failed');
        if (!result.allowed) {
          inform('Viðvörun: DMZ gat ekki maskað þennan texta. Sending heldur áfram.');
          return;
        }
        if (result.transformed === undefined) {
          inform('Viðvörun: DMZ gat ekki yfirfarið textann. Sending heldur áfram.');
          return;
        }
        if (!write(field, result.transformed)) {
          inform('Viðvörun: DMZ gat ekki uppfært textareitinn. Sending heldur áfram; möskun getur verið ófullkomin.');
          return;
        }
        await new Promise<void>((resolve) => requestAnimationFrame(() => requestAnimationFrame(() => resolve())));
        if (sessionEpoch !== epoch || !field.isConnected || !action.isConnected || composer() !== field
          || location.href !== url) {
          inform('Viðvörun: Textareitur breyttist eftir möskun. Sending heldur áfram með núverandi texta.');
          return;
        }
        if (read(field) !== result.transformed) {
          inform('Viðvörun: DMZ gat ekki uppfært textareitinn. Sending heldur áfram; möskun getur verið ófullkomin.');
          return;
        }
        inform(basicOnly ? 'Viðvörun: Þjálfaður nafnagreinir var ekki tiltækur. Sent með grunnmöskun; nöfn geta hafa farið ómöskuð.' : result.findings.some((finding) => finding.reason === 'whole-text-coverage') ? 'DMZ maskaði allan textann þar sem ekki var hægt að afmarka auðkenni örugglega.' : result.findings.length ? 'DMZ maskaði auðkenni áður en sent var.' : 'DMZ yfirfór textann áður en sent var.');
      } catch {
        inform('Viðvörun: DMZ gat ekki yfirfarið textann. Sending heldur áfram; möskun getur verið ófullkomin.');
      } finally {
        // Owner policy: a failed best-effort pass must still replay the user's
        // send action. Never overwrite a changed draft with an older scan.
        // Even synchronous failures must leave the original event stack first:
        // browsers suppress reentrant button.click() and form.requestSubmit().
        await new Promise<void>((resolve) => setTimeout(resolve, 0));
        replaying = true;
        try { replay(); } finally { replaying = false; processing = false; }
      }
    }

    document.addEventListener('submit', (event) => {
      if (!(event.target instanceof HTMLFormElement)) return;
      const form = event.target;
      if (!form.querySelector('textarea,[contenteditable="true"]')) return;
      const field = composer();
      if (!field || !form.contains(field)) {
        inform('Viðvörun: Óþekktur textareitur. Sent án DMZ-möskunar.');
        return;
      }
      const submitter = event.submitter;
      void gate(event, () => form.requestSubmit(submitter ?? undefined), form);
    }, true);

    document.addEventListener('keydown', (event) => {
      if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return;
      const target = event.target;
      if (!(target instanceof Element) || !target.closest('textarea,[contenteditable="true"]')) return;
      const field = composer();
      if (!field || !field.contains(target)) {
        inform('Viðvörun: Óþekktur textareitur. Sent án DMZ-möskunar.');
        return;
      }
      void gate(event, () => field.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })), field);
    }, true);

    document.addEventListener('click', (event) => {
      const target = event.target;
      if (!(target instanceof Element)) return;
      if (target.closest('input[type="file"],button[aria-label*="attach" i],button[aria-label*="upload" i]')) {
        inform('Viðvörun: DMZ maskar ekki skrár, myndir eða viðhengi. Þau verða send óbreytt.');
        return;
      }
      const button = target.closest('button');
      if (!button) return;
      const known = sendButtons.some((selector) => button.matches(selector));
      const sendLike = button.matches('[aria-label*="send" i]') || (button.type === 'submit' && !!button.closest('form')?.querySelector('textarea,[contenteditable="true"]'));
      if (!known && !sendLike) return;
      void gate(event, () => button.click(), button);
    }, true);

    for (const kind of ['input', 'change'] as const) {
      document.addEventListener(kind, (event) => {
        if (!(event.target instanceof HTMLInputElement) || event.target.type !== 'file') return;
        inform('Viðvörun: DMZ maskar ekki skrár, myndir eða viðhengi. Þau verða send óbreytt.');
      }, true);
    }

    for (const kind of ['dragenter', 'dragover', 'drop'] as const) {
      document.addEventListener(kind, (event) => {
        if (!(event instanceof DragEvent) || !event.dataTransfer?.types.includes('Files')) return;
        inform('Viðvörun: DMZ maskar ekki skrár, myndir eða viðhengi. Þau verða send óbreytt.');
      }, true);
    }

    document.addEventListener('paste', (event) => {
      if (!event.clipboardData?.files.length) return;
      inform('Viðvörun: DMZ maskar ekki skrár, myndir eða viðhengi. Þau verða send óbreytt.');
    }, true);

    const core = globalThis as unknown as Core;
    function coreFailed() {
      ready = false;
      if (resetButton) resetButton.disabled = true;
      inform('Viðvörun: DMZ-kjarni bilaður. Sending heldur áfram án DMZ-möskunar.');
    }
    void (async () => {
      const response = await fetch(chrome.runtime.getURL('dmz.wasm'));
      if (!response.ok) throw new Error('WASM unavailable');
      const go = new core.Go();
      const { instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
      void go.run(instance).then(coreFailed, coreFailed);
      for (let i = 0; i < 100 && typeof core.dmzProcess !== 'function'; i++) {
        await new Promise((resolve) => setTimeout(resolve, 10));
      }
      if (typeof core.dmzProcess !== 'function' || typeof core.dmzProcessBestEffort !== 'function') throw new Error('WASM did not start');
      ready = true;
      message = 'DMZ tilbúinn á þessari síðu.';
      new MutationObserver(queueResponseScan).observe(document.documentElement, {
        childList: true, characterData: true, subtree: true, attributes: true,
        attributeFilter: ['contenteditable', 'role', 'id', 'class', 'style', 'hidden', 'aria-hidden', 'disabled', 'readonly', 'data-theme', 'data-mode', 'data-composer-markdown', 'data-testid', 'data-placeholder', 'data-message-author-role', 'data-markdown-text-style'],
      });
      queueResponseScan();
    })().catch(coreFailed);
  },
});
