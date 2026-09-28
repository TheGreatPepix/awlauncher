"use strict";


const $ = (id) => document.getElementById(id);
const host = window.chrome && window.chrome.webview;

const state = {
  accounts: [],
  selected: "",
  lastId: "",
  game: "",
  fxGame: "",
  allowMods: false,
  suggestedGame: "",
  data: "",
  ops: [],
  running: false,
  gameInfo: null,
  availableClients: null,
  availableClientsFailed: false,
  availableVKFailed: false,
  page: "home",
  progress: { active: false },
  prompt: null,
  version: "",
  update: null,
  checking: false,
  autostart: "off",
};

function busy() { return state.ops.length > 0; }
function gameOp() { return state.ops.find((o) => o.game); }
function accountOp(id) { return state.ops.find((o) => o.account === id); }
function opsTitle() { return state.ops.map((o) => tb(o.title)).join(" · "); }
function canPlay(a) { return !!a && !gameOp() && !accountOp(a.id); }

function send(cmd) {
  if (host) host.postMessage(JSON.stringify(cmd));
  else demo.handle(cmd);
}

window.aw = {
  recv(ev) {
    switch (ev.type) {
      case "state": onState(ev); break;
      case "log": appendLog(ev.text); break;
      case "progress": {
        const pausedChanged = !!state.progress.paused !== !!ev.paused;
        const activeChanged = !!state.progress.active !== !!ev.active;
        state.progress = ev;
        renderProgress();
        if (pausedChanged || activeChanged) renderStatus();
        break;
      }
      case "prompt": showPrompt(ev); break;
      case "browsed": onBrowsed(ev); break;
      case "folderInfo": onFolderInfo(ev); break;
      case "done": onDone(ev); break;
      case "notice": toast(tb(ev.message), ev.kind || "info"); break;
      case "clipboard": toast(t(ev.ok ? "Logs copied" : "Could not copy")); break;
      case "game": state.running = ev.running; renderStatus(); break;
      case "signin": renderSignIn(ev); break;
      case "gameInfo": state.gameInfo = ev; renderGame(); break;
      case "availableClients": state.availableClients = ev.clients; state.availableClientsFailed = !!ev.failed; state.availableVKFailed = !!ev.vkFailed; renderGame(); break;
      case "update": onUpdate(ev); break;
    }
  },
};


function el(tag, props, ...children) {
  const node = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v == null || v === false) continue;
    if (k === "class") node.className = v;
    else if (k === "text") node.textContent = v;
    else if (k.startsWith("on")) node.addEventListener(k.slice(2), v);
    else node.setAttribute(k, v === true ? "" : v);
  }
  for (const c of children.flat()) if (c != null && c !== false) node.append(c);
  return node;
}

function icon(name, cls) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("class", "icon" + (cls ? " " + cls : ""));
  const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
  use.setAttribute("href", "#i-" + name);
  svg.append(use);
  return svg;
}

const GiB = 1 << 30;
function formatBytes(n) {
  if (n >= GiB) return (n / GiB).toFixed(1) + " GiB";
  if (n >= 1 << 20) return (n / (1 << 20)).toFixed(1) + " MiB";
  if (n >= 1 << 10) return Math.round(n / 1024) + " KiB";
  return n + " B";
}
function formatPair(done, total) {
  if (total >= GiB) return `${(done / GiB).toFixed(1)} / ${(total / GiB).toFixed(1)} GiB`;
  if (total >= 1 << 20) return `${(done / (1 << 20)).toFixed(1)} / ${(total / (1 << 20)).toFixed(1)} MiB`;
  if (total >= 1 << 10) return `${Math.round(done / 1024)} / ${Math.round(total / 1024)} KiB`;
  return `${done} / ${total} B`;
}
function formatDuration(sec) {
  sec = Math.round(sec);
  const h = Math.floor(sec / 3600), m = Math.floor(sec / 60) % 60, s = sec % 60;
  const [uh, um, us] = i18n.lang === "ru" ? [" ч", " мин", " с"] : ["h", "m", "s"];
  if (h > 0) return `${h}${uh} ${String(m).padStart(2, "0")}${um}`;
  if (m > 0) return `${m}${um} ${String(s).padStart(2, "0")}${us}`;
  return `${s}${us}`;
}
function clipLeft(s, n) { return s.length <= n ? s : "…" + s.slice(s.length - n + 1); }


const HUES = [
  { name: "Lavender", h: 300 },
  { name: "Rose", h: 355 },
  { name: "Amber", h: 60 },
  { name: "Mint", h: 165 },
  { name: "Sky", h: 240 },
];
function validHue(h) { return Number.isFinite(h) && h >= 0 && h < 360 ? Math.round(h) : 300; }

const store = {
  get(k, d) { try { return localStorage.getItem(k) ?? d; } catch { return d; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch {} },
};
const darkQuery = matchMedia("(prefers-color-scheme: dark)");

const prefs = {
  theme: host ? "system" : store.get("aw.theme", "system"),
  hue: host ? 300 : validHue(Number(store.get("aw.hue", "300"))),
  lang: host ? "auto" : store.get("aw.lang", "auto"),
};

function setPrefs(change) {
  Object.assign(prefs, change);
  if (host) send({ cmd: "prefs", theme: prefs.theme, hue: prefs.hue, lang: prefs.lang });
  else { store.set("aw.theme", prefs.theme); store.set("aw.hue", prefs.hue); store.set("aw.lang", prefs.lang); }
  applyTheme();
}

function applyLanguage() {
  i18n.pref = prefs.lang;
  resolveLang();
  applyStatic();
  for (const b of $("lang-mode").children) b.classList.toggle("on", b.dataset.lang === prefs.lang);
}

function setLanguage(lang) {
  setPrefs({ lang });
  applyLanguage();
  renderAll();
}

function renderAll() {
  renderAccounts();
  renderStatus();
  renderProgress();
  renderAutostart(state.autostart);
  renderFolders();
  if (state.page === "game") send({ cmd: "gameInfo" });
}

function applyTheme() {
  const mode = prefs.theme;
  const hue = prefs.hue;
  const root = document.documentElement;
  if (mode === "system") root.removeAttribute("data-theme"); else root.dataset.theme = mode;
  root.classList.toggle("system-dark", darkQuery.matches);
  root.style.setProperty("--h", hue);
  for (const b of $("theme-mode").children) b.classList.toggle("on", b.dataset.theme === mode);
  for (const s of $("swatches").children) s.classList.toggle("on", Number(s.dataset.hue) === hue);
  $("hue-slider").value = hue;
  const css = getComputedStyle(document.body);
  const dark = mode === "dark" || (mode === "system" && darkQuery.matches);
  send({ cmd: "theme", dark, caption: toHex(css.backgroundColor), text: toHex(css.color) });
}

function toHex(color) {
  const c = document.createElement("canvas");
  c.width = c.height = 1;
  const x = c.getContext("2d");
  x.fillStyle = color;
  x.fillRect(0, 0, 1, 1);
  const d = x.getImageData(0, 0, 1, 1).data;
  return "#" + [d[0], d[1], d[2]].map((v) => v.toString(16).padStart(2, "0")).join("");
}

function setupTheme() {
  for (const { name, h } of HUES) {
    const s = el("button", { class: "swatch", title: name, "data-hue": h, style: `--sh:${h}`, onclick: () => setPrefs({ hue: h }) }, icon("check"));
    $("swatches").append(s);
  }
  const slider = $("hue-slider");
  slider.style.setProperty("--hue-track", `linear-gradient(to right, ${Array.from({ length: 13 }, (_, i) => `oklch(0.6 0.14 ${i * 30})`).join(", ")})`);
  slider.addEventListener("input", () => { prefs.hue = validHue(Number(slider.value)); applyTheme(); });
  slider.addEventListener("change", () => setPrefs({ hue: validHue(Number(slider.value)) }));
  for (const b of $("theme-mode").children) b.addEventListener("click", () => setPrefs({ theme: b.dataset.theme }));
  darkQuery.addEventListener("change", applyTheme);
  applyTheme();
}


function showPage(page) {
  state.page = page;
  for (const b of document.querySelectorAll(".nav-item")) b.classList.toggle("on", b.dataset.page === page);
  for (const p of document.querySelectorAll(".page")) p.classList.toggle("on", p.id === "page-" + page);
  if (page === "game") { send({ cmd: "gameInfo" }); send({ cmd: "availableClients" }); }
  if (page === "settings") $("settings-dot").classList.remove("on");
  if (page === "activity") {
    $("activity-dot").classList.remove("on");
    const log = $("log");
    log.scrollTop = log.scrollHeight;
  }
}


function onState(ev) {
  const last = (ev.accounts.find((a) => a.last) || {}).id || "";
  state.accounts = ev.accounts;
  if (last && last !== state.lastId) state.selected = last;
  state.lastId = last;
  if (!state.accounts.some((a) => a.id === state.selected)) state.selected = last || (state.accounts[0] || {}).id || "";
  state.game = ev.game;
  state.fxGame = ev.fxGame || "";
  state.allowMods = !!ev.allowMods;
  state.suggestedGame = ev.suggestedGame || "";
  state.data = ev.data;
  state.ops = ev.ops || [];
  state.running = !!ev.running;
  state.version = ev.version || "";
  state.autostart = ev.autostart || "off";
  if (ev.systemLang) i18n.system = ev.systemLang;
  if (typeof ev.log === "string") { $("log").textContent = ""; appendLog(ev.log, true); }
  if (ev.prefs) { prefs.theme = ev.prefs.theme || "system"; prefs.hue = validHue(ev.prefs.hue); prefs.lang = ev.prefs.lang || "auto"; applyTheme(); }
  if (ev.prefs || ev.systemLang) applyLanguage();
  renderAll();
}

function renderFolders() {
  for (const b of $("mods-mode").children) b.classList.toggle("on", b.dataset.mode === (state.allowMods ? "on" : "off"));
  $("data-folder").textContent = state.data || "%LOCALAPPDATA%\\AWLauncher";
}

function cookiePath(size, lobes, depth) {
  const r = size / 2, pts = [];
  for (let i = 0; i <= 120; i++) {
    const t = (i / 120) * Math.PI * 2;
    const rr = r * (1 - depth + depth * Math.cos(lobes * t));
    pts.push(`${(r + rr * Math.cos(t)).toFixed(2)},${(r + rr * Math.sin(t)).toFixed(2)}`);
  }
  return "M" + pts.join("L") + "Z";
}
const COOKIE = cookiePath(44, 9, 0.07);

function avatar(a) {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  svg.setAttribute("viewBox", "0 0 44 44");
  const path = document.createElementNS("http://www.w3.org/2000/svg", "path");
  path.setAttribute("d", COOKIE);
  path.setAttribute("fill", a.provider === "fxid" ? "var(--fx)" : "var(--vk)");
  svg.append(path);
  const letter = (a.name.trim()[0] || "?").toUpperCase();
  return el("div", { class: "avatar" }, svg, el("div", { class: "avatar-letter", text: letter }));
}

function renderAccounts() {
  const tiles = $("tiles");
  tiles.textContent = "";
  const shown = state.accounts;
  if (!shown.some((a) => a.id === state.selected) && shown.length) state.selected = shown[0].id;
  for (const a of state.accounts) {
    const op = accountOp(a.id);
    const tile = el("div", {
      class: "tile state" + (a.id === state.selected ? " on" : "") + (op ? " working" : ""),
      role: "button", tabindex: "0", title: op ? tb(op.title) : a.name,
      onclick: () => select(a.id),
      ondblclick: () => { select(a.id); play(); },
    },
      avatar(a),
      el("div", { class: "tile-text" },
        el("div", { class: "tile-name" }, a.last ? icon("pin") : null, el("span", { text: a.name })),
        el("div", { class: "tile-meta" },
          el("span", { class: "tag " + a.provider, text: a.provider === "fxid" ? "FX ID" : "VK Play" }),
          a.branch ? el("span", { class: "tag branch", text: a.branch }) : null,
          op ? el("span", { class: "tile-op", title: tb(op.title) }, el("i", { class: "pulse" }), tb(op.title)) : null,
          a.login !== a.name && !op ? el("span", { class: "login", text: a.login }) : null),
      ),
      el("button", {
        class: "tile-play state", title: t("Play as {name}", { name: a.name }), disabled: !canPlay(a),
        onclick: (e) => { e.stopPropagation(); select(a.id); play(); },
        ondblclick: (e) => e.stopPropagation(),
      }, icon("play")),
      el("button", { class: "icon-btn", title: t("More"), onclick: (e) => { e.stopPropagation(); openMenu(e.currentTarget, a); } }, icon("more")),
    );
    tiles.append(tile);
  }
  $("account-count").textContent = shown.length;
  $("empty").classList.toggle("on", shown.length === 0);
  $("tiles").style.display = shown.length ? "" : "none";
  renderHero();
}

function select(id) {
  state.selected = id;
  for (const t of $("tiles").children) t.classList.remove("on");
  const i = state.accounts.findIndex((a) => a.id === id);
  if (i >= 0) $("tiles").children[i].classList.add("on");
  renderHero();
}

function selectedAccount() { return state.accounts.find((a) => a.id === state.selected); }

function renderHero() {
  const a = selectedAccount();
  const own = a && accountOp(a.id), game = gameOp();
  const playBtn = $("play");
  playBtn.disabled = !canPlay(a);
  playBtn.classList.toggle("working", !!own && own.game);
  playBtn.title = !a || canPlay(a) ? t("Play") : t("Wait until {op} finishes", { op: tb((own || game).title) });
  let chip = t("Add an account to start");
  if (own) chip = tb(own.title) + "…";
  else if (a && game) chip = t("{name} · {service} · waits for {op}", { name: a.name, service: a.service, op: tb(game.title) });
  else if (a) chip = `${a.name} · ${a.service}`;
  $("hero-chip").textContent = chip;
  $("close-game").hidden = !state.running;
  renderGame();
}

const CLIENT_NAMES = { vkplay: "VK Play", fxid: "FX ID main branch" };

function clientName(c) { return CLIENT_NAMES[c.kind] ? t(CLIENT_NAMES[c.kind]) : "FX ID " + c.branch; }

function clientRows(info) {
  const rows = [];
  const add = (c) => {
    const branch = c.branch || (c.kind === "fxid" ? "default" : "");
    let row = rows.find((r) => r.kind === c.kind && (c.kind !== "branch" || r.branch.toLowerCase() === branch.toLowerCase()));
    if (!row) rows.push(row = { kind: c.kind, branch });
    if (c.dir) row.installed = c;
    else if (c.version) row.available = c.version;
    if (c.account && !row.account) row.account = c.account;
  };
  const vk = state.accounts.find((a) => a.provider === "vkplay");
  const fx = state.accounts.filter((a) => a.provider === "fxid");
  if (vk) add({ kind: "vkplay", account: vk.id });
  if (fx.length) add({ kind: "fxid", account: fx[0].id });
  for (const a of fx) if (a.branch && a.branch.toLowerCase() !== "default") add({ kind: "branch", branch: a.branch, account: a.id });
  for (const c of state.availableClients || []) if (state.accounts.some((a) => a.id === c.account)) add(c);
  for (const c of Array.isArray(info.clients) ? info.clients : []) add(c);
  return rows;
}

function clientFolder(r, info) {
  if (r.installed) return r.installed.dir;
  if (r.kind === "vkplay") return state.game || state.suggestedGame;
  if (r.kind === "fxid") return state.fxGame;
  return info.branchPaths?.[r.branch.toLowerCase()] || (state.fxGame ? `${state.fxGame} ${r.branch}` : "");
}

function chooseClientFolder(r) {
  if (r.kind === "vkplay") send({ cmd: "gameFolder" });
  else if (r.kind === "fxid") send({ cmd: "fxFolder" });
  else send({ cmd: "branchFolder", branch: r.branch });
}

function openClientMenu(anchor, r) {
  const busy = !!gameOp();
  const target = { value: r.kind, branch: r.kind === "fxid" ? "" : r.branch };
  showMenu(anchor, [
    menuItem("update", t("Check for updates"), () => send({ cmd: "updateClient", ...target }), null, busy),
    menuItem("folder", t("Open folder"), () => send({ cmd: "openClientFolder", ...target })),
    menuItem("edit", t("Change folder…"), () => chooseClientFolder(r), null, busy),
    r.installed.downloads ? menuItem("clear", t("Delete downloaded patches ({size})", { size: formatBytes(r.installed.downloads) }), () => send({ cmd: "clearDownloads", ...target }), null, busy) : null,
    el("hr"),
    menuItem("delete", t("Remove client"), () => send({ cmd: r.kind === "branch" ? "removeBranch" : "removeMainClient", ...target }), "danger", busy),
  ]);
}

function renderGame() {
  const info = state.gameInfo || { clients: [], dir: state.game, downloads: null, free: null };
  const busy = !!gameOp();
  const box = $("game-clients");
  box.textContent = "";
  const rows = clientRows(info);
  for (const r of rows) {
    const c = r.installed;
    const folder = clientFolder(r, info);
    const sub = c ? [c.version, c.dir, c.downloads ? t("patches {size}", { size: formatBytes(c.downloads) }).replace(/ /g, "\u00a0") : ""] : [t("Not installed"), r.available, folder];
    const main = c
      ? el("button", { class: "btn tonal", disabled: busy, title: t("Check every file and download the ones that differ"), onclick: () => send({ cmd: "verify", value: r.kind, branch: r.kind === "fxid" ? "" : r.branch }) }, icon("verify", "sm"), t("Check files"))
      : el("button", { class: "btn tonal", disabled: busy || !r.account, onclick: () => send({ cmd: "downloadClient", account: r.account, value: r.kind, branch: r.branch }) }, t("Download"));
    box.append(el("div", { class: "list-item" },
      icon(r.kind === "branch" ? "branch" : "globe", "lead"),
      el("div", { class: "list-text" }, el("div", { class: "list-title", text: clientName(r) }), el("div", { class: "list-sub", text: sub.filter(Boolean).join(" · ") })),
      c ? null : el("button", { class: "btn text", disabled: busy, onclick: () => chooseClientFolder(r) }, t("Folder…")),
      main,
      c ? el("button", { class: "icon-btn", title: t("More"), onclick: (e) => { e.stopPropagation(); openClientMenu(e.currentTarget, r); } }, icon("more")) : null));
  }
  if (state.availableClientsFailed) {
    box.append(el("div", { class: "list-item" }, icon("info", "lead"),
      el("div", { class: "list-text" }, el("div", { class: "list-sub", text: t("Could not load some FX ID branches.") })),
      el("button", { class: "btn text", onclick: () => send({ cmd: "availableClients" }) }, t("Retry"))));
  }
  if (state.availableVKFailed) {
    box.append(el("div", { class: "list-item" }, icon("info", "lead"),
      el("div", { class: "list-text" }, el("div", { class: "list-sub", text: t("Could not load the VK Play catalog.") })),
      el("button", { class: "btn text", onclick: () => send({ cmd: "availableClients" }) }, t("Retry"))));
  }
  if (!rows.length) {
    box.append(el("div", { class: "list-item" }, icon("info", "lead"),
      el("div", { class: "list-text" }, el("div", { class: "list-title", text: t("No clients yet") }),
        el("div", { class: "list-sub", text: t("Add a VK Play or FX ID account to see its client here.") }))));
  }
}

const AUTOSTART_TEXT = {
  off: "AWLauncher starts only when you open it.",
  window: "Opens its window when you sign in to Windows.",
  tray: "Starts in the notification area when you sign in to Windows; click its icon to open it.",
};

function renderAutostart(mode) {
  state.autostart = mode;
  for (const b of $("autostart-mode").children) b.classList.toggle("on", b.dataset.mode === mode);
  $("autostart-sub").textContent = t(AUTOSTART_TEXT[mode] || AUTOSTART_TEXT.off);
}

function isRelease(v) { return /^v?\d+(\.\d+){0,2}(-|$)/.test(v); }

function renderUpdate() {
  const u = state.update, updating = state.ops.some((o) => o.title === "Updating AWLauncher");
  $("app-version").textContent = isRelease(state.version) ? state.version : "";
  let status = t("Updates come from the releases on GitHub.");
  if (state.version && !isRelease(state.version)) status = t("Development build. Updates come from the releases on GitHub.");
  if (state.checking) status = t("Checking GitHub for a newer release…");
  else if (updating) status = t("Downloading the update. AWLauncher restarts when it is ready.");
  else if (u && u.error) status = t("Could not check: {error}", { error: u.error });
  else if (u && u.available) status = u.published ? t("{latest} is available, released {date}.", { latest: u.latest, date: u.published }) : t("{latest} is available.", { latest: u.latest });
  else if (u && u.latest && isRelease(state.version)) status = t("Up to date. The latest release is {latest}.", { latest: u.latest });
  else if (u && u.latest) status = t("Development build. The latest release is {latest}.", { latest: u.latest });
  $("update-status").textContent = status;
  const available = !!(u && u.available);
  $("update-page").hidden = !available;
  const btn = $("update-check");
  btn.textContent = available ? (u.canApply ? t("Update to {latest}", { latest: u.latest }) : t("Download")) : t("Check for updates");
  btn.classList.toggle("filled", available);
  btn.classList.toggle("tonal", !available);
  const blocked = available && u.canApply && busy();
  btn.disabled = state.checking || updating || blocked;
  btn.title = blocked ? t("Wait until {op} finishes", { op: opsTitle() }) : "";
}

function checkUpdate() {
  const u = state.update;
  if (u && u.available) {
    if (u.canApply) applyUpdate(); else send({ cmd: "openLink", value: u.page });
    return;
  }
  state.checking = true;
  renderUpdate();
  send({ cmd: "checkUpdate" });
}

function applyUpdate() {
  if (busy()) { toast(t("Wait until {op} finishes", { op: opsTitle() })); return; }
  send({ cmd: "applyUpdate" });
}

function onUpdate(ev) {
  state.checking = false;
  state.update = ev;
  renderUpdate();
  if (ev.available && state.page !== "settings") $("settings-dot").classList.add("on");
  if (ev.quiet) return;
  if (ev.error) { toast(t("Could not check for updates: {error}", { error: ev.error })); return; }
  if (!ev.available) {
    toast(isRelease(ev.current) ? t("AWLauncher {version} is up to date", { version: ev.current }) : t("Development build; the latest release is {latest}", { latest: ev.latest }));
    return;
  }
  const notes = (ev.notes || "").replace(/\r/g, "");
  const actions = [
    { label: t("Later"), onClick: closeDialog },
    { label: t("Release page"), kind: ev.canApply ? "text" : "filled", primary: !ev.canApply, onClick: () => { closeDialog(); send({ cmd: "openLink", value: ev.page }); } },
  ];
  if (ev.canApply) actions.push({ label: t("Update"), kind: "filled", primary: true, onClick: () => { closeDialog(); applyUpdate(); } });
  const body = [el("p", { text: t("You have {current}.", { current: ev.current }) + (ev.published ? t(" {latest} was released {date}.", { latest: ev.latest, date: ev.published }) : "") +
    t(ev.canApply ? " AWLauncher downloads it and restarts; your accounts and settings stay." : " Download it from the release page.") })];
  if (notes) body.push(el("pre", { class: "context", text: notes.length > 1500 ? notes.slice(0, 1500) + "…" : notes }));
  whenDialogFree(() => openDialog({ iconName: "update", title: t("AWLauncher {latest} is available", { latest: ev.latest }), body, actions, onEscape: closeDialog }));
}

function renderStatus() {
  document.body.classList.toggle("busy", busy());
  document.body.classList.toggle("running", state.running && !busy());
  const paused = !!state.progress.paused;
  document.body.classList.toggle("paused", paused && busy());
  const loadingGame = gameOp() && (gameOp().title === "Downloading game" || (gameOp().title.startsWith("Starting ") && state.progress.active));
  $("drawer-status-text").textContent = busy() ? (paused ? t("Paused · ") : "") + (loadingGame ? t("Loading game") : opsTitle()) : state.running ? t("The game is running") : t("Ready");
  renderHero();
  renderUpdate();
}

function play() {
  const a = selectedAccount();
  if (canPlay(a)) send({ cmd: "play", account: a.id });
}


function menuItem(name, label, fn, cls, disabled) {
  return el("button", { class: cls, disabled, onclick: () => { closeMenu(); fn(); } }, icon(name), el("span", { text: label }));
}

function showMenu(anchor, items) {
  const menu = $("menu");
  menu.textContent = "";
  for (const item of items) if (item) menu.append(item);
  const r = anchor.getBoundingClientRect();
  menu.classList.add("on");
  const w = menu.offsetWidth, h = menu.offsetHeight;
  menu.style.left = Math.max(8, Math.min(r.right - w, innerWidth - w - 8)) + "px";
  menu.style.top = (r.bottom + h + 8 > innerHeight ? r.top - h - 4 : r.bottom + 4) + "px";
}
function closeMenu() { $("menu").classList.remove("on"); }

function openMenu(anchor, a) {
  const taken = !!accountOp(a.id);
  showMenu(anchor, [
    menuItem("play", t("Play"), () => { select(a.id); play(); }, null, !canPlay(a)),
    !a.last ? menuItem("pin", t("Make main"), () => { select(a.id); send({ cmd: "pin", account: a.id }); }) : null,
    a.provider === "fxid" ? menuItem("branch", t("Client branches"), () => send({ cmd: "branches", account: a.id }), null, taken) : null,
    a.provider === "fxid" ? menuItem("key", t("Activate key"), () => send({ cmd: "key", account: a.id }), null, taken) : null,
    menuItem("edit", t("Rename"), () => showRename(a)),
    el("hr"),
    menuItem("delete", t("Remove"), () => confirmRemove(a), "danger", taken),
  ]);
}


let dialogKeys = null;

const dialogQueue = [];
function whenDialogFree(show) {
  if (dialogOpen()) dialogQueue.push(show);
  else show();
}

function openDialog({ iconName, iconClass, op, title, body = [], actions = [], onEscape }) {
  const iconBox = $("dialog-icon");
  iconBox.textContent = "";
  iconBox.className = "dialog-icon" + (iconName ? " on" : "") + (iconClass ? " " + iconClass : "");
  if (iconName) iconBox.append(icon(iconName));
  $("dialog-op").hidden = !op;
  $("dialog-op").textContent = op || "";
  $("dialog-title").textContent = title;
  const bodyBox = $("dialog-body");
  bodyBox.textContent = "";
  bodyBox.append(...body);
  bodyBox.style.display = body.length ? "" : "none";
  const bar = $("dialog-actions");
  bar.textContent = "";
  let primary = null;
  for (const a of actions) {
    const b = el("button", { class: "btn state " + (a.kind || "text"), onclick: a.onClick }, a.label);
    if (a.primary) primary = b;
    bar.append(b);
  }
  dialogKeys = { onEscape, primary };
  $("scrim").classList.add("on");
  $("dialog").classList.add("on");
  const focus = bodyBox.querySelector("input") || primary;
  setTimeout(() => focus && focus.focus(), 40);
}

function closeDialog() {
  dialogKeys = null;
  $("scrim").classList.remove("on");
  $("dialog").classList.remove("on");
  const next = dialogQueue.shift();
  if (next) setTimeout(() => { if (dialogOpen()) dialogQueue.unshift(next); else next(); }, 160);
}

function dialogOpen() { return $("dialog").classList.contains("on"); }

function showAddAccount() {
  const choice = (provider, title) =>
    el("button", { class: "choice state", onclick: () => { closeDialog(); send({ cmd: "add", provider }); } },
      el("span", { class: "choice-title", text: title }));
  openDialog({
    title: t("Add account"),
    body: [
      el("div", { class: "choices" },
        choice("vkplay", "VK Play"),
        choice("fxid", "FX ID")),
    ],
    actions: [{ label: t("Cancel"), onClick: closeDialog }],
    onEscape: closeDialog,
  });
}

function showRename(a) {
  const input = el("input", { type: "text", autocomplete: "off", spellcheck: "false", id: "rename-input", maxlength: "64" });
  input.value = a.name === a.login ? "" : a.name;
  const save = () => { closeDialog(); send({ cmd: "rename", account: a.id, value: input.value.trim() }); };
  input.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); save(); } });
  openDialog({
    iconName: "edit",
    title: t("Rename account"),
    body: [
      el("p", { text: `${a.service} · ${a.login}` }),
      el("div", { class: "field" }, el("label", { for: "rename-input", text: t("Name") }), input),
      el("p", { class: "hint", text: t(a.provider === "fxid" ? "Leave it empty to show the e-mail." : "Leave it empty to show the account number.") }),
    ],
    actions: [{ label: t("Cancel"), onClick: closeDialog }, { label: t("Save"), kind: "filled", primary: true, onClick: save }],
    onEscape: closeDialog,
  });
  setTimeout(() => input.select(), 60);
}

function renderSignIn(ev) {
  $("signin-card").hidden = !ev.active;
  if (!ev.active) return;
  $("signin-text").textContent = t(ev.window
    ? "Sign in in the AWLauncher window. The browser sign-in keeps waiting too."
    : "Continue in the browser; with a VK Play session there it returns here by itself. If the browser asks, allow vkplay.ru to access apps on this device. Nothing happens? Sign in here.");
}

function confirmCloseGame() {
  openDialog({
    iconName: "close", iconClass: "error",
    title: t("Close the game?"),
    body: [el("p", { text: t("Armored Warfare is asked to close; if it does not respond in 10 seconds, it is ended. If you are in a battle, you leave it.") })],
    actions: [
      { label: t("Cancel"), onClick: closeDialog },
      { label: t("Close game"), kind: "danger", primary: true, onClick: () => { closeDialog(); send({ cmd: "closeGame" }); } },
    ],
    onEscape: closeDialog,
  });
}

function confirmRemove(a) {
  openDialog({
    iconName: "delete", iconClass: "error",
    title: t("Remove account?"),
    body: [el("p", { text: t("{name} ({service}) is removed from this launcher, with its saved sign-in. The game files stay.", { name: a.name, service: a.service }) })],
    actions: [
      { label: t("Cancel"), onClick: closeDialog },
      { label: t("Remove"), kind: "danger", primary: true, onClick: () => { closeDialog(); send({ cmd: "remove", account: a.id }); } },
    ],
    onEscape: closeDialog,
  });
}

function showPrompt(p) { whenDialogFree(() => openPrompt(p)); }

const PROMPT_FIELDS = {
  email: { label: "E-mail", type: "email", mode: "email", icon: "mail" },
  key: { label: "Key", icon: "key" },
  name: { label: "Name", icon: "edit" },
};

function openPrompt(p) {
  state.prompt = p;
  const context = (p.context || []).slice(-14);
  const question = tb(p.question);
  const reply = (value, ok) => { closeDialog(); state.prompt = null; send({ cmd: "answer", prompt: p.id, value, ok }); };

  if (p.kind === "confirm") {
    const yes = { label: t("Yes"), kind: p.default ? "filled" : "text", primary: p.default, onClick: () => reply("yes", true) };
    const no = { label: t("No"), kind: p.default ? "text" : "filled", primary: !p.default, onClick: () => reply("no", true) };
    openDialog({
      iconName: "help",
      op: tb(p.op),
      title: question,
      body: context.length ? [el("pre", { class: "context", text: context.join("\n") })] : [],
      actions: [no, yes],
      onEscape: () => reply("no", true),
    });
    return;
  }
  if (p.kind === "folder") { showFolderPrompt(p, context, reply); return; }
  if (p.kind === "code") { showCodePrompt(p, context, reply); return; }
  if (p.kind === "choice") { showChoicePrompt(p, question, context, reply); return; }

  const field = PROMPT_FIELDS[p.kind] || { label: "Answer", icon: "edit" };
  const input = el("input", { type: field.type || "text", inputmode: field.mode || "text", autocomplete: "off", spellcheck: "false", id: "prompt-input" });
  input.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); reply(input.value, true); } });
  const body = [];
  if (context.length) body.push(el("pre", { class: "context", text: context.join("\n") }));
  body.push(el("div", { class: "field" }, el("label", { for: "prompt-input", text: t(field.label) }), input));
  openDialog({
    iconName: field.icon,
    op: tb(p.op),
    title: question,
    body,
    actions: [
      { label: t("Cancel"), onClick: () => reply("", false) },
      { label: t("OK"), kind: "filled", primary: true, onClick: () => reply(input.value, true) },
    ],
    onEscape: () => reply("", false),
  });
}

function showChoicePrompt(p, question, context, reply) {
  const options = p.options || [];
  let chosen = (options.find((o) => o.current) || options[0] || { value: "" }).value;
  const list = el("div", { class: "options" });
  options.forEach((o, i) => {
    const opt = el("button", {
      class: "option state" + (o.value === chosen ? " on" : ""),
      onclick: () => { chosen = o.value; for (const x of list.children) x.classList.toggle("on", x === opt); },
      ondblclick: () => reply(o.value, true),
    },
    el("span", { class: "num", text: String(i + 1) }),
    el("span", { class: "grow" }, el("span", { text: o.label }), o.detail ? el("small", { class: "detail", text: o.detail }) : null),
    o.current ? el("small", { text: t("current") }) : null);
    list.append(opt);
  });
  const body = [];
  if (context.length) body.push(el("pre", { class: "context", text: context.join("\n") }));
  body.push(list);
  openDialog({
    iconName: "branch",
    op: tb(p.op),
    title: question,
    body,
    actions: [
      { label: t("Cancel"), onClick: () => reply("", false) },
      { label: t("OK"), kind: "filled", primary: true, onClick: () => reply(chosen, true) },
    ],
    onEscape: () => reply("", false),
  });
}

const CODE_LENGTH = 6;

function showCodePrompt(p, context, reply) {
  const boxes = [];
  let sent = false;
  const code = () => boxes.map((b) => b.value).join("");
  const submit = () => {
    if (sent) return;
    const value = code();
    if (value.length !== CODE_LENGTH) {
      const empty = boxes.find((b) => !b.value);
      if (empty) empty.focus();
      return;
    }
    sent = true;
    reply(value, true);
  };
  const fill = (from, digits) => {
    let i = from;
    for (const d of digits) {
      if (i >= CODE_LENGTH) break;
      boxes[i++].value = d;
    }
    boxes[Math.min(i, CODE_LENGTH - 1)].focus();
    if (code().length === CODE_LENGTH) submit();
  };
  const row = el("div", { class: "otp", role: "group", "aria-label": t("Code") });
  for (let i = 0; i < CODE_LENGTH; i++) {
    const box = el("input", {
      type: "text", inputmode: "numeric", autocomplete: i === 0 ? "one-time-code" : "off", spellcheck: "false",
      "aria-label": t("Digit {n}", { n: i + 1 }),
    });
    box.addEventListener("focus", () => box.select());
    box.addEventListener("input", () => {
      const digits = box.value.replace(/\D/g, "");
      box.value = "";
      if (digits) fill(i, digits);
    });
    box.addEventListener("keydown", (e) => {
      if (e.key === "Backspace" && !box.value && i > 0) {
        e.preventDefault();
        boxes[i - 1].value = "";
        boxes[i - 1].focus();
      } else if (e.key === "ArrowLeft" && i > 0) {
        e.preventDefault();
        boxes[i - 1].focus();
      } else if (e.key === "ArrowRight" && i < CODE_LENGTH - 1) {
        e.preventDefault();
        boxes[i + 1].focus();
      } else if (e.key === "Enter") {
        e.preventDefault();
        submit();
      }
    });
    box.addEventListener("paste", (e) => {
      e.preventDefault();
      const digits = (e.clipboardData || window.clipboardData).getData("text").replace(/\D/g, "");
      if (!digits) return;
      if (digits.length >= CODE_LENGTH) {
        for (const b of boxes) b.value = "";
        fill(0, digits.slice(0, CODE_LENGTH));
      } else fill(i, digits);
    });
    boxes.push(box);
    row.append(box);
  }
  const body = [];
  if (context.length) body.push(el("pre", { class: "context", text: context.join("\n") }));
  body.push(row);
  openDialog({
    iconName: "mail",
    op: tb(p.op),
    title: t("Code from the e-mail"),
    body,
    actions: [
      { label: t("Cancel"), onClick: () => { sent = true; reply("", false); } },
      { label: t("OK"), kind: "filled", primary: true, onClick: submit },
    ],
    onEscape: () => { sent = true; reply("", false); },
  });
}

const CLIENT_SIZE = 70 * GiB;

function showFolderPrompt(p, context, reply) {
  const notes = context.map((l) => el("p", { text: l.trim() }));
  const input = el("input", { type: "text", autocomplete: "off", spellcheck: "false", id: "prompt-input" });
  input.value = p.suggest || "";
  const status = el("div", { class: "folder-status", id: "folder-status" });
  const ok = { label: t("Use this folder"), kind: "filled", primary: true, onClick: () => { if (input.value.trim()) reply(input.value.trim(), true); } };
  let timer = 0;
  const check = () => { clearTimeout(timer); timer = setTimeout(() => send({ cmd: "folderInfo", prompt: p.id, value: input.value }), 250); };
  input.addEventListener("input", check);
  input.addEventListener("keydown", (e) => { if (e.key === "Enter") { e.preventDefault(); ok.onClick(); } });
  openDialog({
    iconName: "folder",
    op: tb(p.op),
    title: t("Game folder"),
    body: [
      ...(notes.length ? notes : [el("p", { text: t("Choose the folder of an existing install, or an empty folder to install the game into.") })]),
      el("div", { class: "folder-row" },
        el("div", { class: "field" }, el("label", { for: "prompt-input", text: t("Folder") }), input),
        el("button", { class: "btn tonal state", onclick: () => send({ cmd: "browse", prompt: p.id, value: input.value }) }, icon("folder", "sm"), t("Browse…"))),
      status,
    ],
    actions: [{ label: t("Cancel"), onClick: () => reply("", false) }, ok],
    onEscape: () => reply("", false),
  });
  send({ cmd: "folderInfo", prompt: p.id, value: input.value });
}

function onFolderInfo(ev) {
  const box = $("folder-status");
  const input = $("prompt-input");
  if (!box || !state.prompt || state.prompt.id !== ev.prompt || input.value !== ev.path) return;
  box.textContent = "";
  const row = (name, cls, text) => box.append(el("div", { class: "folder-note " + cls }, icon(name, "sm"), el("span", { text })));
  if (!ev.path.trim()) { row("info", "", t("Type a folder path or choose Browse.")); return; }
  if (!ev.valid) { row("error", "bad", t("Enter a full path on an existing drive, like D:\\Games\\Armored Warfare.")); return; }
  const sameBranch = ev.branch && ev.branch.toLowerCase() === (state.prompt.branch || "").toLowerCase();
  if (sameBranch) row("check", "good", t("This folder contains the selected FX ID branch."));
  else if (ev.branch) row("error", "bad", t("This folder holds the FX ID {branch} branch. Choose another folder.", { branch: ev.branch }));
  else if (ev.install) row("check", "good", t("Armored Warfare is installed here. The launcher checks it and installs updates."));
  else if (ev.used) row("info", "warn", t("The folder is not empty. The client is installed into it next to the existing files."));
  else row("add", "", t("The full client is installed here. The folder is created if it does not exist."));
  const enough = ev.install || sameBranch || ev.free >= CLIENT_SIZE;
  box.append(el("div", { class: "disk" + (enough ? "" : " low") },
    icon("storage", "sm"),
    el("span", { class: "disk-drive", text: t("Drive {drive}", { drive: ev.drive }) }),
    el("span", { class: "disk-free", text: t("{free} free", { free: formatBytes(ev.free) }) + (ev.install || sameBranch ? "" : t(" · the client needs about {size}", { size: formatBytes(CLIENT_SIZE) })) })));
  if (!enough) row("error", "bad", t("There is not enough free space on this drive for a new install."));
}

function onBrowsed(ev) {
  const input = $("prompt-input");
  if (state.prompt && state.prompt.id === ev.prompt && input) {
    input.value = ev.path;
    input.focus();
    if (state.prompt.kind === "folder") send({ cmd: "folderInfo", prompt: ev.prompt, value: ev.path });
  }
}

function onDone(ev) {
  if (state.page === "game") { send({ cmd: "gameInfo" }); send({ cmd: "availableClients" }); }
  switch (ev.status) {
    case "error":
      toast(ev.message, "error", { title: tb(ev.title), action: { label: t("Logs"), onClick: () => showPage("activity") } });
      break;
    case "launched": toast(t("Game started. AWLauncher is in the notification area.")); break;
    case "cancelled": toast(t("Cancelled")); break;
    case "ok":
      if (ev.message) toast(tb(ev.message), "ok");
      break;
  }
}

const TOAST_ICONS = { info: "info", ok: "check", error: "error" };

function toast(text, kind = "info", opts = {}) {
  const box = $("toasts");
  const node = el("div", { class: "toast " + kind, role: kind === "error" ? "alert" : "status" });
  const close = () => { node.classList.remove("on"); setTimeout(() => node.remove(), 250); };
  const body = el("div", { class: "toast-text" }, opts.title ? el("div", { class: "toast-title", text: opts.title }) : null, el("div", { text }));
  if (opts.action) body.append(el("button", { class: "btn text", onclick: () => { close(); opts.action.onClick(); } }, opts.action.label));
  node.append(icon(TOAST_ICONS[kind] || "info"), body, el("button", { class: "toast-close", title: t("Close"), onclick: close }, icon("close", "sm")));
  box.append(node);
  while (box.children.length > 4) box.firstChild.remove();
  requestAnimationFrame(() => node.classList.add("on"));
  setTimeout(close, kind === "error" ? 12000 : 5000);
}


const LOG_LIMIT = 200000;
const urlRe = /https:\/\/[^\s"'<>]+/g;

function appendLog(text, initial) {
  const log = $("log");
  const stick = log.scrollTop + log.clientHeight >= log.scrollHeight - 24;
  const frag = document.createDocumentFragment();
  for (const line of text.split("\n")) {
    if (line === "") continue;
    const span = el("span", { class: /^(Error|.*\bfailed\b)/i.test(line) ? "err" : /^===/.test(line) ? "head" : null });
    let at = 0;
    for (const m of line.matchAll(urlRe)) {
      span.append(line.slice(at, m.index));
      span.append(el("a", { href: "#", "data-link": m[0], text: m[0] }));
      at = m.index + m[0].length;
    }
    span.append(line.slice(at) + "\n");
    frag.append(span);
  }
  log.append(frag);
  while (log.textContent.length > LOG_LIMIT && log.firstChild) log.firstChild.remove();
  if (stick || initial) log.scrollTop = log.scrollHeight;
  if (!initial && state.page !== "activity") $("activity-dot").classList.add("on");
}


const wave = { amp: 0, phase: 0, running: false, last: 0 };

function renderProgress() {
  const p = state.progress;
  let title = t("Ready"), meta = "";
  if (p.active) {
    title = tb(p.title);
    const frac = p.total > 0 ? Math.min(1, p.done / p.total) : 0;
    const parts = [(frac * 100).toFixed(1) + "%", p.files ? t("{done} / {total} files", { done: p.done, total: p.total }) : formatPair(p.done, p.total)];
    if (p.paused && p.pausable) parts.push(t("paused"));
    else if (p.speed > 0) {
      parts.push(formatBytes(p.speed) + "/s");
      if (p.total > p.done) parts.push(t("{time} left", { time: formatDuration((p.total - p.done) / p.speed) }));
    }
    meta = parts.join(" · ");
  } else if (busy()) {
    title = opsTitle();
  }
  if (!p.active && p.paused && busy()) meta = t("paused");
  $("progress-title").textContent = title;
  $("progress-meta").textContent = meta;
  const pauseBtn = $("progress-pause");
  pauseBtn.hidden = !(p.paused ? busy() : p.active && p.pausable);
  pauseBtn.querySelector("use").setAttribute("href", p.paused ? "#i-play" : "#i-pause");
  pauseBtn.querySelector("span").textContent = t(p.paused ? "Resume" : "Pause");
  pauseBtn.title = t(p.paused ? "Go on from where it stopped" : "Pause the download or check; it goes on from the same place");

  const tasks = $("tasks");
  tasks.textContent = "";
  for (const t of (p.active && p.tasks) || []) {
    if (tasks.children.length >= 4) break;
    const f = t.total > 0 ? Math.min(1, t.done / t.total) : 0;
    const bar = el("div", { class: "task-bar" }, el("i", { style: `width:${(f * 100).toFixed(1)}%` }));
    tasks.append(el("div", { class: "task" },
      el("span", { class: "task-name", title: t.label, text: clipLeft(t.label, 64) }),
      bar,
      el("span", { class: "task-num", text: t.total > 0 ? `${(f * 100).toFixed(0)}% · ${formatPair(t.done, t.total)}` : "" })));
  }
  if (!wave.running) { wave.running = true; requestAnimationFrame(drawWave); }
}

function sinePath(x0, x1, mid, amp, phase) {
  if (x1 <= x0) return "";
  let d = `M${x0.toFixed(1)},${(mid + amp * Math.sin(x0 / 7 + phase)).toFixed(2)}`;
  for (let x = x0 + 2; x < x1; x += 2) d += `L${x.toFixed(1)},${(mid + amp * Math.sin(x / 7 + phase)).toFixed(2)}`;
  d += `L${x1.toFixed(1)},${(mid + amp * Math.sin(x1 / 7 + phase)).toFixed(2)}`;
  return d;
}

function drawWave(now) {
  const svg = $("wave");
  const W = svg.clientWidth, mid = 9, pad = 3, gap = 10;
  const p = state.progress;
  const moving = (busy() || p.active) && !p.paused;
  const dt = Math.min(64, now - (wave.last || now));
  wave.last = now;
  wave.amp += ((moving ? 3.2 : 0) - wave.amp) * Math.min(1, dt / 180);
  wave.phase -= dt * 0.006;

  let active = "", track = "", stop = false;
  if (p.active && p.total > 0) {
    const x1 = pad + Math.min(1, p.done / p.total) * (W - 2 * pad);
    active = sinePath(pad, x1, mid, wave.amp, wave.phase);
    if (x1 + gap < W - pad) { track = `M${x1 + gap},${mid}L${W - pad},${mid}`; stop = true; }
  } else if (busy() && !p.paused) {
    const t = (now % 1800) / 1800;
    const a = pad + Math.max(0, (t * 1.5 - 0.5)) * (W - 2 * pad);
    const b = pad + Math.min(1, t * 1.5) * (W - 2 * pad);
    active = sinePath(a, b, mid, wave.amp, wave.phase);
    if (a - gap > pad) track += `M${pad},${mid}L${a - gap},${mid}`;
    if (b + gap < W - pad) track += `M${b + gap},${mid}L${W - pad},${mid}`;
  } else {
    track = `M${pad},${mid}L${W - pad},${mid}`;
  }
  $("wave-active").setAttribute("d", active);
  $("wave-track").setAttribute("d", track);
  const dot = $("wave-stop");
  dot.setAttribute("cx", W - pad);
  dot.setAttribute("cy", mid);
  dot.style.display = stop ? "" : "none";

  if (moving || wave.amp > 0.05) requestAnimationFrame(drawWave);
  else { wave.running = false; wave.last = 0; }
}


function setup() {
  setupTheme();
  collectStatic();
  applyLanguage();
  for (const b of $("lang-mode").children) b.addEventListener("click", () => setLanguage(b.dataset.lang));
  for (const b of document.querySelectorAll(".nav-item")) b.addEventListener("click", () => showPage(b.dataset.page));
  $("play").addEventListener("click", play);
  $("close-game").addEventListener("click", confirmCloseGame);
  for (const [id, cmd] of [["signin-open", "signinOpen"], ["signin-fresh", "signinFresh"], ["signin-here", "signinHere"]]) {
    $(id).addEventListener("click", () => send({ cmd }));
  }
  $("signin-cancel").addEventListener("click", () => send({ cmd: "signinCancel" }));
  $("add-inline").addEventListener("click", showAddAccount);
  $("add-empty").addEventListener("click", showAddAccount);
  for (const b of $("mods-mode").children) b.addEventListener("click", () => send({ cmd: "allowMods", value: b.dataset.mode }));
  $("data-open").addEventListener("click", () => send({ cmd: "openDataFolder" }));
  $("update-check").addEventListener("click", checkUpdate);
  for (const b of $("autostart-mode").children) b.addEventListener("click", () => { renderAutostart(b.dataset.mode); send({ cmd: "autostart", value: b.dataset.mode }); });
  $("progress-pause").addEventListener("click", () => send({ cmd: state.progress.paused ? "resume" : "pause" }));
  $("update-page").addEventListener("click", () => state.update && send({ cmd: "openLink", value: state.update.page }));
  $("log-copy").addEventListener("click", () => send({ cmd: "copyLogs", value: $("log").textContent }));
  $("log-clear").addEventListener("click", () => { $("log").textContent = ""; });
  $("log").addEventListener("click", (e) => {
    const a = e.target.closest("a[data-link]");
    if (a) { e.preventDefault(); send({ cmd: "openLink", value: a.dataset.link }); }
  });
  document.addEventListener("pointerdown", (e) => { if (!e.target.closest("#menu")) closeMenu(); });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      if ($("menu").classList.contains("on")) closeMenu();
      else if (dialogKeys && dialogKeys.onEscape) dialogKeys.onEscape();
      return;
    }
    if (e.key === "Enter" && dialogOpen() && dialogKeys && dialogKeys.primary && e.target.tagName !== "INPUT" && e.target.tagName !== "BUTTON") {
      e.preventDefault();
      dialogKeys.primary.click();
      return;
    }
  });
  addEventListener("resize", () => { if (!wave.running) { wave.running = true; requestAnimationFrame(drawWave); } });
  renderProgress();
  document.fonts.ready.then(() => requestAnimationFrame(() => send({ cmd: "ready" })));
}


const demo = {
  timer: 0,
  handle(cmd) {
    const accounts = demo.accounts || (demo.accounts = [
      { id: "1", name: "Tanker", login: "123456789", service: "VK Play", provider: "vkplay", last: true },
      { id: "2", name: "EU main", login: "player@example.com", service: "FX ID", provider: "fxid", last: false },
      { id: "3", name: "Supertest", login: "tester@example.com", service: "FX ID, supertest", provider: "fxid", branch: "supertest", last: false },
    ]);
    const emit = (ev) => setTimeout(() => aw.recv(ev), 30);
    const ops = demo.ops || (demo.ops = []);
    const stateEv = () => ({ type: "state", accounts, game: "H:\\Games\\Armored Warfare", data: "C:\\Users\\player\\AppData\\Local\\AWLauncher", version: "v0.1.1", autostart: demo.autostart || "off", allowMods: !!demo.allowMods, systemLang: /^ru/i.test(navigator.language) ? "ru" : "en", ops: [...ops] });
    const begin = (op) => { ops.push(op); emit(stateEv()); };
    const end = (id) => { const i = ops.findIndex((o) => o.id === id); if (i >= 0) ops.splice(i, 1); emit(stateEv()); };
    switch (cmd.cmd) {
      case "ready":
        emit({ ...stateEv(), log: "Game: H:\\Games\\Armored Warfare (build 442)\nBuild 442 is up to date.\n" });
        break;
      case "copyLogs":
        if (navigator.clipboard?.writeText) {
          navigator.clipboard.writeText(cmd.value).then(() => emit({ type: "clipboard", ok: true }), () => emit({ type: "clipboard", ok: false }));
        } else {
          emit({ type: "clipboard", ok: false });
        }
        break;
      case "play": {
        begin({ id: 1, title: "Starting Tanker", game: true, account: "1" });
        emit({ type: "log", text: "Checking for updates...\nUpdate available: 442 -> 443 (1 patches).\n" });
        emit({ type: "prompt", id: 1, op: "Starting Tanker", kind: "confirm", question: "Install now?", default: true, context: ["Checking for updates...", "Update available: 442 -> 443 (1 patches)."] });
        break;
      }
      case "answer": {
        if (cmd.prompt === 3 && cmd.ok) {
          emit({ type: "prompt", id: 8, op: "Signing in to FX ID", kind: "code", question: "Code from the e-mail", context: ["A code was sent to " + (cmd.value || "player@example.com") + "."] });
          break;
        }
        if (cmd.prompt === 8) {
          emit({ type: "notice", message: cmd.ok ? "Code " + cmd.value + " sent for verification" : "Cancelled" });
          break;
        }
        if (cmd.prompt === 10 && cmd.value === "yes") {
          demo.mainRemoved = true;
          emit({ type: "done", status: "ok", title: "Removing VK Play", message: "Client removed" });
        }
        if (cmd.prompt !== 1) break;
        let done = 0;
        const total = 3.4 * GiB;
        clearInterval(demo.timer);
        demo.timer = setInterval(() => {
          if (!demo.paused) done = Math.min(total, done + 0.045 * GiB);
          emit({ type: "progress", active: done < total, pausable: true, paused: !!demo.paused, title: "Downloading", done, total, speed: demo.paused ? 0 : 44 << 20,
            tasks: [{ label: "payload-442-443/gamesdk/textures_hi-0134.pak", done: done % (205 << 20), total: 205 << 20 },
                    { label: "payload-442-443/gamesdk/levels/pve_05/level.pak", done: (done * 0.7) % (120 << 20), total: 120 << 20 }] });
          if (done >= total) {
            clearInterval(demo.timer);
            emit({ type: "log", text: "Downloading   done: 3.4 GiB in 1m19s, 44.0 MiB/s\n" });
            end(1);
            emit({ type: "done", status: "launched", title: "Starting Tanker" });
            emit({ type: "game", running: true });
          }
        }, 250);
        break;
      }
      case "branches":
        begin({ id: 2, title: "Loading branches of EU main", account: "2" });
        emit({ type: "prompt", id: 2, op: "Loading branches of EU main", kind: "choice", question: "Which branch should EU main play?", context: ["Asking FX ID for branches of EU main..."], options: [
          { value: "default", label: "default", detail: "0.566.1 (build 5661), 12.08.2026, 68.3 GiB", current: true },
          { value: "supertest", label: "supertest", detail: "0.567.0 (build 5670), 20.09.2026, 68.9 GiB" }] });
        break;
      case "remove":
        emit({ type: "done", status: "ok", title: "Removing", message: "Account removed" });
        const gone = accounts.findIndex((x) => x.id === cmd.account);
        if (gone >= 0) accounts.splice(gone, 1);
        emit(stateEv());
        break;
      case "rename": {
        const a = accounts.find((x) => x.id === cmd.account);
        a.name = cmd.value || a.login;
        emit(stateEv());
        emit({ type: "notice", message: "Renamed to " + a.name });
        break;
      }
      case "signinHere":
        emit({ type: "signin", active: true, window: true });
        break;
      case "signinCancel":
        emit({ type: "signin", active: false });
        end(4);
        emit({ type: "done", status: "cancelled", title: "Signing in to VK Play" });
        break;
      case "closeGame":
        demo.running = false;
        emit({ type: "game", running: false });
        emit({ type: "notice", message: "The game is closed" });
        break;
      case "add":
        if (cmd.provider === "vkplay") {
          begin({ id: 4, title: "Signing in to VK Play" });
          emit({ type: "prompt", id: 4, op: "Signing in to VK Play", kind: "folder", folder: "vkplay", suggest: "H:\\Games\\Armored Warfare", question: "VK Play game folder",
            context: ["Armored Warfare was not found. Enter the folder of an existing install, or any folder to install the game into."] });
        } else emit({ type: "prompt", id: 3, op: "Signing in to FX ID", kind: "email", question: "E-mail of the FX ID account", context: [] });
        break;
      case "folderInfo": {
        const v = cmd.value.trim();
        const valid = /^[a-z]:\\/i.test(v);
        emit({ type: "folderInfo", prompt: cmd.prompt, path: cmd.value, valid, drive: v.slice(0, 2).toUpperCase(),
          free: /^c:/i.test(v) ? 41.3 * GiB : 812.4 * GiB, install: /armored warfare$/i.test(v) && /^d:/i.test(v), used: /^c:/i.test(v) });
        break;
      }
      case "browse":
        emit({ type: "browsed", prompt: cmd.prompt, path: "D:\\Games\\Armored Warfare" });
        break;
      case "gameInfo":
        emit({ type: "gameInfo", dir: "H:\\Games\\Armored Warfare", downloads: 3.2 * GiB, free: 812.4 * GiB, clients: [
          ...(!demo.mainRemoved ? [{ kind: "vkplay", dir: "H:\\Games\\Armored Warfare", version: "build 442", downloads: demo.cleared ? 0 : 3.2 * GiB }] : []),
          { kind: "fxid", branch: "default", dir: "H:\\Games\\Armored Warfare FX ID", version: "0.566.1" },
          ...(demo.downloaded ? [{ kind: "branch", branch: "SuperTest", dir: "H:\\Games\\Armored Warfare FX ID SuperTest", version: "0.567.0" }] : [])] });
        break;
      case "availableClients":
        emit({ type: "availableClients", clients: [
          { kind: "vkplay", account: "1" },
          { kind: "fxid", branch: "default", account: "2", version: "0.566.1" },
          { kind: "branch", branch: "SuperTest", account: "3", version: "0.567.0" }] });
        break;
      case "downloadClient":
        begin({ id: 9, title: "Downloading game", game: true, account: cmd.account });
        setTimeout(() => { demo.downloaded = true; end(9); emit({ type: "done", status: "ok", title: "Downloading game" }); }, 1500);
        break;
      case "verify":
        begin({ id: 5, title: "Checking " + (cmd.value === "vkplay" ? "VK Play" : "FX ID " + cmd.branch), game: true });
        setTimeout(() => { end(5); emit({ type: "notice", kind: "ok", message: cmd.value === "vkplay" ? "VK Play build 442: all files are checked" : "FX ID main branch 0.566.1: all files are checked" }); emit({ type: "done", status: "ok", title: "Checking" }); }, 1500);
        break;
      case "autostart":
        demo.autostart = cmd.value;
        emit(stateEv());
        break;
      case "allowMods":
        demo.allowMods = cmd.value === "on";
        emit(stateEv());
        break;
      case "pin": {
        for (const a of accounts) a.last = a.id === cmd.account;
        emit(stateEv());
        emit({ type: "notice", message: accounts.find((a) => a.last).name + " is the main account" });
        break;
      }
      case "pause":
      case "resume":
        demo.paused = cmd.cmd === "pause";
        emit({ type: "log", text: demo.paused ? "Paused. Press Resume to go on from where it stopped.\n" : "Resumed.\n" });
        break;
      case "checkUpdate":
        setTimeout(() => aw.recv({ type: "update", current: "v0.1.1", latest: "v0.2.0", published: "27.09.2026", available: true, canApply: true,
          page: "https://github.com/TheGreatPepix/awlauncher/releases/tag/v0.2.0", notes: "## What's Changed\n* Check for launcher updates in Settings\n* Update the launcher in place" }), 700);
        break;
      case "applyUpdate":
        begin({ id: 7, title: "Updating AWLauncher", game: true });
        setTimeout(() => { end(7); emit({ type: "done", status: "ok", title: "Updating AWLauncher", message: "The update is installed. It takes effect on the next start." }); }, 2000);
        break;
      case "removeMainClient":
        emit({ type: "prompt", id: 10, op: "Removing VK Play", kind: "confirm", question: "Remove VK Play from H:\\Games\\Armored Warfare?", default: false });
        break;
      case "clearDownloads":
        demo.cleared = true;
        emit({ type: "done", status: "ok", title: "Deleting downloaded patches" });
        break;
      case "updateClient":
        begin({ id: 11, title: "Updating " + (cmd.value === "vkplay" ? "VK Play" : "FX ID"), game: true });
        setTimeout(() => { end(11); emit({ type: "notice", message: cmd.value === "vkplay" ? "VK Play build 442 is up to date" : "FX ID main branch 0.566.1 is up to date" }); emit({ type: "done", status: "ok", title: "Updating" }); }, 1200);
        break;
    }
    if (cmd.cmd === "answer" && cmd.prompt !== 1) end(cmd.prompt);
  },
};

setup();
