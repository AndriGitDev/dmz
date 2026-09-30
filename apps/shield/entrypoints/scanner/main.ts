// One model worker for all tabs. Text is transient and never logged or stored.
let worker: Worker | undefined;
let sequence = 0;
let pending:
  | { id: number; respond: (result: unknown) => void; timer: ReturnType<typeof setTimeout> }
  | undefined;
function close() {
  worker?.terminate();
  worker = undefined;
  if (pending) {
    clearTimeout(pending.timer);
    pending.respond({ error: 'Scanner unavailable; sending blocked' });
    pending = undefined;
  }
}
chrome.runtime.onMessage.addListener((request, sender, respond) => {
  if (
    request?.type !== 'dmz:scanner-run' ||
    sender.id !== chrome.runtime.id ||
    sender.tab ||
    (sender.url && sender.url !== chrome.runtime.getURL('background.js'))
  )
    return;
  if (pending || typeof request.text !== 'string' || request.text.length > 16000) {
    respond({ error: 'Scanner busy or input unsupported' });
    return;
  }
  if (!worker) {
    worker = new Worker(chrome.runtime.getURL('scanner/worker.mjs'), { type: 'module' });
    worker.onerror = (event) => {
      event.preventDefault();
      close();
    };
    worker.onmessageerror = close;
    worker.onmessage = ({ data }) => {
      if (!pending || data.id !== pending.id) return;
      const current = pending;
      pending = undefined;
      clearTimeout(current.timer);
      if (data.error) {
        close();
        current.respond({ error: 'Local scanner failed' });
      } else current.respond({ spans: data.spans });
    };
  }
  const id = ++sequence;
  pending = { id, respond, timer: setTimeout(close, 120000) };
  try {
    worker.postMessage({ id, text: request.text });
  } catch {
    close();
  }
  return true;
});
