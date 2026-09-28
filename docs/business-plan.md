# dmz.is business plan baseline

Source: Andri Pétur's plan supplied 27 September 2026. This is a faithful working baseline, not evidence that market, provider, legal or funding claims have been independently verified. Use [source register](source-register.md) and [open decisions](open-decisions.md) before external claims or applications.

Later prototype decisions supersede its realistic-stand-in approach: see [ADR 0002](adr/0002-visible-masks.md). Kennitala check-digit rules changed in February 2026: see [ADR 0003](adr/0003-kennitala-2026.md). This baseline is retained for provenance, not current product copy.

## Thesis and decision rule

dmz is a RunAtlas product line for data minimisation in AI. It replaces direct Icelandic identifiers with realistic stand-ins before a prompt reaches a model and restores real values in the response. One detection core serves Shield, Portal and Gateway. Real identifiers and the key linking them to stand-ins remain in Iceland. The first wedge is software vendors subject to Stafrænt Ísland's AI rules, followed by regulated mid-size firms and small professional firms. Do not commit to the full build unless at least **3 of 10** target customers commit to a paid pilot at list price by **18 December 2026**.

Founder-fit hypothesis: Andri brings ISO 27001/ISO 42001 lead-auditor credentials, security sales experience and RunAtlas sovereign hosting; the plan says he is already building and publicly documenting a tiered, data-minimisation-first AI architecture. Verify any credential and partner statement before putting it on the public site. Auditor independence is a separate constraint below.

Do not pass through ChatGPT/Claude logins or subscription tokens; do not claim ZDR before a signed agreement; do not claim anonymity, 100% detection, GDPR/AI Act compliance or ISO certification before evidence exists. The product is pseudonymisation and data minimisation, with meaningful residual risk from quasi-identifiers.

## Demand and market hypothesis

| Signal from the plan | Figure | Verification need |
| --- | ---: | --- |
| Icelandic firms using AI | about half; 81% of firms with 250+ staff | Source date and survey population |
| Firms with a formal AI policy | 14% | Source date |
| Firms able to name AI tasks | 25% | Source date |
| Paid/free web AI subscriptions | 39% / 37% | Source date and definitions |
| Organisations able to stop public-AI uploads globally | 17% | Methodology and relevance to Iceland |
| Private firms with 50+ employees in 2015 | 296, excluding finance/insurance | Replace with current PxWeb data |
| Private firms with 100+ employees in 2017 | 179, excluding finance/insurance | Replace with current PxWeb data |

The working buyer universe is **well under 1,000** Icelandic organisations at mid-size scale when finance, insurance and public bodies are included. The base case needs 98 by end-2029, including 20 regulated mid-size firms. Nordic expansion requires new language packs and a separate Gate 4 decision. The plan's external price anchor is Nightfall's estimate of roughly **$55–95/user/year** for M365-level DLP and **$80–140/user/year** for multi-platform coverage; this is a competitor benchmark, not a dmz cost estimate.

### Retention gap hypothesis

The plan distinguishes web-chat subscriptions from commercial API and Bedrock routes:

| Route in plan | Working retention claim | ZDR claim in plan |
| --- | --- | --- |
| ChatGPT Free/Plus | Consumer terms | No |
| ChatGPT Business/Enterprise chat | Retention controls for eligible organisations | No; ZDR is API-only |
| Claude Team/Enterprise chat | Commercial terms | No |
| OpenAI API | Per-API data controls | Possible for approved organisations |
| Anthropic API | 30-day default deletion; named newer model classes excluded from ZDR | Possible for approved organisations and eligible models |
| Bedrock EU, Opus 5/Sonnet 5 as proposed in plan | No retention under tested `none` mode | Provider route to verify in a real account |

These are provider-specific, time-sensitive claims. Verify contract, region, model and account behaviour before using them in sales or routing. A platform-level ZDR agreement is a hypothesis until providers confirm multi-tenant service rights in writing. The plan cites GitHub's Claude arrangement as precedent, not proof of dmz eligibility.

## Product and value

| Surface | Buyer/workflow | Boundary | Phase |
| --- | --- | --- | --- |
| Shield | Staff on ChatGPT, Claude and Gemini web | Detect/swap/restore and mapping vault on device; provider receives stand-ins under the user's chat plan | 1 |
| Portal | Sensitive-work roles | Server-side detection on RunAtlas in Iceland; route to local models or Bedrock EU, then approved ZDR APIs | 1–2 |
| Gateway | Developers, CI and internal apps | OpenAI/Anthropic-compatible proxy; secret and log detection; metadata-only audit export | 2 |
| Connector gateway | CRM/helpdesk data accessed through MCP/plugins | Pseudonymise records before model access, including desktop/mobile paths | 3 |

The marketing site has its own [website brief](website-brief.md). Engineering detail and release criteria live in [product spec](product-spec.md).

## Segments and motion

1. **Stafrænt Ísland vendors, Q1 2027:** Gateway plus Shield and an audit export mapped to the rules that, according to the plan, bar kennitölur, secrets and logs from external AI tools. Founder-led selling; ask Stafrænt Ísland to recognise an approved-proxy pattern. Count the actual vendor population before committing to 18 vendor customers by 2029.
2. **Regulated mid-size, Q2 2027:** finance, insurance, law, healthcare-adjacent firms and municipalities. Shield broadly, Portal for sensitive roles. DPOs, law firms and MSPs are referral/reseller channels; Kastro is the first proposed MSP at 20% margin.
3. **Small professional firms, Q3 2027:** accountants, agencies and small law firms; self-serve Shield with card billing, subject to product and legal readiness.

Demand generation: public Icelandic PII benchmark on every release, founder LinkedIn posts about leak patterns and the rules, and talks at Ský and AI/security groups. Proof order: benchmark, architecture/retention page, sub-processor list, ISO 27001 roadmap, founder credentials.

### Pricing assumptions, EUR ex VAT

| Plan | Monthly seat price | Scope |
| --- | ---: | --- |
| Shield | €6 | Browser extension, Icelandic detectors, policy, metadata audit |
| Portal | €18 | Shield, sensitive-work workspace, fair-use model budget, audit export |
| Gateway | €25 per developer | API proxy; model usage billed at cost +10% |
| Onboarding / 30-day paid pilot | €1,500 once | Policy, dictionaries and tuning |

Minimum 10 seats. The Icelandic public site should use an approved ISK price book; no ISK prices are approved in the supplied plan. Do not invent a contractual ISK conversion. Portal model cost assumption: 30% of Portal revenue. Shield's device-local path is expected to have software-like gross margin.

### Account examples

| Account | Mix | MRR | ARR |
| --- | --- | ---: | ---: |
| Regulated mid-size | 100 Shield + 20 Portal | €960 | €11,520 |
| Stafrænt Ísland vendor | 15 Gateway + 15 Shield | €465 | €5,580 |
| Small professional firm | 30 Shield | €180 | €2,160 |

## Financial baseline (assumptions, not a forecast)

| Year | End customers (small / regulated / vendor) | Revenue | Costs | Net |
| --- | --- | ---: | ---: | ---: |
| 2027 | 19 (10 / 3 / 6) | €73k | €192k | −€119k |
| 2028 | 52 (30 / 10 / 12) | €236k | €353k | −€117k |
| 2029 | 98 (60 / 20 / 18) | €449k | €449k | break-even |

End-2029 mix gives **€38,370 MRR / €460,440 ARR** from recurring seat revenue. The plan projects the first profitable quarter in Q3 2029 and a peak cash need of about €261k in Q2 2029. A half-customer downside reaches 49 customers, never breaks even, and accumulates about €602k loss by end-2029. The source quarterly model was described but not supplied; the annual figures above cannot be reproduced fully without quarterly customer timing, churn and onboarding assumptions. Obtain the spreadsheet before investor use.

Other assumptions: 1.0 FTE in 2027, 2.0 in 2028, 2.5 in 2029 at €130k loaded cost per FTE; founder 0.5 FTE. Annual infrastructure €12k/€24k/€36k; legal, pentest and ISO €25k/€25k/€20k; marketing €15k/€25k/€35k; insurance €8k/€10k/€12k. Grants, R&D refund, Nordics and services revenue are omitted. Hire only after MRR gates: €7.5k at end-2027 and €20k at end-2028. The 18-vendor 2029 count is the weakest demand assumption.

## Competitive hypothesis

| Alternative | Where it wins | Proposed dmz angle |
| --- | --- | --- |
| Microsoft Purview DSPM for AI | E5/E5 Compliance and state Microsoft estate | Rewriting beside Purview auditing; Icelandic inflection and kennitala support |
| Nightfall, Harmonic, Netskope | International DLP platforms | Icelandic language, support and hosting; lower entry price hypothesis |
| Palo Alto, Check Point, SentinelOne, Cisco, Cato suites | Platform-committed enterprises | Integrate alongside, sell to firms without the platform |
| LiteLLM, LLM Guard, Presidio | Engineering-heavy self-builders | Hosted, tuned detectors and evidence |
| Vercel AI Gateway | Developer routing and ZDR-capable providers | Adds Icelandic PII handling |
| Versoria Mímir | Icelandic multi-model workspace | Potential partner; dmz is a PII layer |
| APRÓ Liðsauki, DataLab | Closed AI solutions and services | Potential service partners/resellers |

The sharpest platform threat is Microsoft lowering the AI DLP licence bar. Verify all competitive and licence claims before publishing comparisons.

## Organisation and funding

Run as a RunAtlas product line until 10 paying customers, then reassess a separate dmz ehf. RunAtlas provides sovereign hosting, a security programme and the DPA/sub-processor chain. Kastro is the proposed first reseller/support partner. A separate entity is considered for outside capital or a Nordic cap table. RunAtlas legal form, age and ownership determine grant eligibility.

| Role | Start | FTE / gate |
| --- | --- | --- |
| Founder (product, security, sales) | now | 0.5 |
| Go/TypeScript/browser engineer | Q1 2027 | 0.5 contractor, then 1.0 after Gate 3 |
| Customer success/sales | after Gate 3 | 0.5, possibly Kastro |
| Second engineer | after Gate 4 | 0.5 |

| Funding hypothesis | Plan amount and terms | Decision dependency |
| --- | --- | --- |
| R&D tax refund | 35% of eligible SME R&D cost; plan states 1 Oct 2026 deadline | Rannís project certification and current window |
| Tækniþróunarsjóður Sproti | Up to 2 × 12M ISK across two years, no match | Applicant at most five years old |
| Tækniþróunarsjóður Fræ | Up to 2M ISK over 12 months, no match | Young company or individual; validation work |
| Tækniþróunarsjóður Markaður | Up to 12M ISK for one year, 50% match | SME spends 10% of costs on R&D; subsidiaries excluded; later Nordic GTM |
| RunAtlas operating budget | Remaining gap | Internal allocation |

Check current calls, entity eligibility and tax advice before application. The plan says the 2026 Tækniþróunarsjóður spring deadline was 16 February; do not assume a next call date.

## Risks and explicit mitigations

| Risk | Severity | Plan response |
| --- | --- | --- |
| Missed identifier leaks | High | Public per-type recall, block on uncertainty, insurance and liability cap |
| Microsoft reduces AI DLP licence bar | High | Sell rewriting beside Purview, focus non-E5 buyers |
| Iceland too small | High | MRR gates on hiring, Gate 4 Nordic decision |
| Founder attention split | High | Gate 1 kill criteria; keep dmark on hold |
| dmz core breached | High | Stateless gateway, no prompt logs, tenant-keyed vault, pentest, ISO 27001 programme |
| Chat UI change breaks Shield | Medium | Hook input/submit events, nightly live-site tests |
| Provider rejects ZDR or changes terms | Medium | Bedrock EU and local models from day one; no premature ZDR claim |
| Stand-ins remain personal data | Medium | Precise claims, DPA and DPIA template |
| Auditor independence conflict | Medium | Founder does not audit dmz or its customers; disclose role |
| Browser store/deployment friction | Low | Enterprise force-install and clear store disclosures |

## Decision material

See [roadmap](roadmap.md), [validation protocol](validation.md), [legal and provider dependencies](legal/dependencies.md), and [open decisions](open-decisions.md). The user-supplied financial and market figures are hypotheses until checked against current primary sources and interviews.
