"use strict";

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
