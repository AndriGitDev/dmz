# ADR 0006: DMZ Chromium extension and public alpha testing

Source licensing update, 31 October 2026: original code is now GPL-3.0-only under the owner’s direction. This supersedes earlier source-privacy restrictions, while research/data/model publication still requires rights review. See [publication readiness](../open-source-readiness.md).

Status: accepted owner direction · 27 October 2026

The owner requested use of **DMZ Chromium extension**, and availability of the current trained release for testing on dmz.is. This authorizes public distribution of the compiled test package, including the trained local scanner needed to run it. It does not authorize public source/research/corpus publication, a standalone model offering or a Chrome Web Store release.

Use version 0.1.2 / alpha.2 for the renamed package, retaining the current 0.1.1 Alpha 1 detector, model, tokenizer and inference runtime. Verify the original release archive and member hashes, check the new source's core/scanner parity, rebuild the renamed UI and content script, run browser/scanner checks, record the clean source/build, and publish a versioned ZIP plus SHA-256. Existing Alpha 1 assets remain intact for provenance.

Brand UI, site copy, metadata, testing instructions and Kastro references consistently. Use `/vidbot/` and `/en/extension/` for the public guide; preserve old extension addresses with redirects. The current extension source lives in `apps/dmz`; recorded releases and their original verification identifiers remain unchanged. Historical decisions are not rewritten as if the earlier name never existed.

The test download includes the existing on-device trained model and its attribution/runtime notices; do not substitute the smaller basic scanner. No raw licensed corpora, customer content, secrets or private research reports are distributed. The pinned upstream model card declares CC BY 4.0; retain attribution, modification notice and the runtime's MIT/third-party notices. Existing HMS address vocabulary is retained with its source attribution; no source address records are added.

Serve the large versioned ZIP from the existing website's dedicated persistent download volume rather than Git. The site publishes artifact version/date/size/checksum and plain bilingual Developer-mode installation, update, reset and removal steps. Alpha testing remains synthetic-only, with best-effort continuation, unchanged-attachment and pre-submit page-visibility limits stated prominently. Independent detector acceptance and any broader safety/store/legal claims remain separate.
