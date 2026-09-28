import { createHash } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { readdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const output = path.join(root, '.output/chrome-mv3');
const git = (...args) => execFileSync('git', args, { cwd: root, encoding: 'utf8' }).trim();
if (git('status', '--porcelain')) {
  throw new Error('Commit the working tree before recording a review build.');
}
const files = {};
async function hashDirectory(relative = '') {
  const entries = await readdir(path.join(output, relative), { withFileTypes: true });
  entries.sort((a, b) => a.name.localeCompare(b.name, 'en'));
  for (const entry of entries) {
    const name = relative ? `${relative}/${entry.name}` : entry.name;
    if (name === 'BUILD.json') continue;
    if (entry.isDirectory()) await hashDirectory(name);
    else if (entry.isFile()) {
      files[name] = createHash('sha256').update(await readFile(path.join(output, name))).digest('hex');
    } else throw new Error(`Unsupported build entry: ${name}`);
  }
}
await hashDirectory();
const manifest = JSON.parse(await readFile(path.join(output, 'manifest.json'), 'utf8'));
const record = {
  format: 1,
  sourceCommit: git('rev-parse', 'HEAD'),
  extensionVersion: manifest.version,
  browser: 'Chrome (Manifest V3)',
  sha256: files,
};
await writeFile(path.join(output, 'BUILD.json'), `${JSON.stringify(record, null, 2)}\n`);
console.log(`Recorded Chrome build from ${record.sourceCommit} with ${Object.keys(files).length} file checksums`);
