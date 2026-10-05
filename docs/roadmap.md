# Roadmap and gates

Working baseline from 27 September 2026 plan. The embedded four-phase chart was not supplied as text, so dates below are limited to explicit milestones and release statements in that plan.

| Gate / phase | Timing | Required evidence | Decision |
| --- | --- | --- | --- |
| Validation / Gate 1 | 18 Dec 2026 | 10 target interviews; ≥3 paid-pilot commitments at list price; vendor count; provider and data-licence feasibility; synthetic benchmark v0; clickable Shield demo | Build, re-plan or stop under [kill criteria](validation.md) |
| Product / Gate 2 | 2027 | Shield GA and Portal on Bedrock EU; public benchmark reaches ≥99% kennitala and ≥95% name recall; vault/audit evidence | Release v1.0 only with measured claims |
| Commercial / Gate 3 | End 2027 | €7.5k MRR; Gateway GA and vendor audit export | Expand first engineer from 0.5 to 1.0 FTE and consider customer-success capacity |
| Scale / Gate 4 | End 2028 | €20k MRR; approved ZDR backends, connector plan; security programme and ISO 27001 target | Decide Nordic expansion and second engineer; otherwise stay lean and Iceland-only |

The plan targets ISO 27001 certification by end-2028 through an accredited body, with ISO 42001 later. These are roadmap targets, not existing credentials for dmz. Founder auditor credentials do not substitute for product certification.

## Next implementation sequence

The current detector work targets ≥99% exact entity-set accuracy per excerpt and ≥99% entity recall, as clarified by the owner on 4 October. The [quality contract](detector-quality-contract.md) separates reused development measurements from independent evidence. Current work covers detector and Shield correctness, a frozen model comparison, and model-blind annotation preparation; neither the targets nor independent acceptance have been achieved.

Under [ADR 0004](adr/0004-portal-hosting-and-pii-policy.md), build a shared Go gateway with Portal as its first client: verify one Bedrock route, implement authenticated policy enforcement, add the SvelteKit chat, complete workflow/isolation checks, then integrate one external API client. See [acceptance gates and proposed credit allocation](portal-gateway-plan.md). Original milestone dates above remain hypotheses.

## Dated work

| Date | Deliverable |
| --- | --- |
| 1 Oct 2026, 23:59 Icelandic time | File Rannís R&D project application for detector/benchmark; [official 2026 window verified](https://island.is/styrkjatorg/styrkur/1-8), applicant and cost details still pending |
| 16 Oct 2026 | Apply for OpenAI/Anthropic ZDR; test Bedrock EU `none` on named models in a real account |
| 30 Oct 2026 | Confirm BÍN commercial licence and Mannanafnaskrá export/API |
| 13 Nov 2026 | Synthetic PII benchmark v0 with Presidio, regex and dmz scores |
| 27 Nov 2026 | Clickable Shield demo on ChatGPT/Claude; meet Stafrænt Ísland |
| 4 Dec 2026 | Complete 10 interviews |
| 18 Dec 2026 | Gate 1 decision |

The current [launch checklist](launch-checklist.md) is the operational task list. Add owners and evidence links as items are done; do not treat a checked box as proof without a link or artifact.
