// Package agentgateway implements the experimental device-local coding-client
// adapter. It does not change the Chromium extension's supported scope.
package agentgateway

import (
	"bytes"
	"context"
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
	"sort"
	"strings"
	"sync"
	"sync/atomic"
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
	// NativeOpenAIAuth keeps client-owned credentials and selects only between
	// the OpenAI API and ChatGPT Codex backend using the native account header.
	// It is restricted to Responses/header-auth on the standard API origin.
	NativeOpenAIAuth bool
	// RestoreReplies enables local assistant-text restoration together with
	// exact-value re-masking of restored assistant history on later requests.
	RestoreReplies    bool
	HighlightRestored bool // optional bold Markdown, off by default
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
	mu        sync.Mutex
	core      *surrogates.Session
	bytes     int64
	seen      map[[32]byte]struct{}
	lastUse   uint64 // protected by Server.mu
	inUse     int    // includes callers waiting on this mapping's mutex
	invalid   atomic.Bool
	restored  map[string]string // known originals from this live mapping scope
	noRestore map[string]bool   // whole-message fallbacks are not safe entity values
	replay    *strings.Replacer
	warning   func(string)
}

// Status contains aggregate operational data, never original text or mappings.
type Status struct {
	NewRedactions           uint64            `json:"new_redactions"`
	Activity                []Activity        `json:"activity"`
	Profile                 string            `json:"profile"`
	Coverage                string            `json:"coverage"`
	Requests                uint64            `json:"requests"`
	ModelRequests           uint64            `json:"model_requests"`
	TextParts               uint64            `json:"text_parts"`
	ChangedParts            uint64            `json:"changed_parts"`
	Sessions                int               `json:"sessions"`
	NativeCredentialMissing bool              `json:"native_credential_missing,omitempty"`
	Warnings                map[string]uint64 `json:"warnings"`
	// RecentWarnings contains attention codes seen in the last two minutes.
	// Warnings above retain lifetime counts, including routine scope notices.
	RecentWarnings []string `json:"recent_warnings"`
}

// Activity describes local processing only, never delivery or complete coverage.
// No request identifiers, text, paths, credentials or marker values are retained.
type Activity struct {
	At            time.Time `json:"at"`
	Outcome       string    `json:"outcome"`
	NewRedactions uint64    `json:"new_redactions"`
	ChangedParts  uint64    `json:"changed_parts"`
}

type Server struct {
	highlightRestored atomic.Bool
	cfg               Config
	proxy             *httputil.ReverseProxy
	mu                sync.Mutex
	maps              map[string]*mapping
	status            Status
	workers           chan struct{}
	replySlots        chan struct{}
	warningTimes      map[string]time.Time
	tick              uint64
}

// NewServer accepts only the intended provider or an explicitly enabled
// literal loopback fixture. NativeOpenAIAuth can choose only the two fixed
// OpenAI origins; no request can select an arbitrary host.
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
	if cfg.NativeOpenAIAuth && (cfg.Protocol != Responses || cfg.LocalAuth != AuthHeader || (cfg.Upstream != "https://api.openai.com/v1" && !(cfg.AllowLoopbackUpstream && u.Hostname() == "127.0.0.1"))) {
		return nil, errors.New("native OpenAI authentication requires the standard Responses header route")
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
	s := &Server{cfg: cfg, maps: make(map[string]*mapping), workers: make(chan struct{}, 4), replySlots: make(chan struct{}, 4), warningTimes: make(map[string]time.Time), status: Status{Profile: "basic-experimental", Coverage: "supported-user-message-text-only", Warnings: make(map[string]uint64)}}
	s.highlightRestored.Store(cfg.HighlightRestored)
	if cfg.RestoreReplies {
		s.status.Coverage = "supported-user-text-and-restored-reply-values"
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 120 * time.Second
	s.proxy = &httputil.ReverseProxy{
		ErrorLog: log.New(io.Discard, "", 0),
		Rewrite: func(p *httputil.ProxyRequest) {
			suffix, _ := endpoint(cfg.Protocol, p.In.URL.Path, p.In.Method)
			target := u
			fallbackKey := ""
			if cfg.NativeOpenAIAuth && strings.TrimSpace(p.In.Header.Get("Authorization")) == "" && len(p.In.Header.Values("X-Dmz-OpenAI-Key")) == 1 {
				fallbackKey = strings.TrimSpace(p.In.Header.Get("X-Dmz-OpenAI-Key"))
			}
			if cfg.NativeOpenAIAuth && u.Host == "api.openai.com" && fallbackKey == "" && p.In.Header.Get("ChatGPT-Account-ID") != "" {
				target = &url.URL{Scheme: "https", Host: "chatgpt.com", Path: "/backend-api/codex"}
			}
			p.SetURL(target)
			p.Out.URL.Path = strings.TrimRight(target.Path, "/") + suffix
			p.Out.URL.RawPath = ""
			p.Out.Host = target.Host
			if cfg.RestoreReplies && p.In.Context().Value(replyScopeKey{}) != nil {
				p.Out.Header.Set("Accept-Encoding", "identity")
			}
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
			if fallbackKey != "" {
				p.Out.Header.Set("Authorization", "Bearer "+fallbackKey)
				p.Out.Header.Del("ChatGPT-Account-ID")
			}
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
			if cfg.RestoreReplies {
				s.restoreResponse(r)
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
	s.warningTimes[code] = time.Now()
	s.mu.Unlock()
	if s.cfg.Warning != nil {
		s.cfg.Warning(code)
	}
}

func (s *Server) Snapshot() Status {
	s.mu.Lock()
	defer s.mu.Unlock()
	status := s.status
	status.Activity = append([]Activity{}, s.status.Activity...)
	status.Sessions = len(s.maps)
	status.Warnings = make(map[string]uint64, len(s.status.Warnings))
	status.RecentWarnings = []string{}
	for k, v := range s.status.Warnings {
		status.Warnings[k] = v
		if !informationalWarning(k) && time.Since(s.warningTimes[k]) < 2*time.Minute {
			status.RecentWarnings = append(status.RecentWarnings, k)
		}
	}
	sort.Strings(status.RecentWarnings)
	return status
}

func informationalWarning(code string) bool {
	switch code {
	case "instructions_unchanged", "tool_result_unchanged", WarnToolResultDetectionNotApplied, "attachment_unchanged", "no_user_text", "mapping_evicted_history_may_degrade", "reply_content_type_inferred_sse", "reply_content_type_inferred_json":
		return true
	}
	return false
}

// ClearMappings drops the service's references to live originals. It is used
// after the owning app session stops; counters remain available for review.
func (s *Server) ClearMappings() {
	s.mu.Lock()
	for _, m := range s.maps {
		m.invalid.Store(true)
	}
	s.maps = make(map[string]*mapping)
	s.mu.Unlock()
}

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
	if r.Method == http.MethodPost {
		s.status.ModelRequests++
		if s.cfg.NativeOpenAIAuth {
			s.status.NativeCredentialMissing = strings.TrimSpace(r.Header.Get("Authorization")) == "" && strings.TrimSpace(r.Header.Get("X-Dmz-OpenAI-Key")) == ""
		}
	}
	s.mu.Unlock()
	if r.Method == http.MethodPost {
		if scope := s.prepare(r); scope != nil {
			defer func() { s.mu.Lock(); scope.inUse--; s.mu.Unlock() }()
			*r = *r.WithContext(context.WithValue(r.Context(), replyScopeKey{}, scope))
		}
	}
	// A cancelled caller is no longer asking us to submit a draft. In
	// particular, do not invoke a transport with an unprocessed body after
	// cancellation interrupted waiting for a masking slot.
	if r.Context().Err() != nil {
		return
	}
	s.proxy.ServeHTTP(w, r)
}

func (s *Server) prepare(r *http.Request) (scope *mapping) {
	var newRedactions, changedParts uint64
	outcome := "processing_skipped"
	defer func() {
		if r.Context().Err() != nil {
			outcome = "cancelled"
		}
		s.mu.Lock()
		s.status.NewRedactions += newRedactions
		s.status.Activity = append(s.status.Activity, Activity{At: time.Now().UTC(), Outcome: outcome, NewRedactions: newRedactions, ChangedParts: changedParts})
		if len(s.status.Activity) > 20 {
			s.status.Activity = append([]Activity(nil), s.status.Activity[len(s.status.Activity)-20:]...)
		}
		s.mu.Unlock()
	}()
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
			oldest.invalid.Store(true)
			delete(s.maps, oldestKey)
			evicted = true
		}
	}
	if m == nil && len(s.maps) < s.cfg.MaxSessions {
		m = &mapping{core: surrogates.NewSession(), seen: make(map[[32]byte]struct{}), warning: s.warn}
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
	defer func() {
		if scope == nil {
			s.mu.Lock()
			m.inUse--
			s.mu.Unlock()
		}
	}()
	m.mu.Lock()
	defer m.mu.Unlock()
	before := m.core.MappingCount()
	defer func() { newRedactions = uint64(m.core.MappingCount() - before) }()
	transformed, report := Transform(s.cfg.Protocol, body, func(input string) (output string, err error) {
		knownMasked := input
		defer func() {
			if recover() != nil {
				if knownMasked != input {
					output, err = knownMasked, nil
					s.warn("masking_failed_known_values_remasked")
				} else {
					output = input
					err = errors.New("masking unavailable")
				}
			}
		}()
		if s.cfg.RestoreReplies {
			knownMasked = m.remaskRestored(input)
		}
		fallback := func(reason error) (string, error) {
			// Even when new detection is at capacity, keep already-known
			// originals masked if this supported field can still be rewritten.
			if knownMasked != input {
				return knownMasked, nil
			}
			return input, reason
		}
		if len(input) > s.cfg.MaxTextBytes {
			if knownMasked != input {
				s.warn("oversized_text_detection_skipped")
			} else {
				s.warn("oversized_text_forwarded_unchanged")
			}
			return fallback(errors.New("text capacity reached"))
		}
		digest := sha256.Sum256([]byte(input))
		if _, known := m.seen[digest]; !known {
			// Count unique text rather than repeated history. The small digest
			// set stores no text; reverse mappings stay in the core only.
			if m.bytes+int64(len(input)) > s.cfg.MaxScopeBytes || len(m.seen) >= 32768 {
				if knownMasked != input {
					s.warn("scope_capacity_detection_skipped")
				} else {
					s.warn("scope_capacity_forwarded_unchanged")
				}
				return fallback(errors.New("scope capacity reached"))
			}
			m.bytes += int64(len(input))
			m.seen[digest] = struct{}{}
		}
		// Detection must see the raw boundaries: a known value can be part
		// of a larger identifier (for example, a kennitala inside an IBAN).
		// Exact-value replay masking runs on the detected result below.
		result := m.core.ProcessBestEffort(input, nil)
		for _, finding := range result.Findings {
			if finding.Reason == "whole-text-coverage" {
				if s.cfg.RestoreReplies {
					if m.noRestore == nil {
						m.noRestore = make(map[string]bool)
					}
					m.noRestore[result.Transformed] = true
				}
				s.warn("whole_text_masked_context_may_degrade")
				break
			}
		}
		if !result.Allowed {
			return fallback(errors.New("masking unavailable"))
		}
		if s.cfg.RestoreReplies {
			m.rememberReplyValues(result.Transformed)
			// Compaction and sub-agent results may re-enter as user text
			// without the context that originally identified a sensitive value.
			// Exact known values must stay masked independently of detection.
			return m.remaskRestored(result.Transformed), nil
		}
		return result.Transformed, nil
	})
	replayReport := TransformReport{}
	if s.cfg.RestoreReplies {
		transformed, replayReport = TransformReplyHistory(s.cfg.Protocol, transformed, func(text string) (string, error) { return m.remaskRestored(text), nil })
	}
	changedParts = uint64(report.ChangedParts + replayReport.ChangedParts)
	outcome = "no_changes"
	if changedParts > 0 {
		outcome = "masked"
	}
	for _, code := range append(report.Warnings, replayReport.Warnings...) {
		if IsFailOpenWarning(code) {
			outcome = "processing_skipped"
			break
		}
	}
	if report.ChangedParts > 0 || replayReport.ChangedParts > 0 {
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
	s.status.TextParts += uint64(report.TextParts + replayReport.TextParts)
	s.status.ChangedParts += uint64(report.ChangedParts + replayReport.ChangedParts)
	s.mu.Unlock()
	codes := append(report.Warnings, replayReport.Warnings...)
	seenCodes := make(map[string]bool)
	for _, code := range codes {
		if s.cfg.RestoreReplies && code == WarnToolResultUnchanged {
			code = WarnToolResultDetectionNotApplied
		}
		if seenCodes[code] {
			continue
		}
		seenCodes[code] = true
		s.warn(code)
	}
	if s.cfg.RestoreReplies {
		scope = m
	}
	return
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
