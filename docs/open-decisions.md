# Open decisions and evidence requests

Accepted direction: [ADR 0004](adr/0004-portal-hosting-and-pii-policy.md) selects initial Hetzner EU hosting and policy-controlled masking/approved PII processing. The following account and deployment evidence is still required:

- Verify the actual VPS region, backup locations, operator access and processor arrangements.
- Verify the reported $100 Bedrock credit eligibility/expiry and first invocation; `eu-north-1` is provisional.
- Select one model/API/profile; approve its destinations, retention configuration and applicable terms before live PII.

These items are unresolved in the supplied plan or require time-sensitive verification. Owners and evidence links are intentionally blank until assigned.

| Priority | Decision or evidence | Why it matters |
| --- | --- | --- |
| Immediate | Confirm dmz applicant and owner, Atlas Cloud ehf. group classification and formation date | [Public Atlas terms](https://atlascloud.is/is/terms) identify Atlas Cloud ehf. as service operator, but dmz project ownership, financial risk, company age and grant eligibility still need company records |
| Immediate | Rannís applicant eligibility and eligible R&D costs | [2026 listing](https://island.is/styrkjatorg/styrkur/1-8) confirms the new-project window through 1 Oct 2026, 23:59 Icelandic time; applicant, cost treatment and portal filing remain unresolved. See [readiness note](rannis-2026-readiness.md) |
| Immediate | Approved-proxy interpretation and complete evidence mapping for current Stafrænt Ísland rules | Rules checked 28 Sep; direct-identifier masking alone does not cover confidential/case material or process evidence |
| Immediate | Actual count of vendors delivering to Stafrænt Ísland | Weakest 2029 customer assumption; <40 triggers model reset |
| Immediate | Current firm counts from Hagstofa PxWeb | Old 2015/2017 market sizing is unsuitable for investors |
| Immediate | Pilot mailbox, legal controller and message retention period | Public launch readiness |
| Immediate | Approved ISK price book and VAT/legal review | Public page shows dated, cited indicative FX conversion only; final quoted prices need approval |
| Immediate | Whether Portal seats are additional to or replace Shield seats in account examples | The €960 example uses 100 Shield + 20 Portal; Portal is also described as including Shield |
| Near-term | Bedrock EU model availability and `none` retention in a real account | Portal route and provider claim |
| Near-term | OpenAI/Anthropic written ZDR eligibility and multi-tenant service rights | ZDR roadmap, provider terms |
| Near-term | BÍN commercial use and redistribution; Mannanafnaskrá bulk access | Names detector and extension packaging |
| Near-term | Quarterly financial model source spreadsheet | Annual figures alone cannot reproduce cash peak or Q3 profitability |
| Near-term | HMS address dataset use approved by owner on 28 Sep; 23,636 forms bundled in the private trained preview; independent contextual validation pending | See [external dataset preparation](../bench/datasets/README.md); separate postcode sources require their own evidence |
| Near-term | Security architecture and DPA counsel review | Live-data pilots |
| Later | NIS2 Icelandic status and EEA AI Act status each quarter | Claims and procurement |
| Later | Trademark advice on descriptive “DMZ” and figurative mark | Brand investment |
| Later | Nordic language pack and legal/entity model | Gate 4 expansion |

No open decision should be silently converted into a marketing claim.

## Detector dataset update — 28 September 2026

Owner confirmed use of HMS Staðfangaskrá and MIM-GOLD-NER 2.0. Both are downloaded
and prepared locally outside Git; aggregate provenance and the untuned name
development baseline are in [bench/datasets](../bench/datasets/README.md). This
approval does not cover BÍN or Mannanafnaskrá. Portal/Gateway work is deferred
while detector tuning is the current priority.
