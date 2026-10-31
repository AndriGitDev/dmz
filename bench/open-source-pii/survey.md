# Open-source PII integration survey

Checked against upstream sources on 5 October 2026. Model-card scores are not
comparable to DMZ's exact typed excerpt-set metric. The initial probe is only
100 authored synthetic excerpts and cannot establish independent field quality.
Pinned model/file identities and package metadata are retained privately in the
session's `sources/` and `models/` directories.

| Candidate | Upstream terms observed | Fit and decision at initial review |
| --- | --- | --- |
| [Presidio](https://github.com/data-privacy-stack/presidio) | MIT code; dependencies/models have separate terms | Extensible Python recognizer framework, existing pinned English baseline. Appropriate local server/research comparator. It is not a drop-in Go/TinyGo or browser runtime. Evaluate its added value rather than replacing the Icelandic core. The old Microsoft URL redirects to Data Privacy Stack. |
| [Knowledgator GLiNER-PII base](https://huggingface.co/knowledgator/gliner-pii-base-v1.0) | Model card Apache-2.0; GLiNER code Apache-2.0 | Initial FP32 ONNX is 664,764,803 bytes; tokenizer ~8.6 MB. Existing ONNX/JavaScript path makes a local browser spike feasible. Published quantized variants need their own quality comparisons. |
| [Fastino GLiNER2-PII](https://huggingface.co/fastino/gliner2-privacy-filter-PII-multi) | Model card and [code](https://github.com/fastino-ai/GLiNER2) Apache-2.0 | Current multilingual model, 42 advertised labels, no Icelandic language claim. Safetensors ~1.23 GB. Evaluate CPU inference; direct browser export requires separate work. The initial `person` label also finds role words, so its ontology needs scrutiny. |
| [dslim BERT NER](https://huggingface.co/dslim/bert-base-NER) | Model card MIT | English Person control, not a complete PII detector. ~433 MB safetensors; supports ordinary token-classification export. Do not relabel LOC or ORG as addresses or people. |
| [external provider Privacy Filter](https://github.com/openai/privacy-filter) | Code and [model](https://huggingface.co/openai/privacy-filter) Apache-2.0 | Local CPU/runtime support, primarily English, eight privacy categories and a distinct span-decoding contract. Original checkpoint ~2.80 GB; memory and browser cost must be measured. Account numbers, dates and URLs do not map one-to-one to current DMZ types. |
| [Piiranha](https://huggingface.co/iiiorg/piiranha-v1-detect-personal-information) | Current card CC-BY-NC-ND-4.0 | Excluded from the commercial integration shortlist under these terms. Its token-level headline accuracy is not DMZ's acceptance metric. |
| [NVIDIA GLiNER-PII](https://huggingface.co/nvidia/gliner-PII) | NVIDIA Open Model License Agreement | Separate model terms; not an Apache substitute. Lower priority than permissively licensed candidates for this bounded session. |
| [scrubadub](https://github.com/LeapBeyond/scrubadub) | Inspect exact package/license before integration | Python detector/cleanup framework. Does not itself solve browser-compatible English contextual detection. Reviewed as an alternative framework, not assumed superior to Presidio. |
| [detect-secrets](https://github.com/Yelp/detect-secrets) | Inspect exact package/license before integration | Secret scanning for code; useful specialist comparison, not a Person/address model. Disable any verification/network path if evaluated. |
| [libphonenumber](https://github.com/google/libphonenumber) and [Go port](https://github.com/nyaruka/phonenumbers) | Google Apache-2.0; Go port MIT with upstream data/code notices | Strong candidate for validated international telephone coverage shared between server and WASM. Investigate build compatibility, complete spans and false positives before changing rules. Valid number shape does not prove assignment or ownership. |

## Initial diagnostic finding

Broad additive composition is not automatically an improvement. Presidio and both
GLiNER models add English detections, but also add false positives. The frozen
trained DMZ path gets 25/40 general-English, 27/36 workplace-English and 22/24 mixed
excerpts exactly right. Presidio added without replacing existing spans reaches
27/40 and 30/36 English, but falls to 15/24 mixed. GLiNER2 added the same way reaches
32/40, 26/36 and 21/24. Retain all candidates/results; none is accepted as a default
on this evidence. These counts use the same frozen labels and deterministic
non-overlapping composition; they are not production or independent scores.

A more suitable integration may preserve structured validators and Icelandic
names while using a carefully bounded supplemental model. Schema selection,
normalization and any confidence calibration are model-selection work and must
be recorded as such, followed by separate probes and regression guards.

## Integration findings after runtime and public-English checks

Two narrow rule integrations are implemented in the research branch: pinned
libphonenumber metadata for explicit international phone numbers, and 17 pinned
[Gitleaks](https://github.com/gitleaks/gitleaks) MIT rules covering 18 vendor-token
families. Both run locally in the existing Go/TinyGo core. The full Go phone
library's protobuf initialization failed in TinyGo, so the adapter imports only
public numbering metadata at build time. Gitleaks runtime, network verification,
archive decoding and repository scanning are not part of dmz.

The current TinyGo regex implementation needs a larger reserved stack for these
rules: the rules-only smoke test hung with the default 64 KiB stack. Equivalent
bounded PyPI matching, lazy allowlist checks and a larger stack fix that runtime
path. A 256 KiB trial passed; a subsequently tested 128 KiB setting preserves
the checked results with lower observed latency spikes and is selected for the
build. This is why native accuracy checks alone were insufficient.

On 4,996 public synthetic English excerpts, the original trained composition
scores 2,414 exact excerpts. GLiNER alone scores 3,098, but adding it while keeping
existing core/name matches scores only 2,268. Post-evaluation attribution finds
2,714 retained false core spans and 1,945 newly added false spans. Existing core
matches block 920 correct GLiNER spans; incorrect name/address overlap is a major
source. These counts explain a composition failure, not a validated replacement
policy. Public annotations and unknown training overlap limit all these results.

Both GLiNER and GLiNER2 have working offline browser research prototypes, but
quantization changes predictions and loses quality; their graphs also remain
large. Neither is enabled in dmz. A fixed confidence-only address selection
on 4,995 validation excerpts rejected every declared GLiNER QUInt8 threshold.
The selected GLiNER2 address-only path remains experimental because its fresh
90-excerpt diagnostic added six false spans along with 14 true spans.
On the complete public test set with the updated core, that address supplement
adds 505 true and 297 false spans, improving exact excerpts from 2,443 to 2,493.
The larger sample therefore retains the false-positive concern. Full GLiNER2
addition instead lowers exact excerpts to 2,014 despite increasing recall.

external provider Privacy Filter completed a fixed, outcome-independent 512-excerpt public
sample in about 126 minutes on the shared CPU host. Adding it to the updated
core raises recall from 53.69% to 71.38%, but lowers exact excerpts from 256 to
253 and raises false spans from 291 to 509. Its original checkpoint is about
2.799 GB and observed peak resident memory was roughly 4.1 GiB. The complete
same-sample comparison, rather than an incomplete larger attempt, is retained;
it does not justify a browser integration or default quality change.

[usaddress](https://github.com/datamade/usaddress), a small MIT-licensed US address
parser, was also evaluated as a specialist. Its complete public-test address
diagnostic found 128 true spans, 197 false spans and 1,301 missed spans. It does
not justify a default address integration or browser port on this evidence.

Detailed retained trials, runtime differences and aggregate provenance are in
[the result record](../results/open-source-pii-v1/README.md). Further English work
should evaluate overlap arbitration on separately annotated data, including
mixed-language regression guards; a broad additive model is not sufficient.
