// dmz is an experimental local adapter for coding clients. The supported
// Chromium extension remains a separate application.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AndriGitDev/dmz/internal/agentgateway"
	"github.com/AndriGitDev/dmz/internal/companion"
)

const scopeNotice = "experimental BASIC profile with limited name/address coverage. Selected user text is masked; known answer markers restore locally and known originals are re-masked in supported user/assistant/tool-result text. Tool arguments, instructions, attachments and other network routes are outside coverage. Tool results are not generally scanned. Client history on disk is unchanged."

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		if handled, err := launchAppHost(); handled {
			if err != nil {
				companion.ShowStartupError()
				os.Exit(1)
			}
			return
		}
	}
	code := mainCode(args, os.Stdin, os.Stdout, os.Stderr)
	graphical := len(args) == 0 || (args[0] == "app" && len(args) == 1)
	if code != 0 && graphical {
		companion.ShowStartupError()
	}
	os.Exit(code)
}

func mainCode(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 {
		args = []string{"app"}
	}
	ctx, stop := signal.NotifyContext(context.Background(), lifecycleSignals()...)
	defer stop()
	var err error
	switch args[0] {
	case "app":
		err = app(ctx, args[1:], errOut)
	case "connect":
		return connect(ctx, args[1:], in, out, errOut)
	case "run":
		return run(ctx, args[1:], in, out, errOut)
	case "serve":
		err = serve(ctx, args[1:], errOut)
	case "status", "reset":
		err = control(ctx, args[0], args[1:], out, errOut)
	case "config":
		err = configFragment(args[1:], out, errOut)
	case "version":
		fmt.Fprintf(out, "dmz local gateway experimental basic (%s %s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	case "help", "--help", "-h":
		usage(out)
	default:
		usage(errOut)
		return 2
	}
	if err != nil {
		fmt.Fprintln(errOut, "dmz:", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, `Experimental dmz coding-client adapter

  dmz run [--auth api|chatgpt] [--client-bin PATH] codex [client arguments...]
  dmz run [--client-bin PATH] claude [client arguments...]
  dmz serve --protocol responses|messages [--listen 127.0.0.1:PORT]
  dmz status [--address http://127.0.0.1:PORT]
  dmz reset [--address http://127.0.0.1:PORT]
  dmz config [--address http://127.0.0.1:PORT] codex|claude
  dmz version

Native Mac and Windows builds open a background status icon.
The Linux build retains the optional local app window.
app --background enables persistent local routing for compatible installed tools.

run uses temporary process configuration and a random local credential.
serve requires DMZ_LOCAL_TOKEN (at least 32 characters); it never prints it.
serve --local-auth bearer|api-key accepts that local credential in the
client's standard credential field and uses the provider's API key from
OPENAI_API_KEY or ANTHROPIC_API_KEY. No subscription conversion is implied.
API-mode Codex requires OPENAI_API_KEY. ChatGPT routing is experimental and
must be verified for the user's account. Claude keeps its existing auth.
dmz reads no client authentication stores. Claude routing settings are
checked locally. Desktop is not qualified.
config prints an operator-reviewed fragment only; it changes no files.
`+scopeNotice)
}

func configFragment(args []string, out, errOut io.Writer) error {
	f := flag.NewFlagSet("config", flag.ContinueOnError)
	f.SetOutput(errOut)
	address := f.String("address", "http://127.0.0.1:43127", "local adapter origin")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 1 || (f.Arg(0) != "codex" && f.Arg(0) != "claude") {
		return errors.New("choose codex or claude after config options")
	}
	u, err := url.Parse(*address)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("address must be an HTTP 127.0.0.1 origin")
	}
	base := strings.TrimRight(*address, "/")
	fmt.Fprintln(errOut, "dmz: review and merge this fragment; no configuration files were changed. Desktop is not qualified.")
	if f.Arg(0) == "codex" {
		fmt.Fprintf(out, `# Start dmz serve --protocol responses --local-auth bearer first.
# The desktop process must inherit DMZ_LOCAL_TOKEN. Keep your selected model.
# Place model_provider at TOP LEVEL before any [table]. Do not append this
# entire fragment after an existing TOML table: that would keep the old route.
model_provider = "dmz_local"

[features]
enable_request_compression = false

[model_providers.dmz_local]
name = "dmz local experimental"
base_url = %s
wire_api = "responses"
requires_openai_auth = false
env_key = "DMZ_LOCAL_TOKEN"
supports_websockets = false
`, strconv.Quote(base+"/v1"))
	} else {
		fmt.Fprintf(out, `{
  "inferenceProvider": "gateway",
  "inferenceGatewayBaseUrl": %s,
  "inferenceGatewayAuthScheme": "bearer",
  "inferenceGatewayApiKey": "REPLACE_WITH_LOCAL_DMZ_TOKEN"
}
`, strconv.Quote(base))
		fmt.Fprintln(errOut, "dmz: use the local token only in the desktop gateway field. Start serve --protocol messages --local-auth bearer with ANTHROPIC_API_KEY in the service environment. Test the Code tab; Chat/Cowork are outside this prototype.")
	}
	return nil
}

type options struct {
	upstream, auth, clientBin string
	allowLoopback             bool
}

func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) int {
	return runInternal(ctx, args, in, out, errOut, nil)
}

func runInternal(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer, external *connectorSession) int {
	terminalErrOut := errOut
	var streamMu sync.Mutex
	errOut = lockedWriter{mu: &streamMu, out: errOut}
	f := flag.NewFlagSet("run", flag.ContinueOnError)
	f.SetOutput(errOut)
	var opts options
	f.StringVar(&opts.auth, "auth", "api", "Codex auth route: api or chatgpt")
	f.StringVar(&opts.clientBin, "client-bin", "", "client executable (default codex or claude)")
	f.StringVar(&opts.upstream, "upstream", "", "fixed provider API base URL (default selected by client/auth)")
	f.BoolVar(&opts.allowLoopback, "allow-loopback-upstream", false, "allow a literal loopback synthetic fixture upstream")
	if err := f.Parse(args); err != nil {
		return 2
	}
	rest := f.Args()
	if len(rest) == 0 || (rest[0] != "codex" && rest[0] != "claude") {
		fmt.Fprintln(errOut, "dmz: choose codex or claude after launcher options")
		return 2
	}
	client := rest[0]
	clientArgs := rest[1:]
	if err := validateClientArgs(client, clientArgs); err != nil {
		fmt.Fprintln(errOut, "dmz:", err)
		return 2
	}
	if opts.auth != "api" && opts.auth != "chatgpt" {
		fmt.Fprintln(errOut, "dmz: unsupported authentication route")
		return 2
	}
	if client == "claude" && opts.auth != "api" {
		fmt.Fprintln(errOut, "dmz: --auth selects Codex routing only; Claude retains its existing authentication")
		return 2
	}
	if client == "claude" {
		for _, key := range []string{"CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY", "CLAUDE_CODE_USE_MANTLE"} {
			if value := os.Getenv(key); value != "" && value != "0" && value != "false" {
				fmt.Fprintln(errOut, "dmz: alternate Claude backends are outside this adapter's qualified route")
				return 2
			}
		}
		if existing := strings.TrimRight(os.Getenv("ANTHROPIC_BASE_URL"), "/"); existing != "" && existing != "https://api.anthropic.com" && existing != "https://api.anthropic.com/v1" {
			fmt.Fprintln(errOut, "dmz: an existing Claude upstream override is configured; gateway chaining is not qualified")
			return 2
		}
	} else if existing := strings.TrimRight(os.Getenv("OPENAI_BASE_URL"), "/"); existing != "" && existing != "https://api.openai.com/v1" {
		fmt.Fprintln(errOut, "dmz: an existing OpenAI upstream override is configured; gateway chaining is not qualified")
		return 2
	}
	protocol := agentgateway.Responses
	upstream := "https://api.openai.com/v1"
	if opts.auth == "chatgpt" {
		upstream = "https://chatgpt.com/backend-api/codex"
	}
	if client == "claude" {
		protocol = agentgateway.Messages
		upstream = "https://api.anthropic.com/v1"
	}
	if opts.upstream != "" {
		upstream = opts.upstream
	}
	if external == nil && client == "codex" && opts.auth == "api" && os.Getenv("OPENAI_API_KEY") == "" {
		fmt.Fprintln(errOut, "dmz: API mode requires OPENAI_API_KEY; use --auth chatgpt only for a qualified ChatGPT route")
		return 2
	}
	// A random per-launch credential is never written to client configuration.
	entropy := make([]byte, 32)
	if _, err := rand.Read(entropy); err != nil {
		fmt.Fprintln(errOut, "dmz: local credential generation failed")
		return 1
	}
	token := hex.EncodeToString(entropy)
	if external != nil {
		token = external.GatewayToken
	}
	var outputMu sync.Mutex
	seenWarnings := make(map[string]bool)
	warning := func(code string) {
		outputMu.Lock()
		defer outputMu.Unlock()
		if seenWarnings[code] {
			return
		}
		seenWarnings[code] = true
		switch code {
		case agentgateway.WarnInstructionsUnchanged, agentgateway.WarnToolResultUnchanged, agentgateway.WarnToolResultDetectionNotApplied, agentgateway.WarnNoUserText, "reply_content_type_inferred_sse", "reply_content_type_inferred_json":
			return
		}
		fmt.Fprintln(errOut, "dmz warning:", code)
	}
	cfg := agentgateway.Config{Protocol: protocol, Upstream: upstream, Token: token, AllowLoopbackUpstream: opts.allowLoopback, Warning: warning, RestoreReplies: true}
	if client == "claude" {
		cfg.SessionHeader = "X-Claude-Code-Session-Id"
	}
	var handler *agentgateway.Server
	var httpServer *http.Server
	var err error
	base := ""
	serveDone := make(chan error, 1)
	if external == nil {
		handler, err = agentgateway.NewServer(cfg)
		if err != nil {
			fmt.Fprintln(errOut, "dmz:", err)
			return 2
		}
		listener, listenErr := net.Listen("tcp4", "127.0.0.1:0")
		if listenErr != nil {
			fmt.Fprintln(errOut, "dmz: cannot bind local adapter")
			return 1
		}
		httpServer = newHTTPServer(handler)
		go func() { serveDone <- httpServer.Serve(listener) }()
		defer shutdown(httpServer)
		base = "http://" + listener.Addr().String()
	} else {
		base = external.GatewayURL
	}
	env := os.Environ()
	env = setEnv(env, "DMZ_LOCAL_TOKEN", token)
	if external != nil {
		if external.Auth == "api" {
			env = setEnv(env, "OPENAI_API_KEY", token)
			env = setEnv(env, "ANTHROPIC_API_KEY", "")
			env = setEnv(env, "ANTHROPIC_AUTH_TOKEN", token)
			env = setEnv(env, "CLAUDE_CODE_OAUTH_TOKEN", "")
		} else {
			env = setEnv(env, "OPENAI_API_KEY", "")
			env = setEnv(env, "ANTHROPIC_API_KEY", "")
			env = setEnv(env, "ANTHROPIC_AUTH_TOKEN", "")
		}
		env = setEnv(env, "PATH", filepath.Dir(external.ClientPath)+string(os.PathListSeparator)+os.Getenv("PATH"))
	}
	if client == "codex" {
		overrides := []string{"--no-daemon", "-c", `model_provider="dmz_local"`, "-c", `model_providers.dmz_local.name="dmz local experimental"`, "-c", "model_providers.dmz_local.base_url=" + strconv.Quote(base+"/v1"), "-c", `model_providers.dmz_local.wire_api="responses"`, "-c", `model_providers.dmz_local.supports_websockets=false`, "-c", `features.enable_request_compression=false`, "-c", `model_providers.dmz_local.env_http_headers={"X-Dmz-Token"="DMZ_LOCAL_TOKEN"}`}
		if opts.auth == "chatgpt" {
			overrides = append(overrides, "-c", `model_providers.dmz_local.requires_openai_auth=true`)
		} else {
			overrides = append(overrides, "-c", `model_providers.dmz_local.requires_openai_auth=false`, "-c", `model_providers.dmz_local.env_key="OPENAI_API_KEY"`)
		}
		clientArgs = append(overrides, clientArgs...)
	} else {
		env = setEnv(env, "ANTHROPIC_BASE_URL", base)
		headers := os.Getenv("ANTHROPIC_CUSTOM_HEADERS")
		if strings.Contains(strings.ToLower(headers), "x-dmz-") {
			fmt.Fprintln(errOut, "dmz: remove existing dmz custom headers before starting a new launch")
			return 2
		}
		if headers != "" {
			headers += "\n"
		}
		headers += agentgateway.TokenHeader + ": " + token
		env = setEnv(env, "ANTHROPIC_CUSTOM_HEADERS", headers)
		var cleanup func()
		settingsOpts := claudeLaunchSettings{}
		if external != nil {
			settingsOpts.ProjectDir = external.Project
			settingsOpts.PinAuth = true
			if external.Auth == "api" {
				settingsOpts.LocalBearer = token
			}
		}
		clientArgs, cleanup, err = claudeSettings(clientArgs, base, headers, settingsOpts)
		if err != nil {
			fmt.Fprintln(errOut, "dmz:", err)
			return 2
		}
		defer cleanup()
	}
	bin := opts.clientBin
	if bin == "" {
		bin = client
	}
	command := exec.CommandContext(ctx, bin, clientArgs...)
	cleanupChild := configureChild(command, in, out, terminalErrOut)
	command.Env = env
	if external != nil {
		command.Dir = external.Project
	}
	command.Stdin = in
	command.Stdout = out
	command.Stderr = lockedWriter{mu: &outputMu, out: errOut}
	fmt.Fprintln(errOut, "dmz:", scopeNotice)
	fmt.Fprintln(errOut, "dmz: local adapter active for this launch; known answer markers restore locally. Mappings are discarded on exit; use a fresh conversation after reset or restart.")
	err = command.Run()
	cleanupChild()
	var snapshot agentgateway.Status
	if external == nil {
		shutdown(httpServer)
		snapshot = handler.Snapshot()
	} else {
		snapshot = externalStatus(ctx, external)
	}
	outputMu.Lock()
	fmt.Fprintf(errOut, "dmz: observed %d model-route requests, %d text parts, %d changed parts.\n", snapshot.Requests, snapshot.TextParts, snapshot.ChangedParts)
	codes := make([]string, 0, len(snapshot.Warnings))
	for code := range snapshot.Warnings {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	for _, code := range codes {
		fmt.Fprintf(errOut, "dmz: warning-count %s=%d\n", code, snapshot.Warnings[code])
	}
	if snapshot.Requests == 0 {
		fmt.Fprintln(errOut, "dmz warning: no model-route traffic observed; this invocation does not establish masking coverage")
	}
	outputMu.Unlock()
	select {
	case serverErr := <-serveDone:
		if serverErr != nil && !errors.Is(serverErr, http.ErrServerClosed) {
			fmt.Fprintln(errOut, "dmz: local adapter stopped unexpectedly")
			return 1
		}
	default:
	}
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			if exit.ExitCode() >= 0 {
				return exit.ExitCode()
			}
			return 1
		}
		fmt.Fprintln(errOut, "dmz: client could not start or was interrupted")
		return 1
	}
	return 0
}

func validateClientArgs(client string, args []string) error {
	for i, arg := range args {
		if arg == "--" {
			break
		}
		key := strings.SplitN(arg, "=", 2)[0]
		switch key {
		case "--remote", "--remote-control", "--cloud", "--environment", "--teleport", "--oss", "--local-provider", "--bg", "--background", "--detach":
			return errors.New("remote, background or provider overrides are outside this launcher's supported route")
		}
		if client == "codex" {
			enableValue := ""
			if key == "--enable" && i+1 < len(args) {
				enableValue = args[i+1]
			}
			if strings.HasPrefix(arg, "--enable=") {
				enableValue = strings.TrimPrefix(arg, "--enable=")
			}
			if enableValue == "enable_request_compression" || strings.Contains(enableValue, "websocket") {
				return errors.New("compressed model requests are outside this adapter's masking scope")
			}
			if key == "--profile" || key == "-p" {
				return errors.New("profile overrides are outside this launcher's supported route")
			}
			var config string
			if arg == "-c" || arg == "--config" {
				if i+1 < len(args) {
					config = args[i+1]
				}
			} else if strings.HasPrefix(arg, "--config=") {
				config = strings.TrimPrefix(arg, "--config=")
			} else if strings.HasPrefix(arg, "-c") {
				config = strings.TrimPrefix(arg, "-c")
			}
			configKey := strings.TrimSpace(strings.SplitN(config, "=", 2)[0])
			if strings.ContainsAny(configKey, "\"'\\") {
				return errors.New("quoted configuration override keys are not supported by this launcher")
			}
			if configKey == "model_provider" || configKey == "openai_base_url" || strings.HasPrefix(configKey, "model_providers") || strings.HasPrefix(configKey, "profiles") || configKey == "features" || configKey == "features.enable_request_compression" || strings.HasPrefix(configKey, "features.") && strings.Contains(configKey, "websocket") {
				return errors.New("client configuration cannot override the dmz provider route")
			}
		}
	}
	return nil
}

func serve(ctx context.Context, args []string, errOut io.Writer) error {
	f := flag.NewFlagSet("serve", flag.ContinueOnError)
	f.SetOutput(errOut)
	protocol := f.String("protocol", "", "responses or messages")
	listen := f.String("listen", "127.0.0.1:43127", "literal IPv4 loopback listen address")
	upstream := f.String("upstream", "", "fixed provider API base URL")
	fixture := f.Bool("allow-loopback-upstream", false, "allow a literal loopback synthetic upstream")
	localAuth := f.String("local-auth", agentgateway.AuthHeader, "local authentication: header, bearer, or api-key")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected serve arguments")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || host != "127.0.0.1" {
		return errors.New("listen address must use 127.0.0.1")
	}
	p := agentgateway.Protocol(*protocol)
	if *upstream == "" {
		if p == agentgateway.Messages {
			*upstream = "https://api.anthropic.com/v1"
		} else {
			*upstream = "https://api.openai.com/v1"
		}
	}
	var warningMu sync.Mutex
	cfg := agentgateway.Config{Protocol: p, Upstream: *upstream, Token: os.Getenv("DMZ_LOCAL_TOKEN"), AllowLoopbackUpstream: *fixture, RestoreReplies: true, Warning: func(code string) {
		warningMu.Lock()
		defer warningMu.Unlock()
		fmt.Fprintln(errOut, "dmz warning:", code)
	}}
	cfg.LocalAuth = *localAuth
	if *localAuth != agentgateway.AuthHeader {
		if p == agentgateway.Messages {
			cfg.ProviderKey = os.Getenv("ANTHROPIC_API_KEY")
		} else {
			cfg.ProviderKey = os.Getenv("OPENAI_API_KEY")
		}
	}
	if p == agentgateway.Messages {
		cfg.SessionHeader = "X-Claude-Code-Session-Id"
	}
	handler, err := agentgateway.NewServer(cfg)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", *listen)
	if err != nil {
		return errors.New("cannot bind local adapter")
	}
	server := newHTTPServer(handler)
	defer shutdown(server)
	fmt.Fprintln(errOut, "dmz:", scopeNotice)
	fmt.Fprintln(errOut, "dmz: manual adapter at", listener.Addr(), "— desktop compatibility is not qualified")
	if p == agentgateway.Responses {
		fmt.Fprintln(errOut, "dmz: one explicit shared mapping scope for this service lifetime; do not connect unrelated clients")
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	select {
	case <-ctx.Done():
		return nil
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("local adapter stopped")
	}
}

func control(ctx context.Context, action string, args []string, out, errOut io.Writer) error {
	f := flag.NewFlagSet(action, flag.ContinueOnError)
	f.SetOutput(errOut)
	address := f.String("address", "http://127.0.0.1:43127", "running local adapter origin")
	if err := f.Parse(args); err != nil {
		return err
	}
	if f.NArg() != 0 {
		return errors.New("unexpected control arguments")
	}
	u, err := url.Parse(*address)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("address must be an HTTP 127.0.0.1 origin")
	}
	token := os.Getenv("DMZ_LOCAL_TOKEN")
	if len(token) < 32 {
		return errors.New("DMZ_LOCAL_TOKEN is required")
	}
	method := "GET"
	if action == "reset" {
		method = "POST"
	}
	r, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(*address, "/")+"/_dmz/"+action, nil)
	if err != nil {
		return errors.New("invalid local address")
	}
	r.Header.Set(agentgateway.TokenHeader, token)
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("local redirects unsupported") }}
	response, err := client.Do(r)
	if err != nil {
		return errors.New("local adapter unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return errors.New("local adapter rejected the control request")
	}
	if action == "reset" {
		fmt.Fprintln(out, "dmz mappings reset; old markers can no longer be restored and resumed history may degrade")
		return nil
	}
	_, err = io.Copy(out, io.LimitReader(response.Body, 64<<10))
	return err
}

func newHTTPServer(handler http.Handler) *http.Server {
	return &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 << 10, ErrorLog: log.New(io.Discard, "", 0)}
}
func shutdown(server *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
	_ = server.Close()
}
func setEnv(env []string, key, value string) []string {
	result := make([]string, 0, len(env)+1)
	prefix := key + "="
	for _, entry := range env {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

type lockedWriter struct {
	mu  *sync.Mutex
	out io.Writer
}

func (w lockedWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.out.Write(p)
}
