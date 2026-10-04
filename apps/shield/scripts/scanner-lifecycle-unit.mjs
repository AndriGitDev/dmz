// Exercise offscreen lifecycle without waiting for real 120-second deadlines.
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import vm from 'node:vm';
import ts from 'typescript';
const source = await readFile(new URL('../entrypoints/scanner/main.ts', import.meta.url), 'utf8');
let listener, nextTimer = 0;
const timers = new Map(), workers = [];
class Worker {
  terminated = false;
  constructor() { workers.push(this); }
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
send();
assert.equal(responses.at(-1).code, 'busy');
assert.equal(warm.terminated, false);
warm.reply({ error: 'untrusted private exception', code: 'unsupported_input' });
assert.deepEqual(responses.at(-1), { error: 'Local scanner failed', code: 'unsupported_input' });
assert.equal(warm.terminated, false);
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
workers.at(-1).reply({ spans: [] });
assert.equal(timers.size, 0);
console.log('Scanner input rejection, worker recovery, fixed reasons, authorization and deadline lifecycle checks passed');
