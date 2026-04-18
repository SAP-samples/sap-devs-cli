package notify

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// Send dispatches an OS-level notification. Returns an error if unavailable.
func Send(title, body string) error {
	switch runtime.GOOS {
	case "darwin":
		return sendDarwin(title, body)
	case "linux":
		return sendLinux(title, body)
	case "windows":
		return sendWindows(title, body)
	default:
		return fmt.Errorf("OS notifications not supported on %s", runtime.GOOS)
	}
}

// Available reports whether the current platform supports OS notifications.
func Available() bool {
	switch runtime.GOOS {
	case "darwin":
		return true
	case "linux":
		_, err := exec.LookPath("notify-send")
		return err == nil
	case "windows":
		return true
	default:
		return false
	}
}

func sendDarwin(title, body string) error {
	script := fmt.Sprintf(`display notification %q with title %q`, body, title)
	return exec.Command("osascript", "-e", script).Run()
}

func sendLinux(title, body string) error {
	return exec.Command("notify-send", title, body).Run() //nolint:gosec
}

func sendWindows(title, body string) error {
	ps := fmt.Sprintf(
		`[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] | Out-Null; `+
			`$template = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02); `+
			`$textNodes = $template.GetElementsByTagName('text'); `+
			`$textNodes.Item(0).AppendChild($template.CreateTextNode('%s')) | Out-Null; `+
			`$textNodes.Item(1).AppendChild($template.CreateTextNode('%s')) | Out-Null; `+
			`$toast = [Windows.UI.Notifications.ToastNotification]::new($template); `+
			`[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier('sap-devs').Show($toast)`,
		escapePS(title), escapePS(body),
	)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
	if err := cmd.Run(); err != nil {
		return sendWindowsFallback(title, body)
	}
	return nil
}

func sendWindowsFallback(title, body string) error {
	msg := fmt.Sprintf("%s\n\n%s", title, body)
	return exec.Command("msg", "*", msg).Run() //nolint:gosec
}

func escapePS(s string) string {
	s = strings.ReplaceAll(s, "'", "''")
	s = strings.ReplaceAll(s, "`", "``")
	s = strings.ReplaceAll(s, "$", "`$")
	return s
}
