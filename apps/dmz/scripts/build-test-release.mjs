import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { cp, readFile } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = fileURLToPath(new URL('../', import.meta.url));
const baseline = process.env.DMZ_BASELINE_EXTENSION;
if (!baseline || resolve(baseline).startsWith(resolve(root, '../..') + '/'))
  throw new Error('Set DMZ_BASELINE_EXTENSION to the verified external Alpha 1 extension directory.');
const original = JSON.parse(await readFile(join(baseline, 'BUILD.json'), 'utf8'));
if (original.sourceCommit !== '5b498334302506e3c490cee9f20a8dde494ec4fe' || original.scannerProfile !== 'trained-fp16storage')
  throw new Error('Expected the recorded current trained Alpha 1 release.');
const digest = async (path) => createHash('sha256').update(await readFile(path)).digest('hex');
for (const [name, expected] of Object.entries(original.sha256)) {
  if (name.startsWith('/') || name.split('/').includes('..') || await digest(join(baseline, name)) !== expected)
    throw new Error(`Baseline checksum mismatch: ${name}`);
}
if (original.sha256['scanner/model.onnx'] !== 'e5acd596fde1873c6f4a5620db9c5417f11a59f3ad70cd96423614dd667b4690')
  throw new Error('Unreviewed trained model.');
for (const name of ['dmz.wasm', 'detector-data.json', 'wasm_exec.js']) {
  if (await digest(join(root, 'public', name)) !== original.sha256[name])
    throw new Error(`The current ${name} differs from the trained release; rebuild and revalidate the detector explicitly.`);
}
for (const name of ['worker.mjs', 'text.mjs']) {
  if (await digest(join(root, 'scanner', name)) !== original.sha256[`scanner/${name}`])
    throw new Error(`Scanner code differs from the release: ${name}`);
}
execFileSync('npm', ['run', 'build'], {
  cwd: root, stdio: 'inherit',
  env: { ...process.env, VITE_DMZ_TRAINED_SCANNER: '1' },
});
const output = join(root, '.output/chrome-mv3');
await cp(join(baseline, 'scanner'), join(output, 'scanner'), { recursive: true });
// Preserve detector/model/tokenizer/runtime bytes; use the renamed current UI and content script.
for (const name of Object.keys(original.sha256).filter(name => name.startsWith('scanner/'))) {
  if (await digest(join(output, name)) !== original.sha256[name]) throw new Error(`Copied scanner mismatch: ${name}`);
}
console.log('Built DMZ Chromium extension with the verified Alpha 1 trained scanner unchanged.');
