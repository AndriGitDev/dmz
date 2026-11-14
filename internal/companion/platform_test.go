// SPDX-License-Identifier: GPL-3.0-only

package companion

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type fakeHost struct {
	goos  string
	env   map[string]string
	path  map[string]string // name -> absolute path found by LookPath
	files map[string]bool   // executable files
	globs map[string][]string
}

func (h fakeHost) platformEnv() platformEnv {
	return platformEnv{
		GOOS:   h.goos,
		Getenv: func(k string) string { return h.env[k] },
		LookPath: func(name string) (string, error) {
			if p, ok := h.path[name]; ok {
				return p, nil
			}
			return "", exec.ErrNotFound
		},
		IsExecutable: func(p string) bool { return h.files[p] },
		Glob:         func(p string) ([]string, error) { return h.globs[p], nil },
		RandomName:   func() string { return "0123456789abcdef" },
	}
}

// Paths exercised against every launcher: spaces, quotes, shell
// metacharacters and non-ASCII letters must reach the child unchanged.
var hostileUnixPaths = []string{
	"/Users/Jón Jónsson/Forrit/DMZ.app/Contents/MacOS/DMZ",
	"/tmp/it's here/DMZ",
	`/tmp/$(touch pwned)/DMZ`,
	"/tmp/`id`/DMZ",
	`/tmp/a"b/$HOME;rm -rf ~ &/DMZ`,
	"/tmp/ünïcødé ✓/DMZ",
	`/tmp/back\slash/DMZ`,
}

// posixUnquote parses one POSIX sh word made only of single-quoted strings and
// backslash-escaped characters, which is all shellQuote can produce.
func posixUnquote(t *testing.T, s string) string {
	t.Helper()
	var b strings.Builder
	for i := 0; i < len(s); {
		switch s[i] {
		case '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				t.Fatalf("unterminated quote in %q", s)
			}
			b.WriteString(s[i+1 : i+1+j])
			i += j + 2
		case '\\':
			b.WriteByte(s[i+1])
			i += 2
		default:
			t.Fatalf("unquoted byte %q in %q", s[i], s)
		}
	}
	return b.String()
}

func TestShellQuoteRoundTrip(t *testing.T) {
	for _, p := range append(hostileUnixPaths, "", "'", "''", `\'`) {
		if got := posixUnquote(t, shellQuote(p)); got != p {
			t.Fatalf("round trip %q -> %q", p, got)
		}
	}
}

func TestShellQuoteWithSystemShell(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("no POSIX shell")
	}
	for _, p := range hostileUnixPaths {
		out, err := exec.Command(sh, "-c", "printf %s "+shellQuote(p)).Output()
		if err != nil || string(out) != p {
			t.Fatalf("sh parsed %q as %q (%v)", p, out, err)
		}
	}
}

func TestDarwinTerminalScript(t *testing.T) {
	env := fakeHost{goos: "darwin"}.platformEnv()
	for _, exe := range hostileUnixPaths {
		ticket := "/private/var/folders/xy/T/dmz-ticket-1/ticket.json"
		cmds, err := buildTerminalCommands(env, exe, ticket, "/Users/Jón/My Project $x")
		if err != nil {
			t.Fatalf("%q: %v", exe, err)
		}
		c := cmds[0]
		if c.Path != "/usr/bin/open" || !reflect.DeepEqual(c.Args, []string{"-a", "Terminal", "/private/var/folders/xy/T/dmz-ticket-1/dmz-connect-0123456789abcdef.command"}) {
			t.Fatalf("unexpected command %#v", c)
		}
		if c.ScriptPath != c.Args[2] {
			t.Fatalf("script path mismatch")
		}
		lines := strings.Split(strings.TrimSuffix(c.Script, "\n"), "\n")
		if len(lines) != 3 || lines[0] != "#!/bin/sh" || lines[1] != `rm -f -- "$0"` {
			t.Fatalf("unexpected script %q", c.Script)
		}
		want := "exec " + shellQuote(exe) + " connect --ticket " + shellQuote(ticket)
		if lines[2] != want {
			t.Fatalf("got %q want %q", lines[2], want)
		}
		if strings.Contains(c.Script, "My Project") {
			t.Fatal("project path must come from the ticket, not the script")
		}
	}
}

func TestDarwinScriptRunsWithSystemShell(t *testing.T) {
	sh, err := exec.LookPath("sh")
	if err != nil || runtime.GOOS == "windows" {
		t.Skip("no POSIX shell")
	}
	dir := t.TempDir()
	// A fake "DMZ" that records its argv, placed in a hostile directory name.
	exeDir := filepath.Join(dir, `Jón's $(id) "dir"`)
	if err := os.Mkdir(exeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(exeDir, "DMZ")
	record := filepath.Join(dir, "argv")
	fake := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > " + shellQuote(record) + "\n"
	if err := os.WriteFile(exe, []byte(fake), 0o700); err != nil {
		t.Fatal(err)
	}
	ticket := filepath.Join(dir, "ticket `x`.json")
	cmds, err := buildDarwinTerminal(exe, ticket, "00ff")
	if err != nil {
		t.Fatal(err)
	}
	c := cmds[0]
	if err := writePrivateScript(c.ScriptPath, c.Script); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(c.ScriptPath); info.Mode().Perm() != 0o700 {
		t.Fatalf("script mode %v", info.Mode().Perm())
	}
	if err := writePrivateScript(c.ScriptPath, c.Script); err == nil {
		t.Fatal("existing script must not be overwritten")
	}
	if out, err := exec.Command(sh, c.ScriptPath).CombinedOutput(); err != nil {
		t.Fatalf("script failed: %v %s", err, out)
	}
	got, _ := os.ReadFile(record)
	if string(got) != "connect\n--ticket\n"+ticket+"\n" {
		t.Fatalf("argv %q", got)
	}
	if _, err := os.Stat(c.ScriptPath); !os.IsNotExist(err) {
		t.Fatal("one-shot script should delete itself")
	}
}

func TestDarwinRejectsBadRandomName(t *testing.T) {
	for _, name := range []string{"", "../x", "a b", "ZZ"} {
		if _, err := buildDarwinTerminal("/a/DMZ", "/t/ticket", name); err != ErrUnsafeLaunchPath {
			t.Fatalf("%q: %v", name, err)
		}
	}
}

func TestLinuxTerminalArgv(t *testing.T) {
	host := fakeHost{
		goos: "linux",
		env:  map[string]string{"DISPLAY": ":0", "XDG_CURRENT_DESKTOP": "ubuntu:GNOME"},
		path: map[string]string{"gnome-terminal": "/usr/bin/gnome-terminal", "xterm": "/usr/bin/xterm", "konsole": "/usr/bin/konsole"},
	}
	exe := "/home/jón/Sótt/DMZ $(id)/DMZ"
	ticket := "/run/user/1000/dmz/ticket `x`.json"
	dir := "/home/jón/it's a project"
	cmds, err := buildTerminalCommands(host.platformEnv(), exe, ticket, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cmds) != 3 {
		t.Fatalf("got %d commands", len(cmds))
	}
	want := []platformCommand{
		{Path: "/usr/bin/gnome-terminal", Args: []string{"--working-directory=" + dir, "--", exe, "connect", "--ticket", ticket}, Dir: dir},
		{Path: "/usr/bin/konsole", Args: []string{"--workdir", dir, "-e", exe, "connect", "--ticket", ticket}, Dir: dir},
		{Path: "/usr/bin/xterm", Args: []string{"-e", exe, "connect", "--ticket", ticket}, Dir: dir},
	}
	if !reflect.DeepEqual(cmds, want) {
		t.Fatalf("got %#v", cmds)
	}

	host.env["XDG_CURRENT_DESKTOP"] = "KDE"
	cmds, _ = buildTerminalCommands(host.platformEnv(), exe, ticket, dir)
	if cmds[0].Path != "/usr/bin/konsole" {
		t.Fatalf("KDE should prefer konsole, got %s", cmds[0].Path)
	}
}

func TestLinuxEveryTerminalKeepsArgvIntact(t *testing.T) {
	argv := []string{"/a b/DMZ", "connect", "--ticket", "/t/x y"}
	for _, term := range linuxTerminals {
		args := term.build("/usr/bin/"+term.name, "/proj dir", argv)
		if len(args) < len(argv) || !reflect.DeepEqual(args[len(args)-len(argv):], argv) {
			t.Fatalf("%s: argv not passed as trailing separate arguments: %q", term.name, args)
		}
	}
}

func TestLinuxHeadlessAndMissingTerminal(t *testing.T) {
	host := fakeHost{goos: "linux", env: map[string]string{}, path: map[string]string{"xterm": "/usr/bin/xterm"}}
	if _, err := buildTerminalCommands(host.platformEnv(), "/a/DMZ", "/t/ticket", "/p"); err != ErrNoGraphicalSession {
		t.Fatalf("headless: %v", err)
	}
	host.env["WAYLAND_DISPLAY"] = "wayland-0"
	host.path = map[string]string{"xterm": "relative/xterm"}
	if _, err := buildTerminalCommands(host.platformEnv(), "/a/DMZ", "/t/ticket", "/p"); err != ErrTerminalUnavailable {
		t.Fatalf("no terminal: %v", err)
	}
}

func TestWindowsTerminalDirectLaunch(t *testing.T) {
	env := fakeHost{goos: "windows"}.platformEnv()
	exe := `C:\Users\Jón Jónsson\Downloads\DMZ %USERNAME% & ^x;y\DMZ.exe`
	ticket := `C:\Users\Jón Jónsson\AppData\Local\Temp\dmz-1\ticket.json`
	dir := `D:\verk efni\it's $x`
	cmds, err := buildTerminalCommands(env, exe, ticket, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []platformCommand{{Path: exe, Args: []string{"connect", "--ticket", ticket}, Dir: dir, NewConsole: true}}
	if !reflect.DeepEqual(cmds, want) {
		t.Fatalf("got %#v", cmds)
	}
	for _, bad := range []string{`C:\a"b\DMZ.exe`, `relative\DMZ.exe`, `\\?\C:\DMZ.exe`, `C:\a|b\DMZ.exe`} {
		if _, err := buildTerminalCommands(env, bad, ticket, dir); err != ErrUnsafeLaunchPath {
			t.Fatalf("%q: %v", bad, err)
		}
	}
}

func TestLaunchPathValidation(t *testing.T) {
	for _, bad := range []string{"", "relative/DMZ", "/a\nb", "/a\rb", "/a\x00b", "/a\x1bb", "/a\u202eb", "/a\u2028b", "/a\x7fb", string([]byte{'/', 0xff})} {
		if validateLaunchPath("linux", bad) == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
	for _, ok := range hostileUnixPaths {
		if err := validateLaunchPath("darwin", ok); err != nil {
			t.Fatalf("rejected %q", ok)
		}
	}
}

func TestOpenWindowURLValidation(t *testing.T) {
	good := []string{
		"http://127.0.0.1:43127/",
		"http://127.0.0.1:43127/#bootstrap=0123abcdef",
		"http://127.0.0.1:1/app?x=1#t=AbC-_.~%2B",
	}
	for _, u := range good {
		if err := validateLoopbackURL(u); err != nil {
			t.Fatalf("rejected %q", u)
		}
	}
	bad := []string{
		"", "https://127.0.0.1:43127/", "http://localhost:43127/", "http://127.0.0.2:43127/",
		"http://127.0.0.1/", "http://127.0.0.1:0/", "http://127.0.0.1:080/", "http://127.0.0.1:999999/",
		"http://127.0.0.1.evil.test:80/", "http://user@127.0.0.1:80/", "http://127.0.0.1:80@evil.test/",
		"http://[::1]:80/", "file:///etc/passwd", "http://127.0.0.1:80/ x", "http://127.0.0.1:80/\"&calc",
		"http://127.0.0.1:80/$(id)", "http://127.0.0.1:80/`id`", "javascript:alert(1)", "HTTP://127.0.0.1:80/",
		"http://127.0.0.1:80/\\..\\", "http://127.0.0.1:80/;x",
	}
	for _, u := range bad {
		if err := validateLoopbackURL(u); err != ErrNotLoopbackAddress {
			t.Fatalf("accepted %q (%v)", u, err)
		}
	}
}

func TestOpenWindowCommands(t *testing.T) {
	const u = "http://127.0.0.1:43127/#bootstrap=abc"
	cmds, err := buildOpenWindowCommands(fakeHost{goos: "darwin"}.platformEnv(), u)
	if err != nil || !reflect.DeepEqual(cmds, []platformCommand{{Path: "/usr/bin/open", Args: []string{u}}}) {
		t.Fatalf("darwin %#v %v", cmds, err)
	}
	win := fakeHost{goos: "windows", env: map[string]string{"SystemRoot": `C:\WINDOWS`}}
	cmds, err = buildOpenWindowCommands(win.platformEnv(), u)
	if err != nil || !reflect.DeepEqual(cmds, []platformCommand{{Path: `C:\WINDOWS\System32\rundll32.exe`, Args: []string{"url.dll,FileProtocolHandler", u}}}) {
		t.Fatalf("windows %#v %v", cmds, err)
	}
	win.env["SystemRoot"] = `relative"evil`
	cmds, _ = buildOpenWindowCommands(win.platformEnv(), u)
	if cmds[0].Path != `C:\Windows\System32\rundll32.exe` {
		t.Fatalf("untrusted SystemRoot used: %s", cmds[0].Path)
	}
	linux := fakeHost{goos: "linux", env: map[string]string{"DISPLAY": ":1"}, path: map[string]string{"xdg-open": "/usr/bin/xdg-open", "gio": "/usr/bin/gio"}}
	cmds, err = buildOpenWindowCommands(linux.platformEnv(), u)
	want := []platformCommand{{Path: "/usr/bin/xdg-open", Args: []string{u}}, {Path: "/usr/bin/gio", Args: []string{"open", u}}}
	if err != nil || !reflect.DeepEqual(cmds, want) {
		t.Fatalf("linux %#v %v", cmds, err)
	}
	linux.env = map[string]string{}
	if _, err := buildOpenWindowCommands(linux.platformEnv(), u); err != ErrNoGraphicalSession {
		t.Fatalf("headless %v", err)
	}
	linux.env["DISPLAY"] = ":1"
	linux.path = nil
	if _, err := buildOpenWindowCommands(linux.platformEnv(), u); err != ErrBrowserUnavailable {
		t.Fatalf("no helper %v", err)
	}
	if _, err := buildOpenWindowCommands(fakeHost{goos: "plan9"}.platformEnv(), u); err != ErrUnsupportedPlatform {
		t.Fatalf("plan9 %v", err)
	}
}

func TestPublicHelpersUseMockedStartAndFixedErrors(t *testing.T) {
	const secretURL = "http://127.0.0.1:43127/#bootstrap=SECRETTOKEN"
	var started []platformCommand
	restore := mockPlatform(t, fakeHost{
		goos: "linux",
		env:  map[string]string{"DISPLAY": ":0"},
		path: map[string]string{"xdg-open": "/usr/bin/xdg-open", "gio": "/usr/bin/gio", "xterm": "/usr/bin/xterm"},
	}, func(c platformCommand) error {
		started = append(started, c)
		return errors.New("boom " + strings.Join(c.Args, " "))
	})
	defer restore()

	err := OpenWindow(secretURL)
	if err != ErrBrowserUnavailable || len(started) != 2 {
		t.Fatalf("OpenWindow fallback: %v %d", err, len(started))
	}
	if strings.Contains(err.Error(), "SECRET") || strings.Contains(err.Error(), "127.0.0.1:43127") {
		t.Fatal("error leaks address")
	}
	err = OpenTerminal("/home/u/secret-name/DMZ", "/tmp/t/ticket", "/home/u/proj")
	if err != ErrTerminalLaunchFailed || strings.Contains(err.Error(), "secret-name") {
		t.Fatalf("OpenTerminal: %v", err)
	}
	if err := OpenWindow("http://evil.test/#SECRETTOKEN"); err != ErrNotLoopbackAddress || strings.Contains(err.Error(), "SECRET") {
		t.Fatalf("validation: %v", err)
	}

	started = nil
	platformStart = func(c platformCommand) error { started = append(started, c); return nil }
	if err := OpenWindow(secretURL); err != nil || len(started) != 1 || started[0].Args[0] != secretURL {
		t.Fatalf("OpenWindow success: %v %#v", err, started)
	}
}

func mockPlatform(t *testing.T, host fakeHost, start func(platformCommand) error) func() {
	t.Helper()
	oldEnv, oldStart, oldOutput := platformHostEnv, platformStart, platformOutput
	platformHostEnv = host.platformEnv
	platformStart = start
	return func() { platformHostEnv, platformStart, platformOutput = oldEnv, oldStart, oldOutput }
}

func TestChooseDirectoryCommands(t *testing.T) {
	c, err := buildChooseDirectoryCommand(fakeHost{goos: "darwin"}.platformEnv())
	if err != nil || c.Path != "/usr/bin/osascript" || c.Args[0] != "-e" || c.Args[1] != darwinFolderDialogScript {
		t.Fatalf("darwin %#v %v", c, err)
	}
	c, err = buildChooseDirectoryCommand(fakeHost{goos: "windows", env: map[string]string{"SystemRoot": `C:\Windows`}}.platformEnv())
	if err != nil || c.Path != `C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe` || !c.NoWindow || c.Args[len(c.Args)-1] != windowsFolderDialogScript {
		t.Fatalf("windows %#v %v", c, err)
	}
	linux := fakeHost{goos: "linux", env: map[string]string{"DISPLAY": ":0"}, path: map[string]string{"kdialog": "/usr/bin/kdialog", "zenity": "/usr/bin/zenity"}}
	c, err = buildChooseDirectoryCommand(linux.platformEnv())
	if err != nil || c.Path != "/usr/bin/zenity" {
		t.Fatalf("zenity %#v %v", c, err)
	}
	delete(linux.path, "zenity")
	if c, _ = buildChooseDirectoryCommand(linux.platformEnv()); c.Path != "/usr/bin/kdialog" {
		t.Fatalf("kdialog %#v", c)
	}
	linux.path = nil
	if _, err = buildChooseDirectoryCommand(linux.platformEnv()); err != ErrDialogUnavailable {
		t.Fatalf("none %v", err)
	}
	linux.env = nil
	if _, err = buildChooseDirectoryCommand(linux.platformEnv()); err != ErrNoGraphicalSession {
		t.Fatalf("headless %v", err)
	}
}

func TestParseChooseDirectoryOutput(t *testing.T) {
	if _, err := parseChooseDirectoryOutput("darwin", nil, 1); err != ErrDialogCancelled {
		t.Fatal(err)
	}
	if _, err := parseChooseDirectoryOutput("darwin", nil, 2); err != ErrDialogFailed {
		t.Fatal(err)
	}
	darwinPath := "/Users/Jón/My $proj"
	if runtime.GOOS == "darwin" {
		darwinPath = filepath.Join(t.TempDir(), "Jón", "My $proj")
		if err := os.MkdirAll(darwinPath, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if p, err := parseChooseDirectoryOutput("darwin", []byte(darwinPath+"/\n"), 0); err != nil || p != darwinPath {
		t.Fatalf("%q %v", p, err)
	}
	windowsPath := `D:\verk efni`
	if runtime.GOOS == "windows" {
		windowsPath = filepath.Join(t.TempDir(), "verk efni")
		if err := os.MkdirAll(windowsPath, 0700); err != nil {
			t.Fatal(err)
		}
	}
	if p, err := parseChooseDirectoryOutput("windows", []byte(windowsPath), 0); err != nil || p != windowsPath {
		t.Fatalf("%q %v", p, err)
	}
	for _, bad := range []string{"relative\n", "/a\n/b\n", "/a\x1b[2J\n"} {
		if _, err := parseChooseDirectoryOutput("darwin", []byte(bad), 0); err != ErrDialogFailed {
			t.Fatalf("%q accepted", bad)
		}
	}
	if runtime.GOOS != "windows" {
		dir := t.TempDir()
		if p, err := parseChooseDirectoryOutput(runtime.GOOS, []byte(dir+"\n"), 0); err != nil || p != dir {
			t.Fatalf("%q %v", p, err)
		}
		if _, err := parseChooseDirectoryOutput(runtime.GOOS, []byte(dir+"/missing\n"), 0); err != ErrDialogFailed {
			t.Fatal("missing dir accepted")
		}
	}
}

func TestChooseDirectoryUsesMockedOutput(t *testing.T) {
	dir := t.TempDir()
	restore := mockPlatform(t, fakeHost{goos: runtime.GOOS, env: map[string]string{"DISPLAY": ":0", "SystemRoot": `C:\Windows`}, path: map[string]string{"zenity": "/usr/bin/zenity"}}, nil)
	defer restore()
	platformOutput = func(context.Context, platformCommand) ([]byte, int, error) { return []byte(dir + "\n"), 0, nil }
	if runtime.GOOS == "windows" {
		t.Skip("POSIX temp path")
	}
	if p, err := ChooseDirectory(); err != nil || p != dir {
		t.Fatalf("%q %v", p, err)
	}
	platformOutput = func(context.Context, platformCommand) ([]byte, int, error) {
		return nil, -1, errors.New("exec: " + dir)
	}
	if _, err := ChooseDirectory(); err != ErrDialogFailed {
		t.Fatal(err)
	}
}

func TestDiscoverClients(t *testing.T) {
	host := fakeHost{
		goos:  "darwin",
		env:   map[string]string{"HOME": "/Users/Jón"},
		path:  map[string]string{"codex": "/opt/homebrew/bin/codex"},
		files: map[string]bool{"/opt/homebrew/bin/codex": true, "/Users/Jón/.nvm/versions/node/v9.11.2/bin/claude": true, "/Users/Jón/.nvm/versions/node/v20.1.0/bin/claude": true},
		globs: map[string][]string{"/Users/Jón/.nvm/versions/node/*/bin": {"/Users/Jón/.nvm/versions/node/v20.1.0/bin", "/Users/Jón/.nvm/versions/node/v9.11.2/bin"}},
	}
	got := discoverClients(host.platformEnv())
	want := []ClientInstallation{
		{ID: "codex", Name: "Codex CLI", Path: "/opt/homebrew/bin/codex", Available: true},
		{ID: "claude", Name: "Claude Code", Path: "/Users/Jón/.nvm/versions/node/v20.1.0/bin/claude", Available: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v", got)
	}

	// Minimal GUI PATH: fall back to user install locations.
	host.path = nil
	host.files = map[string]bool{"/Users/Jón/.local/bin/claude": true}
	got = discoverClients(host.platformEnv())
	if got[0].Available || got[0].Path != "" || got[1].Path != "/Users/Jón/.local/bin/claude" {
		t.Fatalf("fallback %#v", got)
	}

	// Relative PATH hits and non-executables are ignored.
	host.path = map[string]string{"codex": "bin/codex"}
	host.files = map[string]bool{"bin/codex": true}
	if got = discoverClients(host.platformEnv()); got[0].Available {
		t.Fatalf("relative accepted %#v", got)
	}
}

func TestDiscoverClientsWindows(t *testing.T) {
	host := fakeHost{
		goos:  "windows",
		env:   map[string]string{"USERPROFILE": `C:\Users\Jón`, "APPDATA": `C:\Users\Jón\AppData\Roaming`, "LOCALAPPDATA": "relative"},
		files: map[string]bool{`C:\Users\Jón\AppData\Roaming\npm\codex.cmd`: true, `C:\Users\Jón\.local\bin\claude.exe`: true},
	}
	got := discoverClients(host.platformEnv())
	if got[0].Path != `C:\Users\Jón\AppData\Roaming\npm\codex.cmd` || got[1].Path != `C:\Users\Jón\.local\bin\claude.exe` {
		t.Fatalf("got %#v", got)
	}
	for _, c := range clientCandidates(host.platformEnv(), "codex") {
		if strings.HasPrefix(c, "relative") {
			t.Fatalf("relative base used: %s", c)
		}
	}
}

func TestDiscoverClientsHostSmoke(t *testing.T) {
	// Real filesystem lookup with an isolated PATH and HOME; never executes the clients.
	dir := t.TempDir()
	bin := filepath.Join(dir, ".local", "bin")
	if err := os.MkdirAll(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	if err := os.WriteFile(filepath.Join(bin, "claude"), []byte("#!/bin/sh\nexit 99\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "codex"), []byte("not executable"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", filepath.Join(dir, "empty"))
	t.Setenv("HOME", dir)
	got := DiscoverClients()
	// codex may legitimately exist in a system directory on this host; the
	// non-executable user file must never be reported.
	if got[0].Path == filepath.Join(bin, "codex") || !got[1].Available || got[1].Path != filepath.Join(bin, "claude") {
		t.Fatalf("got %#v", got)
	}
}

func TestStartPlatformCommandFailureIsFixed(t *testing.T) {
	err := startPlatformCommand(platformCommand{Path: "/nonexistent/secret dir/x"})
	if err != ErrTerminalLaunchFailed || strings.Contains(err.Error(), "secret") {
		t.Fatalf("%v", err)
	}
}

func TestErrorsContainNoPlaceholders(t *testing.T) {
	for _, err := range []error{ErrUnsupportedPlatform, ErrNotLoopbackAddress, ErrBrowserUnavailable, ErrNoGraphicalSession, ErrTerminalUnavailable, ErrTerminalLaunchFailed, ErrUnsafeLaunchPath, ErrDialogUnavailable, ErrDialogCancelled, ErrDialogFailed, ErrConsoleUnavailable} {
		if strings.Contains(err.Error(), "%") {
			t.Fatalf("formatted error %q", err)
		}
	}
}

func TestDispatchRejectsImmediateHelperFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX helper fixture")
	}
	if err := startPlatformCommand(platformCommand{Path: "/bin/sh", Args: []string{"-c", "exit 7"}}); err != ErrTerminalLaunchFailed {
		t.Fatal("immediate terminal dispatcher failure reported as success")
	}
	if err := startPlatformCommand(platformCommand{Path: "/bin/sh", Args: []string{"-c", "exit 0"}}); err != nil {
		t.Fatal("successful terminal dispatcher rejected")
	}
}
