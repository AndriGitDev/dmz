# dmz-web information architecture and messaging

Public repository: `AndriGitDev/dmz-web`. Icelandic is default (`/`); English mirror lives under `/en/`; every page has canonical and hreflang links. One primary action: discuss a paid pilot. No third-party trackers, embeds, fonts or form service. Cookieless Rybbit may be added only after its configuration and privacy text are reviewed. Pilot form should post to an Iceland-hosted dmz API and collect company, seat count, AI tools and sector; until then use a clearly labelled email contact path and do not claim a working form.

## Page map and planned timing

| Icelandic | English | Job | Stage |
| --- | --- | --- | --- |
| `/` | `/en/` | Hero, detect/swap/restore illustration, three product cards, pilot CTA | Gate 1 launch |
| `/stafraent-island` | `/en/stafraent-island` | Vendor wedge, rule-to-control map, audit export example | Gate 1 launch; exact mapping pending verification |
| `/oryggi` | `/en/trust` | Data flow, retention boundaries, sub-processors, DPA status, ISO roadmap | Gate 1 launch |
| `/tilraun` | `/en/pilot` | Pilot intake/conversion | Gate 1 launch; email until API exists |
| `/skilmalar`, `/personuvernd`, `/vinnslusamningur`, `/undirvinnsluadilar` | `/en/terms`, `/en/privacy`, `/en/dpa`, `/en/subprocessors` | Procurement/legal | Gate 1 status pages; final text after counsel |
| `/profun` | `/en/benchmark` | Per-entity measured recall by release, method, synthetic corpus | With benchmark v0, no placeholder scores |
| `/skjoldur` | `/en/shield` | Extension prompt example, supported sites, device deployment | Product detail, before v1.0 |
| `/gatt` | `/en/portal` | Sensitive-work lane, routing and retention distinctions | Product detail, before v1.0 |
| `/forritarar` | `/en/gateway` | Compatible API path, secret/log detection | Product detail, before v1.1 |
| `/verd` | `/en/pricing` | Three plans, onboarding, ten-seat minimum | After approved ISK price book |
| `/greinar` | `/en/blog` | Leak patterns, ZDR explainer, retention updates | With reviewed articles |
| `/um` | `/en/about` | Founder credentials, RunAtlas and Kastro | After factual/partner review |

Subdomains: `portal.dmz.is` app, `api.dmz.is` Gateway/intake, `docs.dmz.is` Starlight, `status.dmz.is` Uptime Kuma. Do not point public navigation to unavailable subdomains.

## Copy rules

**Heading:** “Gervigreind án persónuupplýsinga” / “AI without the personal data.” **Supporting copy:** replacement of kennitölur, names and account numbers before anything reaches ChatGPT/Claude, with real values restored in the answer. Qualify this by product boundary and direct-identifier scope on nearby pages.

| Say | Avoid |
| --- | --- |
| Shield replaces direct identifiers before content leaves the device | “Anonymous” or “no personal data ever” |
| Portal/Gateway keep real values and mapping key in Iceland | ZDR without signed agreement |
| Recall by entity type measured on a public benchmark | 100% detection |
| Helps implement Stafrænt Ísland rules and GDPR data minimisation | “GDPR compliant” or “AI Act compliant” |
| Providers receive stand-ins | “Providers see nothing” |

The line “we cannot leak what we never see” applies to Shield only; Portal/Gateway see real values transiently in Iceland. All demos use synthetic data, with valid 1800s kennitölur. Avoid suggesting that all personal data, including quasi-identifiers, is removed.

## Design and privacy acceptance

Responsive, accessible navigation and text; no external trackers; no cookies unless a documented need arises; security.txt and sitemap; both locales for every public route; internal links and language switch verified. Do not publish a benchmark score, price conversion, legal agreement, partner endorsement, retention mode or certificate that lacks evidence. Production launch additionally requires active mailboxes, final legal controller/privacy details, domain/TLS and working Iceland-hosted intake if the form is shown.
