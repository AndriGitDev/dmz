import { defineConfig } from 'wxt';
import { execFileSync } from 'node:child_process';
import { version, releaseLabel } from './scripts/release-label.mjs';

const sourceCommit = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
const dirty = execFileSync('git', ['status', '--porcelain'], { encoding: 'utf8' }).trim() !== '';

export default defineConfig({
  manifest: {
    name: 'dmz',
    icons: {16:'icons/16.png',32:'icons/32.png',48:'icons/48.png',128:'icons/128.png'},
    permissions: process.env.VITE_DMZ_TRAINED_SCANNER === '1' ? ['offscreen'] : [],
    minimum_chrome_version: '116',
    description: 'Open-source Chromium extension by Kastro. Best-effort local masking on supported AI sites.',
    version,
    version_name: `${version} ${releaseLabel} ${sourceCommit}${dirty ? '-dirty' : ''}`,
    action: { default_title: 'dmz · Stillingar', default_icon: {16:'icons/16.png',32:'icons/32.png'} },
    web_accessible_resources: [{ resources: ['dmz.wasm', 'restored.html'], matches: ['https://chatgpt.com/*', 'https://claude.ai/*'] }],
    content_security_policy: {
      extension_pages: "script-src 'self' 'wasm-unsafe-eval'; object-src 'self'; connect-src 'self'; worker-src 'self'",
    },
  },
});
