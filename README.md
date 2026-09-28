# dmz

Private product stack for dmz.is. The public marketing site is a separate repository: `AndriGitDev/dmz-web`.

dmz is an Icelandic data-minimisation layer for AI. The product is in validation. A **manual, local-only Shield prototype** now runs the Go detector compiled to WASM in a WXT side panel. It masks selected identifiers with visible markers, e.g. `⟦KT-<session>-1: XXXXX-XXXX⟧`, and restores unchanged markers in a pasted answer. It does not intercept AI sites or secure file uploads. Portal, Gateway, a production vault, ZDR agreements and ISO certification do not exist yet.

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
| `packages/surrogates` | Device-local session mapping and visible masks; package name predates ADR 0002 |
| `packages/policy` | Tenant policy schema |
| `packages/ui` | Shared UI components |
| `bench` | Synthetic corpus, scorers and published results |
| `deploy` | Icelandic deployment manifests and key management |
| `docs` | Architecture decisions, threat model and legal drafts |

Empty directories contain `.gitkeep` until the corresponding work starts. Do not add real personal data or licensed BÍN/Mannanafnaskrá datasets to the repository. Synthetic kennitölur must use century digit 8.

## Try the prototype

Go 1.27 and TinyGo 0.42 are needed for the WASM build. From the repository root:

```sh
go test ./...
go run ./bench/scorers -baseline bench/results/v0.json
GO_BIN=go TINYGO_BIN=tinygo ./scripts/build-wasm.sh
cd apps/shield
npm ci
npm run check
npm run build
```

Load `apps/shield/.output/chrome-mv3` as an unpacked extension in Chromium, then click its toolbar icon to open the side panel. Paste text into field 01, mask it, copy **only** field 02 to an AI tool, then paste the answer into field 03 to restore unchanged markers. The optional local dictionary accepts up to 32 exact, case-sensitive terms; saving it starts a new session. Reset clears both the map and dictionary. `npm run test:browser` tests the built extension in Chromium; install Playwright's Chromium or set `CHROMIUM_EXECUTABLE` to an existing binary. No extension host permissions are requested and no prompt is sent to dmz servers.

The draft PR's `shield-browser` GitHub Actions job uploads the tested `shield-chromium-prototype` build for review. Download and extract it, then use Chromium's **Load unpacked** on the extracted directory. It is an unsigned prototype, not a store release.

The [benchmark readme](bench/README.md), [visible-mask decision](docs/adr/0002-visible-masks.md) and [kennitala update](docs/adr/0003-kennitala-2026.md) explain results and constraints.

The Go core also has a `StreamRestorer` that handles markers split across response chunks. It is exercised by unit tests but is not wired into the side panel or a Gateway yet.

## Working source of truth

Start with the [coverage audit](docs/plan-coverage.md). The [business-plan baseline](docs/business-plan.md), [product spec](docs/product-spec.md), [roadmap](docs/roadmap.md), [financial arithmetic check](docs/financial-model.md), [website brief](docs/website-brief.md), [source register](docs/source-register.md) and [open decisions](docs/open-decisions.md) separate requirements from evidence and implementation.

## Validation gate

By 18 December 2026, commit to building only if at least 3 of 10 target customers agree to a paid pilot at list price. The [launch checklist](docs/launch-checklist.md) tracks prerequisites.
