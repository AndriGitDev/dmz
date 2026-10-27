import { defineConfig } from 'wxt';
import { execFileSync } from 'node:child_process';
import { version, releaseLabel, alphaNumber } from './scripts/release-label.mjs';

const sourceCommit = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
const dirty = execFileSync('git', ['status', '--porcelain'], { encoding: 'utf8' }).trim() !== '';

export default defineConfig({
  manifest: {
    name: alphaNumber ? `dmz Shield · Alpha ${alphaNumber}` : process.env.VITE_DMZ_TRAINED_SCANNER === '1' ? 'dmz Shield · trained preview' : 'dmz Shield',
    permissions: process.env.VITE_DMZ_TRAINED_SCANNER === '1' ? ['offscreen'] : [],
    minimum_chrome_version: '116',
    description: 'Free community tool by Kastro. Best-effort local masking for supported ChatGPT and Claude chats.',
    version,
    version_name: `${version} ${releaseLabel} ${sourceCommit}${dirty ? '-dirty' : ''}`,
    action: { default_title: 'dmz Shield · Stillingar' },
    web_accessible_resources: [{ resources: ['dmz.wasm', 'restored.html'], matches: ['https://chatgpt.com/*', 'https://claude.ai/*'] }],
    content_security_policy: {
      extension_pages: "script-src 'self' 'wasm-unsafe-eval'; object-src 'self'; connect-src 'self'; worker-src 'self'",
    },
  },
});
