"use strict";

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

function showPage(page) {
  state.page = page;
  for (const b of document.querySelectorAll(".nav-item")) b.classList.toggle("on", b.dataset.page === page);
  for (const p of document.querySelectorAll(".page")) p.classList.toggle("on", p.id === "page-" + page);
  if (page === "game") { send({ cmd: "gameInfo" }); send({ cmd: "availableClients" }); }
  if (page === "home" && !wave.running) { wave.running = true; requestAnimationFrame(drawWave); }
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

function setup() {
  setupTheme();
  setupPlayShapes();
  setupLogoDrop();
  setupPong();
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

setup();
