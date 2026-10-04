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

for (const separator of ['\t', '\u00a0', '\u202f', '\u2011', '\u2013']) {
  for (const value of [
    `140182${separator}8858`, `+354${separator}000${separator}0002`,
    ['5555', '5555', '5555', '4444'].join(separator),
    ['IS46', '0000', '0000', '0000', '0000', '0000', '00'].join(separator),
  ]) {
    globalThis.dmzReset();
    const result = JSON.parse(globalThis.dmzProcessBestEffort(value, '[]'));
    assert.ok(result.allowed && /^⟦(?:KT|PHONE|CARD|IBAN)-[A-F0-9]{16}-\d+: [^⟦⟧]+⟧$/.test(result.transformed),
      'horizontal numeric typography was not completely masked');
    assert.ok(globalThis.dmzRestore(result.transformed) === value, 'numeric typography did not restore');
  }
}
globalThis.dmzReset();

for (const value of [
  'tilraunagata 12, 999 Sýnibær', 'Tilraunaslóð 12 / 2, 999 Sýnibær',
  'Tilraunavegi 12 – 14, 999 Sýnibær', 'Tilraunabraut 12-14,999',
]) {
  globalThis.dmzReset();
  const result = JSON.parse(globalThis.dmzProcessBestEffort(value, '[]'));
  assert.ok(result.allowed && /^⟦VALUE-[A-F0-9]{16}-\d+: XXXXX⟧$/.test(result.transformed),
    'unknown address shape was not completely masked');
  assert.equal(globalThis.dmzRestore(result.transformed), value);
}
globalThis.dmzReset();

// Optional local-only integration check: source entries never leave this process
// or appear in test output. CI can use a fabricated pack through the same path.
if (process.env.DMZ_LEXICON_FILE) {
  const pack = JSON.parse(readFileSync(process.env.DMZ_LEXICON_FILE, 'utf8'));
  let nameReviewed = false;
  for (const name of pack.names.slice(0, 30)) {
    const result = JSON.parse(globalThis.dmzProcess(name));
    if (!result.allowed && result.findings.some(f => f.reason === 'corpus-name-lexicon')) {
      nameReviewed = true;
      break;
    }
  }
  if (pack.names.length) assert.equal(nameReviewed, true, 'bundled name lexicon did not review a known entry');
  const address = JSON.parse(globalThis.dmzProcess(`${pack.addresses[0]} 99/2, 999`));
  assert.equal(address.allowed, false, 'bundled address lexicon failed');
  assert.ok(address.findings.some(f => f.reason === 'gazetteer-number-postcode'));
  const shortAddress = `${pack.addresses[0]} 99b`;
  const mixed = `Skrifaðu 5 setningar: demo@example.com +354 555 0100 ${shortAddress}`;
  const masked = JSON.parse(globalThis.dmzProcessBestEffort(mixed, '[]'));
  assert.equal(masked.allowed, true);
  assert.ok(masked.findings.some(f => f.kind === 'address' && f.reason === 'gazetteer-number'));
  assert.ok(!masked.transformed.includes(shortAddress));
  assert.equal(globalThis.dmzRestore(masked.transformed), mixed);
  const resubmitted = JSON.parse(globalThis.dmzProcessBestEffort(masked.transformed, '[]'));
  assert.equal(resubmitted.transformed, masked.transformed);
  // Literal disappearance alone would accept a partially masked range. Require
  // the complete input to be replaced by exactly one issued marker instead.
  const spacingAddresses = [
    `${pack.addresses[0]} 97 - 99`, `${pack.addresses[0]} 97\u00a0–\u00a099`,
    `${pack.addresses[0]} 99 / 2`,
  ];
  const multiword = pack.addresses.find(value => value.includes(' '));
  if (multiword) spacingAddresses.push(`${multiword.replaceAll(' ', '\u00a0')} 99`);
  for (const value of spacingAddresses) {
    globalThis.dmzReset();
    const result = JSON.parse(globalThis.dmzProcessBestEffort(value, '[]'));
    assert.ok(result.allowed && /^⟦VALUE-[A-F0-9]{16}-\d+: XXXXX⟧$/.test(result.transformed),
      'address spacing left an unmasked suffix');
    assert.ok(globalThis.dmzRestore(result.transformed) === value, 'address spacing did not restore');
  }
  for (const suffix of ['99\u00a0kr.', '99\u202fISK', '99,\u00a050 kr.']) {
    const value = `${pack.addresses[0]} ${suffix}`;
    const result = JSON.parse(globalThis.dmzProcessBestEffort(value, '[]'));
    assert.ok(result.allowed && result.transformed === value, 'currency spacing caused a false mask');
  }
  if (pack.name_seeds?.length) {
    const seedReviewed = pack.name_seeds.slice(0, 30).some(seed => {
      const result = JSON.parse(globalThis.dmzProcess(seed));
      return !result.allowed && result.findings.some(f => f.reason === 'corpus-name-seed');
    });
    assert.equal(seedReviewed, true, 'bundled name seeds did not review a known entry');
  }
  globalThis.dmzReset();
}


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

for (const input of ['sk-test_key_000000000000', 'jón@example.com', 'Ólafur Sigurðsson']) {
  const masked = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.equal(masked.allowed, true);
  assert.ok(!masked.transformed.includes(input));
  assert.equal(globalThis.dmzRestore(masked.transformed), input);
}
const emailName = JSON.parse(globalThis.dmzProcessBestEffort('anna@example.com', '[{"start":0,"end":4}]'));
assert.equal(emailName.allowed, true);
assert.match(emailName.transformed, /⟦EMAIL-/);
assert.equal(globalThis.dmzRestore(emailName.transformed), 'anna@example.com');
console.log('Shield best-effort secret, uncertain PII and email/name overlap checks passed');

process.exit(0);
