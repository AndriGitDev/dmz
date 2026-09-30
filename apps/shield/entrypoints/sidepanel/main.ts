import './style.css';
import { trainedScanner } from '../../utils/scanner';
if (trainedScanner) {
  const note = document.createElement('p'); note.className = 'hint';
  note.textContent = 'Þjálfaður nafnagreinir er virkur í þessari tilraunaútgáfu. Vinnsla fer fram á tækinu og getur tekið nokkrar sekúndur. Ef nafnagreinir bregst er sent með grunnmöskun og viðvörun; nöfn geta farið ómöskuð. Óháð gæðaprófun er ólokin.';
  document.querySelector('#protection-heading')!.parentElement!.append(note);
}

type TabState = { ready: boolean; composer: boolean; site: 'chatgpt' | 'claude' };
const theme = document.querySelector<HTMLSelectElement>('#theme')!;
const status = document.querySelector<HTMLElement>('#tab-status')!;
const actionStatus = document.querySelector<HTMLElement>('#action-status')!;
const reset = document.querySelector<HTMLButtonElement>('#reset')!;
const confirmReset = document.querySelector<HTMLButtonElement>('#confirm-reset')!;
const confirmation = document.querySelector<HTMLElement>('#confirmation')!;
let currentTab: number | undefined;
let pendingTab: number | undefined;
let generation = 0;

try {
  const saved = localStorage.getItem('dmz.settings.theme');
  if (saved && ['system', 'light', 'dark'].includes(saved)) theme.value = saved;
} catch { /* System appearance remains available when storage is unavailable. */ }
document.documentElement.dataset.theme = theme.value;
theme.addEventListener('change', () => {
  document.documentElement.dataset.theme = theme.value;
  try { localStorage.setItem('dmz.settings.theme', theme.value); }
  catch { actionStatus.textContent = 'Litahamur gildir núna en tókst ekki að vista hann.'; }
});

document.querySelector('#version')!.textContent = `Shield ${chrome.runtime.getManifest().version}`;
function cancelConfirmation() {
  pendingTab = undefined;
  confirmation.hidden = true;
}
async function refresh() {
  const request = ++generation;
  currentTab = undefined;
  reset.disabled = true;
  cancelConfirmation();
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (tab?.id === undefined) throw new Error('No active tab');
    const state = await chrome.tabs.sendMessage(tab.id, { type: 'dmz:settings-status' }) as TabState;
    if (request !== generation) return;
    if (!state || !['chatgpt', 'claude'].includes(state.site)) throw new Error('Unsupported tab');
    currentTab = tab.id;
    const site = state.site === 'chatgpt' ? 'ChatGPT' : 'Claude';
    status.textContent = !state.ready ? `${site}: kjarninn er ekki tilbúinn. Endurhladdu síðuna ef þetta varir.`
      : state.composer ? `${site}: Shield er tilbúinn í studdum textareit.`
      : `${site}: Shield finnur ekki studdan textareit.`;
    reset.disabled = !state.ready;
  } catch {
    if (request !== generation) return;
    status.textContent = 'Opnaðu ChatGPT eða Claude í þessum glugga. Endurhladdu síðuna ef viðbótin var nýlega uppfærð.';
  }
}
reset.addEventListener('click', () => {
  pendingTab = currentTab;
  confirmation.hidden = false;
  confirmReset.focus();
});
document.querySelector('#cancel-reset')!.addEventListener('click', () => { cancelConfirmation(); reset.focus(); });
confirmReset.addEventListener('click', async () => {
  const target = pendingTab;
  cancelConfirmation();
  if (target === undefined) return;
  actionStatus.textContent = '';
  confirmReset.disabled = true;
  reset.disabled = true;
  try {
    const [active] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (active?.id !== target) throw new Error('Tab changed');
    const result = await chrome.tabs.sendMessage(target, { type: 'dmz:settings-reset' });
    if (!result?.ok) throw new Error('Reset unavailable');
    actionStatus.textContent = 'Shield-lota hreinsuð í valda flipanum.';
  } catch { actionStatus.textContent = 'Ekki tókst að hreinsa lotuna. Athugaðu virka flipann og reyndu aftur.'; }
  finally { confirmReset.disabled = false; await refresh(); }
});
document.querySelector('#refresh')!.addEventListener('click', () => void refresh());
chrome.tabs.onActivated.addListener(() => void refresh());
chrome.tabs.onUpdated.addListener((id, change) => { if (id === currentTab && change.status === 'complete') void refresh(); });
window.addEventListener('focus', () => void refresh());
void refresh();
