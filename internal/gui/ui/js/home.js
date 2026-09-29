"use strict";

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
          a.language ? el("span", { class: "tag branch", title: t("Game language"), text: a.language }) : null,
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

function play() {
  const a = selectedAccount();
  if (canPlay(a)) send({ cmd: "play", account: a.id });
}

function openMenu(anchor, a) {
  const taken = !!accountOp(a.id);
  showMenu(anchor, [
    menuItem("play", t("Play"), () => { select(a.id); play(); }, null, !canPlay(a)),
    !a.last ? menuItem("pin", t("Make main"), () => { select(a.id); send({ cmd: "pin", account: a.id }); }) : null,
    a.provider === "fxid" ? menuItem("branch", t("Client branches"), () => send({ cmd: "branches", account: a.id }), null, taken) : null,
    a.provider === "fxid" ? menuItem("key", t("Activate key"), () => send({ cmd: "key", account: a.id }), null, taken) : null,
    menuItem("globe", t("Game language"), () => showGameLanguage(a)),
    menuItem("edit", t("Rename"), () => showRename(a)),
    el("hr"),
    menuItem("delete", t("Remove"), () => confirmRemove(a), "danger", taken),
  ]);
}

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

const GAME_LANGUAGES = { en: "English", de: "Deutsch", fr: "Français", pl: "Polski", ru: "Русский", zh: "中文 (简体)" };

function showGameLanguage(a) {
  const choose = (code) => { closeDialog(); send({ cmd: "language", account: a.id, value: code }); };
  const option = (code, label) => el("button", { class: "option state" + ((a.language || "") === code ? " on" : ""), onclick: () => choose(code) },
    el("span", { class: "grow", text: label }),
    (a.language || "") === code ? icon("check") : null);
  openDialog({
    iconName: "globe",
    title: t("Game language"),
    body: [
      el("p", { text: `${a.service} · ${a.name}` }),
      el("div", { class: "options" },
        option("", t("Automatically ({lang})", { lang: GAME_LANGUAGES[a.autoLanguage] || a.autoLanguage })),
        ...(a.languages || []).map((code) => option(code, GAME_LANGUAGES[code] || code))),
      el("p", { class: "hint", text: t("Automatically follows the Windows languages.") }),
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
