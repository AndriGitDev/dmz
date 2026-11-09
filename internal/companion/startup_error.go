package companion

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// ShowStartupError makes a double-click launch failure visible even when the
// process has no console. Only fixed text reaches native dialogs. Errors and
// local addresses, filesystem paths or credentials are never interpolated.
func ShowStartupError() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var command *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		command = exec.CommandContext(ctx, "/usr/bin/osascript", "-e", `display alert "DMZ could not open its window" message "Try opening DMZ again. A desktop session, web browser and terminal application are required. This private prototype is not a supported release." as warning`)
	case "windows":
		env := hostPlatformEnv()
		path := filepath.Join(env.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		command = exec.CommandContext(ctx, path, "-NoProfile", "-NonInteractive", "-Command", `Add-Type -AssemblyName System.Windows.Forms; [void][System.Windows.Forms.MessageBox]::Show('Try opening DMZ again. A desktop session, web browser and terminal application are required. This private prototype is not a supported release.','DMZ could not open its window')`)
		applyPlatformProcAttr(command, platformCommand{NoWindow: true})
	default:
		if path, err := exec.LookPath("zenity"); err == nil {
			command = exec.CommandContext(ctx, path, "--error", "--title=DMZ", "--text=DMZ could not open its window. Try opening it again. A desktop session, browser and terminal are required.")
		} else if path, err := exec.LookPath("kdialog"); err == nil {
			command = exec.CommandContext(ctx, path, "--error", "DMZ could not open its window. Try opening it again. A desktop session, browser and terminal are required.")
		}
	}
	if command != nil {
		_ = command.Run()
	}
}
