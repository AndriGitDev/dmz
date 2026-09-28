// Benchmark-only client. Worker crash/deadline invalidates the entire worker;
// pending requests reject and no fallback returns unreviewed text.
export class DetectorClient {
  constructor(workerURL = '/worker.mjs') {
    this.worker = new Worker(workerURL, { type: 'module' });
    this.pending = new Map();
    this.sequence = 0;
    this.closed = false;
    this.worker.onmessage = ({ data }) => {
      const entry = this.pending.get(data.id);
      if (!entry) return;
      clearTimeout(entry.timer);
      this.pending.delete(data.id);
      if (data.error) entry.reject(new Error(data.error));
      else entry.resolve(data.result);
    };
    this.worker.onerror = event => { event.preventDefault(); this.close('Worker error'); };
    this.worker.onmessageerror = () => this.close('Worker message error');
  }
  request(type, data = {}, deadlineMs = 60000) {
    if (this.closed) return Promise.reject(new Error('Worker closed'));
    const id = ++this.sequence;
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => this.close('Deadline exceeded'), deadlineMs);
      this.pending.set(id, { resolve, reject, timer });
      try { this.worker.postMessage({ id, type, ...data }); }
      catch { this.close('Message failed'); }
    });
  }
  close(reason = 'Worker terminated') {
    this.closed = true;
    this.worker.terminate();
    for (const entry of this.pending.values()) { clearTimeout(entry.timer); entry.reject(new Error(reason)); }
    this.pending.clear();
  }
}
window.DetectorClient = DetectorClient;
window.makeClient = () => { window.client = new DetectorClient(); };
