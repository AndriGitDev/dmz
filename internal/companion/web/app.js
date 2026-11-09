"use strict";
const strings = {
  en: {
    quit: "Quit DMZ",
    eyebrow: "YOUR TOOLS. LESS DATA.",
    title: "Keep working with your tools.",
    intro:
      "Choose a project. Open your coding tool. DMZ tries to mask supported identifiers in text messages before they are sent.",
    setup: "Start a session",
    local: "On your device",
    tool: "Choose your tool",
    openToolTitle: "Open your tool",
    openTool: "Open {tool} →",
    apiKeyTool: "API key for {tool} (optional)",
    project: "Choose your project",
    folder: "Project folder",
    choose: "Choose folder",
    "project-hint":
      "Your tool opens in this folder. Its file and network access follow its own settings.",
    connection: "Use your connection",
    account: "Existing sign-in",
    "account-hint":
      "Your coding tool handles sign-in. This route is still being verified.",
    api: "API connection",
    "api-hint":
      "Use an API key you already have. Provider charges are separate from DMZ.",
    "api-key": "API key (optional)",
    "connection-default":
      "Uses your tool’s existing account sign-in unless you enter an API key. Account routing is still being verified.",
    "scope-title": "Text masking, with limits.",
    scope:
      "Basic detection has limited name/address coverage. Known answer markers restore locally. Known originals are re-masked in text history and tool results; tools and files are not generally scanned. Review file diffs: markers can appear in edits. Failures warn and continue; unknown markers can remain visible.",
    start: "Start with DMZ →",
    "terminal-hint":
      "Opens your terminal tool. It may ask you to confirm trust in the project folder. DMZ leaves your settings files unchanged.",
    sessions: "Your sessions",
    empty: "No active session. Start one on the left.",
    "simple-title": "No DMZ account.",
    simple:
      "No DMZ cloud service or conversation analytics. Value mappings stay in memory for this session.",
    desktop: "What about desktop apps?",
    "desktop-hint":
      "This build launches your terminal tools. Direct desktop-app connections are still unverified and are not shown as supported here.",
    footer: "DMZ · local processing · original source licensed GPL-3.0-only",
    preview: "Development build — use fabricated test data.",
    installed: "Detected",
    missing: "Not installed",
    terminal: "Terminal",
    waiting: "Opening terminal",
    running: "Client open",
    stopped: "Stopped",
    stop: "Stop session",
    requests: "requests",
    masked: "text parts masked",
    toolsUnchanged: "Tool results outside general detection in {count} requests; known restored values are re-masked.",
    useEnvironment: "Use the API key available to DMZ (API billing)",
    keyEnv:
      "An API key is available in DMZ’s environment. Select the checkbox to use it, or leave it unselected to keep your account sign-in.",
    keyMemory:
      "Optional. Used for this session only, not saved by DMZ. Provider charges apply separately.",
    unavailable: "Open DMZ again to reconnect to the app.",
    folderManual:
      "The folder picker is unavailable here. Enter the full project-folder path.",
    warning:
      "Some content was not masked or its context changed. Keep the terminal open and check the scope before continuing.",
    noTraffic:
      "No model request observed yet. Send a fabricated test message in the terminal to check the route.",
    choose_project: "Choose an existing project folder.",
    client_missing:
      "Install the terminal version of this tool first, then reopen DMZ.",
    api_key_required: "Enter your API key, or use your existing sign-in.",
    api_key_invalid: "Check the API key and try again.",
    connection_invalid:
      "This connection cannot be started with the selected settings.",
    terminal_unavailable:
      "DMZ could not open a desktop terminal. Check that a terminal app is installed.",
    session_limit: "Stop an existing session before opening another.",
    local_service_unavailable:
      "DMZ could not start a local session. Try again.",
    session_ticket_unavailable: "DMZ could not prepare the session. Try again.",
    launch_failed: "The coding tool could not start. Check its terminal.",
    client_exited:
      "The coding tool exited with an error. Check its terminal, then start another session.",
    connection_lost:
      "The terminal connection was lost. This session is closed; start a new one.",
    confirmQuit: "Quit DMZ and stop its active sessions?",
    closed: "DMZ is closed. You can close this window.",
    starting: "Opening your tool…",
  },
  is: {
    quit: "Loka DMZ",
    eyebrow: "ÞÍN TÓL. MINNI GÖGN.",
    title: "Haltu áfram með þín tól.",
    intro:
      "Veldu verkefni. Ræstu kóðunartólið þitt. DMZ reynir að fela studd auðkenni í textaskilaboðum áður en þau eru send.",
    setup: "Byrjaðu nýja lotu",
    local: "Á tækinu þínu",
    tool: "Veldu tól",
    openToolTitle: "Opnaðu tólið þitt",
    openTool: "Opna {tool} →",
    apiKeyTool: "API-lykill fyrir {tool} (valfrjálst)",
    project: "Veldu verkefni",
    folder: "Verkefnamappa",
    choose: "Velja möppu",
    "project-hint":
      "Tólið opnast í þessari möppu. Skráa- og netaðgangur fylgja stillingum tólsins.",
    connection: "Notaðu þína tengingu",
    account: "Núverandi innskráning",
    "account-hint":
      "Kóðunartólið sér um innskráningu. Þessi leið er enn í prófun.",
    api: "API-tenging",
    "api-hint":
      "Notaðu API-lykil sem þú ert þegar með. Kostnaður veitunnar er aðskilinn frá DMZ.",
    "api-key": "API-lykill (valfrjálst)",
    "connection-default":
      "Notar núverandi innskráningu tólsins nema þú sláir inn API-lykil. Innskráningarleiðin er enn í prófun.",
    "scope-title": "Textaskilaboð, með takmörkunum.",
    scope:
      "Grunngreining hefur takmarkaða þekju nafna og heimilisfanga. Þekkt merki í svörum eru afkóðuð staðbundið. Þekkt upprunagildi eru falin aftur í textasögu og niðurstöðum tóla; tól og skrár eru ekki almennt skönnuð. Skoðaðu skráabreytingar: merki geta borist í þær. Bilanir vara við og halda áfram; óþekkt merki geta verið sýnileg.",
    start: "Ræsa með DMZ →",
    "terminal-hint":
      "Opnar terminal-útgáfu tólsins. Tólið gæti beðið þig um að staðfesta traust á verkefnamöppunni. DMZ breytir ekki stillingaskrám þínum.",
    sessions: "Þínar lotur",
    empty: "Engin lota í gangi. Byrjaðu vinstra megin.",
    "simple-title": "Engin DMZ-innskráning.",
    simple:
      "Engin skýjaþjónusta frá DMZ og engin samtalsgreining send til DMZ. Vörpun gilda er í minni þessarar lotu.",
    desktop: "Hvað með desktop-forritin?",
    "desktop-hint":
      "Þessi útgáfa ræsir terminal-tólin þín. Beinar tengingar við desktop-forritin eru enn óstaðfestar og eru ekki sýndar sem studdar hér.",
    footer: "DMZ · staðbundin vinnsla · opinn frumkóði með GPL-3.0-only leyfi",
    preview: "Þróunarútgáfa — notaðu tilbúin prófunargögn.",
    installed: "Fannst",
    missing: "Ekki uppsett",
    terminal: "Terminal",
    waiting: "Opna terminal",
    running: "Tólið er opið",
    stopped: "Stöðvað",
    stop: "Stöðva lotu",
    requests: "beiðnir",
    masked: "textahlutar faldir",
    toolsUnchanged: "Niðurstöður tóla utan almennrar greiningar í {count} beiðnum; þekkt afkóðuð gildi eru falin aftur.",
    useEnvironment: "Nota API-lykil úr umhverfi DMZ (API-gjöld)",
    keyEnv:
      "API-lykill er til staðar í umhverfi DMZ. Veldu reitinn til að nota hann; annars er núverandi innskráning notuð.",
    keyMemory:
      "Valfrjálst. Aðeins notaður í þessari lotu og ekki vistaður af DMZ. Kostnaður veitunnar er aðskilinn.",
    unavailable: "Opnaðu DMZ aftur til að tengjast forritinu.",
    folderManual:
      "Ekki tókst að opna möppuval. Sláðu inn fulla slóð á verkefnamöppuna.",
    warning:
      "Sumt efni var ekki falið eða samhengi breyttist. Hafðu terminal-gluggann opinn og athugaðu takmarkanir áður en haldið er áfram.",
    noTraffic:
      "Engin líkanbeiðni hefur sést enn. Sendu tilbúin prófunarskilaboð í terminal-glugganum til að athuga tenginguna.",
    choose_project: "Veldu verkefnamöppu sem er til.",
    client_missing:
      "Settu fyrst upp terminal-útgáfu tólsins og opnaðu svo DMZ aftur.",
    api_key_required: "Sláðu inn API-lykil eða notaðu núverandi innskráningu.",
    api_key_invalid: "Athugaðu API-lykilinn og reyndu aftur.",
    connection_invalid:
      "Ekki er hægt að ræsa þessa tengingu með völdum stillingum.",
    terminal_unavailable:
      "DMZ gat ekki opnað terminal-glugga. Athugaðu hvort terminal-forrit sé uppsett.",
    session_limit: "Stöðvaðu eldri lotu áður en þú opnar fleiri.",
    local_service_unavailable:
      "DMZ gat ekki ræst staðbundna lotu. Reyndu aftur.",
    session_ticket_unavailable: "DMZ gat ekki undirbúið lotuna. Reyndu aftur.",
    launch_failed: "Ekki tókst að ræsa tólið. Athugaðu terminal-gluggann.",
    client_exited:
      "Tólið lokaðist með villu. Athugaðu terminal-gluggann og ræstu nýja lotu.",
    connection_lost:
      "Tenging við terminal rofnaði. Lotunni hefur verið lokað; ræstu nýja lotu.",
    confirmQuit: "Loka DMZ og stöðva virkar lotur?",
    closed: "DMZ er lokað. Þú getur lokað þessum glugga.",
    starting: "Opna tólið þitt…",
  },
};
function storageRead(name, key) {
  try {
    return window[name].getItem(key);
  } catch {
    return null;
  }
}
function storageWrite(name, key, value) {
  try {
    if (value === null) window[name].removeItem(key);
    else window[name].setItem(key, value);
  } catch {}
}
let language =
  storageRead("localStorage", "dmz.language") === "en" ? "en" : "is";
let state = { clients: [], sessions: [] };
const openCode = new URLSearchParams(location.hash.slice(1)).get("open");
history.replaceState(null, "", location.pathname);
let token = storageRead("sessionStorage", "dmz.control") || "";
const $ = (s) => document.querySelector(s);
const t = (k) => strings[language][k] || strings.en[k] || k;
Object.assign(strings.en, {
  codex_authentication_required:
    "Sign in through Codex or configure its API key. No native credential reached this connection.",
  backgroundTitle: "DMZ runs in the background.",
  backgroundIntro:
    "DMZ runs in the background. Open your coding tool normally; supported text passes through your local connection.",
  backgroundSimple:
    "No DMZ cloud service or conversation analytics. Mappings stay in memory until DMZ stops. Codex chats share a connection scope. Start a fresh conversation after DMZ restarts or mappings reset; older history may contain restored originals.",
  backgroundSetup: "Connections",
  backgroundHint:
    "Installed tools are connected automatically when their settings are compatible. If a tool is already open, restart it once to load the route. Detection alone does not establish masking.",
  backgroundRestore:
    "DMZ must stay open to mask text. Disconnect or Quit restores its routing changes; tools opened afterwards connect directly. Open DMZ again after signing in to your computer. Already-open tools may need a restart. Your tool keeps its sign-in, model and permissions.",
  not_detected: "Not detected",
  restart_required: "Routing configured — not yet confirmed",
  traffic_observed: "Model traffic observed",
  needs_attention: "Connection needs attention",
  disconnected: "Disconnected",
  disconnect: "Disconnect",
  reconnect: "Connect",
  backgroundDesktop:
    "Direct vendor desktop-app connections remain unqualified. Claude Desktop needs a separate one-time gateway setup; its subscription sign-in cannot use that route.",
  routing_custom_provider_present:
    "An existing custom gateway is configured. DMZ left it unchanged.",
  codex_profiles_require_connection_review:
    "Separate Codex profiles need connection review. Their settings were left unchanged.",
  routing_recovery_conflict:
    "Routing recovery needs review. Your edited settings and backup were kept.",
  routing_restore_conflict:
    "DMZ could not safely restore the route. Your settings and backup were kept.",
  routing_settings_changed:
    "The settings changed while DMZ was connecting. Review them, then try again.",
  routing_settings_unavailable:
    "DMZ could not read compatible routing settings. The tool was not connected.",
  background_state_unavailable:
    "DMZ could not save its local recovery state. The tool was not connected.",
  local_service_unavailable:
    "DMZ could not start the local connection. Try connecting again.",
  local_port_unavailable:
    "Another program is using DMZ’s saved connection port. Close that program, then reconnect. Restart your coding tool after reconnecting.",
  backgroundWarning:
    "A processing or connection warning occurred in the last two minutes. Check this connection before continuing; some text may have passed unchanged.",
  warningHistory: "Processing warnings recorded since connection: {count}.",
  evictedMappings: "Older session mappings were discarded {count} times. Their markers may be unresolved.",
  confirmBackgroundQuit:
    "Quit DMZ? Its routes will be restored. Tools opened afterwards connect directly without DMZ masking. Open DMZ again after your next login.",
});
Object.assign(strings.is, {
  codex_authentication_required:
    "Skráðu þig inn í Codex eða stilltu API-lykil þess. Engin innskráningarupplýsing barst á þessari tengingu.",
  backgroundTitle: "DMZ keyrir í bakgrunni.",
  backgroundIntro:
    "DMZ keyrir í bakgrunni. Opnaðu kóðunartólið eins og venjulega; studdur texti fer um staðbundnu tenginguna.",
  backgroundSimple:
    "Engin skýjaþjónusta eða samtalsgreining frá DMZ. Varpanir eru í minni þar til DMZ lokast. Codex-samtöl deila tengingarsviði. Byrjaðu nýtt samtal eftir endurræsingu DMZ eða núllstillingu varpana; eldri saga gæti innihaldið afkóðuð upprunagildi.",
  backgroundSetup: "Tengingar",
  backgroundHint:
    "Uppsett tól eru tengd sjálfkrafa þegar stillingar þeirra passa. Ef tólið er þegar opið skaltu endurræsa það einu sinni. Það eitt að tól finnist staðfestir ekki að texti sé falinn.",
  backgroundRestore:
    "DMZ þarf að vera opið til að fela texta. Aftenging eða lokun tekur leiðarbreytingarnar til baka; tól sem opnast síðar tengjast beint. Opnaðu DMZ aftur eftir innskráningu í tölvuna. Tól sem eru þegar opin gætu þurft endurræsingu. Innskráning, líkan og heimildir fylgja tólinu.",
  not_detected: "Fannst ekki",
  restart_required: "Leið stillt — ekki enn staðfest",
  traffic_observed: "Líkanbeiðnir hafa sést",
  needs_attention: "Athuga þarf tenginguna",
  disconnected: "Aftengt",
  disconnect: "Aftengja",
  reconnect: "Tengja",
  backgroundDesktop:
    "Beinar tengingar við desktop-forrit veitnanna eru enn óstaðfestar. Claude Desktop þarf sérstaka uppsetningu gáttar; áskriftarinnskráning þess virkar ekki á þeirri leið.",
  routing_custom_provider_present:
    "Önnur gátt er þegar stillt. DMZ breytti henni ekki.",
  codex_profiles_require_connection_review:
    "Skoða þarf tengingu aðskildra Codex-prófíla. Stillingum þeirra var ekki breytt.",
  routing_recovery_conflict:
    "Skoða þarf endurheimt leiðarstillinga. Breyttar stillingar og afrit voru varðveitt.",
  routing_restore_conflict:
    "Ekki tókst að taka leiðarbreytinguna örugglega til baka. Stillingar og afrit voru varðveitt.",
  routing_settings_changed:
    "Stillingarnar breyttust meðan DMZ tengdist. Skoðaðu þær og reyndu aftur.",
  routing_settings_unavailable:
    "Ekki tókst að lesa samhæfar leiðarstillingar. Tólið var ekki tengt.",
  background_state_unavailable:
    "Ekki tókst að vista staðbundnar endurheimtarupplýsingar. Tólið var ekki tengt.",
  local_port_unavailable:
    "Annað forrit notar vistað tengi DMZ. Lokaðu því forriti og tengdu aftur. Endurræstu kóðunartólið eftir tengingu.",
  backgroundWarning:
    "Viðvörun kom upp við vinnslu eða tengingu á síðustu tveimur mínútum. Athugaðu tenginguna áður en haldið er áfram; einhver texti gæti hafa farið óbreyttur áfram.",
  warningHistory: "Skráðar vinnsluviðvaranir frá tengingu: {count}.",
  evictedMappings: "Eldri lotuvörpunum var sleppt {count} sinnum. Merki þeirra gætu verið óþekkt.",
  confirmBackgroundQuit:
    "Loka DMZ? Leiðarstillingar verða teknar til baka. Tól sem opnast síðar tengjast beint án þess að DMZ feli texta. Opnaðu DMZ aftur eftir næstu innskráningu.",
});
let backgroundRendered = "";
const pendingConnections = new Set();
let stopped = false;
let starting = false;
let clientsRendered = "";
let sessionsRendered = "";
async function api(path, body) {
  const response = await fetch("/api/" + path, {
    method: body === undefined ? "GET" : "POST",
    headers: {
      Authorization: "Bearer " + token,
      "Content-Type": "application/json",
    },
    body: body === undefined ? undefined : JSON.stringify(body),
    cache: "no-store",
  });
  if (!response.ok) throw Error("unavailable");
  if (response.status === 204) return null;
  return response.json();
}
function text(tag, value, className) {
  const el = document.createElement(tag);
  el.textContent = value;
  if (className) el.className = className;
  return el;
}
function error(code) {
  if (stopped || !$("#error")) return;
  $("#error").textContent = t(
    strings[language][code] || strings.en[code]
      ? code
      : "local_service_unavailable",
  );
  $("#error").hidden = false;
}
function renderClients() {
  if (starting) return;
  clientsRendered = JSON.stringify([language, state.clients]);
  const focused = document.activeElement?.id;
  const root = $("#clients");
  const pending = new Map(
    state.clients.map((client) => [
      client.id,
      {
        key: $("#api-key-" + client.id)?.value || "",
        environment: $("#use-environment-" + client.id)?.checked || false,
      },
    ]),
  );
  root.replaceChildren();
  for (const client of state.clients) {
    const card = document.createElement("article");
    card.className = "tool-card";
    card.dataset.client = client.id;
    const heading = text("h3", client.name);
    heading.id = "tool-title-" + client.id;
    card.setAttribute("aria-labelledby", heading.id);
    card.append(
      heading,
      text(
        "small",
        t(client.available ? "installed" : "missing") + " · " + t("terminal"),
      ),
    );
    const label = text("label", t("apiKeyTool").replace("{tool}", client.name));
    label.htmlFor = "api-key-" + client.id;
    const key = document.createElement("input");
    key.type = "password";
    key.id = label.htmlFor;
    key.autocomplete = "off";
    key.spellcheck = false;
    key.disabled = !client.available;
    key.value = pending.get(client.id).key;
    card.append(label, key);
    if (client.api_key_available) {
      const environmentLabel = document.createElement("label");
      environmentLabel.className = "radio-row";
      const environment = document.createElement("input");
      environment.type = "checkbox";
      environment.id = "use-environment-" + client.id;
      environment.checked = pending.get(client.id).environment;
      environment.disabled = !client.available;
      environmentLabel.append(environment, text("span", t("useEnvironment")));
      card.append(environmentLabel);
    }
    const button = text(
      "button",
      t("openTool").replace("{tool}", client.name),
      "primary tool-start",
    );
    button.id = "start-" + client.id;
    button.type = "submit";
    button.name = "client";
    button.value = client.id;
    button.disabled = !client.available;
    card.append(button);
    root.append(card);
  }
  if (focused) document.getElementById(focused)?.focus({ preventScroll: true });
}
function renderSessions() {
  const signature = JSON.stringify([language, state.sessions]);
  if (signature === sessionsRendered) return;
  sessionsRendered = signature;
  const root = $("#sessions");
  const focused = document.activeElement?.dataset.sessionStop;
  const buttons = new Map(
    Array.from(root.querySelectorAll("button[data-session-stop]"), (b) => [
      b.dataset.sessionStop,
      b,
    ]),
  );
  root.replaceChildren();
  $("#live-count").textContent = state.sessions.filter(
    (s) => s.state !== "stopped",
  ).length;
  if (!state.sessions.length) {
    root.append(text("p", t("empty"), "empty"));
    return;
  }
  for (const session of state.sessions) {
    const el = document.createElement("article");
    el.className = "session";
    const title = document.createElement("div");
    title.className = "session-title";
    title.append(
      text("strong", session.client === "codex" ? "Codex" : "Claude Code"),
      text("span", t(session.state), "status " + session.state),
    );
    el.append(
      title,
      text(
        "p",
        `${session.requests} ${t("requests")} · ${session.changed_parts} ${t("masked")}`,
        "session-stats",
      ),
    );
    const toolsUnchanged = (session.warnings?.tool_result_unchanged || 0) + (session.warnings?.tool_result_detection_not_applied || 0);
    if (toolsUnchanged)
      el.append(
        text(
          "p",
          t("toolsUnchanged").replace("{count}", String(toolsUnchanged)),
          "hint",
        ),
      );
    const warnings = Object.keys(session.warnings || {}).filter(
      (k) =>
        ![
          "instructions_unchanged",
          "tool_result_unchanged",
          "tool_result_detection_not_applied",
          "no_user_text",
        ].includes(k),
    );
    if (session.issue)
      el.append(text("p", t(session.issue), "session-warning"));
    else if (warnings.length)
      el.append(text("p", t("warning"), "session-warning"));
    else if (!session.requests && session.state !== "stopped")
      el.append(text("p", t("noTraffic"), "hint"));
    if (session.state !== "stopped") {
      const previous = buttons.get(session.id);
      const stop = previous || text("button", t("stop"));
      stop.textContent = t("stop");
      stop.type = "button";
      stop.dataset.sessionStop = session.id;
      if (!previous)
        stop.addEventListener("click", async () => {
          stop.disabled = true;
          try {
            await api("stop", { id: session.id });
            await refresh();
          } catch {
            stop.disabled = false;
            error("unavailable");
          }
        });
      el.append(stop);
    }
    root.append(el);
  }
  if (focused)
    buttons.get(focused)?.isConnected &&
      buttons.get(focused).focus({ preventScroll: true });
}
function renderBackground() {
  const enabled = !!state.background_enabled;
  $("#background-section").hidden = !enabled;
  $("#start-form").hidden = enabled;
  $("#session-panel").hidden = enabled;
  if (!enabled) return;
  $("[data-i18n='title']").textContent = t("backgroundTitle");
  $("[data-i18n='intro']").textContent = t("backgroundIntro");
  $("#setup-title").textContent = t("backgroundSetup");
  $("[data-i18n='simple']").textContent = t("backgroundSimple");
  $("[data-i18n='desktop-hint']").textContent = t("backgroundDesktop");
  const signature = JSON.stringify([language, state.background]);
  if (signature === backgroundRendered) return;
  backgroundRendered = signature;
  const root = $("#background-connections");
  const focused = document.activeElement?.dataset.backgroundClient;
  const previous = new Map(
    Array.from(root.querySelectorAll("button[data-background-client]"), (b) => [
      b.dataset.backgroundClient,
      b,
    ]),
  );
  root.replaceChildren();
  for (const route of state.background || []) {
    const card = document.createElement("article");
    card.className = "session";
    card.append(text("h3", route.name), text("p", t(route.state), "status"));
    if (route.requests)
      card.append(
        text(
          "p",
          `${route.requests} ${t("requests")} · ${route.changed_parts} ${t("masked")}`,
          "session-stats",
        ),
      );
    if (route.issue) card.append(text("p", t(route.issue), "session-warning"));
    const warnings = Object.keys(route.warnings || {}).filter(
      (k) =>
        ![
          "instructions_unchanged",
          "tool_result_unchanged",
          "tool_result_detection_not_applied",
          "no_user_text",
          "attachment_unchanged",
          "mapping_evicted_history_may_degrade",
        ].includes(k),
    );
    if (route.recent_warnings?.length)
      card.append(text("p", t("backgroundWarning"), "session-warning"));
    if (warnings.length) {
      const count = warnings.reduce((sum, code) => sum + route.warnings[code], 0);
      card.append(text("p", t("warningHistory").replace("{count}", count), "session-stats"));
    }
    if (route.warnings?.mapping_evicted_history_may_degrade)
      card.append(text("p", t("evictedMappings").replace("{count}", route.warnings.mapping_evicted_history_may_degrade), "hint"));
    if (route.detected || route.routed) {
      let button = previous.get(route.client);
      if (!button) {
        button = document.createElement("button");
        button.type = "button";
        button.dataset.backgroundClient = route.client;
        button.addEventListener("click", async () => {
          const client = button.dataset.backgroundClient;
          if (pendingConnections.has(client)) return;
          pendingConnections.add(client);
          button.disabled = true;
          $("#error").hidden = true;
          try {
            const result = await api("background", {
              client,
              connected: button.dataset.connect === "true",
            });
            if (result?.error) error(result.error);
            await refresh();
          } catch {
            error("unavailable");
          } finally {
            pendingConnections.delete(client);
            button.disabled = false;
          }
        });
      }
      button.dataset.connect = String(!route.routed);
      button.textContent = t(route.routed ? "disconnect" : "reconnect");
      button.disabled = pendingConnections.has(route.client);
      card.append(button);
    }
    root.append(card);
  }
  if (focused)
    previous.get(focused)?.isConnected &&
      previous.get(focused).focus({ preventScroll: true });
}
function translate() {
  document.documentElement.lang = language;
  document
    .querySelectorAll("[data-i18n]")
    .forEach((e) => (e.textContent = t(e.dataset.i18n)));
  $("#language").textContent = language === "is" ? "English" : "Íslenska";
  renderClients();
  renderSessions();
  backgroundRendered = "";
  renderBackground();
}
async function refresh() {
  if (stopped) return;
  try {
    const next = await api("state");
    if (stopped) return;
    state = next;
    if (JSON.stringify([language, state.clients]) !== clientsRendered)
      renderClients();
    renderSessions();
    renderBackground();
  } catch {
    if (!stopped) error("unavailable");
  }
}
$("#language").addEventListener("click", () => {
  language = language === "is" ? "en" : "is";
  storageWrite("localStorage", "dmz.language", language);
  translate();
});
$("#choose-folder").addEventListener("click", async () => {
  const b = $("#choose-folder");
  b.disabled = true;
  try {
    const result = await api("folder", {});
    if (stopped || result.cancelled) return;
    if (result.folder) {
      $("#project").value = result.folder;
      $("#error").hidden = true;
    } else {
      error("folderManual");
      $("#project").focus();
    }
  } catch {
    if (!stopped) error("folderManual");
  } finally {
    b.disabled = false;
  }
});
$("#start-form").addEventListener("submit", async (event) => {
  event.preventDefault();
  const client = state.clients.find(
    (c) => c.id === event.submitter?.value && c.available,
  );
  if (!client || stopped || starting) return;
  starting = true;
  $("#error").hidden = true;
  const button = event.submitter;
  document.querySelectorAll(".tool-start").forEach((e) => (e.disabled = true));
  button.textContent = t("starting");
  const field = $("#api-key-" + client.id);
  const key = field.value;
  try {
    const result = await api("start", {
      client: client.id,
      project: $("#project").value,
      auth:
        key || $("#use-environment-" + client.id)?.checked ? "api" : "account",
      api_key: key,
    });
    if (result.error) error(result.error);
    else field.value = "";
    await refresh();
  } catch {
    error("unavailable");
  } finally {
    starting = false;
    if (JSON.stringify([language, state.clients]) !== clientsRendered)
      renderClients();
    for (const c of state.clients) {
      const b = $("#start-" + c.id);
      if (b) {
        b.disabled = !c.available;
        b.textContent = t("openTool").replace("{tool}", c.name);
      }
    }
  }
});
$("#quit").addEventListener("click", async () => {
  if (
    (state.sessions.some((s) => s.state !== "stopped") ||
      (state.background || []).some((r) => r.routed)) &&
    !confirm(t(state.background_enabled ? "confirmBackgroundQuit" : "confirmQuit"))
  )
    return;
  stopped = true;
  $("#quit").disabled = true;
  $("#language").disabled = true;
  try {
    await api("quit", {});
  } catch {
    stopped = false;
    $("#quit").disabled = false;
    $("#language").disabled = false;
    error("unavailable");
    return;
  }
  storageWrite("sessionStorage", "dmz.control", null);
  document.querySelector("main").replaceChildren(text("h1", t("closed")));
});
async function initialize() {
  if (openCode) {
    try {
      const response = await fetch("/api/bootstrap", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ code: openCode }),
        cache: "no-store",
      });
      if (!response.ok) throw Error("unavailable");
      token = (await response.json()).token;
      storageWrite("sessionStorage", "dmz.control", token);
    } catch {
      token = "";
      storageWrite("sessionStorage", "dmz.control", null);
    }
  }
  if (!token) {
    error("unavailable");
    return;
  }
  $("#quit").disabled = false;
  await refresh();
  setInterval(refresh, 1500);
}
translate();
initialize();
