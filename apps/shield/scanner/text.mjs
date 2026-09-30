// Device-local preprocessing. UTF-8 offsets are the Go/WASM boundary contract.
const encoder = new TextEncoder();
export function wordsFromText(text) {
  if (typeof text !== 'string' || text.length > 16000 || /[\p{M}\p{Cf}\uD800-\uDFFF]/u.test(text))
    throw Error('Unsupported text');
  const words = [],
    offsets = [];
  let cursor = 0,
    bytes = 0;
  const pattern = /[\p{L}\p{N}_]+(?:[-’'][\p{L}\p{N}_]+)*|[^\p{L}\p{N}_\s]/gu;
  for (const match of text.matchAll(pattern)) {
    bytes += encoder.encode(text.slice(cursor, match.index)).length;
    const end = bytes + encoder.encode(match[0]).length;
    words.push(match[0]);
    offsets.push([bytes, end]);
    cursor = match.index + match[0].length;
    bytes = end;
  }
  if (words.length > 2048) throw Error('Text too long');
  return { words, offsets };
}
export function spans(labels, offsets, confidence) {
  const found = [];
  let start = -1;
  const finish = (end) => {
    if (start >= 0 && (!confidence || confidence.slice(start, end).every((x) => x >= 0.75)))
      found.push({ start: offsets[start][0], end: offsets[end - 1][1] });
    start = -1;
  };
  labels.forEach((label, i) => {
    if (!['O', 'B', 'I'].includes(label)) throw Error('Invalid label');
    if (start >= 0 && label !== 'I') finish(i);
    if (label === 'B' || (label === 'I' && start < 0)) start = i;
  });
  finish(labels.length);
  return found;
}
export function mergeNames(primary, secondary) {
  return [
    ...primary,
    ...secondary.filter((b) => !primary.some((a) => a.start < b.end && b.start < a.end)),
  ].sort((a, b) => a.start - b.start);
}
// Python CRF feature parity: title/upper require at least one cased character.
const upper = (c) => /\p{Lu}|\p{Lt}/u.test(c),
  lower = (c) => /\p{Ll}/u.test(c);
const digit = (c) => /^[\p{Nd}²³¹⁰⁴⁵⁶⁷⁸⁹₀-₉]$/u.test(c);
const bool = (x) => (x ? 'True' : 'False');
export function features(words) {
  const basic = words.map((word, index) => {
    const chars = [...word],
      low = word.toLowerCase(),
      lc = [...low];
    let previousCased = false,
      title = true,
      cased = false;
    for (const c of chars) {
      if (upper(c)) {
        if (previousCased) title = false;
        cased = true;
        previousCased = true;
      } else if (lower(c)) {
        if (!previousCased) title = false;
        cased = true;
        previousCased = true;
      } else previousCased = false;
    }
    const single = chars.length === 1 && /^\p{L}$/u.test(word);
    const shape = chars
      .map((c) => (upper(c) ? 'X' : lower(c) ? 'x' : digit(c) ? 'd' : c))
      .join('')
      .replace(/(.)\1+/gu, '$1');
    return [
      'word=' + low,
      'shape=' + shape,
      'title=' + bool(title && cased),
      'upper=' + bool(cased && !chars.some(lower)),
      'digit=' + bool(chars.every(digit)),
      'patronymic=' + bool(/(sonar|syni|son|dóttir|dóttur|burs|bur)$/.test(low)),
      'single_letter=' + bool(single),
      'initial_before_dot=' + bool(single && words[index + 1] === '.'),
      'length=' + Math.min(chars.length, 6),
      'hyphen=' + bool(word.includes('-')),
      'has_digit=' + bool(chars.some(digit)),
      'case_pattern=' +
        chars
          .slice(0, 6)
          .map((c) => (upper(c) ? 'X' : lower(c) ? 'x' : '_'))
          .join(''),
      ...[1, 2, 3, 4, 5].map((n) => 'suffix' + n + '=' + lc.slice(-n).join('')),
      ...[1, 2, 3].map((n) => 'prefix' + n + '=' + lc.slice(0, n).join('')),
    ];
  });
  return words.map((_, i) => [
    'bias',
    ...[-2, -1, 0, 1, 2].flatMap((delta) => {
      const j = i + delta;
      return j < 0 || j >= words.length
        ? [delta + ':boundary']
        : (delta === 0 ? basic[j] : basic[j].slice(0, 12)).map((f) => delta + ':' + f);
    }),
  ]);
}
const logsum = (xs) => {
  const m = Math.max(...xs);
  return m + Math.log(xs.reduce((s, x) => s + Math.exp(x - m), 0));
};
export function crfPredict(words, model) {
  if (!words.length) return { labels: [], confidence: [] };
  const { labels, state, transitions } = model,
    k = labels.length;
  const emit = features(words).map((fs) => {
    const row = Array(k).fill(0);
    for (const f of fs) for (const [i, v] of state[f] || []) row[i] += v;
    return row;
  });
  const forward = [emit[0].slice()],
    best = [emit[0].slice()],
    back = [];
  for (let t = 1; t < words.length; t++) {
    const f = [],
      b = [],
      pointers = [];
    for (let j = 0; j < k; j++) {
      f[j] = emit[t][j] + logsum(forward[t - 1].map((v, i) => v + transitions[i][j]));
      const candidates = best[t - 1].map((v, i) => v + transitions[i][j]);
      const m = Math.max(...candidates);
      pointers[j] = candidates.indexOf(m);
      b[j] = emit[t][j] + m;
    }
    forward.push(f);
    best.push(b);
    back[t] = pointers;
  }
  const chosen = Array(words.length);
  chosen[words.length - 1] = best.at(-1).indexOf(Math.max(...best.at(-1)));
  for (let t = words.length - 1; t > 0; t--) chosen[t - 1] = back[t][chosen[t]];
  const backward = Array(words.length);
  backward[words.length - 1] = Array(k).fill(0);
  for (let t = words.length - 2; t >= 0; t--)
    backward[t] = labels.map((_, i) =>
      logsum(labels.map((_, j) => transitions[i][j] + emit[t + 1][j] + backward[t + 1][j])),
    );
  const z = logsum(forward.at(-1));
  return {
    labels: chosen.map((i) => labels[i]),
    confidence: chosen.map((j, t) => Math.exp(forward[t][j] + backward[t][j] - z)),
  };
}
export class Tokenizer {
  constructor(data) {
    if (
      data.normalizer !== null ||
      data.model.type !== 'BPE' ||
      data.pre_tokenizer.type !== 'ByteLevel'
    )
      throw Error('Unsupported tokenizer');
    this.vocab = data.model.vocab;
    this.ranks = new Map(
      data.model.merges.map((p, i) => [(Array.isArray(p) ? p : p.split(' ')).join('\0'), i]),
    );
    const bytes = [];
    for (let i = 33; i <= 126; i++) bytes.push(i);
    for (let i = 161; i <= 172; i++) bytes.push(i);
    for (let i = 174; i <= 255; i++) bytes.push(i);
    const chars = bytes.slice();
    let n = 0;
    for (let i = 0; i < 256; i++)
      if (!bytes.includes(i)) {
        bytes.push(i);
        chars.push(256 + n++);
      }
    this.byteChars = Object.fromEntries(bytes.map((b, i) => [b, String.fromCodePoint(chars[i])]));
    // No token cache: it would retain fragments of private prompts across calls.
  }
  encodeWord(word) {
    const result = [];
    for (const match of (' ' + word).matchAll(
      /'s|'t|'re|'ve|'m|'ll|'d| ?\p{L}+| ?\p{N}+| ?[^\s\p{L}\p{N}]+|\s+(?!\S)|\s+/gu,
    )) {
      let symbols = Array.from(encoder.encode(match[0]), (b) => this.byteChars[b]);
      while (symbols.length > 1) {
        let rank = Infinity,
          pair;
        for (let i = 0; i < symbols.length - 1; i++) {
          const p = symbols[i] + '\0' + symbols[i + 1],
            r = this.ranks.get(p);
          if (r !== undefined && r < rank) {
            rank = r;
            pair = p;
          }
        }
        if (pair === undefined) break;
        const merged = [];
        for (let i = 0; i < symbols.length; i++) {
          if (i + 1 < symbols.length && symbols[i] + '\0' + symbols[i + 1] === pair)
            merged.push(symbols[i] + symbols[++i]);
          else merged.push(symbols[i]);
        }
        symbols = merged;
      }
      for (const token of symbols) {
        const id = this.vocab[token];
        if (!Number.isInteger(id)) throw Error('Unknown subword');
        result.push(id);
      }
    }
    return result;
  }
  chunks(words) {
    const pieces = words.map((w) => this.encodeWord(w));
    if (pieces.some((p) => !p.length || p.length > 254)) throw Error('Word exceeds context');
    const out = [],
      pending = [[0, words.length]];
    while (pending.length) {
      const [a, b] = pending.pop();
      const size = 2 + pieces.slice(a, b).reduce((s, p) => s + p.length, 0);
      if (size > 256) {
        const middle = a + Math.floor((b - a) / 2);
        pending.push([middle, b], [a, middle]);
        continue;
      }
      const ids = [0, ...pieces.slice(a, b).flat(), 2],
        pooling = new Float32Array((b - a) * size);
      let pos = 1;
      for (let w = a; w < b; w++) {
        for (let j = 0; j < pieces[w].length; j++)
          pooling[(w - a) * size + pos++] = 1 / pieces[w].length;
      }
      out.push({ offset: a, words: b - a, ids, pooling });
    }
    return out;
  }
}
