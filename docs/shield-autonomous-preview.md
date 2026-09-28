# Shield autonomous preview

Status: experimental development build, 28 September 2026. Use fabricated data only. This is an installable browser preview, not a safe path for customer or other sensitive information.

The owner's daily testing target is **desktop Chrome with both ChatGPT and Claude**. Synthetic automated checks currently use Playwright Chromium; successful authenticated flows in the owner's Chrome profile are still required.

## Install and try

For a build that needs no local compiler, open the draft PR's successful **Prototype** Actions run and download **shield-chromium-prototype**. Extract the archive and load that extracted folder using **Load unpacked** in `chrome://extensions`. `BUILD.json` records the source commit and SHA-256 of each packaged file. Include that commit when reporting a problem. CI creates this record only after the browser tests pass; it does not certify live-site behavior.

To build locally:

1. Run `./scripts/build-wasm.sh`, then `cd apps/shield && npm ci && npm run check && npm run build && npm run test:browser`.
2. In Chromium, open `chrome://extensions`, enable developer mode and load `apps/shield/.output/chrome-mv3` as an unpacked extension.
3. Open a new ChatGPT or Claude web tab. Type a synthetic identifier, such as `demo@example.com`, in the chat composer and send using the supported send button or Enter. Shield should replace it with a visible `⟦EMAIL-…: XXXXX@XXXXX.XXX⟧` marker before the page sends. A blocked pattern should leave the text in the composer and show a Shield status message.
4. When the assistant echoes an unchanged marker, the site text stays masked and Shield shows a locally restored copy beside that reply. The manual side panel remains available from the toolbar icon.

Each tab has an in-memory Go/WASM session map. Closing or reloading the tab loses its map; old markers cannot be restored. The manual side panel has a separate map and dictionary. The extension does not send identifiers or maps to dmz servers.

## Current coverage and limits

- Content scripts run only on `https://chatgpt.com/*` and `https://claude.ai/*`. The preview gates recognized text composers on submit, Enter and recognized send-button clicks. It blocks ordinary file-picker, drag/drop and file-paste events on those pages. A website layout or event-handler change can bypass these hooks. Other send paths, mobile apps, voice and images outside those file events are not protected.
- The adapter has synthetic Chromium fixture coverage for both sites, including a page-managed input model and local response display. Additional tests capture the outgoing request body and check that replacing the composer during masking or changing its text during a nested submit prevents the request. A deliberately corrupted WASM build must show an error before any send and prevent outgoing requests. It has **not** been verified on authenticated live site flows. The site may handle programmatic replay differently from the fixtures.
- Uncertain detector findings and known secret shapes stop a recognized submission for review. The detector still exposes annotated literals in the synthetic challenge and exploratory sets; unsupported identifiers can leave the device. No detection claim extends beyond the measured cases.
- A restored reply appears in a closed Shield shadow overlay. The model page keeps the masked reply. Changed or incomplete markers are not restored. The restored copy is plain text, so markdown formatting from the page is not preserved.
- There is no global network interception or guarantee that all outgoing requests are gated. Do not use this build for sensitive data.

## Verification before daily use

The next gate is a supervised live-site smoke pass on signed-in ChatGPT and Claude, with synthetic prompts only: button, Enter, multi-turn marker reuse, blocked review, streaming reply restoration, reload/reset, and upload paths. Record the browser/version and observed outbound request body. If either site bypasses the gate, stop testing there and fix the adapter before using it again.
