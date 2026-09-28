import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import vm from 'node:vm';

const publicDir = new URL('../public/', import.meta.url).pathname;
vm.runInThisContext(readFileSync(join(publicDir, 'wasm_exec.js'), 'utf8'));
const go = new globalThis.Go();
const bytes = readFileSync(join(publicDir, 'dmz.wasm'));
const { instance } = await WebAssembly.instantiate(bytes, go.importObject);
void go.run(instance);
for (let i = 0; i < 100 && typeof globalThis.dmzProcess !== 'function'; i++) {
  await new Promise((resolve) => setTimeout(resolve, 10));
}
assert.equal(typeof globalThis.dmzProcess, 'function', 'WASM exports not ready');

const first = JSON.parse(globalThis.dmzProcess('Arna Jónsdóttir, 140182-8858.'));
assert.equal(first.allowed, true);
assert.match(first.transformed, /⟦KT-[A-F0-9]{16}-\d+: XXXXX-XXXX⟧/);
assert.ok(!first.transformed.includes('140182-8858'));
assert.equal(globalThis.dmzRestore(first.transformed), 'Arna Jónsdóttir, 140182-8858.');

const iban = JSON.parse(globalThis.dmzProcess('IS46-0000-0000-0000-0000-0000-00'));
assert.equal(iban.allowed, true);
assert.match(iban.transformed, /⟦IBAN-[A-F0-9]{16}-\d+:/);
assert.equal(globalThis.dmzRestore(iban.transformed), 'IS46-0000-0000-0000-0000-0000-00');

const unchecked = JSON.parse(globalThis.dmzProcess('140182-8868'));
assert.equal(unchecked.allowed, true);
assert.match(unchecked.transformed, /XXXXX-XXXX/);
assert.equal(unchecked.findings[0].confidence, 'uncertain');

const blocked = JSON.parse(globalThis.dmzProcess('sk-test_key_000000000000'));
assert.equal(blocked.allowed, false);
assert.equal(blocked.transformed, undefined);

globalThis.dmzReset();
assert.equal(globalThis.dmzRestore(first.transformed), first.transformed);
process.stdout.write('TinyGo WASM mask/restore smoke test passed\n');
process.exit(0);
