import { readFileSync } from 'node:fs';

export const version = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8')).version;
export const releaseLabel = process.env.DMZ_RELEASE_LABEL || 'preview';
if (releaseLabel !== 'preview' && !/^alpha\.[1-9]\d*$/.test(releaseLabel)) {
  throw new Error('DMZ_RELEASE_LABEL must be preview or alpha.<number>');
}
export const alphaNumber = releaseLabel.startsWith('alpha.') ? releaseLabel.slice(6) : null;
