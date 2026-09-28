import './style.css';
import { restoredTags, restoredStyles, type RestoredNode } from '../../utils/restored-model';

const nonce = location.hash.slice(1);
const output = document.querySelector<HTMLDivElement>('#restored-text')!;
let parentOrigin = '';
function reportHeight() {
  if (parentOrigin) parent.postMessage({ type: 'dmz:restored-height', nonce, height: Math.ceil(output.getBoundingClientRect().height) }, parentOrigin);
}
new ResizeObserver(reportHeight).observe(output);
function render(node: RestoredNode): Node {
  if ('text' in node) return document.createTextNode(node.text);
  const element = document.createElement(restoredTags.has(node.tag) ? node.tag : 'span');
  for (const key of restoredStyles) {
    if (typeof node.style?.[key] === 'string') element.style.setProperty(key, node.style[key]!);
  }
  element.append(...node.children.map(render));
  return element;
}
window.addEventListener('message', (event: MessageEvent) => {
  if (event.source !== parent || !['https://chatgpt.com', 'https://claude.ai'].includes(event.origin)) return;
  const data = event.data;
  if (!nonce || data?.type !== 'dmz:restored' || data.nonce !== nonce || typeof data.text !== 'string') return;
  parentOrigin = event.origin;
  if (typeof data.surface?.background === 'string' && CSS.supports('background-color', data.surface.background)) {
    document.documentElement.style.backgroundColor = data.surface.background;
  }
  if (data.surface?.colorScheme === 'dark' || data.surface?.colorScheme === 'light') {
    document.documentElement.style.colorScheme = data.surface.colorScheme;
  }
  for (const key of ['color', 'font-family', 'font-size', 'font-weight', 'font-style', 'line-height', 'letter-spacing', 'text-align']) {
    if (typeof data.typography?.[key] === 'string') output.style.setProperty(key, data.typography[key]);
  }
  if (Array.isArray(data.nodes)) output.replaceChildren(...data.nodes.map(render));
  else output.textContent = data.text;
  requestAnimationFrame(reportHeight);
});
