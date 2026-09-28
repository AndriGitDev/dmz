# dmz repository instructions

Read `docs/plan-coverage.md`, `docs/product-spec.md`, `docs/roadmap.md` and `docs/open-decisions.md` before substantive product work. The 27 September 2026 business plan is a hypothesis baseline; distinguish captured requirements from shipped features and independently verified facts.

- Never commit real customer prompts, identifiers, secrets, logs, licensed BÍN/Mannanafnaskrá data or credentials. Benchmark data is synthetic; generated test kennitölur use century digit `8`.
- Keep real values and the stand-in mapping key on device for Shield, or on Icelandic RunAtlas infrastructure for Portal/Gateway. Model providers receive stand-ins. Do not claim anonymity or 100% detection.
- Do not add ChatGPT/Claude login pass-through or subscription-token proxying. Do not claim provider ZDR without an applicable signed agreement and tested route.
- Prompt and response bodies do not belong in gateway logs, traces, audit events or durable storage. Audit is metadata only.
- A release cannot lower recall for any entity type on the synthetic public benchmark. Gate 2 targets are 99% kennitala and 95% name recall; they are not current results.
- Legal documents, ISK prices, provider retention and certification claims require evidence/approval listed in `docs/open-decisions.md` before public publication.

The public marketing site is a separate repository at `/root/work/dmz-web` (`AndriGitDev/dmz-web`).
