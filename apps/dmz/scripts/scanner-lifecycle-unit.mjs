// Exercise offscreen lifecycle without waiting for real 120-second deadlines.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import ts from 'typescript';
const source = await readFile(new URL('../entrypoints/scanner/main.ts', import.meta.url), 'utf8');
let listener, nextTimer = 0, failConstruction = false;
const timers = new Map(), workers = [];
class Worker {
  terminated = false;
  constructor() { if (failConstruction) throw new Error('private constructor error'); workers.push(this); }
  postMessage(data) { this.last = data; }
  terminate() { this.terminated = true; }
  reply(result) { this.onmessage({ data: { id: this.last.id, ...result } }); }
}
const chrome = { runtime: { id: 'synthetic-extension', getURL: path => `chrome-extension://synthetic-extension/${path}`,
  onMessage: { addListener: value => { listener = value; } } } };
vm.runInNewContext(ts.transpile(source, { target: ts.ScriptTarget.ES2022 }), {
  Worker, chrome,
  setTimeout: callback => { timers.set(++nextTimer, callback); return nextTimer; },
  clearTimeout: id => timers.delete(id),
});
const responses = [];
const send = (text = 'Anna', sender = { id: chrome.runtime.id, url: chrome.runtime.getURL('background.js') }) =>
  listener({ type: 'dmz:scanner-run', text }, sender, result => responses.push(JSON.parse(JSON.stringify(result))));
send('Anna', { id: chrome.runtime.id, tab: { id: 1 } });
assert.equal(workers.length, 0, 'content sender bypassed background authorization');
send();
const warm = workers[0];
send('second'); send('third'); send('overflow');
assert.equal(responses.at(-1).code, 'busy');
assert.equal(warm.last.text, 'Anna');
send(null);
assert.equal(responses.at(-1).code, 'unsupported_input');
const beforeStale = responses.length;
warm.onmessage({ data: { id: -1, spans: [] } });
assert.equal(responses.length, beforeStale);
assert.equal(timers.size, 3);
assert.equal(warm.terminated, false);
warm.reply({ error: 'untrusted private exception', code: 'unsupported_input' });
assert.deepEqual(responses.at(-1), { error: 'Local scanner failed', code: 'unsupported_input' });
assert.equal(warm.terminated, false);
assert.equal(warm.last.text, 'second');
warm.reply({ spans: [] });
assert.equal(warm.last.text, 'third');
warm.reply({ spans: [] });
assert.equal(timers.size, 0);
send();
assert.equal(workers.length, 1);
warm.reply({ spans: [{ start: 0, end: 4 }] });
assert.deepEqual(responses.at(-1), { spans: [{ start: 0, end: 4 }] });
for (const code of ['asset_failure', 'inference_failure', 'unknown-private-detail']) {
  send();
  const current = workers.at(-1);
  current.reply({ error: 'private exception text', code });
  assert.equal(current.terminated, true);
  assert.equal(responses.at(-1).code, code === 'unknown-private-detail' ? 'worker_unavailable' : code);
  assert.equal(timers.size, 0);
}
send();
const stuck = workers.at(-1), beforeTimeout = responses.length;
[...timers.values()][0]();
assert.equal(stuck.terminated, true);
assert.equal(responses.length, beforeTimeout + 1);
assert.equal(responses.at(-1).code, 'timeout');
stuck.reply({ spans: [] });
assert.equal(responses.length, beforeTimeout + 1, 'late response resolved request twice');
send();
assert.notEqual(workers.at(-1), stuck);
const replacement = workers.at(-1);
stuck.onerror({ preventDefault() {} });
stuck.onmessageerror();
stuck.reply({ spans: [] });
assert.equal(replacement.terminated, false);
replacement.reply({ spans: [] });
assert.equal(timers.size, 0);
// Expiring a queued job does not interrupt the active scan.
send('active'); send('expired'); send('remaining');
const active = workers.at(-1), beforeExpiry = responses.length;
[...timers.values()][1]();
assert.equal(responses.length, beforeExpiry + 1);
assert.equal(responses.at(-1).code, 'timeout');
assert.equal(active.terminated, false);
active.reply({ spans: [] });
assert.equal(active.last.text, 'remaining');
active.reply({ spans: [] });
assert.equal(timers.size, 0);
send('active'); send('waiting one'); send('waiting two');
const beforeFailure = responses.length;
workers.at(-1).reply({ error: 'private detail', code: 'inference_failure' });
assert.equal(responses.length, beforeFailure + 3);
assert.ok(responses.slice(-3).every(value => value.code === 'inference_failure'));
assert.equal(timers.size, 0);
listener({ type: 'dmz:scanner-run', text: 'closed tab' },
  { id: chrome.runtime.id }, () => { throw new Error('closed port'); });
send('survivor');
workers.at(-1).reply({ spans: [] });
assert.equal(workers.at(-1).last.text, 'survivor');
workers.at(-1).reply({ spans: [] });
assert.equal(timers.size, 0);
const originalPost = Worker.prototype.postMessage;
Worker.prototype.postMessage = () => { throw new Error('private post failure'); };
send();
assert.equal(responses.at(-1).code, 'worker_unavailable');
assert.equal(timers.size, 0);
Worker.prototype.postMessage = originalPost;
failConstruction = true;
send();
assert.equal(responses.at(-1).code, 'worker_unavailable');
assert.equal(timers.size, 0);
failConstruction = false;
send(); workers.at(-1).reply({ spans: [] });
assert.equal(timers.size, 0);
console.log('Scanner bounded FIFO, expiry, stale-worker isolation, closed ports and recovery checks passed');
