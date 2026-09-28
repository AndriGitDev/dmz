import { defineConfig } from 'wxt';

export default defineConfig({
  manifest: {
    name: 'dmz Shield prototype',
    description: 'Local masking for supported AI chats and a manual review side panel.',
    version: '0.1.0',
    permissions: ['clipboardWrite'],
    web_accessible_resources: [{ resources: ['dmz.wasm'], matches: ['https://chatgpt.com/*', 'https://claude.ai/*'] }],
    content_security_policy: {
      extension_pages: "script-src 'self' 'wasm-unsafe-eval'; object-src 'self'",
    },
  },
});
