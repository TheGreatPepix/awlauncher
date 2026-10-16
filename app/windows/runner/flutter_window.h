#ifndef RUNNER_FLUTTER_WINDOW_H_
#define RUNNER_FLUTTER_WINDOW_H_

#include <flutter/dart_project.h>
#include <flutter/encodable_value.h>
#include <flutter/flutter_view_controller.h>
#include <flutter/method_channel.h>
#include <shellapi.h>

#include <chrono>
#include <memory>
#include <string>

#include "win32_window.h"

// The launcher's window: it hosts the Flutter view, keeps the tray icon, and
// hides instead of closing. The Dart side drives it over the
// "awlauncher/desktop" channel.
class FlutterWindow : public Win32Window {
 public:
  // Creates a new FlutterWindow hosting a Flutter view running |project|.
  // With |start_hidden| the window stays in the tray after the first frame.
  FlutterWindow(const flutter::DartProject& project, bool start_hidden);
  virtual ~FlutterWindow();

  // The message another copy of the launcher posts to bring this one up.
  static UINT ShowMessage();

 protected:
  // Win32Window:
  bool OnCreate() override;
  void OnDestroy() override;
  LRESULT MessageHandler(HWND window, UINT const message, WPARAM const wparam,
                         LPARAM const lparam) noexcept override;

 private:
  void HandleCall(
      const flutter::MethodCall<flutter::EncodableValue>& call,
      std::unique_ptr<flutter::MethodResult<flutter::EncodableValue>> result);

  void ShowWindowNow();
  void HideWindow();
  void ToggleWindow();
  bool Visible();
  void ReportVisible();
  void Notify(const std::string& method,
              std::unique_ptr<flutter::EncodableValue> args = nullptr);

  void AddTrayIcon();
  void RemoveTrayIcon();
  void ShowTrayMenu();
  void ApplyTitleBar();

  // The project to run.
  flutter::DartProject project_;

  // The Flutter instance hosted by this window.
  std::unique_ptr<flutter::FlutterViewController> flutter_controller_;
  std::unique_ptr<flutter::MethodChannel<flutter::EncodableValue>> channel_;

  bool start_hidden_;
  bool quitting_ = false;
  int visible_ = -1;

  // The window counts as in front for a tray click if it was active less
  // than a second ago: the click itself takes the focus away.
  bool active_ = false;
  std::chrono::steady_clock::time_point left_at_;

  NOTIFYICONDATAW tray_ = {};
  bool tray_added_ = false;
  UINT taskbar_created_ = 0;
  std::wstring tray_play_, tray_open_ = L"Open AWLauncher", tray_exit_ = L"Exit";
  bool tray_play_enabled_ = false;

  bool title_set_ = false, title_dark_ = false;
  COLORREF title_caption_ = 0, title_text_ = 0;
};

#endif  // RUNNER_FLUTTER_WINDOW_H_
