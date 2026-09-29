"use strict";

function setupLogoDrop() {
  const logo = document.querySelector(".brand-logo");
  const brand = logo.closest(".brand");
  let clicks = 0, last = 0, busy = false;
  const once = (name, then) => {
    const done = (e) => {
      if (e.target !== brand || e.animationName !== name) return;
      brand.removeEventListener("animationend", done);
      then();
    };
    brand.addEventListener("animationend", done);
  };
  logo.addEventListener("click", () => {
    if (busy) return;
    const now = Date.now();
    clicks = now - last > 2000 ? 1 : clicks + 1;
    last = now;
    if (clicks < 15) return;
    clicks = 0;
    busy = true;
    once("brand-fall", () => {
      brand.classList.replace("falling", "fallen");
      setTimeout(() => {
        once("brand-return", () => { brand.classList.remove("returning"); busy = false; });
        brand.classList.replace("fallen", "returning");
      }, 10000);
    });
    brand.classList.add("falling");
    toast(t("Stop fooling around") + " (╯°□°)╯︵ ┻━┻");
  });
}

function setupPong() {
  let clicks = 0, last = 0;
  $("drawer-status").addEventListener("click", () => {
    const now = Date.now();
    clicks = now - last > 2000 ? 1 : clicks + 1;
    last = now;
    if (clicks < 10) return;
    clicks = 0;
    whenDialogFree(openPong);
  });
}

function openPong() {
  const W = 480, H = 300, PAD_W = 10, PAD_H = 64, PAD_X = 18, BALL = 7;
  const canvas = el("canvas", { class: "pong", width: W, height: H });
  const ctx = canvas.getContext("2d");
  const probe = el("span", { style: "display:none" });
  document.body.append(probe);
  const colorOf = (v) => { probe.style.color = `var(${v})`; return getComputedStyle(probe).color; };
  const colors = { player: colorOf("--primary"), cpu: colorOf("--on-surface-variant"), ball: colorOf("--on-surface"), line: colorOf("--outline-variant") };
  probe.remove();
  const font = getComputedStyle(document.body).fontFamily;
  const game = { player: H / 2 - PAD_H / 2, cpu: H / 2 - PAD_H / 2, score: [0, 0], wait: 0.8, x: 0, y: 0, vx: 0, vy: 0 };
  const serve = (dir) => {
    const angle = (Math.random() - 0.5) * Math.PI / 3;
    const speed = 300;
    Object.assign(game, { x: W / 2, y: H / 2, vx: Math.cos(angle) * speed * dir, vy: Math.sin(angle) * speed, wait: 0.8 });
  };
  serve(Math.random() < 0.5 ? -1 : 1);
  const follow = (e) => {
    const rect = canvas.getBoundingClientRect();
    if (!rect.height) return;
    const y = (e.clientY - rect.top) * H / rect.height - PAD_H / 2;
    game.player = Math.min(Math.max(y, 0), H - PAD_H);
  };
  document.addEventListener("pointermove", follow);
  const bounce = (paddleY, dir) => {
    const hit = (game.y - (paddleY + PAD_H / 2)) / (PAD_H / 2);
    const speed = Math.min(Math.hypot(game.vx, game.vy) * 1.06, 720);
    const angle = Math.max(-1, Math.min(1, hit)) * Math.PI / 3;
    game.vx = Math.cos(angle) * speed * dir;
    game.vy = Math.sin(angle) * speed;
  };
  const step = (dt) => {
    const target = (game.vx > 0 ? game.y : H / 2) - PAD_H / 2;
    const reach = 250 * dt;
    game.cpu += Math.max(-reach, Math.min(reach, target - game.cpu));
    game.cpu = Math.min(Math.max(game.cpu, 0), H - PAD_H);
    if (game.wait > 0) { game.wait -= dt; return; }
    game.x += game.vx * dt;
    game.y += game.vy * dt;
    if (game.y < BALL) { game.y = BALL; game.vy = Math.abs(game.vy); }
    if (game.y > H - BALL) { game.y = H - BALL; game.vy = -Math.abs(game.vy); }
    if (game.vx < 0 && game.x - BALL <= PAD_X + PAD_W && game.x > PAD_X && game.y > game.player - BALL && game.y < game.player + PAD_H + BALL) {
      game.x = PAD_X + PAD_W + BALL;
      bounce(game.player, 1);
    }
    if (game.vx > 0 && game.x + BALL >= W - PAD_X - PAD_W && game.x < W - PAD_X && game.y > game.cpu - BALL && game.y < game.cpu + PAD_H + BALL) {
      game.x = W - PAD_X - PAD_W - BALL;
      bounce(game.cpu, -1);
    }
    if (game.x < -BALL) { game.score[1]++; serve(1); }
    if (game.x > W + BALL) { game.score[0]++; serve(-1); }
  };
  const paddle = (x, y, color) => {
    ctx.fillStyle = color;
    ctx.beginPath();
    ctx.roundRect(x, y, PAD_W, PAD_H, PAD_W / 2);
    ctx.fill();
  };
  const draw = () => {
    ctx.clearRect(0, 0, W, H);
    ctx.strokeStyle = colors.line;
    ctx.lineWidth = 2;
    ctx.setLineDash([8, 10]);
    ctx.beginPath();
    ctx.moveTo(W / 2, 12);
    ctx.lineTo(W / 2, H - 12);
    ctx.stroke();
    ctx.setLineDash([]);
    ctx.fillStyle = colors.cpu;
    ctx.font = `600 28px ${font}`;
    ctx.textAlign = "center";
    ctx.fillText(`${game.score[0]}   ${game.score[1]}`, W / 2, 42);
    paddle(PAD_X, game.player, colors.player);
    paddle(W - PAD_X - PAD_W, game.cpu, colors.cpu);
    ctx.fillStyle = colors.ball;
    ctx.beginPath();
    ctx.arc(game.x, game.y, BALL, 0, Math.PI * 2);
    ctx.fill();
  };
  let prev = 0;
  const frame = (now) => {
    if (!canvas.isConnected || !dialogOpen()) {
      document.removeEventListener("pointermove", follow);
      return;
    }
    const dt = prev ? Math.min((now - prev) / 1000, 0.05) : 0;
    prev = now;
    step(dt);
    draw();
    requestAnimationFrame(frame);
  };
  openDialog({
    title: t("Ping-pong"),
    body: [canvas],
    actions: [{ label: t("Close"), kind: "filled", primary: true, onClick: closeDialog }],
    onEscape: closeDialog,
  });
  requestAnimationFrame(frame);
}
