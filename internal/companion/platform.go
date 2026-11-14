// SPDX-License-Identifier: GPL-3.0-only

package companion

// Platform helpers for the local companion: coding-client discovery, opening
// the loopback control window, launching the connect session in a native
// terminal and an optional folder dialog.
//
// All command lines are built by pure functions (see platformCommand) so the
// quoting and validation rules can be tested on a headless host for every
// supported GOOS. No helper here reads client credential stores, changes
// system settings or runs a command through a shell with interpolated input.
// Errors are fixed values: they never contain addresses, tokens or paths.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// ClientInstallation describes a locally found coding client executable.
type ClientInstallation struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Path      string `json:"path,omitempty"`
	Available bool   `json:"available"`
}

// Fixed platform errors. They are safe to show in the UI and logs.
var (
	ErrUnsupportedPlatform  = errors.New("dmz: this operating system is not supported by the companion")
	ErrNotLoopbackAddress   = errors.New("dmz: control window address must be a http://127.0.0.1 URL")
	ErrBrowserUnavailable   = errors.New("dmz: could not open the default browser; open the DMZ window address shown by DMZ manually")
	ErrNoGraphicalSession   = errors.New("dmz: no graphical desktop session was found; the DMZ app needs a desktop session (headless hosts are not supported)")
	ErrTerminalUnavailable  = errors.New("dmz: no supported terminal application was found; install a terminal application such as GNOME Terminal, Konsole or xterm")
	ErrTerminalLaunchFailed = errors.New("dmz: the terminal application could not be started")
	ErrUnsafeLaunchPath     = errors.New("dmz: a launch path is not an absolute path or contains unsupported characters; move DMZ or the project to a simpler folder name")
	ErrDialogUnavailable    = errors.New("dmz: no folder chooser is available; type the project folder path instead")
	ErrDialogCancelled      = errors.New("dmz: folder selection was cancelled")
	ErrDialogFailed         = errors.New("dmz: the folder chooser failed; type the project folder path instead")
	ErrConsoleUnavailable   = errors.New("dmz: could not attach a console window")
)

// platformCommand is a fully resolved process launch. Path is an absolute
// executable path or a helper resolved through the platform lookup; Args are
// passed as an argument vector, never through a shell.
type platformCommand struct {
	Path string
	Args []string
	Dir  string
	// NewConsole asks Windows to create a console for the process.
	NewConsole bool
	// NoWindow asks Windows to suppress a console window for helper tools.
	NoWindow bool
	// Script, when set, is written with mode 0700 to ScriptPath before launch.
	Script     string
	ScriptPath string
}

// platformEnv isolates everything platform code reads from the host so the
// builders can be exercised for every GOOS in headless tests.
type platformEnv struct {
	GOOS     string
	Getenv   func(string) string
	LookPath func(string) (string, error)
	// IsExecutable reports whether path is an existing regular executable file.
	IsExecutable func(string) bool
	Glob         func(string) ([]string, error)
	// RandomName returns a fresh unpredictable file-name component.
	RandomName func() string
}

func hostPlatformEnv() platformEnv {
	return platformEnv{
		GOOS:         runtime.GOOS,
		Getenv:       os.Getenv,
		LookPath:     exec.LookPath,
		IsExecutable: platformIsExecutable,
		Glob:         filepath.Glob,
		RandomName:   platformRandomName,
	}
}

// Test seams. Production code uses the host environment and starts real processes.
var (
	platformHostEnv = hostPlatformEnv
	platformStart   = startPlatformCommand
	platformOutput  = outputPlatformCommand
)

func platformIsExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode().Perm()&0o111 != 0
}

// ---------------------------------------------------------------------------
// Client discovery

type platformClientSpec struct {
	ID   string
	Name string
	// Binary is the base executable name without a Windows extension.
	Binary string
}

var platformClients = []platformClientSpec{
	{ID: "codex", Name: "Codex CLI", Binary: "codex"},
	{ID: "claude", Name: "Claude Code", Binary: "claude"},
}

// DiscoverClients reports the Codex and Claude Code command-line clients found
// on PATH or in common install locations. Apps launched from a desktop often
// receive a minimal PATH, so well-known user and package-manager directories
// are checked as well. It only checks file metadata: it does not run the
// clients or read their configuration or authentication stores.
func DiscoverClients() []ClientInstallation {
	return discoverClients(platformHostEnv())
}

func discoverClients(env platformEnv) []ClientInstallation {
	out := make([]ClientInstallation, 0, len(platformClients))
	for _, spec := range platformClients {
		item := ClientInstallation{ID: spec.ID, Name: spec.Name}
		if path := findClient(env, spec.Binary); path != "" {
			item.Path = path
			item.Available = true
		}
		out = append(out, item)
	}
	return out
}

func findClient(env platformEnv, binary string) string {
	if env.LookPath != nil {
		if path, err := env.LookPath(binary); err == nil && platformAbs(env.GOOS, path) {
			if env.IsExecutable == nil || env.IsExecutable(path) {
				return platformClean(env.GOOS, path)
			}
		}
	}
	for _, candidate := range clientCandidates(env, binary) {
		if env.IsExecutable != nil && env.IsExecutable(candidate) {
			return candidate
		}
	}
	return ""
}

// clientCandidates lists absolute fallback locations, in priority order.
func clientCandidates(env platformEnv, binary string) []string {
	get := func(key string) string {
		if env.Getenv == nil {
			return ""
		}
		return env.Getenv(key)
	}
	var dirs []string
	var globs []string
	switch env.GOOS {
	case "windows":
		profile := get("USERPROFILE")
		appData := get("APPDATA")
		local := get("LOCALAPPDATA")
		dirs = append(dirs,
			winJoin(profile, ".local", "bin"),
			winJoin(appData, "npm"),
			winJoin(local, "Programs", binary),
			winJoin(local, "Microsoft", "WinGet", "Links"),
			winJoin(profile, "scoop", "shims"),
			winJoin(profile, ".bun", "bin"),
			winJoin(local, "Volta", "bin"),
		)
		var out []string
		for _, dir := range dirs {
			if dir == "" {
				continue
			}
			for _, ext := range []string{".exe", ".cmd"} {
				out = append(out, dir+`\`+binary+ext)
			}
		}
		return out
	case "darwin", "linux":
		home := get("HOME")
		if home != "" && strings.HasPrefix(home, "/") {
			dirs = append(dirs,
				home+"/.local/bin",
				home+"/.claude/local",
				home+"/.npm-global/bin",
				home+"/.bun/bin",
				home+"/.volta/bin",
				home+"/bin",
			)
			globs = append(globs, home+"/.nvm/versions/node/*/bin")
		}
		if env.GOOS == "darwin" {
			dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
		} else {
			dirs = append(dirs, "/usr/local/bin", "/usr/bin", "/home/linuxbrew/.linuxbrew/bin", "/snap/bin")
		}
	default:
		return nil
	}
	var out []string
	for _, dir := range dirs {
		out = append(out, dir+"/"+binary)
	}
	if env.Glob != nil {
		for _, pattern := range globs {
			matches, err := env.Glob(pattern)
			if err != nil {
				continue
			}
			sort.Slice(matches, func(i, j int) bool { return nodeVersionLess(matches[j], matches[i]) })
			for _, dir := range matches {
				out = append(out, dir+"/"+binary)
			}
		}
	}
	return out
}

func winJoin(base string, parts ...string) string {
	if base == "" || !platformAbs("windows", base) {
		return ""
	}
	return strings.TrimRight(base, `\/`) + `\` + strings.Join(parts, `\`)
}

// nodeVersionLess orders ".../node/vX.Y.Z/bin" directories numerically.
func nodeVersionLess(a, b string) bool {
	va, vb := nodeVersion(a), nodeVersion(b)
	for i := 0; i < 3; i++ {
		if va[i] != vb[i] {
			return va[i] < vb[i]
		}
	}
	return a < b
}

func nodeVersion(dir string) [3]int {
	var v [3]int
	name := filepath.Base(filepath.Dir(dir))
	name = strings.TrimPrefix(name, "v")
	for i, part := range strings.SplitN(name, ".", 3) {
		n := 0
		for _, r := range part {
			if r < '0' || r > '9' {
				break
			}
			n = n*10 + int(r-'0')
			if n > 1_000_000 {
				break
			}
		}
		v[i] = n
	}
	return v
}

func platformAbs(goos, path string) bool {
	if goos == "windows" {
		if len(path) >= 3 && isASCIILetter(path[0]) && path[1] == ':' && (path[2] == '\\' || path[2] == '/') {
			return true
		}
		return strings.HasPrefix(path, `\\`) && !strings.HasPrefix(path, `\\?\`) && !strings.HasPrefix(path, `\\.\`)
	}
	return strings.HasPrefix(path, "/")
}

func platformClean(goos, path string) string {
	if goos == runtime.GOOS {
		return filepath.Clean(path)
	}
	return path
}

func isASCIILetter(b byte) bool { return (b|0x20) >= 'a' && (b|0x20) <= 'z' }

// ---------------------------------------------------------------------------
// Control window

// OpenWindow opens the local control page in the default browser. Only
// http://127.0.0.1:<port> URLs are accepted. The URL (including any fragment
// bootstrap token) is passed as one argument and is never logged.
func OpenWindow(address string) error {
	env := platformHostEnv()
	cmds, err := buildOpenWindowCommands(env, address)
	if err != nil {
		return err
	}
	for _, cmd := range cmds {
		if platformStart(cmd) == nil {
			return nil
		}
	}
	return ErrBrowserUnavailable
}

// validateLoopbackURL accepts only http://127.0.0.1:<port>/... with a
// conservative character set so the URL cannot be reinterpreted by a helper's
// own command-line parsing (rundll32, xdg-open handlers).
func validateLoopbackURL(address string) error {
	if address == "" || len(address) > 2048 {
		return ErrNotLoopbackAddress
	}
	for i := 0; i < len(address); i++ {
		c := address[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.IndexByte("-._~:/?#=&%+", c) >= 0) {
			return ErrNotLoopbackAddress
		}
	}
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Opaque != "" {
		return ErrNotLoopbackAddress
	}
	if !strings.HasPrefix(address, "http://127.0.0.1:") {
		return ErrNotLoopbackAddress
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || host != "127.0.0.1" || port == "" || len(port) > 5 || strings.TrimLeft(port, "0123456789") != "" || port[0] == '0' {
		return ErrNotLoopbackAddress
	}
	return nil
}

func buildOpenWindowCommands(env platformEnv, address string) ([]platformCommand, error) {
	if err := validateLoopbackURL(address); err != nil {
		return nil, err
	}
	switch env.GOOS {
	case "darwin":
		return []platformCommand{{Path: "/usr/bin/open", Args: []string{address}}}, nil
	case "windows":
		return []platformCommand{{Path: windowsSystemTool(env, "rundll32.exe"), Args: []string{"url.dll,FileProtocolHandler", address}}}, nil
	case "linux":
		if !hasGraphicalSession(env) {
			return nil, ErrNoGraphicalSession
		}
		var cmds []platformCommand
		if path := lookAbs(env, "xdg-open"); path != "" {
			cmds = append(cmds, platformCommand{Path: path, Args: []string{address}})
		}
		if path := lookAbs(env, "gio"); path != "" {
			cmds = append(cmds, platformCommand{Path: path, Args: []string{"open", address}})
		}
		if len(cmds) == 0 {
			return nil, ErrBrowserUnavailable
		}
		return cmds, nil
	default:
		return nil, ErrUnsupportedPlatform
	}
}

func windowsSystemTool(env platformEnv, name string) string {
	root := ""
	if env.Getenv != nil {
		root = env.Getenv("SystemRoot")
	}
	if root == "" || !platformAbs("windows", root) || strings.ContainsAny(root, `"`) {
		root = `C:\Windows`
	}
	return strings.TrimRight(root, `\/`) + `\System32\` + name
}

func hasGraphicalSession(env platformEnv) bool {
	if env.Getenv == nil {
		return false
	}
	return env.Getenv("WAYLAND_DISPLAY") != "" || env.Getenv("DISPLAY") != ""
}

func lookAbs(env platformEnv, name string) string {
	if env.LookPath == nil {
		return ""
	}
	path, err := env.LookPath(name)
	if err != nil || !platformAbs(env.GOOS, path) {
		return ""
	}
	return path
}

// ---------------------------------------------------------------------------
// Terminal session

// OpenTerminal starts `<executable> connect --ticket <ticketPath>` in a new
// native terminal window and returns once the terminal has been dispatched.
// The child reports readiness to the companion separately. projectDir is used
// only as the initial working directory where the terminal supports it; the
// connect command reads the project from the ticket.
func OpenTerminal(executable, ticketPath, projectDir string) error {
	cmds, err := buildTerminalCommands(platformHostEnv(), executable, ticketPath, projectDir)
	if err != nil {
		return err
	}
	for _, cmd := range cmds {
		if platformStart(cmd) == nil {
			return nil
		}
	}
	return ErrTerminalLaunchFailed
}

// validateLaunchPath rejects relative paths and characters that terminals or
// shells could treat as control input. Spaces, quotes, $, backticks and
// non-ASCII letters are allowed and handled by quoting or argv passing.
func validateLaunchPath(goos, path string) error {
	if path == "" || len(path) > 4096 || !utf8.ValidString(path) || !platformAbs(goos, path) {
		return ErrUnsafeLaunchPath
	}
	for _, r := range path {
		if r == 0 || unicode.IsControl(r) || r == '\u2028' || r == '\u2029' || r == utf8.RuneError {
			return ErrUnsafeLaunchPath
		}
		// Bidirectional controls can make a displayed command misleading.
		if unicode.Is(unicode.Bidi_Control, r) {
			return ErrUnsafeLaunchPath
		}
	}
	if goos == "windows" && strings.ContainsAny(path, `"<>|?*`) {
		return ErrUnsafeLaunchPath
	}
	return nil
}

func connectArgs(ticketPath string) []string {
	return []string{"connect", "--ticket", ticketPath}
}

func buildTerminalCommands(env platformEnv, executable, ticketPath, projectDir string) ([]platformCommand, error) {
	for _, path := range []string{executable, ticketPath, projectDir} {
		if err := validateLaunchPath(env.GOOS, path); err != nil {
			return nil, err
		}
	}
	switch env.GOOS {
	case "darwin":
		name := ""
		if env.RandomName != nil {
			name = env.RandomName()
		}
		return buildDarwinTerminal(executable, ticketPath, name)
	case "windows":
		// The companion executable is a GUI-subsystem program; the connect
		// child allocates its own console (EnsureConsole). Launching it
		// directly avoids cmd.exe/PowerShell/Windows Terminal re-parsing of
		// user paths (%VAR%, ^, ;, &). os/exec quotes argv for
		// CommandLineToArgvW.
		return []platformCommand{{Path: executable, Args: connectArgs(ticketPath), Dir: projectDir, NewConsole: true}}, nil
	case "linux":
		return buildLinuxTerminals(env, executable, ticketPath, projectDir)
	default:
		return nil, ErrUnsupportedPlatform
	}
}

// buildDarwinTerminal writes a private one-shot .command script beside the
// ticket and asks LaunchServices to open it with Terminal. This needs no
// Apple Events automation permission and the script is parsed by /bin/sh,
// not by the user's interactive shell, so POSIX single quoting is exact.
func buildDarwinTerminal(executable, ticketPath, name string) ([]platformCommand, error) {
	dir := filepath.Dir(ticketPath)
	if !strings.HasPrefix(dir, "/") || name == "" || strings.Trim(name, "0123456789abcdef") != "" {
		return nil, ErrUnsafeLaunchPath
	}
	scriptPath := strings.TrimRight(dir, "/") + "/dmz-connect-" + name + ".command"
	script := "#!/bin/sh\n" +
		"rm -f -- \"$0\"\n" +
		"exec " + shellQuote(executable) + " connect --ticket " + shellQuote(ticketPath) + "\n"
	return []platformCommand{{
		Path:       "/usr/bin/open",
		Args:       []string{"-a", "Terminal", scriptPath},
		Script:     script,
		ScriptPath: scriptPath,
	}}, nil
}

// shellQuote returns a POSIX sh single-quoted word.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

type linuxTerminal struct {
	name  string
	build func(path, dir string, argv []string) []string
}

// Each builder passes the program and its arguments as separate argv entries;
// none of these forms is shell-interpreted by the terminal.
var linuxTerminals = []linuxTerminal{
	{"gnome-terminal", func(_, dir string, argv []string) []string {
		return append([]string{"--working-directory=" + dir, "--"}, argv...)
	}},
	{"ptyxis", func(_, dir string, argv []string) []string {
		return append([]string{"--new-window", "--working-directory=" + dir, "--"}, argv...)
	}},
	{"konsole", func(_, dir string, argv []string) []string {
		return append([]string{"--workdir", dir, "-e"}, argv...)
	}},
	{"xfce4-terminal", func(_, dir string, argv []string) []string {
		return append([]string{"--working-directory=" + dir, "-x"}, argv...)
	}},
	{"mate-terminal", func(_, dir string, argv []string) []string {
		return append([]string{"--working-directory=" + dir, "-x"}, argv...)
	}},
	{"kitty", func(_, dir string, argv []string) []string {
		return append([]string{"--directory", dir}, argv...)
	}},
	{"alacritty", func(_, dir string, argv []string) []string {
		return append([]string{"--working-directory", dir, "-e"}, argv...)
	}},
	{"wezterm", func(_, dir string, argv []string) []string {
		return append([]string{"start", "--cwd", dir, "--"}, argv...)
	}},
	{"foot", func(_, dir string, argv []string) []string {
		return append([]string{"--working-directory=" + dir}, argv...)
	}},
	{"xterm", func(_, _ string, argv []string) []string { return append([]string{"-e"}, argv...) }},
	{"x-terminal-emulator", func(_, _ string, argv []string) []string { return append([]string{"-e"}, argv...) }},
}

func buildLinuxTerminals(env platformEnv, executable, ticketPath, projectDir string) ([]platformCommand, error) {
	if !hasGraphicalSession(env) {
		return nil, ErrNoGraphicalSession
	}
	argv := append([]string{executable}, connectArgs(ticketPath)...)
	order := linuxTerminalOrder(env)
	var cmds []platformCommand
	for _, term := range order {
		path := lookAbs(env, term.name)
		if path == "" {
			continue
		}
		cmds = append(cmds, platformCommand{Path: path, Args: term.build(path, projectDir, argv), Dir: projectDir})
	}
	if len(cmds) == 0 {
		return nil, ErrTerminalUnavailable
	}
	return cmds, nil
}

// linuxTerminalOrder prefers the desktop's own terminal, then terminals with
// known argv conventions. The ambiguous Debian alternative is a last resort.
func linuxTerminalOrder(env platformEnv) []linuxTerminal {
	desktop := ""
	if env.Getenv != nil {
		desktop = strings.ToUpper(env.Getenv("XDG_CURRENT_DESKTOP"))
	}
	preferred := ""
	switch {
	case strings.Contains(desktop, "KDE"):
		preferred = "konsole"
	case strings.Contains(desktop, "XFCE"):
		preferred = "xfce4-terminal"
	case strings.Contains(desktop, "MATE"):
		preferred = "mate-terminal"
	case strings.Contains(desktop, "GNOME"), strings.Contains(desktop, "UNITY"), strings.Contains(desktop, "CINNAMON"):
		preferred = "gnome-terminal"
	}
	out := make([]linuxTerminal, 0, len(linuxTerminals))
	for _, t := range linuxTerminals {
		if t.name == preferred {
			out = append(out, t)
		}
	}
	for _, t := range linuxTerminals {
		if t.name != preferred {
			out = append(out, t)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Folder dialog

const platformDialogTimeout = 15 * time.Minute

// ChooseDirectory shows a native folder chooser when one is available and
// returns the selected absolute directory. Callers should offer a typed path
// field when it returns ErrDialogUnavailable.
func ChooseDirectory() (string, error) {
	return ChooseDirectoryContext(context.Background())
}

func ChooseDirectoryContext(parent context.Context) (string, error) {
	env := platformHostEnv()
	cmd, err := buildChooseDirectoryCommand(env)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parent, platformDialogTimeout)
	defer cancel()
	out, code, err := platformOutput(ctx, cmd)
	if err != nil {
		return "", ErrDialogFailed
	}
	return parseChooseDirectoryOutput(env.GOOS, out, code)
}

const windowsFolderDialogScript = `$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false)
Add-Type -AssemblyName System.Windows.Forms
$owner=New-Object System.Windows.Forms.Form
$owner.TopMost=$true
$dialog=New-Object System.Windows.Forms.FolderBrowserDialog
$dialog.Description='Choose a project folder for DMZ'
$dialog.ShowNewFolderButton=$false
if($dialog.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK){[Console]::Out.Write($dialog.SelectedPath);exit 0}
exit 1`

const darwinFolderDialogScript = `activate
POSIX path of (choose folder with prompt "Choose a project folder for DMZ")`

func buildChooseDirectoryCommand(env platformEnv) (platformCommand, error) {
	switch env.GOOS {
	case "darwin":
		return platformCommand{Path: "/usr/bin/osascript", Args: []string{"-e", darwinFolderDialogScript}}, nil
	case "windows":
		return platformCommand{
			Path:     windowsSystemTool(env, `WindowsPowerShell\v1.0\powershell.exe`),
			Args:     []string{"-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-Command", windowsFolderDialogScript},
			NoWindow: true,
		}, nil
	case "linux":
		if !hasGraphicalSession(env) {
			return platformCommand{}, ErrNoGraphicalSession
		}
		if path := lookAbs(env, "zenity"); path != "" {
			return platformCommand{Path: path, Args: []string{"--file-selection", "--directory", "--title=Choose a project folder for DMZ"}}, nil
		}
		if path := lookAbs(env, "kdialog"); path != "" {
			return platformCommand{Path: path, Args: []string{"--title", "Choose a project folder for DMZ", "--getexistingdirectory"}}, nil
		}
		return platformCommand{}, ErrDialogUnavailable
	default:
		return platformCommand{}, ErrUnsupportedPlatform
	}
}

func parseChooseDirectoryOutput(goos string, out []byte, exitCode int) (string, error) {
	if exitCode == 1 {
		return "", ErrDialogCancelled
	}
	if exitCode != 0 {
		return "", ErrDialogFailed
	}
	path := strings.TrimRight(string(out), "\r\n")
	if goos == "darwin" && len(path) > 1 {
		path = strings.TrimSuffix(path, "/")
	}
	if strings.ContainsAny(path, "\r\n") || validateLaunchPath(goos, path) != nil {
		return "", ErrDialogFailed
	}
	if goos == runtime.GOOS {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return "", ErrDialogFailed
		}
		path = filepath.Clean(path)
	}
	return path, nil
}

// ---------------------------------------------------------------------------
// Process helpers

func startPlatformCommand(c platformCommand) error {
	if c.Script != "" {
		if err := writePrivateScript(c.ScriptPath, c.Script); err != nil {
			return ErrTerminalLaunchFailed
		}
	}
	cmd := exec.Command(c.Path, c.Args...)
	cmd.Dir = c.Dir
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil
	applyPlatformProcAttr(cmd, c)
	if err := cmd.Start(); err != nil {
		if c.ScriptPath != "" {
			_ = os.Remove(c.ScriptPath)
		}
		return ErrTerminalLaunchFailed
	}
	// Catch immediate dispatcher failures (for example, no usable browser or
	// terminal server) so another known helper can be tried. A successfully
	// dispatched opener can exit immediately or remain attached to its window.
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	timer := time.NewTimer(200 * time.Millisecond)
	defer timer.Stop()
	select {
	case err := <-done:
		if err != nil {
			if c.ScriptPath != "" {
				_ = os.Remove(c.ScriptPath)
			}
			return ErrTerminalLaunchFailed
		}
		return nil
	case <-timer.C:
		return nil
	}
}

func platformRandomName() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

func writePrivateScript(path, content string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o700)
	if err != nil {
		return err
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

func outputPlatformCommand(ctx context.Context, c platformCommand) ([]byte, int, error) {
	cmd := exec.CommandContext(ctx, c.Path, c.Args...)
	cmd.Dir = c.Dir
	applyPlatformProcAttr(cmd, c)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && ctx.Err() == nil {
			return out, exitErr.ExitCode(), nil
		}
		return nil, -1, err
	}
	if len(out) > 64*1024 {
		return nil, -1, ErrDialogFailed
	}
	return out, 0, nil
}
