"use strict";

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
  const up = r.bottom + h + 8 > innerHeight;
  menu.style.top = (up ? r.top - h - 4 : r.bottom + 4) + "px";
  menu.style.transformOrigin = up ? "bottom right" : "top right";
}

function closeMenu() { $("menu").classList.remove("on"); }

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

function onDone(ev) {
  if (state.page === "game") { send({ cmd: "gameInfo" }); send({ cmd: "availableClients" }); }
  switch (ev.status) {
    case "error":
      toast(ev.message, "error", { title: tb(ev.title), action: { label: t("Logs"), onClick: () => showPage("activity") } });
      break;
    case "launched": if (ev.hidden) toast(t("Game started. AWLauncher is in the notification area.")); break;
    case "cancelled": toast(t("Cancelled")); break;
    case "ok":
      if (ev.message) toast(tb(ev.message), "ok");
      break;
  }
}

function toast(text, kind = "info", opts = {}) {
  const box = $("toasts");
  const node = el("div", { class: "toast " + kind, role: kind === "error" ? "alert" : "status" });
  const close = () => { node.classList.remove("on"); setTimeout(() => node.remove(), 250); };
  node.append(...[
    kind === "error" ? icon("error") : null,
    el("div", { class: "toast-text" }, opts.title ? el("div", { class: "toast-title", text: opts.title }) : null, el("div", { text })),
    opts.action ? el("button", { class: "btn text", onclick: () => { close(); opts.action.onClick(); } }, opts.action.label) : null,
    el("button", { class: "toast-close", title: t("Close"), onclick: close }, icon("close", "sm")),
  ].filter(Boolean));
  box.append(node);
  while (box.children.length > 4) box.firstChild.remove();
  requestAnimationFrame(() => node.classList.add("on"));
  setTimeout(close, kind === "error" ? 12000 : 5000);
}
