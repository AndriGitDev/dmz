import './style.css';

type Finding = { kind: string; confidence: 'high' | 'uncertain'; action: string; reason: string };
type Result = { allowed: boolean; transformed?: string; findings: Finding[]; error?: string };
type GoRuntime = { importObject: WebAssembly.Imports; run(instance: WebAssembly.Instance): Promise<void> };

declare global {
  interface Window {
    Go: new () => GoRuntime;
    dmzProcess: (input: string) => string;
    dmzRestore: (input: string) => string;
    dmzReset: () => boolean;
    dmzSetTenantTerms: (json: string) => string;
  }
}

const input = document.querySelector<HTMLTextAreaElement>('#prompt')!;
const masked = document.querySelector<HTMLTextAreaElement>('#masked')!;
const answer = document.querySelector<HTMLTextAreaElement>('#answer')!;
const restored = document.querySelector<HTMLTextAreaElement>('#restored')!;
const maskButton = document.querySelector<HTMLButtonElement>('#mask')!;
const copyButton = document.querySelector<HTMLButtonElement>('#copy')!;
const restoreButton = document.querySelector<HTMLButtonElement>('#restore')!;
const findings = document.querySelector<HTMLDivElement>('#findings')!;
const findingCount = document.querySelector<HTMLSpanElement>('#finding-count')!;
const status = document.querySelector<HTMLSpanElement>('#status')!;
const tenantTerms = document.querySelector<HTMLTextAreaElement>('#tenant-terms')!;
const saveTerms = document.querySelector<HTMLButtonElement>('#save-terms')!;

async function loadCore() {
  const response = await fetch(chrome.runtime.getURL('dmz.wasm'));
  if (!response.ok) throw new Error('WASM asset unavailable');
  const go = new window.Go();
  const { instance } = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
  void go.run(instance);
  for (let i = 0; i < 50 && typeof window.dmzProcess !== 'function'; i++) {
    await new Promise((resolve) => setTimeout(resolve, 10));
  }
  if (typeof window.dmzProcess !== 'function') throw new Error('WASM core did not start');
  maskButton.disabled = false;
  saveTerms.disabled = false;
  status.textContent = 'Kjarninn keyrir á tækinu þínu';
}

function showFindings(items: Finding[]) {
  findings.replaceChildren();
  findingCount.textContent = items.length ? `${items.length} greiningar` : 'Engin greining';
  for (const item of items) {
    const row = document.createElement('div');
    row.className = 'finding';
    const kind = document.createElement('strong');
    kind.textContent = item.kind.replaceAll('_', ' ').toUpperCase();
    const detail = document.createElement('span');
    detail.textContent = `${item.action} · ${item.confidence}`;
    row.append(kind, detail);
    findings.append(row);
  }
}

maskButton.addEventListener('click', () => {
  const result = JSON.parse(window.dmzProcess(input.value)) as Result;
  showFindings(result.findings ?? []);
  restored.value = '';
  if (result.error || !result.allowed) {
    masked.value = '';
    copyButton.disabled = true;
    restoreButton.disabled = true;
    status.textContent = 'Stöðvað: óviss eða óstudd gögn';
    return;
  }
  masked.value = result.transformed ?? '';
  copyButton.disabled = !masked.value;
  restoreButton.disabled = false;
  status.textContent = 'Maskað. Afritaðu aðeins textann í reit 02.';
});

input.addEventListener('input', () => { masked.value = ''; copyButton.disabled = true; });

saveTerms.addEventListener('click', () => {
  const terms = tenantTerms.value.split(/\r?\n/).map((term) => term.trim()).filter(Boolean);
  const result = JSON.parse(window.dmzSetTenantTerms(JSON.stringify(terms))) as { ok?: boolean; error?: string };
  if (!result.ok) {
    status.textContent = 'Orðabók ógild: 3–80 stafir í heiti, mest 32 heiti.';
    return;
  }
  masked.value = '';
  answer.value = '';
  restored.value = '';
  findings.replaceChildren();
  findingCount.textContent = '';
  copyButton.disabled = true;
  restoreButton.disabled = true;
  status.textContent = `${terms.length} heiti vistuð í minni. Ný lota hófst.`;
});

copyButton.addEventListener('click', async () => {
  if (!masked.value) return;
  await navigator.clipboard.writeText(masked.value);
  status.textContent = 'Maskaði textinn var afritaður.';
});

restoreButton.addEventListener('click', () => {
  restored.value = window.dmzRestore(answer.value);
  status.textContent = restored.value.includes('⟦') ? 'Athugaðu merkimiða: sumir kunna að hafa breyst.' : 'Raungildi sett aftur inn á tækinu.';
});

document.querySelector<HTMLButtonElement>('#reset')!.addEventListener('click', () => {
  if (typeof window.dmzReset === 'function') window.dmzReset();
  for (const field of [input, masked, answer, restored]) field.value = '';
  tenantTerms.value = '';
  findings.replaceChildren();
  findingCount.textContent = '';
  copyButton.disabled = true;
  restoreButton.disabled = true;
  status.textContent = 'Lota hreinsuð úr minni.';
});

void loadCore().catch((error: unknown) => {
  const detail = error instanceof Error ? error.message : 'Óþekkt villa';
  status.textContent = `Ekki tókst að hlaða WASM-kjarnanum: ${detail}`;
});
