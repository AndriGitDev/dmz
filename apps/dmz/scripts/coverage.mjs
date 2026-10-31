// Test-only coverage accounting. No text, marker, value, or span leaves reports.
const markerPattern = /⟦(?:KT|EMAIL|IBAN|PHONE|CARD|NAME|TERM|VALUE)-[A-F0-9]{16}-\d+: [^⟦⟧]*⟧/gu;

export function annotationSpans(row) {
  if (typeof row.text !== 'string' || !row.text || !Array.isArray(row.entities) || Buffer.from(row.text).toString() !== row.text)
    throw Error('Complete text and annotations required; null is not negative');
  const seen = new Set();
  return row.entities.map(entity => {
    if (typeof entity.kind !== 'string' || !entity.kind || typeof entity.value !== 'string' || !entity.value)
      throw Error('Invalid annotation');
    const at = row.text.indexOf(entity.value);
    if (at < 0 || row.text.indexOf(entity.value, at + 1) >= 0)
      throw Error('Annotation must identify one occurrence');
    const start = Buffer.byteLength(row.text.slice(0, at)), end = start + Buffer.byteLength(entity.value);
    const key = `${start}:${end}:${entity.kind}`;
    if (seen.has(key)) throw Error('Duplicate annotation');
    seen.add(key);
    return { start, end, kind: entity.kind };
  });
}

export function restoredMaskSpans(input, transformed, restoredMarkers) {
  if (typeof input !== 'string' || typeof transformed !== 'string' || !Array.isArray(restoredMarkers))
    throw Error('Invalid masking result');
  const source = Buffer.from(input), masks = [];
  let outputCursor = 0, sourceCursor = 0, index = 0;
  function consume(value) {
    const bytes = Buffer.from(value);
    if (!source.subarray(sourceCursor, sourceCursor + bytes.length).equals(bytes))
      throw Error('Output does not reconstruct the original input exactly');
    sourceCursor += bytes.length;
  }
  for (const match of transformed.matchAll(markerPattern)) {
    consume(transformed.slice(outputCursor, match.index));
    const restored = restoredMarkers[index++];
    if (typeof restored !== 'string' || !restored || restored === match[0])
      throw Error('Missing or unknown issued marker');
    const start = sourceCursor;
    consume(restored);
    masks.push({ start, end: sourceCursor });
    outputCursor = match.index + match[0].length;
  }
  consume(transformed.slice(outputCursor));
  if (sourceCursor !== source.length || index !== restoredMarkers.length)
    throw Error('Incomplete restoration coverage');
  return masks;
}

export function markers(transformed) {
  return [...transformed.matchAll(markerPattern)].map(match => match[0]);
}

export function coverage(gold, masks) {
  const result = { fully_masked: 0, partially_masked: 0, unmasked: 0, per_type: {} };
  for (const entity of gold) {
    const intervals = masks.filter(m => m.start < entity.end && entity.start < m.end)
      .map(m => [Math.max(entity.start, m.start), Math.min(entity.end, m.end)]).sort((a, b) => a[0] - b[0]);
    let covered = 0, cursor = entity.start;
    for (const [start, end] of intervals) {
      covered += Math.max(0, end - Math.max(cursor, start));
      cursor = Math.max(cursor, end);
    }
    const status = covered === entity.end - entity.start ? 'fully_masked' : covered ? 'partially_masked' : 'unmasked';
    result[status]++;
    const bucket = result.per_type[entity.kind] ||= { fully_masked: 0, partially_masked: 0, unmasked: 0 };
    bucket[status]++;
  }
  return result;
}
