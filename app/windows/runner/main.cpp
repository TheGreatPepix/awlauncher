#include <flutter/dart_project.h>
#include <flutter/flutter_view_controller.h>
#include <flutter_windows.h>
#include <windows.h>
#include <shellapi.h>

#include <algorithm>

#include "flutter_window.h"
#include "utils.h"

namespace {

constexpr wchar_t kTitle[] = L"AWLauncher";
constexpr int kWidth = 1060, kHeight = 720;

// The flag Windows starts the launcher with when it should stay in the tray.
bool StartInTray() {
  int count = 0;
  wchar_t** args = CommandLineToArgvW(GetCommandLineW(), &count);
  bool tray = false;
  for (int i = 1; args && i < count; i++) {
    if (_wcsicmp(args[i], L"--tray") == 0) {
      tray = true;
    }
  }
  LocalFree(args);
  return tray;
}

// Brings up the launcher that already runs. The WebView window of earlier
// versions has the same class, so this finds either.
bool ShowRunning() {
  HWND running = FindWindowW(L"AWLauncherWindow", nullptr);
  if (!running) {
    return false;
  }
  AllowSetForegroundWindow(ASFW_ANY);
  PostMessageW(running, FlutterWindow::ShowMessage(), 0, 0);
  return true;
}

void SayRunning() {
  const wchar_t* text = L"AWLauncher is already running.";
  if (PRIMARYLANGID(GetUserDefaultUILanguage()) == LANG_RUSSIAN) {
    text = L"AWLauncher \x0443\x0436\x0435 \x0437\x0430\x043F\x0443\x0449\x0435\x043D.";
  }
  MessageBoxW(nullptr, text, kTitle, MB_OK | MB_ICONWARNING);
}

// The middle of the main screen's work area, in logical pixels.
Win32Window::Point Centered() {
  RECT area;
  if (!SystemParametersInfoW(SPI_GETWORKAREA, 0, &area, 0)) {
    return Win32Window::Point(10, 10);
  }
  UINT dpi = FlutterDesktopGetDpiForMonitor(
      MonitorFromPoint(POINT{area.left, area.top}, MONITOR_DEFAULTTOPRIMARY));
  double scale = dpi / 96.0;
  int left = static_cast<int>(area.left / scale);
  int top = static_cast<int>(area.top / scale);
  int width = static_cast<int>((area.right - area.left) / scale);
  int height = static_cast<int>((area.bottom - area.top) / scale);
  return Win32Window::Point(
      static_cast<unsigned int>(left + std::max(0, (width - kWidth) / 2)),
      static_cast<unsigned int>(top + std::max(0, (height - kHeight) / 2)));
}

}  // namespace

int APIENTRY wWinMain(_In_ HINSTANCE instance, _In_opt_ HINSTANCE prev,
                      _In_ wchar_t *command_line, _In_ int show_command) {
  // One launcher at a time: a second start brings up the first one.
  HANDLE instance_lock =
      CreateMutexW(nullptr, FALSE, L"Local\\AWLauncher.SingleInstance");
  if (instance_lock && GetLastError() == ERROR_ALREADY_EXISTS) {
    CloseHandle(instance_lock);
    if (!ShowRunning()) {
      SayRunning();
    }
    return EXIT_SUCCESS;
  }

  // Attach to console when present (e.g., 'flutter run') or create a
  // new console when running with a debugger.
  if (!::AttachConsole(ATTACH_PARENT_PROCESS) && ::IsDebuggerPresent()) {
    CreateAndAttachConsole();
  }

  // Initialize COM, so that it is available for use in the library and/or
  // plugins.
  ::CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED);

  flutter::DartProject project(L"data");

  std::vector<std::string> command_line_arguments =
      GetCommandLineArguments();

  project.set_dart_entrypoint_arguments(std::move(command_line_arguments));

  FlutterWindow window(project, StartInTray());
  Win32Window::Size size(kWidth, kHeight);
  if (!window.Create(kTitle, Centered(), size)) {
    return EXIT_FAILURE;
  }
  window.SetQuitOnClose(true);

  ::MSG msg;
  while (::GetMessage(&msg, nullptr, 0, 0)) {
    ::TranslateMessage(&msg);
    ::DispatchMessage(&msg);
  }

  ::CoUninitialize();
  if (instance_lock) {
    CloseHandle(instance_lock);
  }
  return EXIT_SUCCESS;
}
