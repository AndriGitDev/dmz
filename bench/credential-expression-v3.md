# Credential expressions and literal values

This 21-excerpt follow-up probe was fixed before changing the assignment rule.
It contains ten labelled credential literals and eleven negatives, including
matched code-reference, quoted JSON and unquoted dotenv interpretations of the
same character sequences. All data are fabricated. This is an exploratory
development probe, not independent annotation or a field precision estimate.

The initial rule finds all ten literals but also reports nine false positives:
unquoted variable/property references resemble high-entropy credential values.
The revised rule requires a single/double-quoted literal for its assignment
heuristic inside explicitly labelled programming-language fences. It retains all
ten literals and reduces false positives to four. These four unfenced references
remain false positives under this probe's intended source-code interpretation;
their plain text alone does not unambiguously distinguish code from dotenv data.

## Context boundary

Recognised programming-language tags are Python/py, JavaScript/js/jsx,
TypeScript/ts/tsx, Go/golang, Java, C, C++/cpp, C#/cs/csharp, Rust/rs, Ruby/rb,
PHP, Kotlin and Swift, case-insensitively. Only the assignment heuristic uses
this context. Provider-prefix, Bearer, private-key and other detector rules are
not disabled inside code. Quoted credential literals still match.

Env, dotenv, shell, YAML, JSON, plain-text and unknown language tags retain the
previous behavior. This preserves unquoted configuration literals and avoids
globally excluding dotted values just because they resemble member references.
The heuristic assumes an accurately labelled programming-language example:
unquoted literal credentials or wrongly labelled fences can therefore be missed.
The tag is context, not proof that a value cannot be a credential.

The limited fence reader handles backticks or tildes of length at least three,
up to three leading ASCII spaces, matching closing characters of sufficient
length, optional trailing spaces/tabs, and an unclosed fence through end of input.
It tracks unknown outer fences so nested-looking lines do not switch context.
These delimiter conventions follow [CommonMark's fenced-code section](https://spec.commonmark.org/0.31.2/#fenced-code-blocks).
Interpreting particular language tags as program syntax is DMZ's choice.
This is not a complete Markdown or programming-language parser: container/list
fences, indentation-only blocks, embedded languages and unsupported string
literal forms require separate handling.

## Evidence and remaining errors

Native results are TP 10, FP 4, FN 0: 71.43% precision and 100% recall on this
selected probe. Strict processing blocks all ten positive prompts and also the
four ambiguous negatives. Those errors remain in the pinned snapshots; they
must not be removed from the accuracy denominator or called successful masking.

Tests cover references versus identical quoted/env values, variable assignments,
longer/shorter closing fences, tilde/backtick mismatch, trailing text, CRLF,
original UTF-8 offsets, unclosed blocks, unknown outer blocks and preserved
provider-prefix detection. The original 42-case context corpus and its six
labelled misses remain unchanged. Packaged before/after evaluation is separate.

The packaged [before](results/quality-99-v1/credential-expression-before.json)
and [after](results/quality-99-v1/credential-expression-after.json) runs improve
both paths from 12/21 to 17/21 exact excerpts. False positives fall from nine to
four; all ten credentials remain fully masked and all 21 outputs restore exactly.
All ten preceding corpora retain their typed-detection, masking and restoration
metrics. Both runs report zero external requests and retain artifact/measurement
hashes. The after artifact is a dirty development build, not a release archive.
