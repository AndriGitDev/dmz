import { Tokenizer, wordsFromText, crfPredict, spans, mergeNames } from './text.mjs';
import * as ort from './runtime/ort.wasm.min.mjs';
let session,
  tokenizer,
  crf,
  busy = false;
async function checked(name, manifest) {
  const response = await fetch(new URL(name, import.meta.url));
  if (!response.ok) throw Error('Asset unavailable');
  const buffer = await response.arrayBuffer();
  const digest = Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', buffer)), (b) =>
    b.toString(16).padStart(2, '0'),
  ).join('');
  if (buffer.byteLength !== manifest.files[name].bytes || digest !== manifest.files[name].sha256)
    throw Error('Asset integrity failure');
  return buffer;
}
async function load() {
  if (session) return;
  const response = await fetch(new URL('manifest.json', import.meta.url));
  if (!response.ok) throw Error('Manifest unavailable');
  const manifest = await response.json();
  if (
    manifest.schema !== 1 ||
    manifest.model !== 'fp16storage' ||
    manifest.thresholds.crf !== 0.75 ||
    manifest.thresholds.neural !== 0
  )
    throw Error('Unexpected model');
  const decode = (buffer) => JSON.parse(new TextDecoder().decode(buffer));
  tokenizer = new Tokenizer(decode(await checked('tokenizer.json', manifest)));
  crf = decode(await checked('crf.json', manifest));
  ort.env.wasm.numThreads = 1;
  ort.env.wasm.wasmPaths = new URL('runtime/', import.meta.url).href;
  ort.env.logLevel = 'error';
  session = await ort.InferenceSession.create(await checked('model.onnx', manifest), {
    executionProviders: ['wasm'],
    graphOptimizationLevel: 'all',
  });
}
self.onmessage = async ({ data }) => {
  const { id, text } = data;
  if (busy) {
    self.postMessage({ id, error: 'Scanner busy' });
    return;
  }
  busy = true;
  try {
    const { words, offsets } = wordsFromText(text);
    await load();
    const primary = crfPredict(words, crf),
      labels = [];
    if (words.length)
      for (const chunk of tokenizer.chunks(words)) {
        const feeds = {
          input_ids: new ort.Tensor('int64', BigInt64Array.from(chunk.ids, BigInt), [
            1,
            chunk.ids.length,
          ]),
          attention_mask: new ort.Tensor(
            'int64',
            BigInt64Array.from(chunk.ids, () => 1n),
            [1, chunk.ids.length],
          ),
          pooling: new ort.Tensor('float32', chunk.pooling, [1, chunk.words, chunk.ids.length]),
        };
        let output;
        try {
          output = await session.run(feeds);
          const values = output.logits.data;
          if (values.length !== chunk.words * 3 || !Array.from(values).every(Number.isFinite))
            throw Error('Invalid model result');
          for (let i = 0; i < values.length; i += 3) {
            const row = Array.from(values.slice(i, i + 3));
            labels.push(['O', 'B', 'I'][row.indexOf(Math.max(...row))]);
          }
        } finally {
          Object.values(feeds).forEach((t) => t.dispose());
          if (output) Object.values(output).forEach((t) => t.dispose());
        }
      }
    if (labels.length !== words.length) throw Error('Incomplete scan');
    self.postMessage({
      id,
      spans: mergeNames(spans(primary.labels, offsets, primary.confidence), spans(labels, offsets)),
    });
  } catch {
    self.postMessage({ id, error: 'Local scanner failed' });
  } finally {
    busy = false;
  }
};
