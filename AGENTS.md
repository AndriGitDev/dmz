# dmz repository instructions

Read `docs/adr/0006-chromium-name-and-public-test-download.md`, `docs/community-project.md`, `docs/adr/0005-free-community-project.md`, `docs/plan-coverage.md`, `docs/product-spec.md`, `docs/roadmap.md` and `docs/open-decisions.md` before substantive product work. The active direction is free community DMZ with optional separate Kastro help; Portal/Gateway are deferred and source licensing is undecided. The 27 September 2026 business plan is a historical hypothesis baseline; distinguish captured requirements from shipped features and independently verified facts.

- Never commit real customer prompts, identifiers, secrets, logs, licensed BÍN/Mannanafnaskrá data or credentials. Benchmark data is synthetic; generated test kennitölur use century digit `8`.
- Keep DMZ values and mappings on device. For Portal/Gateway, follow ADR 0004: initial hosting is Hetzner Germany/Finland; masking keeps originals and mappings within the approved DMZ boundary, while explicitly approved PII processing may send permitted originals to a verified provider route. Block unapproved routes and mode changes; never silently relax retention or geography. Do not claim anonymity or 100% detection.
- Do not add ChatGPT/Claude login pass-through or subscription-token proxying. Do not claim provider ZDR without an applicable signed agreement and tested route.
- Prompt and response bodies do not belong in gateway logs, traces, audit events or durable storage. Audit is metadata only.
- A release cannot lower recall for any entity type on the synthetic public benchmark. Gate 2 targets are 99% kennitala and 95% name recall; they are not current results.
- Legal documents, ISK prices, provider retention and certification claims require evidence/approval listed in `docs/open-decisions.md` before public publication.

The public marketing site is a separate repository at `/root/work/dmz-web` (`AndriGitDev/dmz-web`).

The product is named **DMZ Chromium extension**. The owner authorized publication of the current compiled trained alpha ZIP on dmz.is; this includes its bundled local scanner but no raw corpora, standalone model downloads or private research. Follow ADR 0006 for build identity, unchanged detector assets, notices and checksum evidence.
