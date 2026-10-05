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

// JavaScript can contain unpaired UTF-16 surrogates. The Go string bridge would
// replace them before detection, so reject processing instead of silently
// changing the draft; restoration must pass such a response through unchanged.
assert.equal(JSON.parse(globalThis.dmzSetTenantTerms('["Gervi term"]')).ok, true);
const preservedMarker = JSON.parse(globalThis.dmzProcess('demo@example.invalid')).transformed;
for (const value of ['\ud800', '\udfff', 'before\ud800after']) {
  for (const result of [globalThis.dmzProcess(value), globalThis.dmzProcessWithNames(value, '[]'),
    globalThis.dmzProcessBestEffort(value, '[]')]) {
    assert.equal(JSON.parse(result).error, 'invalid Unicode input');
  }
  assert.equal(globalThis.dmzRestore(value), value);
  assert.equal(globalThis.dmzRestore(value + preservedMarker), value + preservedMarker);
  assert.ok(JSON.parse(globalThis.dmzSetTenantTerms(JSON.stringify(['demo' + value]))).error);
  assert.ok(JSON.parse(globalThis.dmzSetTenantTerms('["demo' + value + '"]')).error);
  assert.equal(globalThis.dmzRestore(preservedMarker), 'demo@example.invalid', 'rejection reset existing mappings');
}
assert.ok(JSON.parse(globalThis.dmzProcess('Gervi term')).transformed.startsWith('⟦TERM-'));
assert.equal(JSON.parse(globalThis.dmzProcessBestEffort('🙂', '[]')).transformed, '🙂');
assert.equal(JSON.parse(globalThis.dmzSetTenantTerms(JSON.stringify(['demo🙂', 'demo\ufffd']))).ok, true);
globalThis.dmzReset();

for (const value of ['synthetic_token/with+padding==', 'a/b+c~d==', 'synthetic.token.', 'Q']) {
  globalThis.dmzReset();
  const prefix = 'Prófun 🙂\r\nAuthorization: Bearer ', input = prefix + value + '\r\n';
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.ok(result.allowed && result.spans_available);
  assert.deepEqual(result.detected_spans, [{ start: Buffer.byteLength(prefix),
    end: Buffer.byteLength(prefix + value), kind: 'secret' }]);
  assert.equal(globalThis.dmzRestore(result.transformed), input);
  assert.ok(!result.transformed.includes('Bearer ' + value));
  assert.equal(JSON.parse(globalThis.dmzProcess(input)).allowed, false);
}
globalThis.dmzReset();

for (const value of [
  'postgresql://demo:placeholder@invalid.invalid/db',
  'mysql://invalid.invalid/db?password=synthetic)==',
  'postgres:///db?sslpassword=synthetic_value&sslmode=require',
]) {
  globalThis.dmzReset();
  const input = `Prófun: "${value}" lokið.`;
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.ok(result.allowed && result.spans_available);
  assert.deepEqual(result.detected_spans, [{ start: Buffer.byteLength('Prófun: "'),
    end: Buffer.byteLength('Prófun: "' + value), kind: 'secret' }]);
  assert.ok(!result.transformed.includes('synthetic') && !result.transformed.includes('placeholder'));
  assert.equal(globalThis.dmzRestore(result.transformed), input);
}
// A presigned URL is one credential, including path, query and inner identifiers.
for (const tail of ['', '&opaque=a(b).,!']) {
  const value = 'https://objects.example/audit@example.test?X-Amz-Algorithm=AWS4-HMAC-SHA256' +
    '&X-Amz-Credential=ASIAA1B2C3D4E5F6G7H8%2F20261005%2Feu-west-1%2Fs3%2Faws4_request' +
    '&X-Amz-Signature=' + 'a1'.repeat(32) + tail;
  const prefix = 'Prófun 🙂: (', input = prefix + value + ')';
  globalThis.dmzReset();
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.ok(result.allowed && result.spans_available);
  assert.deepEqual(result.detected_spans, [{ start: Buffer.byteLength(prefix),
    end: Buffer.byteLength(prefix + value), kind: 'secret' }]);
  assert.ok(!result.transformed.includes('objects.example') && !result.transformed.includes('X-Amz'));
  assert.equal(globalThis.dmzRestore(result.transformed), input);
  assert.equal(JSON.parse(globalThis.dmzProcess(input)).allowed, false);
}
globalThis.dmzReset();

const wrappedCredential = '(postgresql:///db?password=synthetic))';
const wrappedResult = JSON.parse(globalThis.dmzProcessBestEffort(wrappedCredential, '[]'));
assert.deepEqual(wrappedResult.detected_spans, [{ start: 1, end: Buffer.byteLength(wrappedCredential) - 1, kind: 'secret' }]);
assert.equal(globalThis.dmzRestore(wrappedResult.transformed), wrappedCredential);
for (const input of ['postgres://invalid.invalid/db', 'mysql://demo@invalid.invalid/db', 'postgresql:///db?password=']) {
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.equal(result.transformed, input);
  assert.deepEqual(result.detected_spans, []);
}
globalThis.dmzReset();

for (const label of ['PRIVATE KEY', 'RSA PRIVATE KEY', 'ENCRYPTED PRIVATE KEY', 'OPENSSH PRIVATE KEY']) {
  const block = `-----BEGIN ${label}-----\r\nSYNTHETIC_NOT_A_KEY\r\n-----END ${label}-----`;
  const prefix = 'Prófun 🙂: ', suffix = ' lokið.', input = prefix + block + suffix;
  globalThis.dmzReset();
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.ok(result.allowed && result.spans_available);
  assert.deepEqual(result.detected_spans, [{ start: Buffer.byteLength(prefix),
    end: Buffer.byteLength(prefix + block), kind: 'secret' }]);
  assert.ok(result.transformed.startsWith(prefix) && result.transformed.endsWith(suffix));
  assert.ok(!result.transformed.includes('SYNTHETIC'));
  assert.equal(globalThis.dmzRestore(result.transformed), input);
}
const incompleteKey = 'Prófun: -----BEGIN PRIVATE KEY-----\nSYNTHETIC_TRUNCATED_BODY';
const incompleteResult = JSON.parse(globalThis.dmzProcessBestEffort(incompleteKey, '[]'));
assert.equal(incompleteResult.spans_available, false);
assert.ok(!incompleteResult.transformed.includes('SYNTHETIC'));
assert.equal(globalThis.dmzRestore(incompleteResult.transformed), incompleteKey);
globalThis.dmzReset();

for (const form of ['NFC', 'NFD']) {
  const typedInput = 'Ása demo@example.invalid'.normalize(form);
  const nameLength = Buffer.byteLength('Ása'.normalize(form));
  const typed = JSON.parse(globalThis.dmzProcessBestEffort(typedInput, JSON.stringify([
    { start: 0, end: nameLength }, { start: nameLength + 1, end: nameLength + 5 },
  ])));
  assert.equal(typed.spans_available, true);
  assert.deepEqual(typed.detected_spans, [
    { start: 0, end: nameLength, kind: 'name_demo' },
    { start: nameLength + 1, end: Buffer.byteLength(typedInput), kind: 'email' },
  ]);
  assert.equal(globalThis.dmzRestore(typed.transformed), typedInput);
}
const wholeTyped = JSON.parse(globalThis.dmzProcessBestEffort('x\u0301', '[]'));
assert.equal(wholeTyped.spans_available, false);
assert.equal(wholeTyped.detected_spans, null);
const emptyTyped = JSON.parse(globalThis.dmzProcessBestEffort('Ordinary prose.', '[]'));
assert.equal(emptyTyped.spans_available, true);
assert.deepEqual(emptyTyped.detected_spans, []);
globalThis.dmzReset();

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

for (const forms of [
  ['140182-8858', '140182\u00a08858', '1401828858'],
  ['5555 5555 5555 4444', '5555-5555-5555-4444'],
  ['IS46 0000 0000 0000 0000 0000 00', 'is46' + '0'.repeat(22)],
  ['demo@example.invalid', 'demo@EXAMPLE.INVALID'],
  ['+354 000 0002', '+354\u202f000\u202f0002'],
]) {
  globalThis.dmzReset();
  const text = forms.join(' og ');
  const result = JSON.parse(globalThis.dmzProcessBestEffort(text, '[]'));
  assert.ok(result.allowed);
  assert.equal(globalThis.dmzRestore(result.transformed), text,
    'same-session formatting variants did not restore exactly');
  const issued = forms.map(form => JSON.parse(globalThis.dmzProcessBestEffort(form, '[]')).transformed);
  assert.equal(new Set(issued).size, forms.length, 'different original forms shared a marker');
  for (const marker of issued) assert.ok(result.transformed.includes(marker), 'exact form changed marker');
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
  for (const hyphen of ['‐', '‑']) {
    spacingAddresses.push(`${pack.addresses[0]} 97${hyphen}99`);
    const hyphenated = pack.addresses.find(value => value.includes('-'));
    if (hyphenated) spacingAddresses.push(`${hyphenated.replaceAll('-', hyphen)} 97${hyphen}99`);
  }
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
for (const value of [
  ...['ghp_', 'gho_', 'ghu_', 'ghs_', 'ghr_', 'github_pat_'].map(prefix => prefix + 'aB_'.repeat(40)),
  'ghs_12345_eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.' + 'aB'.repeat(80) + '.' + 'cD'.repeat(420),
]) {
  globalThis.dmzReset();
  const input = `Prófun 🙂: ${value}.`;
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.equal(result.allowed, true);
  assert.equal(result.spans_available, true);
  assert.equal(result.detected_spans.length, 1);
  const span = result.detected_spans[0];
  assert.equal(span.kind, 'secret');
  assert.equal(Buffer.from(input).subarray(span.start, span.end).toString(), value);
  assert.ok(!result.transformed.includes(value));
  assert.equal(globalThis.dmzRestore(result.transformed), input);
  assert.equal(JSON.parse(globalThis.dmzProcess(input)).allowed, false);
}
for (const input of [
  'api_key=X7a9/Q2b8+N4c6~R3d5.T1e0V==',
  '🙂\r\n{"client_secret":"X7a9/Q2b8+N4c6~R3d5.T1e0V=="}',
]) {
  globalThis.dmzReset();
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.equal(result.allowed, true);
  assert.equal(result.spans_available, true);
  assert.equal(result.detected_spans.length, 1);
  const span = result.detected_spans[0];
  assert.equal(span.kind, 'secret');
  assert.equal(Buffer.from(input).subarray(span.start, span.end).toString(), 'X7a9/Q2b8+N4c6~R3d5.T1e0V==');
  assert.ok(!result.transformed.includes('X7a9/Q2b8+N4c6~R3d5.T1e0V=='));
  assert.equal(globalThis.dmzRestore(result.transformed), input);
  assert.equal(JSON.parse(globalThis.dmzProcess(input)).allowed, false);
}
for (const [language, quoted, expected] of [
  ['python', false, 0], ['javascript', false, 0],
  ['python', true, 1], ['dotenv', false, 1],
]) {
  globalThis.dmzReset();
  const value = 'process.env.SERVICE_API_TOKEN_12345';
  const input = '```' + language + '\napi_key=' + (quoted ? '"' + value + '"' : value) + '\n```';
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.equal(result.allowed, true);
  assert.equal(result.spans_available, true);
  assert.equal(result.detected_spans.length, expected);
  assert.equal(globalThis.dmzRestore(result.transformed), input);
  if (expected) {
    const span = result.detected_spans[0];
    assert.equal(span.kind, 'secret');
    assert.equal(Buffer.from(input).subarray(span.start, span.end).toString(), value);
  } else assert.equal(result.transformed, input);
}
for (const value of ["o'connor@example.invalid", 'team!desk@example.invalid',
  '"team one"@example.invalid', '"team\\\"one"@example.invalid',
  '📮@example.invalid', 'team@[IPv6:2001:db8::1]', 'team@example.xn--p1ai']) {
  globalThis.dmzReset();
  const input = 'Contact ' + value + ' today.';
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.equal(result.allowed, true);
  assert.equal(result.spans_available, true);
  const spans = result.detected_spans.filter(span => span.kind === 'email');
  assert.equal(spans.length, 1);
  assert.equal(Buffer.from(input).subarray(spans[0].start, spans[0].end).toString(), value);
  assert.ok(!result.transformed.includes(value));
  assert.equal(globalThis.dmzRestore(result.transformed), input);
}
for (const input of ["EMAIL=demo@example.invalid", "const recipient='demo@example.invalid';",
  'Contact team=one@example.invalid today.']) {
  globalThis.dmzReset();
  const value = input.startsWith('Contact') ? 'team=one@example.invalid' : 'demo@example.invalid';
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.equal(result.allowed, true);
  const span = result.detected_spans.find(span => span.kind === 'email');
  assert.ok(span);
  assert.equal(Buffer.from(input).subarray(span.start, span.end).toString(), value);
  assert.equal(globalThis.dmzRestore(result.transformed), input);
}
for (const access of ['ASIAA1B2C3D4E5F6G7H8', 'ASIAaB_7cD9eF2gH', 'ASIA' + 'aB_7'.repeat(31)]) {
  globalThis.dmzReset();
  const values = [access, 'X7a9Q2b8N4c6R3d5T1e0V8g6H4j2K9m7P5s3U1w0',
    'IQoJb3JpZ2luX2VjEA7b8/C9d0+E1f2G3h4I5j6=='];
  const input = JSON.stringify({ AccessKeyId: values[0], SecretAccessKey: values[1], SessionToken: values[2] });
  const result = JSON.parse(globalThis.dmzProcessBestEffort(input, '[]'));
  assert.equal(result.allowed, true);
  assert.equal(result.spans_available, true);
  assert.deepEqual(result.detected_spans.map(span => {
    assert.equal(span.kind, 'secret');
    return Buffer.from(input).subarray(span.start, span.end).toString();
  }), values);
  for (const value of values) assert.ok(!result.transformed.includes(value));
  assert.equal(globalThis.dmzRestore(result.transformed), input);
  assert.equal(JSON.parse(globalThis.dmzProcess(input)).allowed, false);
}
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
  ['Pro\u0301fru\u0301n Gervido\u0301ttir', 'name_demo'],
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
