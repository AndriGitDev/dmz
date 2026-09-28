# Legal, provider and data dependencies

Working notes from the 27 September 2026 plan, not legal advice. Counsel must approve executed documents and external claims.

## Roles and documents

Customer is controller; dmz is processor for Portal/Gateway under GDPR Article 28 and Icelandic Act 90/2018. Shield handles prompt values on-device but any metadata to dmz still needs contractual coverage. Prepare DPA, public sub-processor list with 30 days' notice of changes, DPIA template, incident SLA, liability cap of 12 months' fees, and cyber/professional-liability insurance. Planned sub-processors are RunAtlas, AWS/Bedrock EU, and OpenAI/Anthropic only once used. Confirm EEA transfer mechanism (DPF or SCCs) for each US provider.

The plan cites the CJEU SRB decision for the possibility that strongly pseudonymised data may fall outside “personal data” for a recipient unable to re-identify it. This is context-dependent and does not remove the controller's transparency obligations or risk from quasi-identifiers. Ship a DPIA template, not a legal opinion.

## Providers and retention

Use API/commercial terms only. Never proxy ChatGPT/Claude subscription credentials or tokens. Obtain written confirmation that serving multiple dmz customers through one API or ZDR organisation is permitted. Do not label a route ZDR before an applicable signed agreement and model-specific verification. Verify Bedrock EU region and retention mode `none` on the actual model/account. The plan says Anthropic banned subscription tokens in third-party tools on 20 February 2026; verify with primary provider terms before citing externally.

## Data licences and intellectual property

BÍN requires licence acceptance before download; confirm commercial use and redistribution inside a browser extension. Confirm Mannanafnaskrá bulk export/API rights. Keep licensed lexicon data out of Git until terms permit it; fetch at build time under controlled credentials. Ask Hugverkastofan whether a figurative mark is viable given “DMZ” is a generic network term.

## Certification and regulation

ISO 27001 certification is a target by end-2028 through an accredited body; ISO 42001 later. Andri must not audit dmz or its customers because of self-review conflict. The plan assumes dmz is not a high-risk AI system; counsel must validate classification. Check AI Act EEA incorporation and NIS2 Icelandic status each quarter. Do not claim “GDPR compliant,” “AI Act compliant,” “anonymous” or zero risk from stand-ins.

## Browser distribution

Prepare Chrome Web Store user-data disclosures and enterprise force-install instructions. Shield must block file/image uploads by policy until redaction exists. Document the limits of desktop/mobile chat apps and managed-device controls.
