// Private original-text replay through the packaged extension worker.
import assert from 'node:assert/strict';
import { readFile, mkdtemp, rm, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
import { mergeNames } from '../scanner/text.mjs';
const [source, crfFile, referenceFile, output] = process.argv.slice(2);
const sourceBytes = await readFile(source),
  crfBytes = await readFile(crfFile),
  referenceBytes = await readFile(referenceFile);
const sha = (b) => createHash('sha256').update(b).digest('hex');
const rows = sourceBytes.toString().trim().split('\n').map(JSON.parse),
  crf = JSON.parse(crfBytes),
  reference = JSON.parse(referenceBytes);
assert.equal(crf.input_sha256, sha(sourceBytes));
assert.equal(reference.sets[0].input_sha256, sha(sourceBytes));
assert.equal(reference.text_mode, 'original');
const profile = await mkdtemp(join(tmpdir(), 'dmz-scanner-corpus-'));
const extension = fileURLToPath(new URL('../.output/chrome-mv3', import.meta.url));
let browser;
const report = {
  scope:
    'Reused synthetic domain original-text replay through packaged extension tokenizer, CRF and neural worker; excludes Go policy/masking and live-site behavior',
  independent_validation: false,
  records: rows.length,
  changedFromReference: 0,
  tp: 0,
  fp: 0,
  fn: 0,
  externalRequests: [],
  inputs: { source: sha(sourceBytes), crf: sha(crfBytes), reference: sha(referenceBytes) },
};
try {
  browser = await chromium.launchPersistentContext(profile, {
    headless: true,
    channel: 'chromium',
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
    args: [`--disable-extensions-except=${extension}`, `--load-extension=${extension}`],
  });
  browser.on('request', (r) => {
    if (!r.url().startsWith('chrome-extension://')) report.externalRequests.push(r.url());
  });
  let sw = browser.serviceWorkers()[0];
  if (!sw) sw = await browser.waitForEvent('serviceworker');
  await sw.evaluate(() =>
    chrome.offscreen.createDocument({
      url: 'scanner.html',
      reasons: [chrome.offscreen.Reason.WORKERS],
      justification: 'Local scanner parity test',
    }),
  );
  report.browser = browser.browser()?.version() || 'Chromium persistent context';
  const started = Date.now();
  for (let i = 0; i < rows.length; i++) {
    const result = await sw.evaluate(
      (text) => chrome.runtime.sendMessage({ type: 'dmz:scanner-run', text }),
      rows[i].original_text,
    );
    assert.ok(!result.error, `Scanner failed on record ${i}`);
    const convert = (values) => values.map(([start, end]) => ({ start, end }));
    const expected = mergeNames(
      convert(crf.records[i].predictions.filter((p) => p[2] >= 0.75)),
      convert(reference.sets[0].records[i].predictions),
    );
    if (JSON.stringify(result.spans) !== JSON.stringify(expected)) report.changedFromReference++;
    const gold = new Set(reference.sets[0].records[i].gold.map((s) => s.join(':'))),
      actual = new Set(result.spans.map((s) => s.start + ':' + s.end));
    report.tp += [...actual].filter((s) => gold.has(s)).length;
    report.fp += [...actual].filter((s) => !gold.has(s)).length;
    report.fn += [...gold].filter((s) => !actual.has(s)).length;
    if ((i + 1) % 64 === 0)
      console.log(JSON.stringify({ records: i + 1, seconds: (Date.now() - started) / 1000 }));
  }
  report.seconds = (Date.now() - started) / 1000;
  report.precision = report.tp / (report.tp + report.fp);
  report.recall = report.tp / (report.tp + report.fn);
  assert.deepEqual(report.externalRequests, []);
  assert.equal(report.changedFromReference, 0);
} finally {
  if (browser) await browser.close();
  await rm(profile, { recursive: true, force: true });
  await writeFile(output, JSON.stringify(report, null, 2) + '\n', { flag: 'wx' });
}
console.log(JSON.stringify(report));
