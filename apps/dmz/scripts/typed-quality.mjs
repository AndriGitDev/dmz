// Evaluation only: adapt the actual core result to the shared exact-set scorer.
// Records remain in memory; aggregate reports contain neither text nor spans.
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

const kinds = Object.freeze({
  name_demo: 'Person', address: 'Address', kennitala: 'Kennitala', email: 'Email',
  phone: 'Phone', iban: 'IBAN', card: 'Card', secret: 'Secret', tenant_dictionary: 'TenantTerm',
});
export const annotationScope = Object.freeze(Object.values(kinds).sort());

export function typedRecord(text, gold, result) {
  if (typeof text !== 'string' || Buffer.from(text).toString() !== text ||
      typeof result?.spans_available !== 'boolean') throw Error('Missing typed-result contract');
  const bytes = Buffer.from(text);
  const boundary = offset => offset === bytes.length || (bytes[offset] & 0xc0) !== 0x80;
  function validate(spans) {
    if (!Array.isArray(spans)) throw Error('Missing span array');
    const sorted = spans.map(span => {
      const { start, end, kind } = span || {};
      if (!Number.isInteger(start) || !Number.isInteger(end) || start < 0 || end <= start ||
          end > bytes.length || !boundary(start) || !boundary(end) || !Object.hasOwn(kinds, kind))
        throw Error('Invalid typed span');
      return [start, end, kinds[kind]];
    }).sort((a, b) => a[0] - b[0] || a[1] - b[1]);
    for (let i = 1; i < sorted.length; i++) {
      if (sorted[i][0] < sorted[i - 1][1]) throw Error('Duplicate or overlapping typed spans');
    }
    return sorted;
  }
  if (!result.spans_available && result.detected_spans !== null)
    throw Error('Unavailable typed result must not supply spans');
  return { gold: validate(gold),
    predicted: result.spans_available ? validate(result.detected_spans) : [],
    abstained: !result.spans_available };
}

export function aggregateQuality(records) {
  const scorer = fileURLToPath(new URL('../../../bench/contextual/quality.py', import.meta.url));
  return JSON.parse(execFileSync(process.env.DMZ_PYTHON || 'python3', [scorer], {
    input: JSON.stringify({ records, kinds: annotationScope }), encoding: 'utf8', maxBuffer: 1024 * 1024,
    timeout: 30000,
  }));
}
