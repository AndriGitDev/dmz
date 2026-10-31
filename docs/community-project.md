# dmz: open-source Chromium extension

Owner direction, 31 October 2026: the product is **dmz**, the free Chromium extension. Original code in dmz and dmz-web uses **GPL-3.0-only**. See each repository's LICENSE. This supersedes earlier scope and source-publication decisions.

Help users reduce accidental sharing of selected direct identifiers in AI text submissions, with device-local processing, understandable warnings and reviewable source. No account or payment is required for local dmz use. The AI site's own terms and costs remain separate.

The current runtime is best effort: host-page scripts can see the original draft; failures warn and continue; files, images and attachments pass through unchanged. Missed identifiers and context can still identify people. Preserve this behavior and its warnings when changing positioning.

The project consists of the extension, its detector and this supporting website. The extension source lives in `apps/dmz`; active scripts, tests and documentation use the dmz name. No hosted product or additional service is promised.

Source licensing covers original code, not rights belonging to upstream data/model owners. Preserve attribution and verify every asset before redistribution. Research publication remains limited to reviewed summaries until data and publication rights are established. See [readiness](open-source-readiness.md), [contribution guidance](../CONTRIBUTING.md) and [third-party materials](../THIRD_PARTY.md).

Use fabricated-data bug reports, accessibility work, translations and code review for participation. Report vulnerabilities privately. Keep the website Icelandic-first with an English mirror, canonical URLs and hreflang links. No trackers, third-party embeds or prompt collection.
