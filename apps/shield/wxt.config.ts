import { defineConfig } from 'wxt';
import { execFileSync } from 'node:child_process';

const sourceCommit = execFileSync('git', ['rev-parse', 'HEAD'], { encoding: 'utf8' }).trim();
const dirty = execFileSync('git', ['status', '--porcelain'], { encoding: 'utf8' }).trim() !== '';

export default defineConfig({
  manifest: {
    name: 'dmz Shield prototype',
    description: 'Local masking for supported AI chats and a manual review side panel.',
    version: '0.1.0',
    version_name: `0.1.0 preview ${sourceCommit}${dirty ? '-dirty' : ''}`,
    permissions: ['clipboardWrite'],
    web_accessible_resources: [{ resources: ['dmz.wasm', 'restored.html'], matches: ['https://chatgpt.com/*', 'https://claude.ai/*'] }],
    content_security_policy: {
      extension_pages: "script-src 'self' 'wasm-unsafe-eval'; object-src 'self'",
    },
  },
});
