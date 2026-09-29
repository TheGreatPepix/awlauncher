"use strict";

const demo = {
  timer: 0,
  handle(cmd) {
    const accounts = demo.accounts || (demo.accounts = [
      { id: "1", name: "Tanker", login: "123456789", service: "VK Play", provider: "vkplay", last: true, autoLanguage: "ru", languages: ["ru", "en"] },
      { id: "2", name: "EU main", login: "player@example.com", service: "FX ID", provider: "fxid", last: false, language: "pl", autoLanguage: "ru", languages: ["en", "de", "fr", "pl", "ru"] },
      { id: "3", name: "Supertest", login: "tester@example.com", service: "FX ID, supertest", provider: "fxid", branch: "supertest", last: false, autoLanguage: "ru", languages: ["en", "de", "fr", "pl", "ru"] },
    ]);
    const emit = (ev) => setTimeout(() => aw.recv(ev), 30);
    const ops = demo.ops || (demo.ops = []);
    const stateEv = () => ({ type: "state", accounts, game: "H:\\Games\\Armored Warfare", data: "C:\\Users\\player\\AppData\\Local\\AWLauncher", version: "v0.1.1", autostart: demo.autostart || "off", allowMods: !!demo.allowMods, patchBackups: !demo.noBackups, systemLang: /^ru/i.test(navigator.language) ? "ru" : "en", ops: [...ops] });
    const begin = (op) => { ops.push(op); emit(stateEv()); };
    const end = (id) => { const i = ops.findIndex((o) => o.id === id); if (i >= 0) ops.splice(i, 1); emit(stateEv()); };
    switch (cmd.cmd) {
      case "ready":
        emit({ ...stateEv(), log: "Game: H:\\Games\\Armored Warfare (build 442)\nBuild 442 is up to date.\n" });
        break;
      case "copyLogs":
        if (navigator.clipboard?.writeText) {
          navigator.clipboard.writeText(cmd.value).then(() => emit({ type: "clipboard", ok: true }), () => emit({ type: "clipboard", ok: false }));
        } else {
          emit({ type: "clipboard", ok: false });
        }
        break;
      case "play": {
        begin({ id: 1, title: "Starting Tanker", game: true, account: "1" });
        emit({ type: "log", text: "Checking for updates...\nUpdate available: 442 -> 443 (1 patches).\n" });
        emit({ type: "prompt", id: 1, op: "Starting Tanker", kind: "confirm", question: "Install now?", default: true, context: ["Checking for updates...", "Update available: 442 -> 443 (1 patches)."] });
        break;
      }
      case "answer": {
        if (cmd.prompt === 3 && cmd.ok) {
          emit({ type: "prompt", id: 8, op: "Signing in to FX ID", kind: "code", question: "Code from the e-mail", context: ["A code was sent to " + (cmd.value || "player@example.com") + "."] });
          break;
        }
        if (cmd.prompt === 8) {
          emit({ type: "notice", message: cmd.ok ? "Code " + cmd.value + " sent for verification" : "Cancelled" });
          break;
        }
        if (cmd.prompt === 10 && cmd.value === "yes") {
          demo.mainRemoved = true;
          emit({ type: "done", status: "ok", title: "Removing VK Play", message: "Client removed" });
        }
        if (cmd.prompt !== 1) break;
        let done = 0;
        const total = 3.4 * GiB;
        clearInterval(demo.timer);
        demo.timer = setInterval(() => {
          if (!demo.paused) done = Math.min(total, done + 0.045 * GiB);
          emit({ type: "progress", active: done < total, pausable: true, paused: !!demo.paused, title: "Downloading", done, total, speed: demo.paused ? 0 : 44 << 20,
            tasks: [{ label: "payload-442-443/gamesdk/textures_hi-0134.pak", done: done % (205 << 20), total: 205 << 20 },
                    { label: "payload-442-443/gamesdk/levels/pve_05/level.pak", done: (done * 0.7) % (120 << 20), total: 120 << 20 }] });
          if (done >= total) {
            clearInterval(demo.timer);
            emit({ type: "log", text: "Downloading   done: 3.4 GiB in 1m19s, 44.0 MiB/s\n" });
            end(1);
            emit({ type: "done", status: "launched", title: "Starting Tanker" });
            emit({ type: "game", running: true });
          }
        }, 250);
        break;
      }
      case "branches":
        begin({ id: 2, title: "Loading branches of EU main", account: "2" });
        emit({ type: "prompt", id: 2, op: "Loading branches of EU main", kind: "choice", question: "Which branch should EU main play?", context: ["Asking FX ID for branches of EU main..."], options: [
          { value: "default", label: "default", detail: "0.566.1 (build 5661), 12.08.2026, 68.3 GiB", current: true },
          { value: "supertest", label: "supertest", detail: "0.567.0 (build 5670), 20.09.2026, 68.9 GiB" }] });
        break;
      case "remove":
        emit({ type: "done", status: "ok", title: "Removing", message: "Account removed" });
        const gone = accounts.findIndex((x) => x.id === cmd.account);
        if (gone >= 0) accounts.splice(gone, 1);
        emit(stateEv());
        break;
      case "language":
        accounts.find((x) => x.id === cmd.account).language = cmd.value;
        emit(stateEv());
        break;
      case "rename": {
        const a = accounts.find((x) => x.id === cmd.account);
        a.name = cmd.value || a.login;
        emit(stateEv());
        emit({ type: "notice", message: "Renamed to " + a.name });
        break;
      }
      case "signinHere":
        emit({ type: "signin", active: true, window: true });
        break;
      case "signinCancel":
        emit({ type: "signin", active: false });
        end(4);
        emit({ type: "done", status: "cancelled", title: "Signing in to VK Play" });
        break;
      case "closeGame":
        demo.running = false;
        emit({ type: "game", running: false });
        emit({ type: "notice", message: "The game is closed" });
        break;
      case "add":
        if (cmd.provider === "vkplay") {
          begin({ id: 4, title: "Signing in to VK Play" });
          emit({ type: "prompt", id: 4, op: "Signing in to VK Play", kind: "folder", folder: "vkplay", suggest: "H:\\Games\\Armored Warfare", question: "VK Play game folder",
            context: ["Armored Warfare was not found. Enter the folder of an existing install, or any folder to install the game into."] });
        } else emit({ type: "prompt", id: 3, op: "Signing in to FX ID", kind: "email", question: "E-mail of the FX ID account", context: [] });
        break;
      case "folderInfo": {
        const v = cmd.value.trim();
        const valid = /^[a-z]:\\/i.test(v);
        emit({ type: "folderInfo", prompt: cmd.prompt, path: cmd.value, valid, drive: v.slice(0, 2).toUpperCase(),
          free: /^c:/i.test(v) ? 41.3 * GiB : 812.4 * GiB, install: /armored warfare$/i.test(v) && /^d:/i.test(v), used: /^c:/i.test(v) });
        break;
      }
      case "browse":
        emit({ type: "browsed", prompt: cmd.prompt, path: "D:\\Games\\Armored Warfare" });
        break;
      case "gameInfo":
        emit({ type: "gameInfo", dir: "H:\\Games\\Armored Warfare", downloads: 3.2 * GiB, free: 812.4 * GiB, clients: [
          ...(!demo.mainRemoved ? [{ kind: "vkplay", dir: "H:\\Games\\Armored Warfare", version: "build 442", downloads: demo.cleared ? 0 : 3.2 * GiB }] : []),
          { kind: "fxid", branch: "default", dir: "H:\\Games\\Armored Warfare FX ID", version: "0.566.1" },
          ...(demo.downloaded ? [{ kind: "branch", branch: "SuperTest", dir: "H:\\Games\\Armored Warfare FX ID SuperTest", version: "0.567.0" }] : [])] });
        break;
      case "availableClients":
        emit({ type: "availableClients", clients: [
          { kind: "vkplay", account: "1" },
          { kind: "fxid", branch: "default", account: "2", version: "0.566.1" },
          { kind: "branch", branch: "SuperTest", account: "3", version: "0.567.0" }] });
        break;
      case "downloadClient":
        begin({ id: 9, title: "Downloading game", game: true, account: cmd.account });
        setTimeout(() => { demo.downloaded = true; end(9); emit({ type: "done", status: "ok", title: "Downloading game" }); }, 1500);
        break;
      case "verify":
        begin({ id: 5, title: "Checking " + (cmd.value === "vkplay" ? "VK Play" : "FX ID " + cmd.branch), game: true });
        setTimeout(() => { end(5); emit({ type: "notice", kind: "ok", message: cmd.value === "vkplay" ? "VK Play build 442: all files are checked" : "FX ID main branch 0.566.1: all files are checked" }); emit({ type: "done", status: "ok", title: "Checking" }); }, 1500);
        break;
      case "autostart":
        demo.autostart = cmd.value;
        emit(stateEv());
        break;
      case "patchBackups":
        demo.noBackups = cmd.value !== "on";
        emit(stateEv());
        break;
      case "allowMods":
        demo.allowMods = cmd.value === "on";
        emit(stateEv());
        break;
      case "pin": {
        for (const a of accounts) a.last = a.id === cmd.account;
        emit(stateEv());
        emit({ type: "notice", message: accounts.find((a) => a.last).name + " is the main account" });
        break;
      }
      case "pause":
      case "resume":
        demo.paused = cmd.cmd === "pause";
        emit({ type: "log", text: demo.paused ? "Paused. Press Resume to go on from where it stopped.\n" : "Resumed.\n" });
        break;
      case "checkUpdate":
        setTimeout(() => aw.recv({ type: "update", current: "v0.1.1", latest: "v0.2.0", published: "27.09.2026", available: true, canApply: true,
          page: "https://github.com/TheGreatPepix/awlauncher/releases/tag/v0.2.0", notes: "## What's Changed\n* Check for launcher updates in Settings\n* Update the launcher in place" }), 700);
        break;
      case "applyUpdate":
        begin({ id: 7, title: "Updating AWLauncher", game: true });
        setTimeout(() => { end(7); emit({ type: "done", status: "ok", title: "Updating AWLauncher", message: "The update is installed. It takes effect on the next start." }); }, 2000);
        break;
      case "removeMainClient":
        emit({ type: "prompt", id: 10, op: "Removing VK Play", kind: "confirm", question: "Remove VK Play from H:\\Games\\Armored Warfare?", default: false });
        break;
      case "clearDownloads":
        demo.cleared = true;
        emit({ type: "done", status: "ok", title: "Deleting downloaded patches" });
        break;
      case "updateClient":
        begin({ id: 11, title: "Updating " + (cmd.value === "vkplay" ? "VK Play" : "FX ID"), game: true });
        setTimeout(() => { end(11); emit({ type: "notice", message: cmd.value === "vkplay" ? "VK Play build 442 is up to date" : "FX ID main branch 0.566.1 is up to date" }); emit({ type: "done", status: "ok", title: "Updating" }); }, 1200);
        break;
    }
    if (cmd.cmd === "answer" && cmd.prompt !== 1) end(cmd.prompt);
  },
};
