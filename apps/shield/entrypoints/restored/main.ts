import './style.css';

const nonce = location.hash.slice(1);
const output = document.querySelector<HTMLDivElement>('#restored-text')!;
window.addEventListener('message', (event: MessageEvent) => {
  if (event.source !== parent || !['https://chatgpt.com', 'https://claude.ai'].includes(event.origin)) return;
  const data = event.data;
  if (!nonce || data?.type !== 'dmz:restored' || data.nonce !== nonce || typeof data.text !== 'string') return;
  output.textContent = data.text;
  requestAnimationFrame(() => {
    parent.postMessage({ type: 'dmz:restored-height', nonce, height: document.documentElement.scrollHeight }, event.origin);
  });
});
