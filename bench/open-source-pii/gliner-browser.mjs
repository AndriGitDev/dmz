// Research-only local adapter for the pinned GLiNER PII graph. No input-word cache.
export const labels = ['name', 'location address', 'phone number', 'email address', 'iban', 'credit card', 'password', 'api key'];
const kinds = ['name_demo', 'address', 'phone', 'email', 'iban', 'card', 'secret', 'secret'];
const encoder = new TextEncoder();
const wordsRE = /[\p{L}\p{N}_]+(?:[-_][\p{L}\p{N}_]+)*|\S/gu;

export function prepare(text, tokenizer) {
  if (typeof text !== 'string' || !text.isWellFormed() || encoder.encode(text).length > 4096 || /[\p{Cf}\p{Cc}]/u.test(text.replace(/[\r\n\t]/g, ''))) throw Error('unsupported_input');
  const offsets = [...text.matchAll(wordsRE)].map(m => [m.index, m.index + m[0].length]);
  const ids = [1], mask = [0];
  const append = (word, index) => {
    const tokens = tokenizer.encode(word, { add_special_tokens: false }).ids;
    if (!Array.isArray(tokens) || !tokens.length || tokens.some(n => !Number.isSafeInteger(n) || n < 0)) throw Error('unsupported_tokenization');
    ids.push(...tokens);
    mask.push(...tokens.map((_, i) => i === 0 ? index : 0));
  };
  for (const label of labels) { append('<<ENT>>', 0); append(label, 0); }
  append('<<SEP>>', 0);
  offsets.forEach(([s, e], i) => append(text.slice(s, e), i + 1));
  ids.push(2); mask.push(0);
  if (ids.length > 512 || offsets.length > 384) throw Error('unsupported_input');
  const spans = [], valid = [];
  offsets.forEach((_, start) => {
    for (let width = 0; width < 12; width++) {
      spans.push(start, start + width);
      valid.push(start + width < offsets.length ? 1 : 0);
    }
  });
  return { ids, mask, offsets, spans, valid };
}

function byteSpan(text, start, end) {
  if (start < 0 || end <= start || end > text.length || !text.slice(start, end).isWellFormed() || /^\p{M}/u.test(text.slice(start)) || /^\p{M}/u.test(text.slice(end))) throw Error('invalid_offsets');
  return [encoder.encode(text.slice(0, start)).length, encoder.encode(text.slice(0, end)).length];
}

export async function createRuntime(ort, tokenizer, config, path) {
  if (config.max_width !== 12 || config.class_token_index !== 128001 || config.ent_token !== '<<ENT>>' || config.sep_token !== '<<SEP>>') throw Error('unexpected_config');
  if (tokenizer.encode('<<ENT>>', { add_special_tokens: false }).ids.join() !== '128001' || tokenizer.encode('<<SEP>>', { add_special_tokens: false }).ids.join() !== '128002') throw Error('unexpected_special_tokens');
  const session = await ort.InferenceSession.create(path, { executionProviders: ['wasm'], graphOptimizationLevel: 'all' });
  const tensor = (values, dims) => new ort.Tensor('int64', BigInt64Array.from(values, BigInt), dims);
  let busy = false;
  return {
    async predict(text) {
      if (busy) throw Error('busy');
      busy = true;
      const inputs = {}; let outputs;
      try {
        const p = prepare(text, tokenizer), count = p.offsets.length;
        if (!count) return [];
        Object.assign(inputs, {
          input_ids: tensor(p.ids, [1, p.ids.length]),
          attention_mask: tensor(p.ids.map(() => 1), [1, p.ids.length]),
          words_mask: tensor(p.mask, [1, p.mask.length]),
          text_lengths: tensor([count], [1, 1]),
          span_idx: tensor(p.spans, [1, count * 12, 2]),
          span_mask: new ort.Tensor('bool', Uint8Array.from(p.valid), [1, count * 12]),
        });
        outputs = await session.run(inputs);
        const logits = outputs.logits;
        if (!logits || logits.type !== 'float32' || logits.dims.join() !== [1, count, 12, labels.length].join()) throw Error('unexpected_output');
        const candidates = [];
        for (let start = 0; start < count; start++) for (let width = 0; width < 12 && start + width < count; width++) for (let kind = 0; kind < labels.length; kind++) {
          const value = logits.data[(start * 12 + width) * labels.length + kind];
          if (!Number.isFinite(value)) throw Error('invalid_score');
          const score = 1 / (1 + Math.exp(-value));
          if (score > 0.3) candidates.push({ start, end: start + width, kind, score });
        }
        candidates.sort((a, b) => b.score - a.score);
        const selected = [];
        for (const span of candidates) if (!selected.some(old => span.start <= old.end && old.start <= span.end)) selected.push(span);
        if (selected.length > 512) throw Error('too_many_entities');
        selected.sort((a, b) => a.start - b.start);
        return selected.map(s => ({ span: [...byteSpan(text, p.offsets[s.start][0], p.offsets[s.end][1]), kinds[s.kind]], score: s.score }));
      } finally {
        for (const value of Object.values(inputs)) value.dispose();
        if (outputs) for (const value of Object.values(outputs)) value.dispose();
        busy = false;
      }
    },
    async dispose() { await session.release(); },
  };
}
