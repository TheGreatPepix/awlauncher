"use strict";

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
