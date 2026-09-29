"use strict";

const PALETTE_STYLES = {
  tonal: { name: "Tonal", h1: 0, h2: 0, h3: 60, pc: 1, sc: 1, nc: 1 },
  vibrant: { name: "Vibrant", h1: 0, h2: 15, h3: 45, pc: 1.45, sc: 1.8, nc: 1.4 },
  expressive: { name: "Expressive", h1: 0, h2: -45, h3: 120, pc: 1.2, sc: 1.8, nc: 1.6 },
  fruit: { name: "Fruit salad", h1: -50, h2: -50, h3: 0, pc: 1.25, sc: 1.6, nc: 1.1 },
  neutral: { name: "Neutral", h1: 0, h2: 0, h3: 60, pc: 0.4, sc: 0.45, nc: 0.4 },
  mono: { name: "Monochrome", h1: 0, h2: 0, h3: 0, pc: 0, sc: 0, nc: 0 },
};

const PRESETS = [
  { name: "Lavender", hue: 300, style: "tonal" },
  { name: "Rose", hue: 355, style: "tonal" },
  { name: "Amber", hue: 60, style: "tonal" },
  { name: "Mint", hue: 165, style: "tonal" },
  { name: "Sky", hue: 240, style: "tonal" },
  { name: "Coral", hue: 25, style: "vibrant" },
  { name: "Aurora", hue: 280, style: "expressive" },
  { name: "Graphite", hue: 300, style: "mono" },
];

const MAX_PALETTES = 12;
const HUE_TRACK = `linear-gradient(to right, ${Array.from({ length: 13 }, (_, i) => `oklch(0.6 0.14 ${i * 30})`).join(", ")})`;

function validHue(h) { return Number.isFinite(h) && h >= 0 && h < 360 ? Math.round(h) : 300; }

function validPalette(p) {
  return { hue: validHue(Number(p && p.hue)), style: PALETTE_STYLES[p && p.style] ? p.style : "tonal" };
}

function samePalette(a, b) { return a.hue === b.hue && a.style === b.style; }

function paletteVars(p) {
  const s = PALETTE_STYLES[p.style];
  const at = (shift) => (p.hue + shift + 360) % 360;
  return { "--h": at(s.h1), "--h2": at(s.h2), "--h3": at(s.h3), "--pc": s.pc, "--sc": s.sc, "--nc": s.nc };
}

function paletteStyle(p) { return Object.entries(paletteVars(p)).map(([k, v]) => `${k}:${v}`).join(";"); }

function paletteName(p) {
  const preset = PRESETS.find((x) => samePalette(x, p));
  return t(preset ? preset.name : "Custom") + " · " + t(PALETTE_STYLES[p.style].name);
}

const store = {
  get(k, d) { try { return localStorage.getItem(k) ?? d; } catch { return d; } },
  set(k, v) { try { localStorage.setItem(k, v); } catch {} },
};

function storedJSON(k, d) {
  try { return JSON.parse(store.get(k, "")) ?? d; } catch { return d; }
}

const darkQuery = matchMedia("(prefers-color-scheme: dark)");

const prefs = {
  theme: host ? "system" : store.get("aw.theme", "system"),
  palette: host ? validPalette({}) : validPalette(storedJSON("aw.palette", { hue: Number(store.get("aw.hue", "300")) })),
  palettes: host ? [] : storedJSON("aw.palettes", []).map(validPalette),
  lang: host ? "auto" : store.get("aw.lang", "auto"),
};

function loadPrefs(p) {
  prefs.theme = p.theme || "system";
  prefs.palette = validPalette({ hue: p.hue, style: p.style });
  prefs.palettes = (p.palettes || []).map(validPalette);
  prefs.lang = p.lang || "auto";
  renderPalettes();
  applyTheme();
}

function setPrefs(change) {
  Object.assign(prefs, change);
  if (host) {
    send({ cmd: "prefs", theme: prefs.theme, hue: prefs.palette.hue, style: prefs.palette.style, palettes: prefs.palettes, lang: prefs.lang });
  } else {
    store.set("aw.theme", prefs.theme);
    store.set("aw.palette", JSON.stringify(prefs.palette));
    store.set("aw.palettes", JSON.stringify(prefs.palettes));
    store.set("aw.lang", prefs.lang);
  }
  applyTheme();
}

function applyLanguage() {
  i18n.pref = prefs.lang;
  resolveLang();
  applyStatic();
  for (const b of $("lang-mode").children) b.classList.toggle("on", b.dataset.lang === prefs.lang);
  renderPalettes();
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

function applyTheme(preview) {
  const mode = prefs.theme;
  const palette = preview || prefs.palette;
  const root = document.documentElement;
  if (mode === "system") root.removeAttribute("data-theme"); else root.dataset.theme = mode;
  root.classList.toggle("system-dark", darkQuery.matches);
  for (const [k, v] of Object.entries(paletteVars(palette))) root.style.setProperty(k, v);
  for (const b of $("theme-mode").children) b.classList.toggle("on", b.dataset.theme === mode);
  markPalette(palette);
  const css = getComputedStyle(document.body);
  const dark = mode === "dark" || (mode === "system" && darkQuery.matches);
  send({ cmd: "theme", dark, caption: toHex(css.backgroundColor), text: toHex(css.color) });
}

function paletteKey(p) { return p.style + ":" + p.hue; }

function paletteSwatch(p, title, cls = "") {
  return el("button", {
    class: "palette " + cls, title, "data-key": paletteKey(p), style: paletteStyle(p), "aria-label": title,
    onclick: () => setPrefs({ palette: { ...p } }),
  }, icon("check"));
}

function renderPalettes() {
  const box = $("palettes");
  box.textContent = "";
  for (const p of PRESETS) box.append(paletteSwatch(validPalette(p), t(p.name) + " · " + t(PALETTE_STYLES[p.style].name)));
  for (const p of prefs.palettes) {
    box.append(el("div", { class: "palette-custom" },
      paletteSwatch(p, paletteName(p)),
      el("button", { class: "palette-remove", title: t("Remove palette"), onclick: () => removePalette(p) }, icon("close", "sm"))));
  }
  if (prefs.palettes.length < MAX_PALETTES) {
    box.append(el("button", { class: "palette-add", title: t("Create a palette"), "aria-label": t("Create a palette"), onclick: openPaletteEditor }, icon("add")));
  }
  markPalette(prefs.palette);
}

function markPalette(p) {
  for (const s of document.querySelectorAll("#palettes .palette")) s.classList.toggle("on", s.dataset.key === paletteKey(p));
  $("palette-name").textContent = paletteName(p);
}

function removePalette(p) {
  const palettes = prefs.palettes.filter((x) => !samePalette(x, p));
  const palette = samePalette(prefs.palette, p) ? validPalette(PRESETS[0]) : prefs.palette;
  setPrefs({ palettes, palette });
  renderPalettes();
}

function openPaletteEditor() {
  const draft = { ...prefs.palette };
  const big = el("div", { class: "palette big" });
  const slider = el("input", { type: "range", class: "hue-slider", min: 0, max: 359, step: 1, value: draft.hue, "aria-label": t("Source color") });
  slider.style.setProperty("--hue-track", HUE_TRACK);
  const choices = Object.entries(PALETTE_STYLES).map(([id, s]) =>
    el("button", { class: "style-choice", "data-style": id, onclick: () => { draft.style = id; preview(); } },
      el("span", { class: "palette mini" }), el("span", { text: t(s.name) })));
  const preview = () => {
    applyTheme(draft);
    big.setAttribute("style", paletteStyle(draft));
    for (const c of choices) {
      c.classList.toggle("on", c.dataset.style === draft.style);
      c.firstChild.setAttribute("style", paletteStyle({ hue: draft.hue, style: c.dataset.style }));
    }
  };
  slider.addEventListener("input", () => { draft.hue = validHue(Number(slider.value)); preview(); });
  const cancel = () => { closeDialog(); applyTheme(); };
  const save = () => {
    closeDialog();
    const palette = { ...draft };
    const palettes = PRESETS.some((p) => samePalette(p, palette)) ? prefs.palettes : [...prefs.palettes.filter((p) => !samePalette(p, palette)), palette].slice(-MAX_PALETTES);
    setPrefs({ palette, palettes });
    renderPalettes();
  };
  openDialog({
    iconName: "palette",
    title: t("New palette"),
    body: [
      big,
      el("div", { class: "editor-label", text: t("Source color") }), slider,
      el("div", { class: "editor-label", text: t("Style") }), el("div", { class: "styles" }, choices),
    ],
    actions: [
      { label: t("Cancel"), onClick: cancel },
      { label: t("Save"), kind: "filled", primary: true, onClick: save },
    ],
    onEscape: cancel,
  });
  preview();
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
  renderPalettes();
  for (const b of $("theme-mode").children) b.addEventListener("click", () => setPrefs({ theme: b.dataset.theme }));
  darkQuery.addEventListener("change", () => applyTheme());
  applyTheme();
}
