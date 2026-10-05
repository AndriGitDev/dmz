# GLiNER2 local runtime feasibility

This is a research adapter under `bench/`, not a Shield feature or new remote
service. Original model, FP32 export, quantized variants and caches stay outside
Git. The fixed comparison is original seven labels at 0.5, with address-only
additions selected before the fresh 90-excerpt evaluation.

The reviewed community Python export/runtime is
[`lmoe/gliner2-onnx`](https://github.com/lmoe/gliner2-onnx), revision
`35655a6b18fa24e81584645415a1733e9d47e321` (MIT). Its FP32 encoder,
span representation and CountLSTM counting graph were exported offline from the
pinned Fastino model. The classifier graph is unused for NER. The community
Node implementation was also reviewed at
`lmoe/gliner-onnx.js@c1f2937da282ab38dd61c889d1b0d54cb03c23c7` (MIT), but
its filesystem/download dependencies and persistent cache of input words are
unsuitable for direct reuse in Shield.

The browser prototype uses ORT Web 1.23.2 and local assets only. It does not cache
input words, log prompts, download models from a hub or call an inference API.
Public label strings and model weights may remain loaded; prompt strings and
intermediate arrays are request-local. This does not promise physical memory
zeroization by JavaScript or the inference runtime.

Tokenization uses pinned `@huggingface/tokenizers@0.2.0` (Apache-2.0) with the
export's original Unigram tokenizer. A parity probe found 666/667 exact word-ID
matches; the exception was U+FEFF, which the JS tokenizer dropped. The prototype
rejects unsupported format/control characters before tokenization and rejects
any unexpectedly empty tokenization. It matches against the original string,
then lowercases token values, as GLiNER2 2.0.0 does; the community Python runtime
instead lowercases the whole string first. Thus the unmodified Python runtime
is a parity diagnostic, not an assumed authority for Unicode offsets.

Bounds are fixed before browser inference: 4,096 original UTF-8 bytes, 512 total
subword/schema tokens, eight-word candidate spans, at most 512 returned entities
and one request at a time. No truncation or untested long-document chunking.
Return UTF-8 byte spans against original text; reject malformed, surrogate-split
or combining-sequence-split offsets for the entire candidate. Model failure
must preserve the frozen core/name result in any eventual additive integration.

The browser computes span-label dot products in JavaScript before sigmoid,
whereas Python uses NumPy float32 einsum. Measure prediction and score differences;
do not assume bitwise floating-point parity. Quantization and runtime changes
must retain their own quality/runtime reports. Neither a successful export nor
a smaller file establishes readiness for a default detector.
