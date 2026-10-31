# dmz

dmz is a free, open-source **Chromium extension** by Kastro, licensed under [GPL-3.0-only](LICENSE). It attempts to mask selected direct identifiers in supported AI-site text submissions and restores unchanged known markers locally. The bilingual website lives in [dmz-web](https://github.com/AndriGitDev/dmz-web).

Detection, masking and restoration run on device. Original values and per-tab mappings are not sent to dmz servers. The host website can already see the original draft in its composer before masking. Detection can miss identifiers; failures warn and continue submission, and attachments are sent unchanged. **Use fabricated data when testing this alpha.** Source availability does not establish independent detector quality or suitability for confidential information.

## Build the basic extension

Go 1.27, TinyGo 0.42 and Node.js/npm are required by the current toolchain.

```sh
go test ./...
go run ./bench/scorers -baseline bench/results/v0.json
GO_BIN=go TINYGO_BIN=tinygo ./scripts/build-wasm.sh
cd apps/shield
npm ci
npm run check
npm run build
```

Open `chrome://extensions`, enable Developer mode, select **Load unpacked**, and choose `apps/shield/.output/chrome-mv3`. Reload supported chat tabs after installation or updates. Remove the extension from the same browser page. Resetting a session, reloading or closing its chat tab clears the mapping and prevents restoration of old markers.

The ordinary build contains the basic rule-based detector. The [trained build](apps/shield/scanner/README.md) uses separately prepared model and address assets and has different coverage. External assets are not automatically licensed by the GPL grant. Never substitute a basic build for a trained release without identifying that difference. See [third-party materials](THIRD_PARTY.md).

Run `npm run test:browser` for extension workflows and `npm run test:coverage` for coverage accounting. Browser checks require Playwright Chromium or `CHROMIUM_EXECUTABLE`. Use the existing scanner suites when changing trained inference. The source directory `apps/shield` is retained for build compatibility; the product name is **dmz**.

## Source map

- `apps/shield`: Manifest V3 extension, local scanner integration and browser checks.
- `packages/detect`: Go detector and WASM build.
- `packages/surrogates`: local mappings and visible markers.
- `bench`: synthetic regression checks and evaluation tooling.
- `docs`: [current scope](docs/community-project.md), [product behavior](docs/product-spec.md), [quality contract](docs/detector-quality-contract.md), [roadmap](docs/roadmap.md) and [release readiness](docs/open-source-readiness.md).

## Review and contribute

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [SECURITY.md](SECURITY.md). Never submit real prompts, personal data, secrets or licensed source datasets. Synthetic kennitölur use century digit `8`. Public measurements are development evidence with limitations, not field-quality guarantees.

Original project code is GPL-3.0-only; third-party components keep their own terms. Distributed modified versions must meet applicable change-notice and corresponding-source obligations. No license prevents malicious copies: verify official source and release identity.
