// Package agentgateway implements the experimental device-local coding-client
// adapter. It does not change the Chromium extension's supported scope.
package agentgateway

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/AndriGitDev/dmz/packages/surrogates"
)

const TokenHeader = "X-Dmz-Token"

const (
	AuthHeader = "header"
	AuthBearer = "bearer"
	AuthAPIKey = "api-key"
)

// Config is fixed for a server's lifetime. Token belongs only to this local
// adapter and is never a provider credential. Upstream includes the API prefix.
type Config struct {
	Protocol Protocol
	Upstream string
	Token    string
	// For desktop clients, consume a local bearer/API key and replace it
	// with the provider key held in memory by this service.
	LocalAuth   string
	ProviderKey string
	// SessionHeader is an optional, verified client root-session header. A
	// launcher without one has one explicit mapping scope for its lifetime.
	SessionHeader         string
	AllowLoopbackUpstream bool // synthetic upstreams only; never arbitrary HTTP
	MaxBodyBytes          int64
	MaxScopeBytes         int64
	MaxTextBytes          int
	MaxSessions           int
	WorkerWait            time.Duration
	Warning               func(string) // fixed reason code only; may run concurrently
}

type mapping struct {
	mu      sync.Mutex
	core    *surrogates.Session
	bytes   int64
	seen    map[[32]byte]struct{}
	lastUse uint64 // protected by Server.mu
	inUse   int    // includes callers waiting on this mapping's mutex
}

// Status contains aggregate operational data, never original text or mappings.
type Status struct {
	Profile      string            `json:"profile"`
	Coverage     string            `json:"coverage"`
	Requests     uint64            `json:"requests"`
	TextParts    uint64            `json:"text_parts"`
	ChangedParts uint64            `json:"changed_parts"`
	Sessions     int               `json:"sessions"`
	Warnings     map[string]uint64 `json:"warnings"`
}

type Server struct {
	cfg     Config
	proxy   *httputil.ReverseProxy
	mu      sync.Mutex
	maps    map[string]*mapping
	status  Status
	workers chan struct{}
	tick    uint64
}

// NewServer accepts only the intended provider or an explicitly enabled
// literal loopback fixture. Request headers and paths cannot select a host.
func NewServer(cfg Config) (*Server, error) {
	if cfg.Protocol != Responses && cfg.Protocol != Messages {
		return nil, errors.New("unsupported protocol")
	}
	if len(cfg.Token) < 32 || strings.ContainsAny(cfg.Token, "\r\n") {
		return nil, errors.New("local token must contain at least 32 characters without line breaks")
	}
	if cfg.LocalAuth == "" {
		cfg.LocalAuth = AuthHeader
	}
	if cfg.LocalAuth != AuthHeader && cfg.LocalAuth != AuthBearer && cfg.LocalAuth != AuthAPIKey {
		return nil, errors.New("unsupported local authentication mode")
	}
	if cfg.LocalAuth != AuthHeader && (cfg.ProviderKey == "" || strings.ContainsAny(cfg.ProviderKey, "\r\n")) {
		return nil, errors.New("desktop authentication mode requires a provider key without line breaks")
	}
	if cfg.ProviderKey == cfg.Token {
		return nil, errors.New("local and provider credentials must differ")
	}
	if cfg.LocalAuth == AuthHeader && cfg.ProviderKey != "" {
		return nil, errors.New("provider key replacement requires explicit bearer or api-key local authentication")
	}
	u, err := url.Parse(cfg.Upstream)
	if err != nil || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" {
		return nil, errors.New("invalid upstream URL")
	}
	if !validUpstream(cfg, u) {
		return nil, errors.New("upstream must be the selected provider HTTPS endpoint or an explicitly enabled loopback fixture")
	}
	if cfg.LocalAuth != AuthHeader && u.Host == "chatgpt.com" {
		return nil, errors.New("provider API key replacement cannot use the ChatGPT backend")
	}
	if cfg.SessionHeader != "" && cfg.SessionHeader != "X-Claude-Code-Session-Id" {
		return nil, errors.New("unverified session header")
	}
	if cfg.MaxBodyBytes <= 0 {
		cfg.MaxBodyBytes = 8 << 20
	}
	if cfg.MaxScopeBytes <= 0 {
		cfg.MaxScopeBytes = 8 << 20
	}
	if cfg.MaxTextBytes <= 0 {
		cfg.MaxTextBytes = 256 << 10
	}
	if cfg.MaxSessions <= 0 {
		cfg.MaxSessions = 16
	}
	if cfg.WorkerWait <= 0 {
		cfg.WorkerWait = 30 * time.Second
	}
	s := &Server{cfg: cfg, maps: make(map[string]*mapping), workers: make(chan struct{}, 4), status: Status{Profile: "basic-experimental", Coverage: "supported-user-message-text-only", Warnings: make(map[string]uint64)}}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 120 * time.Second
	s.proxy = &httputil.ReverseProxy{
		ErrorLog: log.New(io.Discard, "", 0),
		Rewrite: func(p *httputil.ProxyRequest) {
			suffix, _ := endpoint(cfg.Protocol, p.In.URL.Path, p.In.Method)
			p.SetURL(u)
			p.Out.URL.Path = strings.TrimRight(u.Path, "/") + suffix
			p.Out.URL.RawPath = ""
			p.Out.Host = u.Host
			for key := range p.Out.Header {
				if strings.HasPrefix(strings.ToLower(key), "x-dmz-") {
					p.Out.Header.Del(key)
				}
			}
			p.Out.Header.Del("Forwarded")
			p.Out.Header.Del("X-Forwarded-For")
			p.Out.Header.Del("X-Forwarded-Host")
			p.Out.Header.Del("X-Forwarded-Proto")
			p.Out.Header.Del("Proxy-Authorization")
			p.Out.Header.Del("Cookie")
			if cfg.LocalAuth != AuthHeader {
				p.Out.Header.Del("Authorization")
				p.Out.Header.Del("X-Api-Key")
				if cfg.Protocol == Messages {
					p.Out.Header.Set("X-Api-Key", cfg.ProviderKey)
				} else {
					p.Out.Header.Set("Authorization", "Bearer "+cfg.ProviderKey)
				}
			} else {
				// A mistaken client configuration must not send the local
				// credential to the provider in its standard auth fields.
				if containsLocalCredential(p.Out.Header.Values("Authorization"), cfg.Token) {
					p.Out.Header.Del("Authorization")
					s.warn("local_credential_in_provider_field_removed")
				}
				if containsLocalCredential(p.Out.Header.Values("X-Api-Key"), cfg.Token) {
					p.Out.Header.Del("X-Api-Key")
					s.warn("local_credential_in_provider_field_removed")
				}
			}
		},
		Transport:     transport,
		FlushInterval: -1,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, _ error) {
			if r.Context().Err() != nil {
				return
			}
			s.warn("upstream_unavailable")
			http.Error(w, "dmz: upstream connection failed", http.StatusBadGateway)
		},
		ModifyResponse: func(r *http.Response) error {
			// A client following a redirect could forward its custom gateway
			// credential outside loopback. Model endpoints must not redirect.
			if r.StatusCode >= 300 && r.StatusCode < 400 && r.Header.Get("Location") != "" {
				return errors.New("upstream redirect unsupported")
			}
			return nil
		},
	}
	return s, nil
}

func validUpstream(cfg Config, u *url.URL) bool {
	if u.Host == "" || strings.Contains(u.Path, "..") || path.Clean(u.Path) != u.Path {
		return false
	}
	ip := net.ParseIP(u.Hostname())
	if cfg.AllowLoopbackUpstream && ip != nil && ip.IsLoopback() && (u.Scheme == "http" || u.Scheme == "https") {
		return true
	}
	if u.Scheme != "https" || u.Port() != "" {
		return false
	}
	if cfg.Protocol == Messages {
		return u.Host == "api.anthropic.com" && u.Path == "/v1"
	}
	return (u.Host == "api.openai.com" && u.Path == "/v1") || (u.Host == "chatgpt.com" && u.Path == "/backend-api/codex")
}

func endpoint(protocol Protocol, p, method string) (string, bool) {
	if protocol == Responses {
		if (p == "/v1/responses" || p == "/responses") && method == http.MethodPost {
			return "/responses", true
		}
		if (p == "/v1/responses/compact" || p == "/responses/compact") && method == http.MethodPost {
			return "/responses/compact", true
		}
		if (p == "/v1/models" || p == "/models") && method == http.MethodGet {
			return "/models", true
		}
	} else {
		if (p == "/v1/messages" || p == "/messages") && method == http.MethodPost {
			return "/messages", true
		}
		if (p == "/v1/messages/count_tokens" || p == "/messages/count_tokens") && method == http.MethodPost {
			return "/messages/count_tokens", true
		}
		if (p == "/v1/models" || p == "/models") && method == http.MethodGet {
			return "/models", true
		}
	}
	return "", false
}

func (s *Server) warn(code string) {
	s.mu.Lock()
	s.status.Warnings[code]++
	s.mu.Unlock()
	if s.cfg.Warning != nil {
		s.cfg.Warning(code)
	}
}

func (s *Server) Snapshot() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.status
	status.Sessions = len(s.maps)
	status.Warnings = make(map[string]uint64, len(s.status.Warnings))
	for k, v := range s.status.Warnings {
		status.Warnings[k] = v
	}
	return status
}

// ClearMappings drops the service's references to live originals. It is used
// after the owning app session stops; counters remain available for review.
func (s *Server) ClearMappings() { s.mu.Lock(); s.maps = make(map[string]*mapping); s.mu.Unlock() }

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// DNS rebinding and browser callers are outside this local client API.
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host != "127.0.0.1" && host != "[::1]" && host != "::1" {
		http.Error(w, "dmz: loopback host required", http.StatusForbidden)
		return
	}
	if r.Header.Get("Origin") != "" || r.Method == http.MethodOptions {
		http.Error(w, "dmz: browser requests unsupported", http.StatusForbidden)
		return
	}
	if !s.authenticated(r) {
		http.Error(w, "dmz: local authentication required", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	if r.URL.Path == "/_dmz/status" && r.Method == http.MethodGet {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(s.Snapshot())
		return
	}
	if r.URL.Path == "/_dmz/reset" && r.Method == http.MethodPost {
		s.ClearMappings()
		s.warn("mapping_reset_history_may_degrade")
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, ok := endpoint(s.cfg.Protocol, r.URL.Path, r.Method); !ok {
		http.Error(w, "dmz: unsupported endpoint or method", http.StatusNotFound)
		return
	}
	if r.URL.RawPath != "" || r.URL.IsAbs() || r.Header.Get("Upgrade") != "" {
		http.Error(w, "dmz: unsupported request transport", http.StatusBadRequest)
		return
	}
	s.mu.Lock()
	s.status.Requests++
	s.mu.Unlock()
	if r.Method == http.MethodPost {
		s.prepare(r)
	}
	// A cancelled caller is no longer asking us to submit a draft. In
	// particular, do not invoke a transport with an unprocessed body after
	// cancellation interrupted waiting for a masking slot.
	if r.Context().Err() != nil {
		return
	}
	s.proxy.ServeHTTP(w, r)
}

func (s *Server) prepare(r *http.Request) {
	coding := requestCoding(r.Header)
	if coding == codingUnsupported {
		s.warn(warnEncodingUnsupported)
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			s.warn("unsupported_content_type_forwarded_unchanged")
			return
		}
	}
	timer := time.NewTimer(s.cfg.WorkerWait)
	defer timer.Stop()
	select {
	case s.workers <- struct{}{}:
		defer func() { <-s.workers }()
	case <-r.Context().Done():
		return
	case <-timer.C:
		s.warn("masking_busy_forwarded_unchanged")
		return
	}
	// The wire body is bounded before any decoding; decoded output has the
	// same bound (decodeBody).
	wire, err := io.ReadAll(io.LimitReader(r.Body, s.cfg.MaxBodyBytes+1))
	if err != nil {
		// Bytes already consumed still belong to the outgoing request. Keep
		// the underlying reader (and its error) rather than sending a prefix.
		r.Body = &joinedBody{Reader: io.MultiReader(bytes.NewReader(wire), r.Body), closer: r.Body}
		s.warn("body_read_failed_forwarded_unchanged")
		return
	}
	if int64(len(wire)) > s.cfg.MaxBodyBytes {
		r.Body = &joinedBody{Reader: io.MultiReader(bytes.NewReader(wire), r.Body), closer: r.Body}
		s.warn("oversized_body_forwarded_unchanged")
		return
	}
	_ = r.Body.Close()
	// Until a change is made, the original wire bytes and headers (including
	// Content-Encoding and digests) are what gets forwarded.
	setBody(r, wire)
	body := wire
	if coding != codingIdentity {
		decoded, code := decodeBody(coding, wire, s.cfg.MaxBodyBytes)
		if code != "" {
			s.warn(code)
			return
		}
		body = decoded
	}
	key := "launch"
	if s.cfg.SessionHeader != "" {
		key = r.Header.Get(s.cfg.SessionHeader)
		if len(key) == 0 || len(key) > 128 || len(r.Header.Values(s.cfg.SessionHeader)) != 1 {
			s.warn("missing_session_identity_forwarded_unchanged")
			return
		}
	}
	s.mu.Lock()
	m := s.maps[key]
	evicted := false
	if m == nil && len(s.maps) >= s.cfg.MaxSessions {
		var oldestKey string
		var oldest *mapping
		for candidateKey, candidate := range s.maps {
			if candidate.inUse == 0 && (oldest == nil || candidate.lastUse < oldest.lastUse) {
				oldestKey = candidateKey
				oldest = candidate
			}
		}
		if oldest != nil {
			delete(s.maps, oldestKey)
			evicted = true
		}
	}
	if m == nil && len(s.maps) < s.cfg.MaxSessions {
		m = &mapping{core: surrogates.NewSession(), seen: make(map[[32]byte]struct{})}
		s.maps[key] = m
	}
	if m != nil {
		s.tick++
		m.lastUse = s.tick
		m.inUse++
	}
	s.mu.Unlock()
	if evicted {
		s.warn("mapping_evicted_history_may_degrade")
	}
	if m == nil {
		s.warn("session_capacity_forwarded_unchanged")
		return
	}
	defer func() { s.mu.Lock(); m.inUse--; s.mu.Unlock() }()
	m.mu.Lock()
	defer m.mu.Unlock()
	transformed, report := Transform(s.cfg.Protocol, body, func(input string) (output string, err error) {
		defer func() {
			if recover() != nil {
				output = input
				err = errors.New("masking unavailable")
			}
		}()
		if len(input) > s.cfg.MaxTextBytes {
			s.warn("oversized_text_forwarded_unchanged")
			return input, errors.New("text capacity reached")
		}
		digest := sha256.Sum256([]byte(input))
		if _, known := m.seen[digest]; !known {
			// Count unique text rather than repeated history. The small digest
			// set stores no text; reverse mappings stay in the core only.
			if m.bytes+int64(len(input)) > s.cfg.MaxScopeBytes || len(m.seen) >= 32768 {
				s.warn("scope_capacity_forwarded_unchanged")
				return input, errors.New("scope capacity reached")
			}
			m.bytes += int64(len(input))
			m.seen[digest] = struct{}{}
		}
		result := m.core.ProcessBestEffort(input, nil)
		for _, finding := range result.Findings {
			if finding.Reason == "whole-text-coverage" {
				s.warn("whole_text_masked_context_may_degrade")
				break
			}
		}
		if !result.Allowed {
			return input, errors.New("masking unavailable")
		}
		return result.Transformed, nil
	})
	if report.ChangedParts > 0 {
		// A changed decoded body is sent as identity JSON: re-encoding would
		// add a second compressor implementation for no masking benefit.
		setBody(r, transformed)
		if coding != codingIdentity {
			r.Header.Del("Content-Encoding")
		}
		// Entity-body digests describe the old bytes and cannot be forwarded.
		for _, h := range []string{"Content-MD5", "Digest", "Content-Digest", "Repr-Digest"} {
			r.Header.Del(h)
		}
	}
	s.mu.Lock()
	s.status.TextParts += uint64(report.TextParts)
	s.status.ChangedParts += uint64(report.ChangedParts)
	s.mu.Unlock()
	for _, code := range report.Warnings {
		s.warn(code)
	}
}

func setBody(r *http.Request, body []byte) {
	r.Body = io.NopCloser(bytes.NewReader(body))
	r.ContentLength = int64(len(body))
	r.GetBody = nil
}

func (s *Server) authenticated(r *http.Request) bool {
	value := ""
	if len(r.Header.Values(TokenHeader)) == 1 {
		value = r.Header.Get(TokenHeader)
	} else if len(r.Header.Values(TokenHeader)) > 1 {
		return false
	} else {
		switch s.cfg.LocalAuth {
		case AuthBearer:
			if len(r.Header.Values("Authorization")) != 1 {
				return false
			}
			value = bearerCredential(r.Header.Get("Authorization"))
		case AuthAPIKey:
			if len(r.Header.Values("X-Api-Key")) != 1 {
				return false
			}
			value = r.Header.Get("X-Api-Key")
		}
	}
	return subtle.ConstantTimeCompare([]byte(value), []byte(s.cfg.Token)) == 1
}

func bearerCredential(value string) string {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return ""
	}
	return parts[1]
}

func containsLocalCredential(values []string, token string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == token || bearerCredential(value) == token {
			return true
		}
	}
	return false
}

type joinedBody struct {
	io.Reader
	closer io.Closer
}

func (b *joinedBody) Close() error { return b.closer.Close() }
