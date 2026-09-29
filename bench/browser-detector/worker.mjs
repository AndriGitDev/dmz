import * as ort from '/runtime/ort.wasm.min.mjs';
let session;
let busy = false;
self.onmessage = async ({ data }) => {
  const { id, type } = data;
  if (busy) { self.postMessage({ id, error: 'Worker already busy' }); return; }
  busy = true;
  try {
    if (type === 'load') {
      if (session) throw new Error('Already loaded');
      ort.env.wasm.numThreads = data.threads;
      ort.env.wasm.wasmPaths = new URL('/runtime/', self.location).href;
      const start = performance.now();
      session = await ort.InferenceSession.create(data.url, { executionProviders: ['wasm'], graphOptimizationLevel: 'all' });
      self.postMessage({ id, result: { loadMs: performance.now() - start, isolated: self.crossOriginIsolated } });
    } else if (type === 'infer') {
      if (!session) throw new Error('Not ready');
      const start = performance.now();
      const outputs = [];
      for (const chunk of data.chunks) {
        const feeds = Object.fromEntries(Object.entries(chunk.feeds).map(([key, value]) => [key,
          key === 'pooling' ? new ort.Tensor('float32', Float32Array.from(value.data), value.shape)
            : new ort.Tensor('int64', BigInt64Array.from(value.data, BigInt), value.shape)]));
        let result;
        try {
          result = await session.run(feeds);
          outputs.push(Array.from(result.logits.data));
        } finally {
          if (result) Object.values(result).forEach(tensor => tensor.dispose());
          Object.values(feeds).forEach(tensor => tensor.dispose());
        }
      }
      self.postMessage({ id, result: { inferenceMs: performance.now() - start, outputs } });
    } else throw new Error('Unknown operation');
  } catch (error) {
    self.postMessage({ id, error: String(error.message ?? error) });
  } finally { busy = false; }
};
