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
