"use strict";

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
