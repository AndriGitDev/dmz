import { readFile, mkdir, copyFile, writeFile } from 'node:fs/promises';
import { createHash } from 'node:crypto';
import { createReadStream } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { resolve, join } from 'node:path';
import { fileURLToPath } from 'node:url';
const root = fileURLToPath(new URL('../', import.meta.url));
const assets = process.env.DMZ_SCANNER_ASSETS;
if (!assets || resolve(assets).startsWith(resolve(root, '../..') + '/'))
  throw Error('Set DMZ_SCANNER_ASSETS to private external assets');
const manifest = JSON.parse(await readFile(join(assets, 'manifest.json'), 'utf8'));
if (
  manifest.schema !== 1 ||
  manifest.model !== 'fp16storage' ||
  manifest.files['model.onnx'].sha256 !==
    'e5acd596fde1873c6f4a5620db9c5417f11a59f3ad70cd96423614dd667b4690' ||
  manifest.crf_source_sha256 !== '85b436a7f80c5b42f1a90ea6d5fbbc2e1e134c356fd2235adec1956f2ca6ca9c'
)
  throw Error('Unreviewed model');
if (
  manifest.files['tokenizer.json'].sha256 !==
    '0305755b907b7a616fed8aa81aa0d12fe45a0916111988189b8646cc066fe20a' ||
  manifest.files['crf.json'].sha256 !==
    'b777808dee5c9eecc5596912aca46c5f65692acd51b0e632b9c55e4bdf0f0419'
)
  throw Error('Unreviewed preprocessing assets');
for (const name of ['model.onnx', 'tokenizer.json', 'crf.json']) {
  const hash = createHash('sha256');
  let bytes = 0;
  for await (const chunk of createReadStream(join(assets, name))) {
    hash.update(chunk);
    bytes += chunk.length;
  }
  if (hash.digest('hex') !== manifest.files[name].sha256 || bytes !== manifest.files[name].bytes)
    throw Error('Asset checksum mismatch');
}
execFileSync('npm', ['run', 'build'], {
  cwd: root,
  stdio: 'inherit',
  env: { ...process.env, VITE_DMZ_TRAINED_SCANNER: '1' },
});
const output = join(root, '.output/chrome-mv3/scanner');
await mkdir(join(output, 'runtime'), { recursive: true });
for (const name of ['model.onnx', 'tokenizer.json', 'crf.json', 'manifest.json'])
  await copyFile(join(assets, name), join(output, name));
for (const name of ['text.mjs', 'worker.mjs'])
  await copyFile(join(root, 'scanner', name), join(output, name));
const runtime = resolve(root, '../../bench/browser-detector/node_modules/onnxruntime-web');
const pkg = JSON.parse(await readFile(join(runtime, 'package.json'), 'utf8'));
if (pkg.version !== '1.23.2') throw Error('Unexpected ONNX runtime');
for (const name of [
  'ort.wasm.min.mjs',
  'ort-wasm-simd-threaded.mjs',
  'ort-wasm-simd-threaded.wasm',
])
  await copyFile(join(runtime, 'dist', name), join(output, 'runtime', name));
await copyFile(join(runtime, 'README.md'), join(output, 'runtime', 'README.md'));
for (const name of ['onnxruntime-LICENSE.txt', 'onnxruntime-ThirdPartyNotices.txt'])
  await copyFile(join(root, 'scanner/licenses', name), join(output, 'runtime', name));
await writeFile(
  join(output, 'NOTICE.txt'),
  'Private trained preview. Model and tokenizer derive from mideind/IceBERT, whose pinned model card declares CC BY 4.0: https://huggingface.co/mideind/IceBERT/tree/24c145af7b7d092ecb082897db2e16107301d657 . Licence: https://creativecommons.org/licenses/by/4.0/ . DMZ modified the model through fine-tuning, name-classification layers and storage conversion. No endorsement by the original authors is implied. ONNX Runtime is MIT licensed; its licence and third-party notices are included under runtime/. Independent validation and distribution review remain pending.\n',
);
console.log('Built trained preview with checked local assets. No remote model fetch.');
