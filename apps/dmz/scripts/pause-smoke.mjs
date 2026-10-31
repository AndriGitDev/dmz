import assert from "node:assert/strict";
import { mkdtemp, rm, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { fileURLToPath } from "node:url";
import { chromium } from "playwright";
const root = fileURLToPath(new URL("../", import.meta.url)),
  extension = join(root, ".output/chrome-mv3");
const trained = JSON.parse(
  await readFile(join(extension, "manifest.json"), "utf8"),
).permissions.includes("offscreen");
let browser, profile;
async function launch() {
  profile = await mkdtemp(join(tmpdir(), "dmz-pause-"));
  browser = await chromium.launchPersistentContext(profile, {
    headless: true,
    channel: "chromium",
    viewport: { width: 380, height: 900 },
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
    args: [
      `--disable-extensions-except=${extension}`,
      `--load-extension=${extension}`,
    ],
  });
  const sw =
    browser.serviceWorkers()[0] ||
    (await browser.waitForEvent("serviceworker"));
  const panel = await browser.newPage();
  await panel.goto(
    `chrome-extension://${new URL(sw.url()).host}/sidepanel.html`,
  );
  return panel;
}
async function close() {
  if (browser) await browser.close();
  if (profile) await rm(profile, { recursive: true, force: true });
  browser = undefined;
}
async function chat(site) {
  const p = await browser.newPage(),
    host = site === "chatgpt" ? "chatgpt.com" : "claude.ai";
  const field =
    site === "chatgpt"
      ? '<textarea id="prompt-textarea"></textarea>'
      : '<div class="ProseMirror" contenteditable="true"></div>';
  const button =
    site === "chatgpt"
      ? 'data-testid="send-button"'
      : 'aria-label="Send message"';
  const attr = site === "chatgpt" ? "data-message-author-role" : "data-testid",
    value = site === "chatgpt" ? "assistant" : "assistant-message";
  await p.route(`https://${host}/**`, (r) =>
    r.fulfill({
      contentType: "text/html",
      body: `<!doctype html><html><body><form>${field}<button ${button}>Send</button></form><main></main><script>window.sent=[];document.querySelector('form').onsubmit=e=>{e.preventDefault();const f=document.querySelector('textarea,[contenteditable]'),v=f.value??f.innerText;window.sent.push(v);const a=document.createElement('article');a.setAttribute('${attr}','${value}');a.textContent=v;document.querySelector('main').append(a);};</script></body></html>`,
    }),
  );
  await p.goto(`https://${host}/`);
  await p.locator("#dmz-status").waitFor();
  return p;
}
const click = (p, s) => p.locator(s).evaluate((e) => e.click());
async function active(panel, p) {
  await p.bringToFront();
  await click(panel, "#refresh");
  await panel.waitForFunction(() => !document.querySelector("#pause").disabled);
}
async function pause(panel, p, on) {
  await active(panel, p);
  assert.equal(
    await panel.locator("#pause").getAttribute("aria-pressed"),
    String(!on),
  );
  await click(panel, "#pause");
  await panel.waitForFunction(
    (v) =>
      document.querySelector("#pause").getAttribute("aria-pressed") ===
        String(v) && !document.querySelector("#pause").disabled,
    on,
  );
}
async function send(p, text) {
  const n = await p.evaluate(() => window.sent.length);
  await p.locator("textarea,[contenteditable]").fill(text);
  await p.locator("button").click();
  await p.waitForFunction((n) => window.sent.length === n + 1, n, {
    timeout: 130000,
  });
  return p.evaluate(() => window.sent.at(-1));
}
async function world(p) {
  const c = await browser.newCDPSession(p),
    contexts = [];
  c.on("Runtime.executionContextCreated", (e) => contexts.push(e.context));
  const { frameTree } = await c.send("Page.getFrameTree");
  await c.send("Runtime.enable");
  // Restored replies create short-lived extension frames. Their contexts can
  // disappear while probing; only the main frame contains our content script.
  for (const x of contexts.filter(
    (context) => context.auxData?.isDefault === false &&
      context.auxData?.frameId === frameTree.frame.id,
  )) {
    const v = await c.send("Runtime.evaluate", {
      contextId: x.id,
      expression: 'typeof chrome!=="undefined"&&chrome.runtime?.id',
    });
    if (v.result.value === new URL(browser.serviceWorkers()[0].url()).host)
      return {
        eval: (expression) =>
          c.send("Runtime.evaluate", { contextId: x.id, expression }),
        close: () => c.detach(),
      };
  }
  throw Error("No extension isolated world");
}
try {
  const panel = await launch();
  await panel.waitForFunction(() => document.querySelector("#pause").disabled);
  for (const site of ["chatgpt", "claude"]) {
    const p = await chat(site);
    await active(panel, p);
    const first = await send(p, "first@example.com");
    assert.match(first, /⟦EMAIL-/);
    await p.locator("iframe[data-dmz-restored]").first().waitFor();
    await pause(panel, p, true);
    if (site === "chatgpt")
      await panel.screenshot({
        path: join(profile, "dmz-pause-active-settings.png"),
        fullPage: true,
      });
    await p.waitForFunction(
      () => !document.querySelector("iframe[data-dmz-restored]"),
    );
    assert.match(
      await panel.locator("#pause-status").innerText(),
      /án möskunar/,
    );
    assert.equal(await send(p, "paused@example.com"), "paused@example.com");
    const other = await chat(site === "chatgpt" ? "claude" : "chatgpt");
    assert.match(await send(other, "other@example.com"), /⟦EMAIL-/);
    await other.close();
    await pause(panel, p, false);
    assert.equal(
      await send(p, "first@example.com"),
      first,
      "Pause erased mappings",
    );
    await p.locator("iframe[data-dmz-restored]").first().waitFor();
    // Freeze only extension animation frames to exercise pause during editor reconciliation.
    for (const edit of [false, true]) {
      const w = await world(p);
      await w.eval(
        "globalThis.__dmzRAF=requestAnimationFrame;globalThis.__dmzFrames=[];requestAnimationFrame=cb=>(__dmzFrames.push(cb),__dmzFrames.length)",
      );
      const n = await p.evaluate(() => window.sent.length);
      await p.locator("textarea,[contenteditable]").fill("race@example.com");
      await p.locator("button").click();
      await p.waitForFunction(
        () => {
          const f = document.querySelector("textarea,[contenteditable]");
          return (f.value ?? f.innerText).includes("⟦EMAIL-");
        },
        null,
        { timeout: 130000 },
      );
      if (edit)
        await p
          .locator("textarea,[contenteditable]")
          .fill("edited@example.com");
      await pause(panel, p, true);
      for (let i = 0; i < 10; i++) {
        await w.eval(
          "for(const cb of __dmzFrames.splice(0))cb(performance.now())",
        );
        await p.waitForTimeout(20);
      }
      await p.waitForFunction((n) => window.sent.length === n + 1, n);
      assert.equal(
        await p.evaluate(() => window.sent.at(-1)),
        edit ? "edited@example.com" : "race@example.com",
      );
      await w.eval(
        "requestAnimationFrame=__dmzRAF;for(const cb of __dmzFrames.splice(0))requestAnimationFrame(cb)",
      );
      await w.close();
      await pause(panel, p, false);
    }
    await pause(panel, p, true);
    const clock = await world(p);
    await clock.eval(
      "globalThis.__dmzNow=Date.now;Date.now=()=>__dmzNow()+16*60*1000",
    );
    assert.match(await send(p, "expired@example.com"), /⟦EMAIL-/);
    await clock.eval("Date.now=__dmzNow");
    await clock.close();
    await active(panel, p);
    assert.equal(
      await panel.locator("#pause").getAttribute("aria-pressed"),
      "false",
    );
    await pause(panel, p, true);
    await p.reload();
    await p.locator("#dmz-status").waitFor();
    assert.match(await send(p, "reloaded@example.com"), /⟦EMAIL-/);
    await active(panel, p);
    assert.equal(
      await panel.locator("#pause").getAttribute("aria-pressed"),
      "false",
    );
    await p.close();
  }
  const blank = await browser.newPage();
  await blank.goto("about:blank");
  await blank.bringToFront();
  await click(panel, "#refresh");
  await panel.waitForFunction(() => document.querySelector("#pause").disabled);
  assert.ok(
    await panel.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  );
  await panel.screenshot({
    path: join(profile, "dmz-pause-settings.png"),
    fullPage: true,
  });
  await close();
  if (trained) {
    const path = join(extension, "scanner/worker.mjs"),
      original = await readFile(path);
    try {
      await writeFile(
        path,
        Buffer.concat([
          Buffer.from(
            "await new Promise(resolve=>setTimeout(resolve,4000));\n",
          ),
          original,
        ]),
      );
      const panel = await launch(),
        p = await chat("chatgpt");
      await active(panel, p);
      const c = await browser.newCDPSession(p);
      await p.locator("textarea").fill("Anna pending@example.com");
      await p.locator("button").click();
      let found = false;
      for (let i = 0; i < 50; i++) {
        const t = await c.send("Accessibility.getFullAXTree");
        if (t.nodes.some((n) => n.name?.value?.includes("DMZ yfirfer nöfn"))) {
          found = true;
          break;
        }
        await p.waitForTimeout(20);
      }
      assert.ok(found, "No pending scan");
      await pause(panel, p, true);
      await p.waitForFunction(() => window.sent.length === 1, null, {
        timeout: 1500,
      });
      assert.equal(
        await p.evaluate(() => window.sent[0]),
        "Anna pending@example.com",
      );
      await p.locator("textarea").fill("newer@example.com");
      await p.waitForTimeout(5500);
      assert.equal(await p.evaluate(() => window.sent.length), 1);
      assert.equal(
        await p.locator("textarea").inputValue(),
        "newer@example.com",
      );
      await c.detach();
    } finally {
      await close();
      await writeFile(path, original);
    }
  }
  console.log(
    "Pause/resume, mappings, both sites, tab isolation, expiry/reload, unsupported UI and reconciliation races passed" +
      (trained
        ? "; pending trained scan cancels promptly without late writes or duplicate sends."
        : "."),
  );
} finally {
  await close();
}
