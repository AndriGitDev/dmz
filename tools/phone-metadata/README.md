# Phone numbering metadata import

This native Go build-time tool imports the general and subscriber-type patterns
from Google libphonenumber **v9.0.40**, through the exact Go module
`github.com/nyaruka/phonenumbers/v2 v2.0.14`. Its protobuf dependency must not
enter the browser/core module: TinyGo compiles it but fails during startup.

Regenerate from this directory:

```sh
go run . ../../packages/detect/phone-plans.json
```

The output is deterministic and contains public numbering rules, no subscriber
records or model weights. Review the generated diff, run the international phone
probe and Go/WASM checks, and update provenance when advancing the upstream pin.
Do not silently download fresh metadata at runtime.

The runtime adapter is narrower than libphonenumber: explicit `+` country codes,
complete national numbers, supported numeric separators and optional numeric
extensions. It does not infer a domestic region, parse vanity numbers, or remove
international trunk prefixes. Existing Icelandic local-number detection remains.

Upstream licences and attribution are distributed with Shield under
`apps/shield/public/licenses/`: `libphonenumber-Apache-2.0.txt`,
`phonenumbers-MIT.txt`, and `phone-numbering-NOTICE.txt`.
