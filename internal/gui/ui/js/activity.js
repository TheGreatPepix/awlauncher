"use strict";

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

  renderTasks(p.active ? (p.tasks || []).slice(0, 4) : []);
  if (!wave.running) { wave.running = true; requestAnimationFrame(drawWave); }
}

const taskRows = new Map();

function renderTasks(list) {
  const box = $("tasks");
  const keep = new Set(list.map((t) => t.label));
  for (const [label, row] of taskRows) {
    if (!keep.has(label)) { row.remove(); taskRows.delete(label); }
  }
  list.forEach((t, i) => {
    let row = taskRows.get(t.label);
    if (!row) {
      row = el("div", { class: "task" },
        el("span", { class: "task-name", title: t.label, text: clipLeft(t.label, 64) }),
        el("div", { class: "task-bar" }, el("i")),
        el("span", { class: "task-num" }));
      taskRows.set(t.label, row);
    }
    if (box.children[i] !== row) box.insertBefore(row, box.children[i] || null);
    const f = t.total > 0 ? Math.min(1, t.done / t.total) : 0;
    const bar = row.children[1];
    bar.classList.toggle("reset", f * 100 < (Number(bar.style.getPropertyValue("--p")) || 0));
    bar.style.setProperty("--p", (f * 100).toFixed(2));
    row.children[2].textContent = t.total > 0 ? `${(f * 100).toFixed(0)}% · ${formatPair(t.done, t.total)}` : "";
  });
}

function shownFraction(p, now) {
  const target = Math.min(1, p.done / p.total);
  if (wave.to === undefined || wave.title !== p.title || target < wave.to) {
    Object.assign(wave, { from: target, to: target, t0: now, title: p.title });
  } else if (target !== wave.to) {
    const shown = wave.from + (wave.to - wave.from) * Math.min(1, (now - wave.t0) / 260);
    Object.assign(wave, { from: shown, to: target, t0: now });
  }
  return wave.from + (wave.to - wave.from) * Math.min(1, (now - wave.t0) / 260);
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
    const x1 = pad + shownFraction(p, now) * (W - 2 * pad);
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
