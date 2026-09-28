# Shield autonomous preview

Status: experimental development build, 28 September 2026. Use fabricated data only. This is an installable browser preview, not a safe path for customer or other sensitive information.

The owner's daily testing target is **desktop Chrome with both ChatGPT and Claude**; initial live testing is in **Brave with Brave Shields disabled**. Synthetic automated checks use Playwright Chromium. The first live report found unsupported composers on both new-chat pages. Updated selectors cover the reported ChatGPT markdown textbox and a Claude ProseMirror editor with a child placeholder; the owner then confirmed masking on both sites and clean restoration on Claude. ChatGPT restoration was missing; supplied reply markup identified its new `data-markdown-text-style="assistant-message"` selector. That fix and inline rendering await live retesting. The full verification checklist remains open.

## Install and try

For a build that needs no local compiler, open the fix PR's successful **Prototype** Actions run and download **shield-chromium-prototype**. Extract the archive and load that extracted folder using **Load unpacked** in `chrome://extensions`. Chrome's manifest version name and `BUILD.json` identify the source commit; the record also gives the SHA-256 of each packaged file. Include that commit when reporting a problem. The record command rejects a dirty checkout or a manifest from a different commit. CI creates this record only after the browser tests pass; it does not certify live-site behavior.

To build locally:

1. Run `./scripts/build-wasm.sh`, then `cd apps/shield && npm ci && npm run check && npm run build && npm run test:browser`.
2. In Chromium, open `chrome://extensions`, enable developer mode and load `apps/shield/.output/chrome-mv3` as an unpacked extension.
3. Open a new ChatGPT or Claude web tab. Type a synthetic identifier, such as `demo@example.com`, in the chat composer and send using the supported send button or Enter. Shield should replace it with a visible `⟦EMAIL-…: XXXXX@XXXXX.XXX⟧` marker before the page sends. A blocked pattern should leave the text in the composer and show a Shield status message.
4. When the assistant echoes an unchanged marker, Shield displays the restored answer in place using an isolated extension frame. The site retains its masked text underneath, hidden only after the frame has rendered; prompts stay visibly masked. The toolbar icon opens settings: active-tab status, menu appearance, and confirmed session reset.

Each tab has an in-memory Go/WASM session map. Closing or reloading the tab loses its map; old markers cannot be restored. The settings panel has no separate map or dictionary; it can reset the active chat tab and stores only its appearance preference. The extension does not send identifiers or maps to dmz servers.

Exact markers from the current session can be submitted again without masking their placeholders. Unknown, stale, edited or nested markers block for review; begin a new test with the original fabricated value after a reload or reset. Keep only one Shield copy enabled. Live testing found nested email markers from different sessions and a Claude restored copy containing duplicate text and interface metadata. The updated extraction reads semantic answer blocks, preserving autolink text and excluding hidden elements and message controls. The owner confirmed clean Claude restoration after those fixes; the latest ChatGPT selector and inline display changes have synthetic regression coverage and still require live retesting.

The floating Shield status panel includes **Hreinsa Shield-lotu** (clear Shield session). It clears this tab's mapping and restored copies and cancels a send still waiting for replay. Old markers in the conversation cannot be restored after reset; a new prompt starts a fresh mapping. Restored copies are also removed when their source reply disappears or stops being an assistant reply.

When the detector blocks a message, the status lists only the categories and counts that need review. **Breyta texta** (edit text) returns focus to the unchanged draft. Remove or change those items and send again; Shield scans the new text. This control does not override a block or send anything itself.

## Current coverage and limits

- Content scripts run only on `https://chatgpt.com/*` and `https://claude.ai/*`. The preview gates recognized text composers on submit, Enter and recognized send-button clicks. It blocks ordinary file-picker, drag/drop and file-paste events on those pages. A website layout or event-handler change can bypass these hooks. Other send paths, mobile apps, voice and images outside those file events are not protected.
- The adapter has synthetic Chromium fixture coverage for both sites, including a page-managed input model and local response display. Additional tests capture the outgoing request body and check that replacing the composer during masking or changing its text during a nested submit prevents the request. A deliberately corrupted WASM build must show an error before any send and prevent outgoing requests. The owner has reported successful live masking on both sites and restoration on Claude; the complete authenticated verification checklist is still pending. The site may handle programmatic replay differently from the fixtures.
- Uncertain detector findings and known secret shapes stop a recognized submission for review. The detector still exposes annotated literals in the synthetic challenge and exploratory sets; unsupported identifiers can leave the device. No detection claim extends beyond the measured cases.
- A restored reply appears in an extension-origin frame, isolated from the model page by the browser's same-origin boundary. The model page keeps the masked reply, hidden while the restored frame is shown in its place. Clearing the Shield session reveals it again. Changed or incomplete markers are not restored. The frame preserves basic paragraph, heading, emphasis, list, table and code structure and selected typography using an allowlisted element tree, without copying scripts, images, event handlers or URLs. The frame uses an explicit background composited from the reply’s ancestor colours and a matching colour scheme, avoiding a white canvas behind light text. Regression tests cover light/dark mode on both adapters, background-only updates and system theme changes. Site-specific widgets and exact custom fonts are not reproduced. Site Copy/Share controls still operate on masked text; select text in the restored frame to copy the local version. Earlier preview builds used a closed shadow root; a regression probe showed that the page could read those restored values through `window.find()` and selection APIs. Replace those earlier builds before continuing tests.
- There is no global network interception or guarantee that all outgoing requests are gated. Do not use this build for sensitive data.

## Verification before daily use

The next gate is a supervised live-site smoke pass on signed-in ChatGPT and Claude, with synthetic prompts only: button, Enter, multi-turn marker reuse, blocked review, streaming reply restoration, reload/reset, and upload paths. Record the browser/version and observed outbound request body. If either site bypasses the gate, stop testing there and fix the adapter before using it again.

Record results for **each** site using the following checklist. Share outcomes and the build commit, not a browser profile, cookies, credentials or a full network archive.

- Chrome version and `BUILD.json` source commit.
- Button and Enter sends of `Repeat exactly: demo@example.com`: outgoing chat request contains an EMAIL marker and no literal email; the page's reply stays masked and the Shield frame restores the email.
- Repeat the email in the same tab: the marker stays consistent. Confirm the restored reply also updates when the reply arrives in chunks.
- Send `sk-test_key_000000000000`: no chat submission, a secret review appears, and **Breyta texta** returns to the draft. Replace it with benign text and confirm a fresh send works.
- Try attaching, dropping and pasting a fabricated file: no attachment is created.
- Reset Shield: restored copies disappear and the next masked prompt uses a new namespace. Reload: old markers remain unrestored.

Automated fixtures prove these mechanisms only against the fixture pages. Passing CI and a downloadable package do not close the authenticated-site gate. The current environment has no access to the owner's signed-in Chrome session; public-site inspection reached challenge pages.
