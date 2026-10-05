// Research-only local GLiNER2 NER adapter. No product import, downloads or word cache.
// Graph interface reviewed against lmoe/gliner2-onnx at 35655a6b18fa24e81584645415a1733e9d47e321
// and Fastino GLiNER2 2.0.0. See runtime-feasibility.md for differences and limits.
export const labels = ['person', 'address', 'phone_number', 'email', 'iban', 'payment_card', 'secret'];
const kinds = ['name_demo', 'address', 'phone', 'email', 'iban', 'card', 'secret'];
const wordPattern = /(?:https?:\/\/[^\s]+|www\.[^\s]+)|[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}|@[a-z0-9_]+|[\p{L}\p{N}_]+(?:[-_][\p{L}\p{N}_]+)*|\S/giu;
const bytes = new TextEncoder();

export function prepare(text, tokenizer, special) {
  if (typeof text !== 'string' || !text.isWellFormed() || bytes.encode(text).length > 4096 || /[\p{Cf}\p{Cc}]/u.test(text.replace(/[\r\n\t]/g, ''))) throw Error('unsupported_input');
  const encode = word => {
    const ids = tokenizer.encode(word, { add_special_tokens: false }).ids;
    if (!Array.isArray(ids) || !ids.length || ids.some(n => !Number.isSafeInteger(n) || n < 0)) throw Error('unsupported_tokenization');
    return ids;
  };
  const open = encode('('), close = encode(')');
  const tokens = [...open, special['[P]'], ...encode('entities'), ...open];
  const labelPositions = [];
  for (const label of labels) {
    labelPositions.push(tokens.length);
    tokens.push(special['[E]'], ...encode(label));
  }
  tokens.push(...close, ...close, special['[SEP_TEXT]']);
  const textStart = tokens.length, offsets = [], first = [];
  // Preserve offsets in original UTF-16; lower-case each token value only.
  for (const match of text.matchAll(wordPattern)) {
    offsets.push([match.index, match.index + match[0].length]);
    first.push(tokens.length - textStart);
    tokens.push(...encode(match[0].toLowerCase()));
  }
  if (tokens.length > 512) throw Error('unsupported_input');
  return { tokens, textStart, offsets, first, labelPositions };
}

function byteSpan(text, start, end) {
  if (!Number.isInteger(start) || !Number.isInteger(end) || start < 0 || end <= start || end > text.length || !text.slice(0, start).isWellFormed() || !text.slice(start, end).isWellFormed()) throw Error('invalid_offsets');
  // A candidate may abstain; it may not split a combining sequence or clip it.
  if (/^\p{M}/u.test(text.slice(start)) || /^\p{M}/u.test(text.slice(end))) throw Error('invalid_offsets');
  return [bytes.encode(text.slice(0, start)).length, bytes.encode(text.slice(0, end)).length];
}

export async function createRuntime(ort, tokenizer, config, paths) {
  if (config.max_width !== 8) throw Error('unexpected_model_config');
  for (const token of ['[P]', '[E]', '[SEP_TEXT]']) if (!Number.isSafeInteger(config.special_tokens[token])) throw Error('unexpected_model_config');
  const sessions = {};
  try {
    // Sequential loading avoids simultaneous model-download/initialization peaks.
    for (const name of ['encoder', 'span_rep', 'count_embed']) {
      sessions[name] = await ort.InferenceSession.create(paths[name], { executionProviders: ['wasm'], graphOptimizationLevel: 'all' });
    }
  } catch (error) {
    for (const session of Object.values(sessions)) await session.release();
    throw error;
  }
  const tensor64 = (values, shape) => new ort.Tensor('int64', BigInt64Array.from(values, BigInt), shape);
  const run = async (name, input) => {
    const output = await sessions[name].run(input);
    const values = Object.values(output);
    if (values.length !== 1 || values[0].type !== 'float32') throw Error('unexpected_model_output');
    return values[0];
  };
  let busy = false, closed = false;
  return {
    async predict(text) {
      if (closed || busy) throw Error('runtime_unavailable');
      busy = true;
      try {
        const p = prepare(text, tokenizer, config.special_tokens);
        if (!p.offsets.length) return [];
        const encoded = await run('encoder', {
          input_ids: tensor64(p.tokens, [1, p.tokens.length]),
          attention_mask: tensor64(p.tokens.map(() => 1), [1, p.tokens.length]),
        });
        if (encoded.dims.length !== 3 || encoded.dims[0] !== 1 || encoded.dims[1] !== p.tokens.length || encoded.dims[2] !== 768) throw Error('unexpected_model_shape');
        const hidden = 768, textCount = p.tokens.length - p.textStart;
        const labelVectors = new Float32Array(labels.length * hidden);
        p.labelPositions.forEach((position, i) => labelVectors.set(encoded.data.subarray(position * hidden, (position + 1) * hidden), i * hidden));
        const starts = [], ends = [], wordSpans = [];
        for (let start = 0; start < p.offsets.length; start++) {
          for (let width = 0; width < Math.min(8, p.offsets.length - start); width++) {
            starts.push(p.first[start]); ends.push(p.first[start + width]); wordSpans.push([start, start + width]);
          }
        }
        const represented = await run('span_rep', {
          hidden_states: new ort.Tensor('float32', encoded.data.slice(p.textStart * hidden), [1, textCount, hidden]),
          span_start_idx: tensor64(starts, [1, starts.length]), span_end_idx: tensor64(ends, [1, ends.length]),
        });
        const counted = await run('count_embed', { label_embeddings: new ort.Tensor('float32', labelVectors, [labels.length, hidden]) });
        if (represented.data.length !== starts.length * hidden || counted.data.length !== labels.length * hidden) throw Error('unexpected_model_shape');
        const found = [];
        for (let span = 0; span < starts.length; span++) {
          for (let label = 0; label < labels.length; label++) {
            let dot = 0;
            for (let k = 0; k < hidden; k++) dot += represented.data[span * hidden + k] * counted.data[label * hidden + k];
            const confidence = 1 / (1 + Math.exp(-dot));
            if (!Number.isFinite(confidence)) throw Error('invalid_score');
            if (confidence >= .5) {
              const [first, last] = wordSpans[span];
              found.push({ span: [p.offsets[first][0], p.offsets[last][1], kinds[label]], score: confidence });
            }
          }
        }
        found.sort((a, b) => b.score - a.score);
        const result = [];
        for (const item of found) {
          if (!result.some(old => old.span[2] === item.span[2] && old.span[0] < item.span[1] && item.span[0] < old.span[1])) result.push(item);
        }
        if (result.length > 512) throw Error('too_many_spans');
        return result.map(item => ({ span: [...byteSpan(text, item.span[0], item.span[1]), item.span[2]], score: item.score }));
      } finally { busy = false; }
    },
    async close() {
      if (busy) throw Error('runtime_busy');
      if (closed) return;
      closed = true;
      for (const session of Object.values(sessions)) await session.release();
    },
  };
}
