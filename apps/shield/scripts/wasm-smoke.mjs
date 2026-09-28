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

const email = JSON.parse(globalThis.dmzProcess('demo@example.com'));
const repeat = JSON.parse(globalThis.dmzProcess(email.transformed));
assert.equal(repeat.allowed, true);
assert.equal(repeat.transformed, email.transformed, 'resubmission nested an email marker');
assert.equal(globalThis.dmzRestore(repeat.transformed), 'demo@example.com');
const nested = JSON.parse(globalThis.dmzProcess(email.transformed.replace('XXXXX@XXXXX.XXX', email.transformed)));
assert.equal(nested.allowed, false, 'nested marker accepted');

const iban = JSON.parse(globalThis.dmzProcess('IS46-0000-0000-0000-0000-0000-00'));
assert.equal(iban.allowed, true);
assert.match(iban.transformed, /⟦IBAN-[A-F0-9]{16}-\d+:/);
assert.equal(globalThis.dmzRestore(iban.transformed), 'IS46-0000-0000-0000-0000-0000-00');

const unchecked = JSON.parse(globalThis.dmzProcess('140182-8868'));
assert.equal(unchecked.allowed, true);
assert.match(unchecked.transformed, /XXXXX-XXXX/);
assert.equal(unchecked.findings[0].confidence, 'uncertain');

// Reset between variants so restoration verifies the exact original formatting.
for (const input of ['140182.8858', '140182\u202f8868', '4111.1111.1111.1111',
  '4111\u00a01111\u00a01111\u00a01111', '(+354) 000 0001', '+354 000.0001']) {
  globalThis.dmzReset();
  const result = JSON.parse(globalThis.dmzProcess(input));
  assert.equal(result.allowed, true, input);
  assert.notEqual(result.transformed, input);
  assert.equal(globalThis.dmzRestore(result.transformed), input);
  assert.equal(JSON.parse(globalThis.dmzProcess(result.transformed)).transformed, result.transformed);
}

const blocked = JSON.parse(globalThis.dmzProcess('sk-test_key_000000000000'));
assert.equal(blocked.allowed, false);
assert.equal(blocked.transformed, undefined);
const jwtBlocked = JSON.parse(globalThis.dmzProcess('eyJ0ZXN0Ijp0cnVlfQ.eyJhdWQiOiJ0ZXN0In0.c2lnbmF0dXJl'));
assert.equal(jwtBlocked.allowed, false);
assert.equal(jwtBlocked.transformed, undefined);
const nameBlocked = JSON.parse(globalThis.dmzProcess('Guðrún Sigurðardóttir'));
assert.equal(nameBlocked.allowed, false);
assert.equal(nameBlocked.findings[0].action, 'review');
const addressBlocked = JSON.parse(globalThis.dmzProcess('Prófgata 99, 999 Reykjavík'));
assert.equal(addressBlocked.allowed, false);
assert.equal(addressBlocked.findings[0].kind, 'address');
for (const [input, kind] of [
  ['próf@example.invalid', 'email'],
  ['demo@dæmi.invalid', 'email'],
  ['00354 000.0001', 'phone'],
  ['00354 000 0000', 'phone'],
  ['Prófgata 99, 999', 'address'],
  ['Prófvegur 12 999 Reykjavík', 'address'],
  ['Prófrún Gerviburs', 'name_demo'],
  ['ghp_000000000000000000000000000000000000', 'secret'],
  ['demo@exa\u200bmple.invalid', 'unicode_review'],
  ['Pro\u0301fru\u0301n Gervido\u0301ttir', 'unicode_review'],
]) {
  const result = JSON.parse(globalThis.dmzProcess(input));
  assert.equal(result.allowed, false, `${kind} should block outbound text`);
  assert.equal(result.transformed, undefined);
  assert.equal(result.findings[0].kind, kind);
}

assert.equal(JSON.parse(globalThis.dmzSetTenantTerms(JSON.stringify(['Verkefni Gervi']))).ok, true);
const tenant = JSON.parse(globalThis.dmzProcess('Ræddu Verkefni Gervi.'));
assert.equal(tenant.allowed, true);
assert.match(tenant.transformed, /⟦TERM-[A-F0-9]{16}-\d+: XXXXX⟧/);
assert.equal(globalThis.dmzRestore(tenant.transformed), 'Ræddu Verkefni Gervi.');
assert.equal(globalThis.dmzRestore(first.transformed), first.transformed, 'term update did not reset map');

globalThis.dmzReset();
assert.equal(globalThis.dmzRestore(first.transformed), first.transformed);
assert.equal(JSON.parse(globalThis.dmzProcess(email.transformed)).allowed, false, 'stale marker accepted after reset');
process.stdout.write('TinyGo WASM mask/restore smoke test passed\n');
process.exit(0);
