# AWLauncher

**English** · [Русский](README.ru.md)

A standalone launcher for Armored Warfare, for Windows, with a console version for Linux and Steam Deck. It installs, updates and starts the game for **VK Play** (Russian servers) and **FX ID / Wishlist Games** (international servers) accounts from one window, and keeps several accounts ready to play. It is an unofficial fan project; see the [disclaimer](#disclaimer).

## Features

- **Several accounts, two services.** Keep VK Play and FX ID accounts side by side, name them, and start any of them with one click.
- **Install and update.** Installs the full client into any folder, resumes interrupted downloads, installs patches and checks every file it downloads. Downloads and checks can be paused.
- **One install for both services.** VK Play and the FX ID main branch share one game folder; the launcher switches the service settings before each start.
- **Closed FX ID branches.** Activate a key, see the branches open to an account and choose which one it plays. A branch is installed next to the main folder and reuses its files, so only the difference is downloaded.
- **File check and repair.** Before each start the launcher checks that the client is complete and offers to download only missing or damaged files. A full check of every file can be run at any time.
- **Game management.** See the installed clients and their versions, free the space taken by downloaded patches, remove a closed branch or uninstall the game.
- **Works in the background.** While the game installs, you can rename or remove accounts, add new ones and manage branches of other accounts.
- **Stays out of the way.** After the game starts, the launcher hides in the notification area. It can also close a running game, start with Windows and update itself.
- **English and Russian.** The window follows the Windows language, or the one you choose.

## Requirements

- Windows 10 or 11, 64-bit. On Linux, the console version runs on any 64-bit distribution; see [Linux and Steam Deck](#linux-and-steam-deck).
- Microsoft Edge WebView2 Runtime. It is part of Windows 11 and current Windows 10; if it is missing, the launcher offers the download page.
- About 70 GiB of free space for a new install.

## Getting started

1. Download `AWLauncher-Setup.exe` from the [latest release](../../releases/latest) and run it. It installs the launcher for your Windows user into `%LOCALAPPDATA%\Programs\AWLauncher`, without administrator rights, and adds it to the Start menu and, if you like, the desktop. It is removed like any other app, in **Settings → Apps → Installed apps**. Windows may warn about an unknown publisher the first time; choose **More info → Run anyway**.

   You can also skip the installer: download `AWLauncher.exe` from the same release and put it in any folder. Both kinds keep their data in the same place and update themselves the same way.
2. Start AWLauncher and press **+** to add an account:
   - **VK Play**: the sign-in page opens in your browser. If you are signed in to VK Play there, it returns to the launcher by itself.
   - **FX ID**: enter your e-mail, then the 6-digit code from the letter: type it or paste it with Ctrl+V, and it is sent as soon as the last digit is in.
3. Select the account and press **Play**.
4. On the first start the launcher looks for an existing install. If it finds none, choose a folder: an existing install is used as is, and an empty folder gets a fresh install. The dialog shows the free space on the drive.
5. The launcher installs updates if there are any, checks the files and starts the game.

## Using the launcher

**Accounts.** Double-click an account or press its ▶ button to play. The ⋮ menu has **Make main**, which pins the account for the **Play** button and the tray menu without starting the game, **Rename** and **Remove**, and for FX ID accounts also **Client branches** and **Activate key**. The filter at the top of the side bar shows all accounts or only one service.

**Branches.** **Activate key** adds a closed branch (for example a supertest) to an FX ID account. **Client branches** lists the branches open to the account with their version and size, and lets you choose which one it plays.

**Progress and Logs.** The progress bar shows downloads and checks with speed and time left. **Pause** holds a download or a file check and **Resume** goes on from the same place; replacing game files while a patch installs cannot be paused. **Logs** keeps the full output, which you can copy.

**Game.** Shows the main game folder, the installed clients (VK Play, the FX ID main branch and closed branches) with their versions, and the space taken by downloaded patches. Clients and branches available to your accounts have a **Download** button, so you can install them without starting the game. **Check files** compares every file of a client with its official list and downloads the ones that differ; for FX ID it needs an FX ID account. **Delete** removes downloaded patches, **Remove** removes a closed branch, and **Uninstall** removes the game and its branches. Files the launcher did not install, such as screenshots, are kept unless you agree to delete the whole folder.

**Settings.** Open the launcher's data folder, choose whether AWLauncher starts with Windows (**Off**, **Window**, or **Tray** for the notification area), choose a light or dark theme and a color, and the language: **Auto** follows Windows (Russian for a Russian Windows, English otherwise), or pick **English** or **Русский**. The language covers the window and the tray menu; the logs stay in English. **Version** shows the launcher version; **Check for updates** looks for a newer [release](../../releases) on GitHub. The launcher also checks once at start and marks **Settings** with a dot when an update is out. **Update** downloads the new `AWLauncher.exe` (and `AWLauncherConsole.exe`, if it is next to it), checks it, puts it in place of the old one and restarts; accounts and settings stay. If the launcher cannot write to its folder, it offers the release page instead.

**Running the game.** After the game starts, the launcher hides in the notification area. Click its icon to open it, click again to hide it; right-click it to play the main account, the one marked with a pin (for example **Play Tanker · VK Play**), or for **Open AWLauncher** and **Exit**. Closing the window also hides it. While the game runs, **Close game** asks it to close and ends it after 10 seconds.

**Several things at once.** One install, update or start runs at a time. Meanwhile you can rename any account, and add, remove, or manage branches of the other accounts. When two tasks ask something at the same time, their dialogs wait for each other, and each names the task that asks.

Only one copy of the launcher runs at a time; starting another brings the open window forward.

### Console version

`AWLauncherConsole.exe` has the same features as a text menu, for scripts and troubleshooting:

| Input | Action |
| --- | --- |
| Enter | play the last used account |
| `N` | play account number N |
| `+` | add an account |
| `-N` | remove account N |
| `r [N]` | rename an account |
| `b [N]` | list FX ID branches and choose one |
| `k [N]` | activate an FX ID key |
| `g` | game: check files, delete downloaded patches, remove a branch, uninstall |
| `x` | close the running game |
| `q` | quit |

`AWLauncherConsole.exe play [ACCOUNT]` asks nothing: it installs updates, repairs files, starts the game for ACCOUNT (a number from the menu, a name or a login; the last used account without it) and waits until the game exits. It never starts a new install or a sign-in; do those in the menu. `AWLauncherConsole.exe version` prints the version.

### Linux and Steam Deck

On Linux the launcher is the console version only: `awlauncher-linux-amd64` from the [latest release](../../releases/latest). The launcher itself runs natively; the game, a Windows program, runs through [umu-launcher](https://github.com/Open-Wine-Components/umu-launcher) (Proton outside Steam) or, without it, Wine. Whether the game works under Proton is up to the game; the launcher only starts it.

1. Install umu-launcher as its README describes, so that `umu-run` is on the `PATH`. SteamOS keeps its system read-only, so on a Steam Deck use a build that installs into your home folder, such as the zipapp from the umu-launcher releases unpacked into `~/.local/bin`.
2. Make the file executable and start it in a terminal:
   ```bash
   chmod +x awlauncher-linux-amd64 && ./awlauncher-linux-amd64
   ```
   Add accounts and install the game or choose an existing folder, as in the menu above. VK Play sign-in opens in the default browser.
3. For Game Mode, add the file to Steam as a non-Steam game and set its launch options to `play` (or `play ACCOUNT`). Steam then shows the game as running until it exits. Updates are installed before the start without a window, so install big updates in Desktop Mode first.

Details:

- Data lives in `~/.local/share/awlauncher` (or `$XDG_DATA_HOME/awlauncher`). Saved sign-ins are files readable only by your user; Linux has no DPAPI.
- The Wine prefix is `~/.local/share/awlauncher/prefix` unless `WINEPREFIX` is set. umu-launcher picks the Proton version; set `PROTONPATH=GE-Proton` for the latest GE-Proton. The output of Proton or Wine goes to `~/.local/share/awlauncher/game.log`.
- `AWLAUNCHER_RUNNER` replaces the runner, for example `AWLAUNCHER_RUNNER="umu-run"` or a path to `wine`.
- Linux file systems tell `Bin64` from `bin64`; the launcher keeps the case of the files that are already there, so patches made on Windows land on the right files.

## Where data is kept

Everything lives in `%LOCALAPPDATA%\AWLauncher`:

- `config.json`: the main game folder and the account list, without passwords or tokens;
- `accounts\*.bin`: each account's saved sign-in, encrypted with Windows DPAPI so only your Windows user can read it;
- `ui.json`: the theme, the color and the language; `WebView2`: the window's browser data.

Inside the game folder, the launcher keeps its own state and patch downloads in `-gup-`. Each service's own `user.cfg`, as it comes from that service's files, is kept in `-gup-\awlauncher` and put in place before the game starts; closed branches have their own folders with their own `user.cfg`. Removing an account deletes its saved sign-in; the game files stay.

## Troubleshooting

- **VK Play sign-in does not come back to the launcher.** If the browser asks whether vkplay.ru may access apps on this device, allow it. Otherwise press **Sign in here** on the sign-in card: the sign-in opens in the launcher's own window and does not depend on the browser. **Another account** signs in with a different VK Play account.
- **"Local port 51200 is busy".** Another VK Play launcher is running. Close it and try again.
- **The game does not start or crashes.** On the **Game** page press **Check files** for its client: every file is checked and the damaged ones are downloaded again. **Logs** shows what happened.

## Disclaimer

AWLauncher is an unofficial fan project. It is not made, endorsed or supported by VK, MY.GAMES, Wishlist Games or the makers of Armored Warfare. Armored Warfare, VK Play, FX ID and the other names belong to their owners and appear here only to say what the launcher works with. The launcher contains none of their files or logos.

The launcher signs in to VK Play and FX ID the way their own launchers do, and it changes the game's settings files to switch between the services. The terms of these services may not allow third-party launchers, and an account may be restricted for using one. You use AWLauncher at your own risk; it is provided as is, without any warranty.

The launcher never sees or keeps your passwords: VK Play sign-in happens on the VK Play page, and FX ID uses a code from your e-mail. Saved sign-ins stay on your computer and are sent only to the service they belong to. Apart from the services and their download servers, the launcher contacts only GitHub, to check for updates.

If you represent one of these companies and have a concern, open an issue and it will be dealt with promptly.

## Building

Go 1.26.6 or newer is needed only to build the launcher.

```powershell
go build -ldflags '-H=windowsgui' -o AWLauncher.exe ./cmd/awlauncher
```

Such a build calls itself a development build and does not update itself. To give it a version, add `-X github.com/TheGreatPepix/awlauncher/internal/launcher.Version=v1.2.3` to `-ldflags`; GitHub Actions does that with the output of `git describe --tags`.

```powershell
go build -o AWLauncherConsole.exe ./cmd/awlauncher-console
```

```powershell
go test ./...
```

GitHub Actions builds and tests both executables on every push and pull request, builds the installer from `installer/awlauncher.iss` with [Inno Setup 6](https://jrsoftware.org/isinfo.php) and attaches all three to the run. To build the installer yourself, put both executables into `dist` and run `ISCC.exe -DAppVersion=1.2.3 -DFileVersion=1.2.3.0 installer\awlauncher.iss`. Pushing a tag such as `v1.0.0`, or publishing a release for a new tag, attaches them to that release. The launcher compares its version with the latest release, so tags must look like `v1.2.3`; a tag with a suffix, such as `v1.2.3-rc1`, is never offered as an update.

The window is a web page shown in WebView2; its files are in `internal/launcher/ui` and are built into the executable. Opened in an ordinary browser, `index.html` runs a demo with sample accounts, which is handy for design work. Set `AWLAUNCHER_DEVTOOLS=1` to get the developer tools in the launcher window. The icon is the file `internal/appicon/awlauncher.ico`; the window, its side bar and the tray use it as is, and nothing else draws the icon. To change it, replace that file (sizes from 16 to 256 px) and run `go generate ./cmd/...`, which also embeds it into the executables; run the same after changing the application manifest.

The launcher uses [bodgit/sevenzip](https://github.com/bodgit/sevenzip), [go-deltasync/vcdiff](https://github.com/go-deltasync/vcdiff), [jchv/go-webview2](https://github.com/jchv/go-webview2) and [fyne.io/systray](https://github.com/fyne-io/systray). The interface font is Rubik, under the SIL Open Font License.

## License

AWLauncher is released under the [MIT License](LICENSE). The Rubik font keeps its own license, the SIL Open Font License, in `internal/launcher/ui/fonts/Rubik-OFL.txt`.
