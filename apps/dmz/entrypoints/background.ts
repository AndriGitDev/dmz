import { trainedScanner } from '../utils/scanner';
export default defineBackground(() => {
  let creating: Promise<void> | undefined;
  async function ensureScanner() {
    if (await chrome.offscreen.hasDocument()) return;
    if (!creating)
      creating = chrome.offscreen
        .createDocument({
          url: 'scanner.html',
          reasons: [chrome.offscreen.Reason.WORKERS],
          justification: 'Run the device-local name scanner in one shared worker.',
        })
        .finally(() => {
          creating = undefined;
        });
    await creating;
  }
  chrome.runtime.onMessage.addListener((request, sender, respond) => {
    if (request?.type !== 'dmz:scan-names') return;
    if (
      !trainedScanner ||
      sender.id !== chrome.runtime.id ||
      sender.frameId !== 0 ||
      !sender.tab ||
      !/^https:\/\/(chatgpt\.com|claude\.ai)\//.test(sender.url || '') ||
      typeof request.text !== 'string' ||
      request.text.length > 16000
    ) {
      respond({ error: 'Unsupported scanner request' });
      return;
    }
    void ensureScanner()
      .then(() => chrome.runtime.sendMessage({ type: 'dmz:scanner-run', text: request.text }))
      .then(respond, () => respond({ error: 'Scanner unavailable' }));
    return true;
  });
  if (chrome.sidePanel?.setPanelBehavior) {
    void chrome.sidePanel.setPanelBehavior({ openPanelOnActionClick: true });
  }
});
