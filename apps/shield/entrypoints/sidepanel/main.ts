import './style.css';
import { trainedScanner } from '../../utils/scanner';
if (trainedScanner) {
  const note = document.createElement('p'); note.className = 'hint';
  note.textContent = 'Þessi tilraunaútgáfa inniheldur þjálfaðan nafnagreinir. Vinnslan er í hléi þegar DMZ er í hléi. Vinnsla fer fram á tækinu og getur tekið nokkrar sekúndur. Ef nafnagreinir bregst er sent með grunnmöskun og viðvörun; nöfn geta farið ómöskuð. Óháð gæðaprófun er ólokin.';
  document.querySelector('#protection-heading')!.parentElement!.append(note);
}

type TabState = { ready: boolean; composer: boolean; site: 'chatgpt' | 'claude'; pausedUntil?: number };
const theme = document.querySelector<HTMLSelectElement>('#theme')!;
const status = document.querySelector<HTMLElement>('#tab-status')!;
const actionStatus = document.querySelector<HTMLElement>('#action-status')!;
const reset = document.querySelector<HTMLButtonElement>('#reset')!;
const pause = document.querySelector<HTMLButtonElement>('#pause')!;
const pauseStatus = document.querySelector<HTMLElement>('#pause-status')!;
const pauseFeedback = document.querySelector<HTMLElement>('#pause-feedback')!;
let currentPausedUntil = 0;
let pauseRefresh: ReturnType<typeof setTimeout> | undefined;
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

const manifest = chrome.runtime.getManifest();
const alpha = manifest.version_name?.match(/^\S+ alpha\.(\d+) /)?.[1];
if (alpha) document.querySelector('#release-label')!.textContent = `DMZ / ALPHA ${alpha}`;
document.querySelector('#version')!.textContent = `DMZ ${manifest.version}${alpha ? ` · Alpha ${alpha}` : ''}`;
function cancelConfirmation() {
  pendingTab = undefined;
  confirmation.hidden = true;
}
async function refresh() {
  const request = ++generation;
  currentTab = undefined;
  reset.disabled = true;
  pause.disabled = true;
  currentPausedUntil = 0;
  if (pauseRefresh) clearTimeout(pauseRefresh);
  pauseRefresh = undefined;
  cancelConfirmation();
  try {
    const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (tab?.id === undefined) throw new Error('No active tab');
    const state = await chrome.tabs.sendMessage(tab.id, { type: 'dmz:settings-status' }) as TabState;
    if (request !== generation) return;
    if (!state || !['chatgpt', 'claude'].includes(state.site)) throw new Error('Unsupported tab');
    currentTab = tab.id;
    currentPausedUntil = Number.isFinite(state.pausedUntil) && state.pausedUntil! > Date.now() ? state.pausedUntil! : 0;
    pause.disabled = false;
    pause.setAttribute('aria-pressed', String(Boolean(currentPausedUntil)));
    pause.textContent = currentPausedUntil ? 'Virkja DMZ aftur' : 'Setja DMZ í 15 mínútna hlé';
    pauseStatus.textContent = currentPausedUntil
      ? `DMZ er í hléi í þessum flipa til ${new Date(currentPausedUntil).toLocaleTimeString('is-IS', { hour: '2-digit', minute: '2-digit' })}. Texti er sendur án möskunar.`
      : 'DMZ er ekki í hléi í þessum flipa. Endurhleðsla eða lokun flipans lýkur einnig hléi.';
    if (currentPausedUntil) pauseRefresh = setTimeout(() => void refresh(), Math.max(1, currentPausedUntil - Date.now() + 20));
    const site = 'AI-vefur';
    status.textContent = currentPausedUntil ? `${site}: DMZ er í hléi. Texti er sendur án möskunar.`
      : !state.ready ? `${site}: kjarninn er ekki tilbúinn. Endurhladdu síðuna ef þetta varir.`
      : state.composer ? `${site}: DMZ er tilbúinn í studdum textareit.`
      : `${site}: DMZ finnur ekki studdan textareit.`;
    reset.disabled = !state.ready;
  } catch {
    if (request !== generation) return;
    pause.setAttribute('aria-pressed', 'false');
    pause.textContent = 'Setja DMZ í 15 mínútna hlé';
    pauseStatus.textContent = 'Opnaðu studdan samtalsflipa til að breyta þessari stillingu.';
    status.textContent = 'Opnaðu studdan AI-vef í þessum glugga. Endurhladdu síðuna ef viðbótin var nýlega uppfærð.';
  }
}
pause.addEventListener('click', async () => {
  const target = currentTab;
  if (target === undefined) return;
  const request = ++generation;
  const paused = !currentPausedUntil;
  pause.disabled = true;
  pauseFeedback.textContent = '';
  cancelConfirmation();
  try {
    const [active] = await chrome.tabs.query({ active: true, currentWindow: true });
    if (active?.id !== target || request !== generation) throw new Error('Tab changed');
    const result = await chrome.tabs.sendMessage(target, { type: 'dmz:settings-pause', paused });
    if (!result?.ok) throw new Error('Pause unavailable');
  } catch {
    if (request === generation) pauseFeedback.textContent = 'Ekki tókst að breyta hléi. Athugaðu virka flipann og reyndu aftur.';
  } finally { await refresh(); }
});
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
    actionStatus.textContent = 'DMZ-lota hreinsuð í valda flipanum.';
  } catch { actionStatus.textContent = 'Ekki tókst að hreinsa lotuna. Athugaðu virka flipann og reyndu aftur.'; }
  finally { confirmReset.disabled = false; await refresh(); }
});
document.querySelector('#refresh')!.addEventListener('click', () => void refresh());
chrome.tabs.onActivated.addListener(() => void refresh());
chrome.tabs.onUpdated.addListener((id, change) => { if (id === currentTab && change.status === 'complete') void refresh(); });
window.addEventListener('focus', () => void refresh());
void refresh();
