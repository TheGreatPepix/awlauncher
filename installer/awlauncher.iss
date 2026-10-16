#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
#ifndef FileVersion
  #define FileVersion "0.0.0.0"
#endif
#ifndef DistDir
  #define DistDir "..\dist"
#endif

; The Flutter window's folder: AWLauncher.exe, its DLLs, data and awcore.exe.
#define AppDir DistDir + "\app"

#define AppGUID "B06FF05E-7E61-4055-B1EA-2336934729B7"
#define AppMutexName "Local\AWLauncher.SingleInstance"

[Setup]
AppId={{{#AppGUID}}
AppName=AWLauncher
AppVersion={#AppVersion}
AppPublisher=TheGreatPepix
AppPublisherURL=https://github.com/TheGreatPepix/awlauncher
AppSupportURL=https://github.com/TheGreatPepix/awlauncher/issues
AppUpdatesURL=https://github.com/TheGreatPepix/awlauncher/releases
VersionInfoVersion={#FileVersion}
VersionInfoProductVersion={#FileVersion}
VersionInfoDescription=AWLauncher Setup
PrivilegesRequired=lowest
DefaultDirName={autopf}\AWLauncher
DisableProgramGroupPage=yes
DisableDirPage=auto
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
CloseApplications=no
OutputDir={#DistDir}
OutputBaseFilename=AWLauncher-Setup
SetupIconFile=..\internal\gui\appicon\awlauncher.ico
UninstallDisplayName=AWLauncher
UninstallDisplayIcon={app}\AWLauncher.exe
WizardStyle=modern
Compression=lzma2/max
SolidCompression=yes
ShowLanguageDialog=auto

[Languages]
Name: "en"; MessagesFile: "compiler:Default.isl"
Name: "ru"; MessagesFile: "compiler:Languages\Russian.isl"

[CustomMessages]
en.RemoveData=Also remove your AWLauncher accounts and settings from this computer?%n%nThe game files stay. To remove the game as well, remove its clients on the Game page in AWLauncher before you uninstall it, or delete the game folder.
en.CloseLauncher=AWLauncher is running. Exit it from the tray menu, then click Retry.
ru.CloseLauncher=AWLauncher запущен. Закройте его через меню в трее и нажмите «Повторить».
ru.RemoveData=Удалить с этого компьютера и ваши аккаунты и настройки AWLauncher?%n%nФайлы игры останутся. Чтобы удалить и игру, перед удалением лаунчера удалите её клиенты на странице «Игра» или удалите папку игры вручную.

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
Source: "{#AppDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{autoprograms}\AWLauncher"; Filename: "{app}\AWLauncher.exe"
Name: "{autodesktop}\AWLauncher"; Filename: "{app}\AWLauncher.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\AWLauncher.exe"; Description: "{cm:LaunchProgram,AWLauncher}"; Flags: nowait postinstall skipifsilent; Check: not Updating
; An update started by the launcher starts the new version when it is done.
Filename: "{app}\AWLauncher.exe"; Parameters: "--updated"; Flags: nowait; Check: Updating

[InstallDelete]
; The Flutter assets of the previous version; the new ones replace them.
Type: filesandordirs; Name: "{app}\data"
Type: files; Name: "{app}\AWLauncher-next.exe"
Type: files; Name: "{app}\AWLauncherConsole.exe"
Type: files; Name: "{app}\AWLauncherConsole-next.exe"
Type: files; Name: "{app}\AWLauncherConsole-previous.exe"

[UninstallDelete]
Type: files; Name: "{app}\AWLauncher-next.exe"
Type: files; Name: "{app}\AWLauncher-previous.exe"
Type: files; Name: "{app}\AWLauncherConsole-next.exe"
Type: files; Name: "{app}\AWLauncherConsole-previous.exe"
Type: dirifempty; Name: "{app}"

[Code]
// The launcher passes /UPDATE when it starts the installer to update itself.
function Updating: Boolean;
var
  I: Integer;
begin
  Result := False;
  for I := 1 to ParamCount do
    if CompareText(ParamStr(I), '/UPDATE') = 0 then
      Result := True;
end;

// The launcher has to be closed for its files to be replaced or removed.
function LauncherClosed: Boolean;
begin
  Result := True;
  while CheckForMutexes('{#AppMutexName}') do
    if SuppressibleMsgBox(CustomMessage('CloseLauncher'), mbError, MB_RETRYCANCEL, IDCANCEL) <> IDRETRY then
    begin
      Result := False;
      Exit;
    end;
end;

// An update waits for the launcher that started it to quit.
function InitializeSetup: Boolean;
var
  Waited: Integer;
begin
  if Updating then
  begin
    Waited := 0;
    while CheckForMutexes('{#AppMutexName}') and (Waited < 30000) do
    begin
      Sleep(250);
      Waited := Waited + 250;
    end;
  end;
  Result := LauncherClosed;
end;

function InitializeUninstall: Boolean;
begin
  Result := LauncherClosed;
end;

procedure CurUninstallStepChanged(CurUninstallStep: TUninstallStep);
var
  Data: String;
begin
  if CurUninstallStep <> usPostUninstall then
    Exit;
  RegDeleteValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Run', 'AWLauncher');
  RegDeleteValue(HKCU, 'Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run', 'AWLauncher');
  Data := ExpandConstant('{localappdata}\AWLauncher');
  if DirExists(Data) and not UninstallSilent then
    if SuppressibleMsgBox(CustomMessage('RemoveData'), mbConfirmation, MB_YESNO or MB_DEFBUTTON2, IDNO) = IDYES then
      DelTree(Data, True, True, True);
end;
