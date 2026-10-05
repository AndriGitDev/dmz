// One model worker for all tabs. At most one active scan and two waiting drafts.
// Text is transient, never logged/stored, and released after posting or settling.
type Scan = {
  id: number;
  text?: string;
  respond: (result: unknown) => void;
  timer: ReturnType<typeof setTimeout>;
};
let worker: Worker | undefined;
let sequence = 0;
let pending: Scan | undefined;
const waiting: Scan[] = [];
function settle(scan: Scan, result: unknown) {
  clearTimeout(scan.timer);
  scan.text = undefined;
  // A tab may have closed its message port while waiting. Keep the queue moving.
  try { scan.respond(result); } catch { /* Closed port. */ }
}
function close(code = 'worker_unavailable') {
  const old = worker;
  worker = undefined;
  old?.terminate();
  const scans = pending ? [pending, ...waiting] : [...waiting];
  pending = undefined;
  waiting.length = 0;
  for (const scan of scans) settle(scan, { error: 'Scanner unavailable', code });
}
function pump() {
  if (pending || !waiting.length) return;
  pending = waiting.shift()!;
  try {
    if (!worker) {
      const currentWorker = new Worker(chrome.runtime.getURL('scanner/worker.mjs'), { type: 'module' });
      worker = currentWorker;
      currentWorker.onerror = (event) => {
        event.preventDefault();
        if (worker === currentWorker) close();
      };
      currentWorker.onmessageerror = () => {
        if (worker === currentWorker) close('worker_message_error');
      };
      currentWorker.onmessage = ({ data }) => {
        if (worker !== currentWorker || !pending || data?.id !== pending.id) return;
        if (data.error) {
          const code = ['unsupported_input', 'asset_failure', 'inference_failure', 'busy'].includes(data.code)
            ? data.code : 'worker_unavailable';
          if (code !== 'unsupported_input') { close(code); return; }
          const scan = pending;
          pending = undefined;
          settle(scan, { error: 'Local scanner failed', code });
        } else {
          const scan = pending;
          pending = undefined;
          settle(scan, { spans: data.spans });
        }
        pump();
      };
    }
    worker.postMessage({ id: pending.id, text: pending.text });
    pending.text = undefined;
  } catch { close(); }
}
chrome.runtime.onMessage.addListener((request, sender, respond) => {
  if (
    request?.type !== 'dmz:scanner-run' ||
    sender.id !== chrome.runtime.id ||
    sender.tab ||
    (sender.url && sender.url !== chrome.runtime.getURL('background.js'))
  ) return;
  if (typeof request.text !== 'string' || request.text.length > 16000) {
    respond({ error: 'Scanner input unsupported', code: 'unsupported_input' });
    return;
  }
  if (pending && waiting.length >= 2) {
    respond({ error: 'Scanner busy', code: 'busy' });
    return;
  }
  const id = ++sequence;
  const scan: Scan = { id, text: request.text, respond, timer: setTimeout(() => {
    if (pending?.id === id) { close('timeout'); return; }
    const index = waiting.findIndex(item => item.id === id);
    if (index >= 0) {
      const [expired] = waiting.splice(index, 1);
      settle(expired!, { error: 'Scanner unavailable', code: 'timeout' });
    }
  }, 120000) };
  waiting.push(scan);
  pump();
  return true;
});
