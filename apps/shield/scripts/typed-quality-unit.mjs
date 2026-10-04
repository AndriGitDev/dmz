import assert from 'node:assert/strict';
import { aggregateQuality, typedRecord } from './typed-quality.mjs';

const text = 'Ása Demo';
const gold = [{ start: 0, end: 4, kind: 'name_demo' }];
const exact = typedRecord(text, gold, { spans_available: true, detected_spans: gold });
const wrongType = typedRecord(text, gold, { spans_available: true,
  detected_spans: [{ start: 0, end: 4, kind: 'address' }] });
const fallback = typedRecord(text, gold, { spans_available: false, detected_spans: null });
const negativeFallback = typedRecord(text, [], { spans_available: false, detected_spans: null });
const negative = typedRecord(text, [], { spans_available: true, detected_spans: [] });
const report = aggregateQuality([exact, wrongType, fallback, negativeFallback, negative]);
assert.equal(report.overall.accuracy, 2 / 5);
assert.equal(report.overall.recall, 1 / 3);
assert.equal(report.overall.tp, 1);
assert.equal(report.overall.fp, 1);
assert.equal(report.overall.fn, 2);
assert.equal(report.overall.abstained_excerpts, 2);
assert.equal(report.overall.abstained_negative_excerpts, 1);
assert.equal(report.independent_validation, false);
assert.equal(report.overall.point_targets_met, false);
assert.ok(!JSON.stringify(report).includes('Ása'));

for (const result of [
  {}, { spans_available: true, detected_spans: null },
  { spans_available: false, detected_spans: gold },
  ...[
    [{ start: 1, end: 4, kind: 'name_demo' }],
    [{ start: 0, end: 99, kind: 'name_demo' }],
    [{ start: 0, end: 4, kind: 'unicode_review' }],
    [gold[0], gold[0]],
    [{ start: 0, end: 4, kind: 'constructor' }],
  ].map(detected_spans => ({ spans_available: true, detected_spans })),
]) assert.throws(() => typedRecord(text, gold, result));
assert.throws(() => typedRecord(text, null, { spans_available: true, detected_spans: [] }));
console.log('Exact typed set, abstention, UTF-8 boundary and privacy checks passed');
