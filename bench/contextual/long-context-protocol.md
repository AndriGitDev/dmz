# Long-prompt chunk-boundary diagnostic

Declared before inference on 5 October 2026. All existing general/domain
Person development excerpts fit one encoder chunk, so their scores do not test
name spans crossing the scanner's chunk boundaries. This separate synthetic
probe tests that behavior with the unchanged frozen packaged model.

Use the eight fabricated `DEV_NAMES` nominative forms from `domain.py`, retaining
all eight. Create four nine-excerpt corpora: a short control, long text with a
name crossing an actual tokenizer chunk cut, a nearby long-text interior control,
and the crossing text in NFD. Each also includes one explicit name-free control.
All names have been used in earlier synthetic development; these are reused
stress cases, not an independent sample or representative long-text evaluation.

The clause is `Umsækjandi er NAME. Beiðni bíður yfirferðar.`. Long text adds
exactly 24 copies of `Þetta er tilbúinn texti um almenna skráningu og afgreiðslu.`.
Search only mechanical tokenizer layouts, with ordered intros ``, `Athugið.`,
`Til upplýsinga.`, `Skráning stendur yfir.`, `Þetta er dæmi.` and 0–24 filler
sentences before the clause. Take the first layout whose name crosses a chunk
cut. Take the nearest non-crossing filler placement with at least four words
between the name and an interior cut; ties prefer the lower placement. Keep the
same intro and total filler count for that paired control. Fail construction
rather than dropping a name if no such layout exists.

Use the checked current tokenizer and production preprocessing, without model
inference. Record source/tokenizer hashes, UTF-8 gold boundaries, word indices,
chunk offsets, text sizes and corpus hashes. NFD variants must preserve normalized
words and cuts. Keep every generated case and freeze all four corpus hashes before
scoring. Do not select names or layouts by detector results.

Replay all four through the existing packaged browser/core harness. Report exact
sets, recall, full masking, restoration, false detections, fallback and elapsed
time separately by corpus and path. Do not pool these correlated controls into a
headline accuracy or interpret elapsed time under concurrent training as latency.
A failure may motivate a separately declared implementation experiment; passing
these few cases does not prove long-prompt reliability. No production chunking,
model, threshold or training change is part of this diagnostic.
