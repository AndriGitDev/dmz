package agentgateway

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type nativeAuthTransport func(*http.Request) (*http.Response, error)

func (f nativeAuthTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestNativeCredentialsSelectOnlyFixedOpenAIOrigins(t *testing.T) {
	for _, account := range []string{"", "synthetic-account"} {
		t.Run(account, func(t *testing.T) {
			s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: syntheticToken, NativeOpenAIAuth: true})
			if err != nil {
				t.Fatal(err)
			}
			observed := false
			s.proxy.Transport = nativeAuthTransport(func(r *http.Request) (*http.Response, error) {
				observed = true
				host, path := "api.openai.com", "/v1/responses"
				if account != "" {
					host, path = "chatgpt.com", "/backend-api/codex/responses"
				}
				if r.URL.Scheme != "https" || r.URL.Host != host || r.Host != host || r.URL.Path != path {
					t.Fatal("unapproved native auth destination")
				}
				if r.Header.Get("Authorization") != "Bearer synthetic-native-credential" || r.Header.Get(TokenHeader) != "" {
					t.Fatal("native credential boundary changed")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
			})
			r := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"input":"pilot@example.test"}`))
			r.Host = "127.0.0.1:1234"
			r.Header.Set(TokenHeader, syntheticToken)
			r.Header.Set("Authorization", "Bearer synthetic-native-credential")
			if account != "" {
				r.Header.Set("ChatGPT-Account-ID", account)
			}
			r.Header.Set("X-Upstream-URL", "https://outside.example.test")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != 200 || !observed {
				t.Fatal("native route not observed")
			}
		})
	}
	for _, cfg := range []Config{
		{Protocol: Messages, Upstream: "https://api.anthropic.com/v1", Token: syntheticToken, NativeOpenAIAuth: true},
		{Protocol: Responses, Upstream: "https://chatgpt.com/backend-api/codex", Token: syntheticToken, NativeOpenAIAuth: true},
		{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: syntheticToken, NativeOpenAIAuth: true, LocalAuth: AuthBearer, ProviderKey: "synthetic-provider-key"},
	} {
		if _, err := NewServer(cfg); err == nil {
			t.Fatal("unsupported native auth configuration accepted")
		}
	}
}

func TestNativeEnvironmentKeyFallbackDoesNotOverrideClientAuth(t *testing.T) {
	for _, nativeAuth := range []string{"", "Bearer synthetic-native-account"} {
		t.Run(nativeAuth, func(t *testing.T) {
			s, err := NewServer(Config{Protocol: Responses, Upstream: "https://api.openai.com/v1", Token: syntheticToken, NativeOpenAIAuth: true})
			if err != nil {
				t.Fatal(err)
			}
			s.proxy.Transport = nativeAuthTransport(func(r *http.Request) (*http.Response, error) {
				expected, host := "Bearer synthetic-environment-key", "api.openai.com"
				if nativeAuth != "" {
					expected, host = nativeAuth, "chatgpt.com"
				}
				if r.Header.Get("Authorization") != expected || r.URL.Host != host || r.Header.Get("X-Dmz-OpenAI-Key") != "" || r.Header.Get(TokenHeader) != "" {
					t.Fatal("environment fallback changed native auth or leaked local header")
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
			})
			r := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{"input":"synthetic"}`))
			r.Host = "127.0.0.1:1234"
			r.Header.Set(TokenHeader, syntheticToken)
			r.Header.Set("Authorization", nativeAuth)
			r.Header.Set("ChatGPT-Account-ID", "synthetic-account")
			r.Header.Set("X-Dmz-OpenAI-Key", "synthetic-environment-key")
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code != 200 || s.Snapshot().NativeCredentialMissing {
				t.Fatal("native auth fallback failed")
			}
		})
	}
}
