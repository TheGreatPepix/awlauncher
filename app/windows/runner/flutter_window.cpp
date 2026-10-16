#include "flutter_window.h"

#include <dwmapi.h>
#include <flutter/standard_method_codec.h>
#include <flutter_windows.h>

#include <optional>

#include "flutter/generated_plugin_registrant.h"
#include "resource.h"

namespace {

// The tray icon reports clicks to the window with this message.
constexpr UINT kTrayMessage = WM_APP + 1;
constexpr UINT kTrayId = 1;

// The tray menu's commands.
constexpr UINT kMenuPlay = 1, kMenuOpen = 2, kMenuExit = 3;

// The smallest window the pages still fit in, in logical pixels.
constexpr int kMinWidth = 780, kMinHeight = 560;

// Redefined for Windows SDKs older than Windows 11's.
constexpr DWORD kDarkMode = 20, kCaptionColor = 35, kTextColor = 36;

std::wstring Wide(const std::string& utf8) {
  if (utf8.empty()) {
    return std::wstring();
  }
  int size = MultiByteToWideChar(CP_UTF8, 0, utf8.data(),
                                 static_cast<int>(utf8.size()), nullptr, 0);
  std::wstring out(size, L'\0');
  MultiByteToWideChar(CP_UTF8, 0, utf8.data(), static_cast<int>(utf8.size()),
                      out.data(), size);
  return out;
}

// Menus take & as the mark of a shortcut letter; account names keep theirs.
std::wstring MenuText(const std::wstring& text) {
  std::wstring out;
  for (wchar_t c : text) {
    if (c == L'&') {
      out += L'&';
    }
    out += c;
  }
  return out;
}

const flutter::EncodableValue* Arg(const flutter::EncodableMap& map,
                                   const char* key) {
  auto it = map.find(flutter::EncodableValue(key));
  return it == map.end() ? nullptr : &it->second;
}

std::wstring StringArg(const flutter::EncodableMap& map, const char* key) {
  auto value = Arg(map, key);
  auto text = value ? std::get_if<std::string>(value) : nullptr;
  return text ? Wide(*text) : std::wstring();
}

bool BoolArg(const flutter::EncodableMap& map, const char* key) {
  auto value = Arg(map, key);
  auto flag = value ? std::get_if<bool>(value) : nullptr;
  return flag && *flag;
}

// An ARGB colour from Dart as a COLORREF.
COLORREF ColorArg(const flutter::EncodableMap& map, const char* key) {
  auto value = Arg(map, key);
  if (!value || !(std::holds_alternative<int32_t>(*value) ||
                  std::holds_alternative<int64_t>(*value))) {
    return 0;
  }
  auto argb = static_cast<uint32_t>(value->LongValue());
  return RGB((argb >> 16) & 0xFF, (argb >> 8) & 0xFF, argb & 0xFF);
}

}  // namespace

FlutterWindow::FlutterWindow(const flutter::DartProject& project,
                             bool start_hidden)
    : project_(project), start_hidden_(start_hidden) {}

FlutterWindow::~FlutterWindow() {}

UINT FlutterWindow::ShowMessage() {
  static UINT message = RegisterWindowMessageW(L"AWLauncher.Show");
  return message;
}

bool FlutterWindow::OnCreate() {
  if (!Win32Window::OnCreate()) {
    return false;
  }

  RECT frame = GetClientArea();

  // The size here must match the window dimensions to avoid unnecessary surface
  // creation / destruction in the startup path.
  flutter_controller_ = std::make_unique<flutter::FlutterViewController>(
      frame.right - frame.left, frame.bottom - frame.top, project_);
  // Ensure that basic setup of the controller was successful.
  if (!flutter_controller_->engine() || !flutter_controller_->view()) {
    return false;
  }
  RegisterPlugins(flutter_controller_->engine());
  SetChildContent(flutter_controller_->view()->GetNativeWindow());

  channel_ = std::make_unique<flutter::MethodChannel<flutter::EncodableValue>>(
      flutter_controller_->engine()->messenger(), "awlauncher/desktop",
      &flutter::StandardMethodCodec::GetInstance());
  channel_->SetMethodCallHandler([this](const auto& call, auto result) {
    HandleCall(call, std::move(result));
  });

  taskbar_created_ = RegisterWindowMessageW(L"TaskbarCreated");
  AddTrayIcon();

  flutter_controller_->engine()->SetNextFrameCallback([&]() {
    if (!start_hidden_) {
      this->Show();
    }
    ReportVisible();
  });

  // Flutter can complete the first frame before the "show window" callback is
  // registered. The following call ensures a frame is pending to ensure the
  // window is shown. It is a no-op if the first frame hasn't completed yet.
  flutter_controller_->ForceRedraw();

  return true;
}

void FlutterWindow::OnDestroy() {
  RemoveTrayIcon();
  channel_ = nullptr;
  if (flutter_controller_) {
    flutter_controller_ = nullptr;
  }

  Win32Window::OnDestroy();
}

void FlutterWindow::HandleCall(
    const flutter::MethodCall<flutter::EncodableValue>& call,
    std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result) {
  static const flutter::EncodableMap no_args;
  auto map = call.arguments()
                 ? std::get_if<flutter::EncodableMap>(call.arguments())
                 : nullptr;
  const auto& args = map ? *map : no_args;
  const auto& method = call.method_name();
  if (method == "show") {
    ShowWindowNow();
  } else if (method == "hide") {
    HideWindow();
  } else if (method == "visible") {
    result->Success(flutter::EncodableValue(Visible()));
    return;
  } else if (method == "tray") {
    tray_play_ = StringArg(args, "play");
    tray_play_enabled_ = BoolArg(args, "enabled");
    auto open_label = StringArg(args, "open");
    auto exit_label = StringArg(args, "exit");
    if (!open_label.empty()) {
      tray_open_ = open_label;
    }
    if (!exit_label.empty()) {
      tray_exit_ = exit_label;
    }
  } else if (method == "titleBar") {
    title_set_ = true;
    title_dark_ = BoolArg(args, "dark");
    title_caption_ = ColorArg(args, "caption");
    title_text_ = ColorArg(args, "text");
    ApplyTitleBar();
  } else if (method == "quit") {
    quitting_ = true;
    RemoveTrayIcon();
    PostMessageW(GetHandle(), WM_CLOSE, 0, 0);
  } else {
    result->NotImplemented();
    return;
  }
  result->Success();
}

bool FlutterWindow::Visible() {
  HWND hwnd = GetHandle();
  return hwnd && IsWindowVisible(hwnd) && !IsIconic(hwnd);
}

void FlutterWindow::ReportVisible() {
  int visible = Visible() ? 1 : 0;
  if (visible == visible_) {
    return;
  }
  visible_ = visible;
  Notify("visible", std::make_unique<flutter::EncodableValue>(visible == 1));
}

void FlutterWindow::Notify(const std::string& method,
                           std::unique_ptr<flutter::EncodableValue> args) {
  if (channel_) {
    channel_->InvokeMethod(method, std::move(args));
  }
}

// Brings the window up from the tray, the taskbar or behind other windows.
// Windows lets a background process take the focus only in some cases, so
// the window joins the input of the one in front for a moment.
void FlutterWindow::ShowWindowNow() {
  HWND hwnd = GetHandle();
  if (!hwnd) {
    return;
  }
  ShowWindow(hwnd, IsIconic(hwnd) ? SW_RESTORE : SW_SHOW);
  HWND foreground = GetForegroundWindow();
  if (foreground != hwnd) {
    DWORD ours = GetCurrentThreadId();
    DWORD theirs = GetWindowThreadProcessId(foreground, nullptr);
    bool attached = theirs != 0 && theirs != ours &&
                    AttachThreadInput(theirs, ours, TRUE);
    const UINT flags = SWP_NOSIZE | SWP_NOMOVE | SWP_SHOWWINDOW;
    SetWindowPos(hwnd, HWND_TOPMOST, 0, 0, 0, 0, flags);
    SetWindowPos(hwnd, HWND_NOTOPMOST, 0, 0, 0, 0, flags);
    BringWindowToTop(hwnd);
    if (!SetForegroundWindow(hwnd)) {
      FlashWindow(hwnd, TRUE);
    }
    if (attached) {
      AttachThreadInput(theirs, ours, FALSE);
    }
  }
  ReportVisible();
}

void FlutterWindow::HideWindow() {
  if (HWND hwnd = GetHandle()) {
    ShowWindow(hwnd, SW_HIDE);
  }
  ReportVisible();
}

// A click on the tray icon hides the window when it is in front and brings
// it up otherwise.
void FlutterWindow::ToggleWindow() {
  bool front = active_ || std::chrono::steady_clock::now() - left_at_ <
                              std::chrono::seconds(1);
  if (Visible() && front) {
    HideWindow();
  } else {
    ShowWindowNow();
  }
}

void FlutterWindow::AddTrayIcon() {
  if (tray_added_ || quitting_) {
    return;
  }
  tray_.cbSize = sizeof(tray_);
  tray_.hWnd = GetHandle();
  tray_.uID = kTrayId;
  tray_.uFlags = NIF_ICON | NIF_MESSAGE | NIF_TIP;
  tray_.uCallbackMessage = kTrayMessage;
  if (!tray_.hIcon) {
    tray_.hIcon = static_cast<HICON>(LoadImageW(
        GetModuleHandleW(nullptr), MAKEINTRESOURCEW(IDI_APP_ICON), IMAGE_ICON,
        GetSystemMetrics(SM_CXSMICON), GetSystemMetrics(SM_CYSMICON), 0));
  }
  wcscpy_s(tray_.szTip, L"AWLauncher");
  tray_added_ = Shell_NotifyIconW(NIM_ADD, &tray_) != FALSE;
}

void FlutterWindow::RemoveTrayIcon() {
  if (tray_added_) {
    Shell_NotifyIconW(NIM_DELETE, &tray_);
    tray_added_ = false;
  }
  if (tray_.hIcon) {
    DestroyIcon(tray_.hIcon);
    tray_.hIcon = nullptr;
  }
}

void FlutterWindow::ShowTrayMenu() {
  HWND hwnd = GetHandle();
  HMENU menu = CreatePopupMenu();
  if (!tray_play_.empty()) {
    UINT flags = MF_STRING;
    if (!tray_play_enabled_) {
      flags |= MF_GRAYED;
    }
    AppendMenuW(menu, flags, kMenuPlay, MenuText(tray_play_).c_str());
  }
  AppendMenuW(menu, MF_STRING, kMenuOpen, MenuText(tray_open_).c_str());
  AppendMenuW(menu, MF_SEPARATOR, 0, nullptr);
  AppendMenuW(menu, MF_STRING, kMenuExit, MenuText(tray_exit_).c_str());
  POINT at;
  GetCursorPos(&at);
  // Without the window in front the menu stays open after a click elsewhere.
  SetForegroundWindow(hwnd);
  UINT command = static_cast<UINT>(TrackPopupMenu(
      menu, TPM_RETURNCMD | TPM_NONOTIFY | TPM_RIGHTBUTTON | TPM_BOTTOMALIGN,
      at.x, at.y, 0, hwnd, nullptr));
  PostMessageW(hwnd, WM_NULL, 0, 0);
  DestroyMenu(menu);
  switch (command) {
    case kMenuPlay:
      Notify("play");
      break;
    case kMenuOpen:
      ShowWindowNow();
      break;
    case kMenuExit:
      Notify("exit");
      break;
  }
}

// Paints the title bar in the page's surface colours, as the WebView window
// did. Windows 10 takes only the dark mode; the colours need Windows 11.
void FlutterWindow::ApplyTitleBar() {
  HWND hwnd = GetHandle();
  if (!hwnd || !title_set_) {
    return;
  }
  BOOL dark = title_dark_;
  DwmSetWindowAttribute(hwnd, kDarkMode, &dark, sizeof(dark));
  DwmSetWindowAttribute(hwnd, kCaptionColor, &title_caption_,
                        sizeof(title_caption_));
  DwmSetWindowAttribute(hwnd, kTextColor, &title_text_, sizeof(title_text_));
}

LRESULT
FlutterWindow::MessageHandler(HWND hwnd, UINT const message,
                              WPARAM const wparam,
                              LPARAM const lparam) noexcept {
  if (message == ShowMessage()) {
    ShowWindowNow();
    return 0;
  }
  if (message == taskbar_created_ && taskbar_created_ != 0) {
    // Explorer restarted and lost the icon.
    tray_added_ = false;
    AddTrayIcon();
    return 0;
  }
  switch (message) {
    case kTrayMessage:
      if (LOWORD(lparam) == WM_LBUTTONUP) {
        ToggleWindow();
      } else if (LOWORD(lparam) == WM_RBUTTONUP) {
        ShowTrayMenu();
      }
      return 0;
    case WM_CLOSE:
      // Closing the window keeps the launcher in the tray; it quits from
      // the tray menu or the settings.
      if (!quitting_) {
        HideWindow();
        return 0;
      }
      break;
    case WM_ACTIVATE:
      active_ = LOWORD(wparam) != WA_INACTIVE;
      if (!active_) {
        left_at_ = std::chrono::steady_clock::now();
      }
      break;
    case WM_GETMINMAXINFO: {
      UINT dpi = FlutterDesktopGetDpiForMonitor(
          MonitorFromWindow(hwnd, MONITOR_DEFAULTTONEAREST));
      auto info = reinterpret_cast<MINMAXINFO*>(lparam);
      info->ptMinTrackSize.x = MulDiv(kMinWidth, static_cast<int>(dpi), 96);
      info->ptMinTrackSize.y = MulDiv(kMinHeight, static_cast<int>(dpi), 96);
      return 0;
    }
  }

  // Give Flutter, including plugins, an opportunity to handle window messages.
  if (flutter_controller_) {
    std::optional<LRESULT> result =
        flutter_controller_->HandleTopLevelWindowProc(hwnd, message, wparam,
                                                      lparam);
    if (result) {
      return *result;
    }
  }

  switch (message) {
    case WM_FONTCHANGE:
      flutter_controller_->engine()->ReloadSystemFonts();
      break;
    case WM_DWMCOLORIZATIONCOLORCHANGED: {
      // The base window follows the system's dark mode; the page's theme
      // wins.
      LRESULT result = Win32Window::MessageHandler(hwnd, message, wparam, lparam);
      ApplyTitleBar();
      return result;
    }
    case WM_SIZE: {
      LRESULT result = Win32Window::MessageHandler(hwnd, message, wparam, lparam);
      ReportVisible();
      return result;
    }
  }

  return Win32Window::MessageHandler(hwnd, message, wparam, lparam);
}
