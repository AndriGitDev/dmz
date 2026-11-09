package companion

import (
	"bytes"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

//go:embed web/*
var assets embed.FS

func decode(r *http.Request, dest any) error {
	body, err := io.ReadAll(io.LimitReader(r.Body, 16385))
	if err != nil || len(body) > 16384 {
		return errors.New("invalid_request")
	}
	d := json.NewDecoder(bytes.NewReader(body))
	d.DisallowUnknownFields()
	if err := d.Decode(dest); err != nil {
		return errors.New("invalid_request")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("invalid_request")
	}
	return nil
}
func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; connect-src 'self'; img-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'none'")
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil || host != "127.0.0.1" {
		http.Error(w, "not_allowed", 403)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		http.Error(w, "not_allowed", 403)
		return
	}
	if r.URL.Path == "/_dmz/identity" {
		if r.Method != "POST" || r.Header.Get("Origin") != "" {
			http.Error(w, "not_allowed", 403)
			return
		}
		var req struct {
			Challenge string `json:"challenge"`
		}
		if decode(r, &req) != nil || len(req.Challenge) != 64 {
			http.Error(w, "invalid_request", 400)
			return
		}
		writeJSON(w, map[string]string{"proof": instanceProof(a.token, req.Challenge)})
		return
	}
	if a.sessionControl(w, r) {
		return
	}
	if r.URL.Path == "/api/bootstrap" && r.Method == "POST" {
		var req struct {
			Code string `json:"code"`
		}
		if decode(r, &req) != nil {
			http.Error(w, "invalid_request", 400)
			return
		}
		a.mu.Lock()
		expiry, ok := a.windowCodes[req.Code]
		delete(a.windowCodes, req.Code)
		a.mu.Unlock()
		if !ok || time.Now().After(expiry) {
			http.Error(w, "not_authorized", 401)
			return
		}
		writeJSON(w, map[string]string{"token": a.token})
		return
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		if r.Method != "GET" {
			http.Error(w, "method_not_allowed", 405)
			return
		}
		name := map[string]string{"/": "web/index.html", "/app.js": "web/app.js", "/style.css": "web/style.css", "/icon.svg": "web/icon.svg"}[r.URL.Path]
		if name == "" {
			http.NotFound(w, r)
			return
		}
		body, err := assets.ReadFile(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", map[string]string{"/": "text/html; charset=utf-8", "/app.js": "text/javascript; charset=utf-8", "/style.css": "text/css; charset=utf-8", "/icon.svg": "image/svg+xml"}[r.URL.Path])
		w.Write(body)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+a.token)) != 1 {
		http.Error(w, "not_authorized", 401)
		return
	}
	switch {
	case r.URL.Path == "/api/window" && r.Method == "POST":
		address, err := a.windowAddress()
		if err != nil {
			http.Error(w, "window_unavailable", 503)
			return
		}
		writeJSON(w, map[string]string{"address": address})
	case r.URL.Path == "/api/state" && r.Method == "GET":
		clients := make([]map[string]any, 0, len(a.cfg.Clients))
		for _, c := range a.cfg.Clients {
			env := "OPENAI_API_KEY"
			if c.ID == "claude" {
				env = "ANTHROPIC_API_KEY"
			}
			clients = append(clients, map[string]any{"id": c.ID, "name": c.Name, "available": c.Available, "api_key_available": a.cfg.Environment(env) != ""})
		}
		writeJSON(w, map[string]any{"app": "dmz-companion", "clients": clients, "sessions": a.Views(), "background_enabled": a.cfg.Background != nil, "background": a.BackgroundViews(), "highlight_restored": a.highlightRestored(), "profile": "basic", "desktop_qualified": false})
	case r.URL.Path == "/api/preferences" && r.Method == "POST":
		var req struct {
			HighlightRestored *bool `json:"highlight_restored"`
		}
		if decode(r, &req) != nil || req.HighlightRestored == nil {
			http.Error(w, "invalid_request", 400)
			return
		}
		a.mu.Lock()
		background := a.background
		a.mu.Unlock()
		if background == nil {
			http.Error(w, "background_unavailable", 409)
			return
		}
		if background.setHighlightRestored(*req.HighlightRestored) != nil {
			http.Error(w, "preferences_unavailable", 503)
			return
		}
		w.WriteHeader(204)
	case r.URL.Path == "/api/background" && r.Method == "POST":
		var req struct {
			Client    string `json:"client"`
			Connected bool   `json:"connected"`
		}
		if decode(r, &req) != nil {
			http.Error(w, "invalid_request", 400)
			return
		}
		a.mu.Lock()
		background := a.background
		a.mu.Unlock()
		if background == nil {
			http.Error(w, "background_unavailable", 409)
			return
		}
		if err := background.SetConnected(req.Client, req.Connected); err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		w.WriteHeader(204)
	case r.URL.Path == "/api/start" && r.Method == "POST":
		var req StartRequest
		if decode(r, &req) != nil {
			http.Error(w, "invalid_request", 400)
			return
		}
		view, err := a.Start(req)
		if err != nil {
			writeJSON(w, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, view)
	case r.URL.Path == "/api/stop" && r.Method == "POST":
		var req struct {
			ID string `json:"id"`
		}
		if decode(r, &req) != nil {
			http.Error(w, "invalid_request", 400)
			return
		}
		a.stop(req.ID, "")
		w.WriteHeader(204)
	case r.URL.Path == "/api/folder" && r.Method == "POST":
		ctx, cancel := context.WithCancel(r.Context())
		stop := context.AfterFunc(a.helperCtx, cancel)
		defer stop()
		defer cancel()
		folder, err := a.cfg.ChooseDirectory(ctx)
		if errors.Is(err, ErrDialogCancelled) {
			writeJSON(w, map[string]bool{"cancelled": true})
			return
		}
		if err != nil {
			writeJSON(w, map[string]string{"error": "folder_picker_unavailable"})
			return
		}
		writeJSON(w, map[string]string{"folder": folder})
	case r.URL.Path == "/api/quit" && r.Method == "POST":
		w.WriteHeader(204)
		if flush, ok := w.(http.Flusher); ok {
			flush.Flush()
		}
		a.quitOnce.Do(func() { close(a.quit) })
	default:
		http.NotFound(w, r)
	}
}

// Serve opens the local app window. A private control file is for automated
// development checks only; normal users never see tokens or command lines.
func (a *App) Serve(ctx context.Context, open bool, controlFile string) error {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return errors.New("app_window_unavailable")
	}
	a.mu.Lock()
	a.origin = "http://" + listener.Addr().String()
	origin := a.origin
	a.mu.Unlock()
	if controlFile == "" {
		controlFile = a.cfg.InstanceFile
	}
	if controlFile != "" {
		f, err := os.OpenFile(controlFile, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			listener.Close()
			if errors.Is(err, os.ErrExist) && a.cfg.InstanceFile == controlFile {
				ok, reopenErr := Reopen(ctx, controlFile, a.cfg.OpenWindow)
				if ok || reopenErr != nil {
					return reopenErr
				}
			}
			return errors.New("control_file_unavailable")
		}
		err = json.NewEncoder(f).Encode(instanceDescriptor{Origin: origin, Token: a.token, PID: os.Getpid()})
		f.Close()
		if err != nil {
			listener.Close()
			_ = os.Remove(controlFile)
			return errors.New("control_file_unavailable")
		}
		defer removeOwnInstance(controlFile, a.token)
	}
	server := &http.Server{Handler: a, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: time.Minute, ErrorLog: log.New(io.Discard, "", 0)}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	defer func() { a.Close(); _ = server.Close() }()
	if a.cfg.Background != nil {
		background, err := newBackground(*a.cfg.Background)
		if err != nil {
			return err
		}
		a.mu.Lock()
		a.background = background
		a.mu.Unlock()
		go func() {
			ticker := time.NewTicker(5 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-a.helperCtx.Done():
					return
				case <-ticker.C:
					background.Refresh()
				}
			}
		}()
	}
	if open {
		address, err := a.windowAddress()
		if err != nil {
			return err
		}
		if err := a.cfg.OpenWindow(address); err != nil {
			return errors.New("app_window_unavailable")
		}
	}
	select {
	case <-ctx.Done():
		return nil
	case <-a.quit:
		return nil
	case <-done:
		return errors.New("app_window_closed")
	}
}

func (a *App) windowAddress() (string, error) {
	code, err := randomToken()
	if err != nil {
		return "", err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for old, expiry := range a.windowCodes {
		if time.Now().After(expiry) {
			delete(a.windowCodes, old)
		}
	}
	if len(a.windowCodes) >= 8 {
		return "", errors.New("too_many_windows")
	}
	a.windowCodes[code] = time.Now().Add(time.Minute)
	return a.origin + "/#open=" + code, nil
}
