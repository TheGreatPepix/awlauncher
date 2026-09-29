"use strict";

function renderFolders() {
  for (const b of $("mods-mode").children) b.classList.toggle("on", b.dataset.mode === (state.allowMods ? "on" : "off"));
  for (const b of $("backups-mode").children) b.classList.toggle("on", b.dataset.mode === (state.patchBackups ? "on" : "off"));
  for (const b of $("hide-mode").children) b.classList.toggle("on", b.dataset.mode === (state.hideOnLaunch ? "on" : "off"));
  $("data-folder").textContent = state.data || "%LOCALAPPDATA%\\AWLauncher";
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

let exitAsked = false;

function confirmExit() {
  if (!busy()) {
    send({ cmd: "exit" });
    return;
  }
  exitAsked = true;
  const answer = (quit) => {
    exitAsked = false;
    closeDialog();
    if (quit) send({ cmd: "exit" });
  };
  openDialog({
    iconName: "logout", iconClass: "error",
    title: t("Exit AWLauncher?"),
    body: [el("p", { text: t("{ops} is still in progress. If you exit now, it is interrupted.", { ops: opsTitle() }) })],
    actions: [
      { label: t("Cancel"), onClick: () => answer(false) },
      { label: t("Exit"), kind: "danger", primary: true, onClick: () => answer(true) },
    ],
    onEscape: () => answer(false),
  });
}

function askExit() {
  if (exitAsked) return;
  exitAsked = true;
  whenDialogFree(confirmExit);
}
