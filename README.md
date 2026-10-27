# dmz

DMZ is a free community project and local masking tool from Kastro for reducing accidental sharing of sensitive details with AI, with the DMZ Chromium extension. Read the [active community concept and release path](docs/community-project.md). There are no planned paid DMZ tiers or seat minimums; Portal/Gateway are deferred.

This product repository remains private; free access does not yet mean open source. The public website is a separate repository: `AndriGitDev/dmz-web`. The public website and Kastro copy follow the community direction; public extension distribution remains subject to the [artifact checklist](docs/community-distribution.md).

dmz is an Icelandic data-minimisation layer for AI. The product is in validation. DMZ runs the Go detector compiled to WASM in an experimental ChatGPT/Claude content script, with a WXT settings side panel. It masks selected identifiers with visible markers, e.g. `⟦KT-<session>-1: XXXXX-XXXX⟧`, and restores unchanged markers locally. The content script intercepts selected text submissions and shows a local restored copy of recognized assistant markers. It does not cover every send path or secure file uploads. Portal, Gateway, a production vault, ZDR agreements and ISO certification do not exist yet.

## Repository map

DMZ and the detector are active. Portal, gateway, vault, server audit and tenant-policy scaffolds below are retained for reference and carry no delivery commitment.

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
| `deploy` | Approved deployment configuration and key management |
| `docs` | Architecture decisions, threat model and legal drafts |

Empty directories contain `.gitkeep` until the corresponding work starts. Do not add real personal data or licensed BÍN/Mannanafnaskrá datasets to the repository. Synthetic kennitölur must use century digit 8.

## Trained scanner preview

A separate local build now integrates the trained Icelandic name detector into DMZ's text-submission and masking flow. See [build, installation and evidence](apps/shield/scanner/README.md). It processes text on device and shares one model across tabs. Under the owner-approved best-effort policy, scanner failure produces a visible warning and sends the basic detector’s masked result; failures warn and submission continues. Attachments are sent unchanged with a warning. The quality-first model is large; weights remain outside Git and are packaged explicitly from verified local assets. The ordinary CI artifact below still contains the small rule-based scanner. Independent quality and owner-browser validation remain pending.

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

Load `apps/shield/.output/chrome-mv3` as an unpacked extension in Chromium. The [autonomous preview guide](docs/shield-autonomous-preview.md) describes the supported chat pages, test workflow and limits. Click the toolbar icon to open settings: inspect the active chat tab, choose a light/dark/system menu appearance, or clear that tab’s session with confirmation. The old manual masking demo and its separate dictionary session have been removed. `npm run test:browser` tests both the panel and synthetic supported-site flows in Chromium; install Playwright's Chromium or set `CHROMIUM_EXECUTABLE` to an existing binary. The extension requests no broad host permissions and sends no prompt to dmz servers.

The draft PR's `shield-browser` GitHub Actions job uploads the tested `shield-chromium-prototype` build for review. Download and extract it, then use Chromium's **Load unpacked** on the extracted directory. It is an unsigned prototype, not a store release.

**Do not use this prototype for sensitive data.** The original untuned 18-case synthetic challenge set exposed eight annotated identifiers in allowed outbound text. After targeted detector changes, the same set still exposes two; the original untuned 29-case exploratory probe exposed 13. Its current reused regression result exposes five under strict core policy; see [current measurements and limitations](bench/README.md). None of these small synthetic sets estimates field quality. Initial owner testing in Brave confirms masking on both sites and restoration on Claude; full authenticated validation remains pending, and DOM changes or unsupported send paths can bypass it. Use fabricated examples when reviewing it.

The [benchmark readme](bench/README.md), [detector follow-up](docs/detector-follow-up.md), [visible-mask decision](docs/adr/0002-visible-masks.md) and [kennitala update](docs/adr/0003-kennitala-2026.md) explain results, constraints and next steps.

The Go core also has a `StreamRestorer` that handles markers split across response chunks. It is exercised by unit tests but is not wired into the side panel or a Gateway yet.

## Working source of truth

Start with the [community concept](docs/community-project.md); it supersedes the commercial priorities in the older records. The [coverage audit](docs/plan-coverage.md), [business-plan baseline](docs/business-plan.md), [product spec](docs/product-spec.md), [roadmap](docs/roadmap.md), [financial arithmetic check](docs/financial-model.md), [website brief](docs/website-brief.md), [source register](docs/source-register.md) and [open decisions](docs/open-decisions.md) separate requirements from evidence and implementation.

## Validation gate

Validate installation, useful local masking workflows, understandable warnings, detector quality and sustainable maintenance using the [community release path](docs/community-project.md#first-release-path). The old three-paid-pilot gate is retired. The [launch checklist](docs/launch-checklist.md) tracks the transition.

## Deferred Portal and Gateway direction

Portal and Gateway are outside the initial community scope and carry no delivery commitment. If revisited, the accepted [hosting and PII policy decision](docs/adr/0004-portal-hosting-and-pii-policy.md) uses existing Hetzner EU infrastructure initially, with an Iceland-hosted option for specific customer requirements. Portal will use a shared Go gateway with either masking or explicitly approved original-PII processing on a verified provider route. See the [preview implementation plan](docs/portal-gateway-plan.md); no Portal/Gateway or Bedrock route is implemented yet.

## Public alpha testing

The owner-authorized trained alpha is distributed as **DMZ Chromium extension 0.1.3 alpha.3** at [dmz.is](https://www.dmz.is/vidbot/) and [English installation guide](https://www.dmz.is/en/extension/). It preserves the existing trained scanner and adds the renamed UI/evaluation kit. See [ADR 0006](docs/adr/0006-chromium-name-and-public-test-download.md) and the [distribution process](docs/community-distribution.md). Internal `apps/shield` paths remain for build compatibility; Shield is no longer a public product name.

The settings menu can pause the active chat tab for 15 minutes and resume immediately; other tabs stay enabled, the mapping is retained, and reload/expiry re-enables DMZ. See [temporary-disable plan and verification](docs/temporary-disable-plan.md).
