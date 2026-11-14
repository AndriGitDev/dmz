# Notices for compiled routing dependencies

The companion routing planner (`internal/companion/routing`) parses and encodes local Codex settings with:

- Module: `github.com/pelletier/go-toml/v2` `v2.4.3` (`go.sum` `h1:GTRvJQutkOSftxIFD5xw9aepkYNuPWmVJpffdDPYVpY=`)
- Source: <https://github.com/pelletier/go-toml>, tag `v2.4.3` (commit `071a36c2a57244f2e70369bfc69889fda2a1f60f`)
- Version choice: v2.4.3 is the release that fixes GHSA-frc4-6h9q-39fq (unbounded parser recursion, affected `<= v2.4.2`). The module has no non-test dependencies.

This file is an unmodified copy from the module at that version:

| File | Upstream path | Covers | SHA-256 |
| --- | --- | --- | --- |
| `pelletier-go-toml-v2-LICENSE` | `LICENSE` | MIT (Thomas Pelletier) for the whole module | `26844e4b53c5adec04e557fd7dfef281cc0205a7d355626b1c68b778b99e9e7b` |
