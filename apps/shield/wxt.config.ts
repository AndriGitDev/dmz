import { defineConfig } from 'wxt';

export default defineConfig({
  manifest: {
    name: 'dmz Shield prototype',
    description: 'Local, visible masking demo for Icelandic identifiers. Manual copy and restore.',
    version: '0.1.0',
    permissions: ['clipboardWrite'],
    content_security_policy: {
      extension_pages: "script-src 'self' 'wasm-unsafe-eval'; object-src 'self'",
    },
  },
});
