#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
#endif
#ifndef FileVersion
  #define FileVersion "0.0.0.0"
#endif
#ifndef DistDir
  #define DistDir "..\dist"
#endif

#define AppGUID "B06FF05E-7E61-4055-B1EA-2336934729B7"

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
AppMutex=Local\AWLauncher.SingleInstance
CloseApplications=no
OutputDir={#DistDir}
OutputBaseFilename=AWLauncher-Setup
SetupIconFile=..\internal\appicon\awlauncher.ico
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
en.RemoveData=Also remove your AWLauncher accounts and settings from this computer?%n%nThe game files stay. To remove the game as well, use Game > Uninstall in AWLauncher before you uninstall it, or delete the game folder.
ru.RemoveData=Удалить с этого компьютера и ваши аккаунты и настройки AWLauncher?%n%nФайлы игры останутся. Чтобы удалить и игру, перед удалением лаунчера выберите в нём Game > Uninstall или удалите папку игры вручную.

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"

[Files]
Source: "{#DistDir}\AWLauncher.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#DistDir}\AWLauncherConsole.exe"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\AWLauncher"; Filename: "{app}\AWLauncher.exe"
Name: "{autodesktop}\AWLauncher"; Filename: "{app}\AWLauncher.exe"; Tasks: desktopicon

[Run]
Filename: "{app}\AWLauncher.exe"; Description: "{cm:LaunchProgram,AWLauncher}"; Flags: nowait postinstall skipifsilent

[UninstallDelete]
Type: files; Name: "{app}\AWLauncher-next.exe"
Type: files; Name: "{app}\AWLauncher-previous.exe"
Type: files; Name: "{app}\AWLauncherConsole-next.exe"
Type: files; Name: "{app}\AWLauncherConsole-previous.exe"
Type: dirifempty; Name: "{app}"

[Code]
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
