# dmz threat model

Status: outline, not a completed assessment.

## Assets and boundaries

Original text, detected direct identifiers and per-tab restoration mappings are sensitive. dmz processes them on device. The AI site's composer is controlled by the website; its scripts can read the original draft and input events before submission. Local masking does not hide pre-submit text from that site. This describes possible access, not evidence that a specific site sends telemetry.

Restored replies appear in an extension-origin frame; the host page retains masked text. Reset, reload and tab closure discard the mapping. Source availability allows review but does not establish safety or authenticate a downloaded binary.

## Cases requiring regression coverage

- Detection misses, false positives and identifying context left in text.
- Cross-tab mapping leakage, wrong restoration and stale asynchronous results overwriting edits.
- Site changes and unsupported send paths bypassing interception.
- Scanner/core failures continuing with visible warnings and reduced or absent masking.
- Unchanged attachments and a temporary pause allowing unmasked submission.
- Prompt contents entering logs, exceptions, reports or support tooling.
- Tampered dependencies, release packages or misleading forks.

Use fabricated examples. Preserve browser-origin isolation checks, per-tab state and release/source identity. GPL obligations cannot prevent malicious copies. Independent quality evaluation and a completed security assessment remain separate work.
