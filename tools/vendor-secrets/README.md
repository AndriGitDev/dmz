# Portable vendor-secret rules

`source.json` is a selected snapshot of the MIT-licensed Gitleaks configuration
at `b58d3f102cf3a2c84cb7f923d05c25c9b1aed84b`. The complete source configuration
SHA-256 is `e163e53b9e7e8a8511e77271e2b323ed057759542a6d988258afe3a1fa329caf`.
This snapshot contains public detection rules, no issued credentials.

Run `python3 tools/vendor-secrets/generate.py --check` to verify the embedded
pack, or omit `--check` to regenerate it. The one-time `--extract PATH` operation
accepts only the pinned upstream configuration and refuses an existing snapshot.
There is no automatic update or network fetch in builds or detection.

The subset has 17 rules covering DigitalOcean, GitLab, Hugging Face, Linear,
Notion, npm, PyPI, SendGrid, Shopify, Slack, Stripe and Twilio token families.
It preserves token-body regexes, entropy thresholds, keyword prefilters and
applicable global secret allowlists. The original trailing code delimiter is
replaced with DMZ's whole-token boundary checks so sentence punctuation is
supported. Original non-secret matches retain precedence. The runtime does not
import Gitleaks or any new Go module dependency.

This is a limited adapter, not behavioral equivalence with all of Gitleaks.
It excludes generic/contextual rules, client IDs, repository/path policy,
suppression comments, decoding, archives and verification. Low-entropy tokens,
unusual token revisions and tokens embedded in paths/encoded strings may be
missed. A recognizable shape is not proof of an active credential. Keep fresh
quality, false-positive and native/WASM evidence alongside changes.

The upstream MIT licence is distributed in
`apps/shield/public/licenses/gitleaks-MIT.txt`; the accompanying notice describes
DMZ's changes.
