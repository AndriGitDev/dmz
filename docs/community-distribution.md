# Community preview distribution

A free tool still needs an identified artifact and truthful release status. Existing rule-based CI and separately prepared trained alpha packages are previews, not proof of a store release or independent detector quality.

## Maintainer process

1. Choose a source commit and scanner profile. Verify every bundled code/model/dataset license for the intended audience; do not infer public redistribution rights from private preview approval.
2. Build and run the existing Go, WASM and browser gates for the affected artifact. Keep the trained and basic profiles explicit, including download size, first-scan delay and fallback limitations.
3. Record a clean build using the existing `apps/shield/scripts/record-build.mjs`; verify manifest/source identity and all file checksums. Never call a dirty build a recorded release.
4. Include bilingual install/update/reset/remove instructions, fabricated examples and a feedback template. State that failures continue with warnings, attachments pass unchanged and host-page scripts can read the original composer draft before masking.
5. Use the current private alpha packaging workflow only for its intended version/profile. Do not rename an Alpha 1 package to imply a new release or weaken its clean-build/checksum guards for a positioning change.
6. Link a public download/store listing only after it exists and the selected package and user-data disclosures are verified. Store submission/publication is a distinct action, not implied by a website copy change.
7. Publish reviewed release notes and supported paths. Feedback is voluntary and synthetic-only; private security reports go through Kastro's established contact. Do not include customer text, model recipes, weights or private research in public release notes.

## User guidance

Public bilingual guidance lives at `dmz.is/skjoldur/` and `dmz.is/en/shield/`. For an already supplied preview: extract the archive, load its manifest folder in Chromium Developer mode, reload supported chat tabs and verify build/version in settings. Test only fabricated text. Install no duplicate Shield copies. Update from a verified replacement package, reload extension/tabs, or remove through the browser's Extensions page. Reset/reload/close loses prior session mappings; uninstalling does not recall text sent to an AI service.

## Outstanding release evidence

Independent quality acceptance, exact public artifact/redistribution approval, final operator/privacy/retention details and store status remain unresolved. Neither this document nor free community positioning closes those gates. No automatic collection of prompts, paid account requirement or premium masking capability is introduced.
