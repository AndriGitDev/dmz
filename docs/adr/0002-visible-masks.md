# ADR 0002: Visible masks for direct identifiers

Date: 28 September 2026
Status: accepted for the prototype


The marker itself reveals the category and that a value was present. It does not make the surrounding prompt anonymous. A model can change or remove a marker, in which case the prototype cannot restore it. The current dmz side panel restores only complete pasted text. A Go stream restorer now handles markers divided across chunks, but it is not connected to a model route; automatic browser interception also remains future work. The side panel map exists only in memory and is removed on reset or panel close.

Product copy, demos, policy defaults and benchmarks should use *mask* or *redact*. Future research may test whether some use cases need a different transform, but a realistic stand-in is not the default.

Implementation correction, 4 October 2026: marker reuse is keyed by entity type
and the exact original bytes. Different formatting or domain-name capitalization
gets a separate marker, even when the canonical identifier is equal. Reusing one
marker for both forms previously restored every occurrence to the first spelling.
Repeating the same exact form still reuses its marker across prompts. This preserves
exact restoration; markers do not currently express that two spellings refer to
the same identifier.
