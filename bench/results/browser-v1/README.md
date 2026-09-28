# Frozen detector: browser deployment benchmark

Private research evidence, 28 September 2026. No model was retrained, no threshold
was retuned, and no production Shield code changed. The unreviewed public-document
pilot was not scored. Reproduction: [harness](../../browser-detector/README.md).

**Result:** the full precision export preserves native development predictions;
the smaller int8 graph loses recall in both native and Chromium execution. Do not
promote this int8 configuration into Shield.

## Accuracy

Complete fixed development sets; exact Person spans after the frozen CRF-first
merge. General: 5,582 sentences / 1,506 names. Legal/financial synthetic development:
256 documents / 480 names. **These are development results, not independent field
validation.** Previously reused development data and sentence-level splitting
remain limitations; encoder pretraining overlap is not excluded.

| Runtime / export / batch | General precision | General recall | Domain precision | Domain recall |
| --- | ---: | ---: | ---: | ---: |
| Frozen reference | 95.92% | 95.29% | 98.51% | 96.25% |
| Native FP32 / 8 | 95.92% | 95.29% | 98.51% | 96.25% |
| Native int8 / 8 | 95.63% | 94.56% | 97.44% | 95.00% |
| Native int8 / 1 | 95.497% | 94.36% | 97.44% | 95.00% |
| Chromium int8 / 1 | 96.15% | 94.49% | 97.64% | 94.79% |

Native FP32 has **zero changed neural or combined prediction records** on either set.
This validates this export against the frozen cached reference, including mean
subword pooling and the float16 round-trip before the head.

Native int8 adds 11 false negatives / 4 false positives on general development at batch
8; at batch 1 it adds **14 false negatives / 6 false positives**. Both batch sizes
add 6 false negatives / 5 false positives on domain development. The exact changed
records differ by batch, as dynamic activation quantization is input dependent.
Batch-1 general precision is below 95.5% despite rounding to 95.50% at two decimals.
The frozen reference itself still misses the dual 95.5% general development target.

**Reject this int8 configuration for automatic promotion.** Its smaller file and
speed cannot compensate for the measured accuracy regression. This is a result
for this configuration, not a claim that all quantization must fail.

## Artifacts

| Export | Bytes | MiB |
| --- | ---: | ---: |
| FP32 ONNX | 496,010,421 | 473.03 |
| Int8 ONNX | 125,119,018 | 119.32 |

Int8 reduces the ONNX file by 74.77%. Both graphs include encoder, word pooling
and the classification head. The original pretrained checkpoint also contains
unused masked-language-model tensors, so compare the two exported files directly.
Weights, tokenizers and per-record predictions remain outside Git.

Per-run exact counts, timings, model/data hashes and runtime versions are in
`fp32-general.json`, `fp32-domain.json`, `int8-general.json`, `int8-domain.json`,
`int8-general-batch1.json`, `int8-domain-batch1.json` and `export.json`.

## Chromium runtime measurements

Headless Chromium 153.0.8010.12, ONNX Runtime Web 1.23.2, WASM CPU on a
4-vCPU Xeon Skylake VPS with about 7.6 GiB RAM. See `hardware.json`. Five warm
repeats per size; medians below are observations, not p95 or desktop guarantees.

| Model / WASM threads | Local session load | 32 words | 128 words | 512 words | Peak tree RSS |
| --- | ---: | ---: | ---: | ---: | ---: |
| fp32 / 1 | 6.57 s | 0.57 s | 1.94 s | 7.62 s | 2.04 GiB |
| fp32 / 3 | 6.39 s | 0.22 s | 0.77 s | 2.89 s | 2.04 GiB |
| int8 / 1 | 2.62 s | 0.29 s | 1.03 s | 3.94 s | 0.88 GiB |
| int8 / 3 | 2.05 s | 0.12 s | 0.42 s | 1.54 s | 0.90 GiB |

The FP32/3 measurement is in `chromium-fp32-threads3.json`, collected after
corpus replay finished, with no concurrent detector workload. Its two additional
configuration-specific fault checks also passed, as did the repeated three
general fault checks (15 successful check executions across both timing runs).

The 512-word fixture is processed in four chunks; it is not silently truncated.
The 32-word first run is the first inference in each fresh worker; first runs at
longer sizes follow earlier cases in that worker. Load includes local HTTP model
transfer, WASM initialization and session construction; it is not a WAN download
or fresh OS-cache cold start. Timings include worker input/output transfer but
exclude browser tokenization, CRF, masking and Shield UI/send interception.

RSS is the sum across the Chromium process tree sampled every 200 ms, so shared
pages can be counted multiple times. Baselines were about 0.42–0.43 GiB. This is
not unique physical memory, and brief peaks may be missed.

Three simultaneous independent int8 workers (one thread each) loaded in 3.83 s
wall time and completed concurrent 512-word requests in 4.61–5.07 s each. Tree RSS
was 1.77 GiB after loading and peaked at 1.98 GiB, versus a 0.62 GiB baseline.
Duplicating the model in every tab is therefore a material memory cost; evaluate
a shared inference lifetime before choosing the extension architecture.

All ten initial fault checks passed: invalid input and deadline termination in
three runtime configurations, pending-request rejection on worker termination,
inference before readiness, corrupt model and deliberate worker crash. No
external page/worker requests were observed; routing allowed loopback only.
These checks cover the harness, not production Shield's interception or low-RAM
OS process killing. No hosted inference fallback exists.

## Native/browser discrepancy

Fabricated vectors retained identical word labels on every run, but int8 absolute
logit differences versus native ORT reached **5.0743**. FP32 differences stayed
below 0.00066. Matching a few simple final labels was insufficient to transfer
the native accuracy result to WASM, so the same two complete development sets
were replayed in Chromium at batch 1 with three WASM threads.

ONNX Runtime documents possible U8S8 saturation on AVX2/non-VNNI processors and
reduced-range or U8U8 alternatives. That is a plausible explanation on this
Skylake host, **not an isolated root cause** of our discrepancy. No revised
quantization or threshold search was performed in this benchmark.
[Official quantization documentation](https://onnxruntime.ai/docs/performance/model-optimizations/quantization.html).

### Complete Chromium development replay

`chromium-accuracy.json` records all 5,582 general sentences and all 256 domain
documents through the int8 graph in WASM (batch 1, three threads). Tokenization
was performed locally beforehand; the exact frozen CRF merge was scored offline.
This checks the complete datasets through the browser neural runtime, **not** a
complete browser tokenizer/CRF/Shield pipeline. No external requests occurred.

| Set | Browser combined TP / FP / FN | Precision | Recall | Change in missed names vs frozen |
| --- | --- | ---: | ---: | ---: |
| General | 1,423 / 57 / 83 | 96.15% | 94.49% | +12 net |
| Legal/financial development | 455 / 11 / 25 | 97.64% | 94.79% | +7 |

General: 14 previously correct names became misses, while two old misses were
recovered. Domain: seven previously correct names became misses, with none
recovered. Browser and native batch-1 combined spans differ in 20 general and
10 domain records. The full replay confirms that the numerical discrepancy is
material, and confirms the recall regression in the actual target runtime.
It took 486.66 s and 49.80 s respectively; these replay times include automation
and should not replace the dedicated synthetic latency measurements above.

## Decision and next work

- **FP32 is an accurate export reference:** complete native development span
  parity, plus fabricated browser label parity. Its 473 MiB download and large
  browser memory footprint are substantial costs. Full browser corpus parity
  for FP32 has not been established.
- **Reject this int8 configuration for Shield:** its 119 MiB file and lower
  latency do not meet the frozen reference's recall in the browser. The user's
  dual 95.5% objective is not met. Preserve the failed evidence; do not retune a
  threshold to disguise the loss.
- **Owner decision after this benchmark: train for full precision quality first.**
  Use 95.5% precision and recall as the existing minimum, pursue a stronger stable
  reference, and keep development gains distinct from independent validation.
  Pause further compression experiments until that reference is selected. Then
  measure compression losses against the exact frozen checkpoint; do not trade
  away detection quality merely to fit the browser.
- Before any production integration: implement faithful local tokenization and
  CRF inference with original-text offsets, evaluate a shared inference lifetime,
  run complete send/interception failure gates, and measure the owner's Brave
  hardware. The current harness does not prove these parts.
- Fresh public legal/financial validation still needs reviewed gold annotations.
  No unreviewed pilot document became a claimed independent test result here.

All implementation and detailed evidence stay in the private product repository.
The legacy public-artifact sync command now fails before copying anything, in
line with the existing disclosure policy. No website or production Shield change
is included in this benchmark.
