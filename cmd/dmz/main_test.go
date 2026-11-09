package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/AndriGitDev/dmz/internal/agentgateway"
)

// The test executable doubles as a deterministic child client. TestMain runs
// before testing's flag parser, so it accepts the actual launcher overrides.
func TestMain(m *testing.M) {
	if os.Getenv("DMZ_SYNTHETIC_CHILD") == "1" {
		os.Exit(syntheticChild(os.Args[1:]))
	}
	if os.Getenv("DMZ_SYNTHETIC_CHILD") == "signal" {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, syscall.SIGTERM)
		fmt.Println("synthetic-child-ready")
		<-ch
		fmt.Println("synthetic-child-graceful-exit")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func syntheticChild(args []string) int {
	base := os.Getenv("ANTHROPIC_BASE_URL")
	protocol := os.Getenv("DMZ_SYNTHETIC_PROTOCOL")
	if protocol == "responses" {
		base = ""
		for _, arg := range args {
			if strings.HasPrefix(arg, "model_providers.dmz_local.base_url=") {
				value, err := strconv.Unquote(strings.TrimPrefix(arg, "model_providers.dmz_local.base_url="))
				if err != nil {
					return 30
				}
				base = value
			}
		}
		if !strings.HasPrefix(base, "http://127.0.0.1:") {
			return 31
		}
	}
	if base == "" {
		return 32
	}
	endpoint := base + "/responses"
	body := `{"input":"Email pilot@example.test"}`
	if protocol == "messages" {
		endpoint = base + "/v1/messages"
		body = `{"messages":[{"role":"user","content":"Email pilot@example.test"}]}`
	}
	r, err := http.NewRequest("POST", endpoint, strings.NewReader(body))
	if err != nil {
		return 33
	}
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set(agentgateway.TokenHeader, os.Getenv("DMZ_LOCAL_TOKEN"))
	r.Header.Set("X-Claude-Code-Session-Id", "synthetic-session")
	r.Header.Set("Authorization", "Bearer synthetic-provider-key")
	response, err := http.DefaultClient.Do(r)
	if err != nil {
		return 34
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return 35
	}
	got, err := io.ReadAll(response.Body)
	if err != nil {
		return 36
	}
	fmt.Print(string(got))
	return 0
}

func TestLauncherRoutesBothClientsWithoutWritingConfig(t *testing.T) {
	for _, client := range []string{"codex", "claude"} {
		t.Run(client, func(t *testing.T) {
			protocol := "responses"
			if client == "claude" {
				protocol = "messages"
			}
			t.Setenv("DMZ_SYNTHETIC_CHILD", "1")
			t.Setenv("DMZ_SYNTHETIC_PROTOCOL", protocol)
			t.Setenv("OPENAI_API_KEY", "synthetic-provider-key")
			t.Setenv("OPENAI_BASE_URL", "")
			t.Setenv("ANTHROPIC_BASE_URL", "")
			t.Setenv("ANTHROPIC_CUSTOM_HEADERS", "")
			t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if bytes.Contains(body, []byte("pilot@example.test")) || !bytes.Contains(body, []byte("EMAIL-")) {
					t.Error("raw identifier reached fixture")
				}
				if r.Header.Get(agentgateway.TokenHeader) != "" {
					t.Error("local credential reached fixture")
				}
				fmt.Fprint(w, `{"synthetic":"ok"}`)
			}))
			defer provider.Close()
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			var out, errOut bytes.Buffer
			code := run(context.Background(), []string{"--client-bin", executable, "--upstream", provider.URL + "/v1", "--allow-loopback-upstream", client}, strings.NewReader(""), &out, &errOut)
			if code != 0 {
				t.Fatalf("launcher failed (%d): %s", code, errOut.String())
			}
			if out.String() != `{"synthetic":"ok"}` || !strings.Contains(errOut.String(), "1 changed parts") {
				t.Fatal("client output or aggregate summary missing")
			}
			if strings.Contains(errOut.String(), "pilot@example.test") || strings.Contains(errOut.String(), "synthetic-provider-key") {
				t.Fatal("request or provider credentials appeared in status")
			}
		})
	}
}

func TestRoutingOverridesRejected(t *testing.T) {
	for _, args := range [][]string{
		{"--remote", "ws://example.test"}, {"--oss"}, {"-p", "other"}, {"--profile=other"},
		{"-c", `model_provider="other"`}, {`-cmodel_providers.other.base_url="https://example.test"`},
		{`--config=openai_base_url="https://example.test"`}, {"--config", `model_providers = {}`},
		{"--enable", "responses_websockets"}, {"--enable=enable_request_compression"}, {"-c", `"model_provider"="other"`}, {"--bg"},
	} {
		if validateClientArgs("codex", args) == nil {
			t.Errorf("accepted route override %q", args)
		}
	}
	for _, args := range [][]string{{"-p", "benign synthetic prompt"}, {"--print", "benign synthetic prompt"}} {
		if err := validateClientArgs("claude", args); err != nil {
			t.Fatal("Claude print flag rejected")
		}
	}
	if err := validateClientArgs("codex", []string{"exec", "-c", `model="synthetic-model"`, "benign synthetic prompt"}); err != nil {
		t.Fatal("model selection should remain available")
	}
	if err := validateClientArgs("codex", []string{"exec", "--", "--remote"}); err != nil {
		t.Fatal("literal prompt after -- was treated as a routing flag")
	}
}

func TestEnvironmentReplacementDoesNotDuplicateCredential(t *testing.T) {
	got := setEnv([]string{"OTHER=unchanged", "DMZ_LOCAL_TOKEN=old", "DMZ_LOCAL_TOKEN=duplicate"}, "DMZ_LOCAL_TOKEN", "synthetic-new")
	if len(got) != 2 || got[0] != "OTHER=unchanged" || got[1] != "DMZ_LOCAL_TOKEN=synthetic-new" {
		t.Fatal("environment replacement failed")
	}
}

func TestServeRequiresLocalCredentialAndLoopback(t *testing.T) {
	t.Setenv("DMZ_LOCAL_TOKEN", "")
	var errOut bytes.Buffer
	if err := serve(context.Background(), []string{"--protocol", "messages"}, &errOut); err == nil {
		t.Fatal("missing credential accepted")
	}
	if err := serve(context.Background(), []string{"--protocol", "messages", "--listen", "0.0.0.0:43127"}, &errOut); err == nil {
		t.Fatal("nonloopback listener accepted")
	}
}

func TestStatusShapeHasNoMapping(t *testing.T) {
	data, err := json.Marshal(agentgateway.Status{Profile: "basic-experimental"})
	if err != nil {
		t.Fatal(err)
	}
	for _, word := range []string{"forward", "reverse", "original", "token"} {
		if bytes.Contains(data, []byte(word)) {
			t.Fatal("status schema contains mapping or credential field")
		}
	}
}

type serviceAddressWriter struct{ ready chan string }

func (w serviceAddressWriter) Write(p []byte) (int, error) {
	if strings.Contains(string(p), "dmz: manual adapter at ") {
		fields := strings.Fields(string(p))
		if len(fields) > 4 {
			select {
			case w.ready <- "http://" + fields[4]:
			default:
			}
		}
	}
	return len(p), nil
}

func TestManualServiceControlsAndProviderKeyMode(t *testing.T) {
	for _, protocol := range []string{"responses", "messages"} {
		t.Run(protocol, func(t *testing.T) {
			t.Setenv("DMZ_LOCAL_TOKEN", "synthetic-local-credential-long-enough-for-test")
			t.Setenv("OPENAI_API_KEY", "synthetic-provider-key")
			t.Setenv("ANTHROPIC_API_KEY", "synthetic-provider-key")
			provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ := io.ReadAll(r.Body)
				if bytes.Contains(body, []byte("pilot@example.test")) {
					t.Error("manual service did not mask user text")
				}
				if r.Header.Get(agentgateway.TokenHeader) != "" || strings.Contains(r.Header.Get("Authorization"), "synthetic-local-") {
					t.Error("local credential escaped")
				}
				if protocol == "messages" {
					if r.Header.Get("X-Api-Key") != "synthetic-provider-key" {
						t.Error("provider API key not supplied")
					}
				} else {
					if r.Header.Get("Authorization") != "Bearer synthetic-provider-key" {
						t.Error("provider bearer key not supplied")
					}
				}
				io.Copy(w, bytes.NewReader(body))
			}))
			defer provider.Close()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			ready := make(chan string, 1)
			done := make(chan error, 1)
			go func() {
				done <- serve(ctx, []string{"--protocol", protocol, "--listen", "127.0.0.1:0", "--local-auth", "bearer", "--upstream", provider.URL + "/v1", "--allow-loopback-upstream"}, serviceAddressWriter{ready})
			}()
			var address string
			select {
			case address = <-ready:
			case err := <-done:
				t.Fatalf("service stopped before ready: %v", err)
			case <-time.After(3 * time.Second):
				t.Fatal("service startup timed out")
			}
			body := `{"input":"pilot@example.test"}`
			if protocol == "messages" {
				body = `{"messages":[{"role":"user","content":"pilot@example.test"}]}`
			}
			r, _ := http.NewRequest("POST", address+"/v1/"+protocol, strings.NewReader(body))
			r.Header.Set("Authorization", "Bearer "+os.Getenv("DMZ_LOCAL_TOKEN"))
			r.Header.Set("X-Claude-Code-Session-Id", "synthetic-manual-session")
			resp, err := http.DefaultClient.Do(r)
			if err != nil {
				t.Fatal(err)
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != 200 {
				t.Fatalf("model status %d", resp.StatusCode)
			}
			var out, errOut bytes.Buffer
			if err := control(ctx, "status", []string{"--address", address}, &out, &errOut); err != nil {
				t.Fatal(err)
			}
			var status agentgateway.Status
			if json.Unmarshal(out.Bytes(), &status) != nil || status.ChangedParts != 1 || status.Sessions != 1 {
				t.Fatal("manual status missing aggregate evidence")
			}
			out.Reset()
			if err := control(ctx, "reset", []string{"--address", address}, &out, &errOut); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), "mappings reset") {
				t.Fatal("reset feedback missing")
			}
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(4 * time.Second):
				t.Fatal("service did not stop")
			}
		})
	}
}

func TestConfigFragmentsContainNoActualCredentials(t *testing.T) {
	t.Setenv("DMZ_LOCAL_TOKEN", "synthetic-local-credential-must-not-be-exported")
	t.Setenv("OPENAI_API_KEY", "synthetic-provider-key-must-not-be-exported")
	for _, client := range []string{"codex", "claude"} {
		var out, errOut bytes.Buffer
		if err := configFragment([]string{"--address", "http://127.0.0.1:43128", client}, &out, &errOut); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "must-not-be-exported") {
			t.Fatal("configuration fragment printed a credential")
		}
		if client == "claude" && !json.Valid(out.Bytes()) {
			t.Fatal("desktop JSON fragment invalid")
		}
		if client == "codex" && (!strings.Contains(out.String(), "enable_request_compression = false") || !strings.Contains(out.String(), `env_key = "DMZ_LOCAL_TOKEN"`)) {
			t.Fatal("desktop TOML fragment missing route requirements")
		}
	}
}

func TestControlsRejectNonlocalAddresses(t *testing.T) {
	for _, address := range []string{"https://127.0.0.1:1234", "http://localhost:1234", "http://127.0.0.1.example.test", "http://127.0.0.1:1234/?token=synthetic"} {
		if err := control(context.Background(), "status", []string{"--address", address}, io.Discard, io.Discard); err == nil {
			t.Fatal("nonlocal control address accepted")
		}
	}
}
