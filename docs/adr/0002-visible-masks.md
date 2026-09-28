# ADR 0002: Visible masks for direct identifiers

Date: 28 September 2026
Status: accepted for the prototype

The original business plan proposed realistic stand-ins. The founder instead wants redaction to be obvious: a kennitala should appear as `XXXXX-XXXX`, not as another plausible kennitala. The prototype emits a typed marker such as `⟦KT-<random session namespace>-1: XXXXX-XXXX⟧`. A second distinct kennitala receives a different marker, while repeated occurrences of the same value reuse its marker in the session. The random namespace prevents a response from an older session being restored to a different real value after reset. That distinction permits exact restoration if a model returns the marker unchanged.

The marker itself reveals the category and that a value was present. It does not make the surrounding prompt anonymous. A model can change or remove a marker, in which case the prototype cannot restore it. The current Shield side panel restores only complete pasted text. A Go stream restorer now handles markers divided across chunks, but it is not connected to a model route; automatic browser interception also remains future work. The side panel map exists only in memory and is removed on reset or panel close.

Product copy, demos, policy defaults and benchmarks should use *mask* or *redact*. Future research may test whether some use cases need a different transform, but a realistic stand-in is not the default.
