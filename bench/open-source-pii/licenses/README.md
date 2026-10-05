# Research reference implementation notices

The browser experiments reconstruct local model graph/tokenizer interfaces and
retain these notices alongside the interoperability work:

- `lmoe-MIT.txt`: identical upstream MIT notice for
  [lmoe/gliner2-onnx at 35655a6](https://github.com/lmoe/gliner2-onnx/tree/35655a6b18fa24e81584645415a1733e9d47e321)
  and [lmoe/gliner-onnx.js at c1f2937](https://github.com/lmoe/gliner-onnx.js/tree/c1f2937da282ab38dd61c889d1b0d54cb03c23c7).
- `Knowledgator-GLiNER-js-Apache-2.0.txt`: the upstream LICENSE at
  [Knowledgator/GLiNER.js cdb10b1](https://github.com/Knowledgator/GLiNER.js/tree/cdb10b122afb45fdb29053d74503018e46b4bb9d).
  That revision's package.json says MIT, while its LICENSE is Apache-2.0. The
  mismatch is retained in the research record; the package is not imported into
  Shield.

The DMZ browser adapters add original UTF-8 offset validation, bounded inputs,
no input-word cache, local-only assets and explicit failure handling. They are
research code, not enabled product detectors. Model weights and exported graphs
remain outside Git and retain their separate upstream model terms. ORT Web and
Hugging Face tokenizer dependencies also retain their own notices.

Product phone/vendor-rule notices are separately bundled under
`apps/shield/public/licenses`; these research files do not replace them.
