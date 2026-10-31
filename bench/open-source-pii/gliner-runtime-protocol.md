# GLiNER published quantized runtime comparison

Keep the original eight labels, 0.3 threshold, flat nonoverlapping decoding and
tokenizer from `knowledgator/gliner-pii-base-v1.0` revision
`61726e0ad791dcab3e29339bbec3ad42ded65641`. Compare its published QUInt8 ONNX
graph against the already evaluated FP32 graph. The publisher's quantized file
is 196,757,174 bytes, SHA-256
`0514c8fd86d0513ce5351a3267f132b57d5bcd8f99a90d43cde1228092881d19`.
Do not tune labels or thresholds on these comparisons.

First evaluate the existing 100- and 90-excerpt synthetic diagnostics. They are
reused runtime comparisons, not new blind validation. Keep original-byte spans,
complete-response abstention on invalid offsets and offline inference. Measure
prediction-set differences, standalone quality and frozen-core-first additive
quality separately. If the runtime is viable, evaluate the same public test
projection without changing its labels. A smaller graph does not overturn the
original candidate's mixed-language and false-positive limitations.

Review `Knowledgator/GLiNER.js` at
`cdb10b122afb45fdb29053d74503018e46b4bb9d` for browser feasibility. Its repository
LICENSE is Apache-2.0 while package.json declares MIT; record that mismatch and
do not silently assert a single package licence. Any prototype must use local
assets, block external requests, avoid input-word caches, preserve original
UTF-8 offsets, and measure parity instead of assuming the JS implementation
matches Python. Keep any prototype under `bench/`; no default dmz wiring.
