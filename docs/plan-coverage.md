# Business-plan coverage audit

Checked against the user-supplied 27 September 2026 plan. **Captured** means a requirement or assumption has a durable home in this repo. It does **not** mean shipped, verified, approved or legally effective.

| Plan section | Durable record | Implementation state |
| --- | --- | --- |
| Executive thesis and Gate 1 decision | [business plan](business-plan.md), [validation](validation.md) | Captured; interviews and commitments pending |
| Market size and demand signals | [business plan](business-plan.md), [sources](source-register.md) | Captured as hypotheses; current PxWeb count pending |
| ZDR gap and provider terms | [business plan](business-plan.md), [legal dependencies](legal/dependencies.md) | Captured; provider-specific verification pending |
| Product surfaces and data flow | [product spec](product-spec.md) | Requirements captured; apps/services not built |
| Detector scope and algorithm | [product spec](product-spec.md), policy schema | Captured; detector/benchmark not built |
| Architecture, vault, routing and audit | [product spec](product-spec.md), [threat model](threat-model.md) | Captured; implementation and controls pending |
| Positioning, precise claims and proof | [website brief](website-brief.md) | Gate 1 copy drafted; evidence pending |
| Competitive landscape | [business plan](business-plan.md), [sources](source-register.md) | Captured; current comparisons not verified |
| Pricing, segments and GTM | [business plan](business-plan.md) | EUR assumptions captured; ISK price book pending |
| Unit economics and financial model | [business plan](business-plan.md), [arithmetic check](financial-model.md) | Year-end ARR checked; quarterly model source missing |
| Roadmap, MRR and recall gates | [roadmap](roadmap.md), [product spec](product-spec.md) | Captured; execution pending |
| Organisation, grants and hiring | [business plan](business-plan.md), [open decisions](open-decisions.md) | Captured; entity/grant eligibility pending |
| Legal, data licences and ISO roadmap | [legal dependencies](legal/dependencies.md) | Captured; counsel/licence approvals pending |
| Risks and mitigations | [business plan](business-plan.md), [threat model](threat-model.md) | Captured; controls not proven |
| Interviews and kill criteria | [validation](validation.md), [launch checklist](launch-checklist.md) | Captured; interviews pending |
| Repo blueprint and CI | [product spec](product-spec.md), root README | Directory scaffold only; CI gates pending code |
| Website map, copy and build notes | [website brief](website-brief.md), public `dmz-web` | Gate 1 pages built; later pages staged |

## What the first 10-minute pass missed

It created a working launch-site skeleton and directory scaffold, but omitted durable records for market evidence, exact financial assumptions, risk register, full detector and architecture spec, provider/legal dependencies, detailed interview protocol, complete website map, and the distinction between plan claims and verified facts. This audit closes the documentation gap; engineering and validation remain future work.
