"use strict";

function setupPlayShapes() {
  const points = 96, size = 21;
  const wave = (k, a) => (th) => 1 + a * Math.cos(k * th);
  const oval = (ratio) => (th) => ratio / Math.hypot(ratio * Math.cos(th), Math.sin(th));
  const pill = (th) => 1 / Math.pow(Math.pow(Math.abs(Math.cos(th)), 4) + Math.pow(Math.abs(Math.sin(th)) / 0.62, 4), 0.25);
  const shapes = [wave(10, 0.07), wave(9, 0.11), wave(5, 0.13), pill, wave(8, 0.13), wave(4, 0.16), oval(0.74)];
  const path = (radius) => {
    const angles = Array.from({ length: points }, (_, i) => (i / points) * Math.PI * 2 - Math.PI / 2);
    const r = angles.map(radius);
    const scale = size / Math.max(...r);
    const xy = angles.map((th, i) => (24 + Math.cos(th) * r[i] * scale).toFixed(2) + " " + (24 + Math.sin(th) * r[i] * scale).toFixed(2));
    return "M" + xy.join("L") + "Z";
  };
  const d = shapes.map(path);
  const spring = getComputedStyle(document.documentElement).getPropertyValue("--spring").trim();
  const step = 100 / d.length, morph = step * 0.45;
  const frames = [];
  d.forEach((shape, i) => {
    const next = d[(i + 1) % d.length];
    frames.push(`${(i * step).toFixed(3)}% { d: path("${shape}"); animation-timing-function: ${spring}; }`);
    frames.push(`${(i * step + morph).toFixed(3)}% { d: path("${next}"); animation-timing-function: linear; }`);
  });
  frames.push(`100% { d: path("${d[0]}"); }`);
  document.head.append(el("style", { text: `@keyframes play-morph {\n${frames.join("\n")}\n}` }));
  $("play-shape").setAttribute("d", d[0]);
}
