# Rannís R&D tax deduction: 2026 application readiness

Checked 28 September 2026. This is a working evidence pack for a possible **new-project application**, not an eligibility determination or a submitted application.

## Deadline and route

The [official 2026 grant listing](https://island.is/styrkjatorg/styrkur/1-8) says the application is open from 20 August through **1 October 2026, 23:59 Icelandic time**. It links the autumn 2026 handbook and application route. The [Rannís application portal](https://kerfi.rannis.is/menu/) uses Ísland.is authentication; an applicant acting for a company needs the appropriate Rannís delegation. Confirm the submission state and receipt in the portal before treating the checklist item as done.

The [Rannís 2025 handbook](https://www.rannis.is/media/skattfrdr.-rannsokna-og-throunarv/Handbok_skattfradrattar_RTh_H2025.pdf) describes company ownership and financial risk, specialist capability, cost planning, numeric business outcomes and a business plan as material to an application. It is a prior-year guide. The applicant must use the **autumn 2026 handbook linked from the grant listing** to settle current requirements and eligible cost treatment before filing. The business plan's 35% SME assumption is not a dmz entitlement; company classification and eligible costs have not been established.

The public [RunAtlas/Atlas terms](https://atlascloud.is/is/terms) identify the service operator as **Atlas Cloud ehf.**; the [Icelandic Intellectual Property Office's Atlas trademark record](https://www.hugverk.is/en/leit/trademark/V0141261) names the same company as owner. This supports the service's public legal name and company form, but does **not** establish that Atlas Cloud ehf. owns the dmz R&D project, will bear its financial risk, or has authority and cost records to apply. The company's formation date, group classification and ownership need company records or the applicant's confirmation.

## Candidate project scope for the application

**Working title:** Icelandic direct-identifier detection and reversible visible masking for AI workflows.

**Technical problem:** The current Go detector and TinyGo WASM side panel handle selected direct identifiers, but a 29-case exploratory synthetic probe leaves 13 annotated literals in allowed outbound text. Icelandic inflection, address structure, varied identifier formatting and arbitrary secret formats are unresolved. Broad rules also stop benign text. The research question is whether a shared detector can improve per-type recall and precision under a privacy boundary where real values and mapping keys remain on device or in an Icelandic core.

**Proposed experimental work, subject to owner and Rannís review:**

1. Define licensed-data-safe Icelandic name and address recognizers; compare rules, dictionaries and inflection handling against frozen synthetic cases and separately authored evaluation cases.
2. Test identifier format and Unicode handling, including exact UTF-8 byte spans, false positives and conservative review behavior.
3. Test Go/TinyGo conformance and device-side latency and size as recognition complexity grows.
4. Test visible-mask consistency and streaming restoration under chunk splitting, marker modification, session reset and cross-tenant boundaries. Keep prompt bodies out of logs and durable storage.

The first three points contain measurable technical uncertainty. Routine website work, sales outreach, legal review, hosting operation and product administration should be costed separately until the current handbook and an accountant establish what is eligible. This scope is a draft; it does not assert that Rannís will classify each activity as R&D.

## Evidence already in the repository

| Evidence | Current record | Limit |
| --- | --- | --- |
| Problem and product boundaries | [Product spec](product-spec.md), [threat model](threat-model.md) | Requirements and prototype, not a production security proof |
| Detector and WASM prototype | `packages/detect/`, `apps/shield/wasm/`, `scripts/build-wasm.sh` | Manual side panel; no live-site interception |
| Baseline and failure discovery | `bench/corpus/`, `bench/results/`, [benchmark method](../bench/README.md) | Small synthetic sets, not representative field recall |
| Experimental controls | `bench/gates/`, `.github/workflows/prototype.yml` | CI floors and snapshots, not an independent audit |
| Work plan and commercial hypothesis | [Roadmap](roadmap.md), [business plan](business-plan.md), [validation plan](validation.md) | Customer interviews and cost schedule still pending |

## Inputs the owner must supply before filing

| Item | Status in repo |
| --- | --- |
| Applicant legal entity, registration and authority to file; whether Atlas Cloud ehf. owns and bears the financial risk of the dmz project | Public operator name/form found; applicant and project ownership still unresolved in [open decisions](open-decisions.md) |
| Company size and group/ownership classification for the relevant year | Unresolved |
| Named project lead and documented specialist experience | Unresolved |
| Dated project start, work packages, milestones and staff time | Draft scope above; costed schedule missing |
| 2026 cost ledger and forward budget by category, including any outside suppliers and other public support | Missing; do not use the commercial P&L as an eligible-cost schedule |
| Numeric business outcomes and marketing assumptions required by the current application | Business-plan hypotheses exist; owner must approve defensible figures |
| Review of the autumn 2026 handbook and actual application fields | Pending portal access |

**Next action:** The authorized company representative should open the official 2026 listing and portal now, check delegation and the autumn 2026 handbook, then supply the missing applicant and cost facts. The project narrative above can be adapted to the form after those facts are known. Do not mark the Rannís checklist item complete without the submitted application receipt.
