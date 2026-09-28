# Shield settings and detector runtime assessment

## Settings implementation

The toolbar now opens a settings menu instead of the manual masking demo:

- Charcoal/red dmz branding, with light, dark and system appearance.
- Live status from the active ChatGPT/Claude tab; unsupported pages do not show
  an active-protection badge.
- Confirmed reset of that tab's mapping and restored displays. Switching tabs
  cancels confirmation, and the active tab is checked again before reset.
- Fixed protection behavior explained without a switch that disables interception.
- Only the menu appearance is saved. No prompt, original value, mapping or separate
  detector instance lives in the settings page.
- Clipboard-write permission removed; toolbar action explicitly declared.
- The legacy panel-only manual dictionary is removed along with the demo. It was
  never applied to automatic chat interception. No replacement dictionary is claimed.

Internal status/reset messages are accepted only from this extension's exact
settings page. Status contains readiness, composer availability and site only.
Existing content-script reset logic is shared with the settings control.

Verification: TypeScript check, production WXT build, settings browser checks
(theme persistence/system changes, status/reset on both sites, cancelled reset on
switching tabs), existing chat smoke and network-body/race/broken-core suites.
Screenshots inspected at 380px width in both themes. These are synthetic Chromium
fixtures; owner Brave checks on authenticated sites remain separate.

## Research model runtime

Reproduce with `bench/contextual/runtime_probe.py` using the local IceBERT base
and selected epoch-12 head. Aggregate evidence:
[`python-cpu.json`](../bench/results/runtime-v1/python-cpu.json).

| Measurement | Result |
| --- | --- |
| Base safetensors artifact | 651,803,232 bytes / 621.6 MiB |
| Loaded encoder parameters | 123,851,520 |
| Imports + local loading | 8.16 s |
| Peak Python process RSS | 660.6 MiB |
| 32 words, warm median of 5 | 145 ms |
| 128 words, warm median of 5 | 260 ms |
| 512 words, warm median of 5 | 1.013 s |

Three CPU threads, one local Linux host, fabricated repeated text. The timing
includes tokenization, encoder inference, mean subword pooling, float16 round-trip
and BIO head. It excludes CRF inference, browser startup, DOM work and export /
quantization overhead. The first inference is also recorded. This is a feasibility
measurement, not a browser latency promise or accuracy evaluation. The artifact
contains more tensors than the encoder loads; artifact bytes and loaded parameter
count describe different quantities.

## Deployment decision

Do not bundle the full research checkpoint into every tab. First export a local
quantized inference candidate and check exact-span parity on development data,
then measure Chromium CPU/WASM (and optional WebGPU) startup, memory and latency.
Use one extension-owned worker if the eventual runtime supports it; retain
per-tab mapping isolation. Quantization may change predictions, so the frozen
research candidate and the shipping candidate need separate hashes/evaluations.
If memory or latency is unsuitable, distill a smaller local model using only
approved training data. Do not send Shield prompts to a hosted model as fallback.
No contextual neural model was added to the shipped extension in this change.

Independent-data preparation is recorded in [the holdout pilot](../bench/holdout/README.md).
The public-data pilot remains unlabelled; no new precision/recall claim is made.
