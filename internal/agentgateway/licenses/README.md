# Notices for compiled gateway dependencies

The local coding-client gateway (`internal/agentgateway`, linked into `cmd/dmz`) decodes zstd request bodies with:

- Module: `github.com/klauspost/compress` `v1.20.4` (`go.sum` `h1:dD0v/bvxtjCBqQu6sx1TNWkalEY5lNTIe+HY9w8TeNI=`)
- Source: <https://github.com/klauspost/compress>, tag `v1.20.4` (commit `361b8081ef604c0cebc85200e61950b06d88080a`)
- Packages linked into `cmd/dmz` (checked with `go tool nm` on a `-trimpath` build): `zstd`, `zstd/internal/xxhash`, `huff0`, `fse`, `internal/cpuinfo`, `internal/le`, `internal/regmask`, `internal/snapref`

These files are unmodified copies from the module at that version:

| File | Upstream path | Covers | SHA-256 |
| --- | --- | --- | --- |
| `klauspost-compress-LICENSE` | `LICENSE` | Module license: BSD-3-Clause (The Go Authors, Klaus Post) for `zstd`, `huff0`, `fse` and internal packages, plus the per-directory sections it lists | `0d9e582ee4bff57bf1189c9e514e6da7ce277f9cd3bc2d488b22fbb39a6d87cf` |
| `klauspost-compress-zstd-internal-xxhash-LICENSE.txt` | `zstd/internal/xxhash/LICENSE.txt` | MIT (Caleb Spare) | `f566a9f97bacdaf00d9f21dd991e81dc11201c4e016c86b470799429a1c9a79c` |
| `klauspost-compress-internal-snapref-LICENSE` | `internal/snapref/LICENSE` | BSD-3-Clause (The Snappy-Go Authors) | `f69f157b0be75da373605dbc8bbf142e8924ee82d8f44f11bcaf351335bf98cf` |
| `klauspost-compress-snappy-AUTHORS` | `snappy/AUTHORS` | Author list referenced by the Snappy-Go Authors copyright | `f6f3821e64460346c1ed1a15fb32b94ad241617276b60726c1350ca315309463` |

gzip decoding uses the Go standard library (`compress/gzip`), covered by the Go distribution's license.
