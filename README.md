# dmz

Private product stack for dmz.is. The public marketing site is a separate repository: `AndriGitDev/dmz-web`.

dmz is an Icelandic data-minimisation layer for AI. The product is in validation. No production detector, extension, gateway, ZDR agreement, or ISO certification is claimed by this repository.

## Repository map

| Path | Planned responsibility |
| --- | --- |
| `apps/docs` | Starlight product and security docs |
| `apps/portal` | SvelteKit sensitive-work workspace and admin |
| `apps/shield` | WXT Manifest V3 extension |
| `services/gateway` | Go model routing and streaming rehydration |
| `services/vault` | Encrypted, expiring per-session mapping |
| `services/audit` | Metadata-only audit events and export |
| `packages/detect` | Go detector core and TinyGo WASM build |
| `packages/lexicon` | Licensed Icelandic name data build pipeline |
| `packages/surrogates` | Gender- and case-aware stand-ins |
| `packages/policy` | Tenant policy schema |
| `packages/ui` | Shared UI components |
| `bench` | Synthetic corpus, scorers and published results |
| `deploy` | Icelandic deployment manifests and key management |
| `docs` | Architecture decisions, threat model and legal drafts |

Empty directories contain `.gitkeep` until the corresponding work starts. Do not add real personal data or licensed BÍN/Mannanafnaskrá datasets to the repository. Synthetic kennitölur must use century digit 8.

## Validation gate

By 18 December 2026, commit to building only if at least 3 of 10 target customers agree to a paid pilot at list price. The [launch checklist](docs/launch-checklist.md) tracks prerequisites.
