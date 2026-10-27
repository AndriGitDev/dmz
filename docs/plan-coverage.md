# Business-plan coverage audit

Status — 27 October 2026: historical commercial baseline. The [free community concept](community-project.md) supersedes commercial priorities, pricing and paid-pilot/revenue gates. Retained for provenance; not the active delivery plan or evidence of current availability.

Checked against the user-supplied 27 September 2026 plan. **Captured** means a requirement or assumption has a durable home in this repo. It does **not** mean shipped, verified, approved or legally effective.

| Plan section | Durable record | Implementation state |
| --- | --- | --- |
| Executive thesis and Gate 1 decision | [business plan](business-plan.md), [validation](validation.md) | Captured; interviews and commitments pending |
| Market size and demand signals | [business plan](business-plan.md), [sources](source-register.md) | Captured as hypotheses; current PxWeb count pending |
| ZDR gap and provider terms | [business plan](business-plan.md), [legal dependencies](legal/dependencies.md) | Captured; provider-specific verification pending |
| Product surfaces and data flow | [product spec](product-spec.md) | Manual Shield side panel and experimental ChatGPT/Claude content script built; live-site verification and Portal/Gateway pending |
| Detector scope and algorithm | [product spec](product-spec.md), policy schema, [benchmark](../bench/README.md) | Go/WASM detector and local trained name preview; device-local exact terms; 23,636 approved HMS address forms bundled. Shield masks uncertain findings under best-effort policy; strict core callers retain blockers. Reused synthetic and development results are tracked in the [quality contract](detector-quality-contract.md); independent ≥99% accuracy/recall evidence and licensed name data remain pending |
| Architecture, vault, routing and audit | [product spec](product-spec.md), [threat model](threat-model.md) | Captured; implementation and controls pending |
| Positioning, precise claims and proof | [website brief](website-brief.md) | Bilingual marketing site and preliminary synthetic results page built; market, legal and provider evidence pending |
| Competitive landscape | [business plan](business-plan.md), [sources](source-register.md) | Captured; current comparisons not verified |
| Pricing, segments and GTM | [business plan](business-plan.md), public pricing page | EUR assumptions captured; dated indicative ISK conversion shown publicly, approved ISK price book pending |
| Unit economics and financial model | [business plan](business-plan.md), [arithmetic check](financial-model.md) | Year-end ARR checked; quarterly model source missing |
| Roadmap, MRR and recall gates | [roadmap](roadmap.md), [product spec](product-spec.md) | Captured; execution pending |
| Organisation, grants and hiring | [business plan](business-plan.md), [open decisions](open-decisions.md) | Captured; entity/grant eligibility pending |
| Legal, data licences and ISO roadmap | [legal dependencies](legal/dependencies.md) | Captured; counsel/licence approvals pending |
| Risks and mitigations | [business plan](business-plan.md), [threat model](threat-model.md) | Captured; controls not proven |
| Interviews and kill criteria | [validation](validation.md), [launch checklist](launch-checklist.md) | Captured; interviews pending |
| Repo blueprint and CI | [product spec](product-spec.md), root README | Prototype core and Shield build implemented; Go benchmark and Chromium extension gates added; service stack pending |
| Website map, copy and build notes | [website brief](website-brief.md), public `dmz-web` | All 30 bilingual routes built; product and article pages clearly label planned features, and price conversion is indicative |

## Accepted architecture update

[ADR 0004](adr/0004-portal-hosting-and-pii-policy.md) supersedes the baseline’s universal Iceland-hosting and provider-masking assumptions for Portal/Gateway. Initial hosting is Hetzner EU; customers may use masking or explicitly approved original-PII processing on a verified route. The [preview plan](portal-gateway-plan.md) is captured, not implemented.

## What the first 10-minute pass missed

It created a working launch-site skeleton and directory scaffold, but omitted durable records for market evidence, exact financial assumptions, risk register, full detector and architecture spec, provider/legal dependencies, detailed interview protocol, complete website map, and the distinction between plan claims and verified facts. The documentation gap was closed in the subsequent audit. A limited detector and Shield side panel now exist; customer validation, production security controls and the broader stack remain future work.
