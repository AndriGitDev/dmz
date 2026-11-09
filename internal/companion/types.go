// Package companion provides the device-local DMZ control window. It does not
// collect conversation text or send app analytics.
package companion

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"runtime"
	"time"
)

// Ticket contains scoped local credentials only, never a provider key or a
// prompt. The connector consumes and deletes it before starting the client.
type Ticket struct {
	Version      int       `json:"version"`
	SessionID    string    `json:"session_id"`
	Client       string    `json:"client"`
	ClientPath   string    `json:"client_path"`
	Project      string    `json:"project"`
	Auth         string    `json:"auth"`
	Arguments    []string  `json:"arguments,omitempty"`
	GatewayURL   string    `json:"gateway_url"`
	GatewayToken string    `json:"gateway_token"`
	ControlURL   string    `json:"control_url"`
	ControlToken string    `json:"control_token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", errors.New("local_token_unavailable")
	}
	return hex.EncodeToString(b), nil
}

func localOrigin(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "http" && u.Hostname() == "127.0.0.1" && u.Port() != "" && u.User == nil && u.RawQuery == "" && u.Fragment == "" && (u.Path == "" || u.Path == "/")
}

// ReadTicket deliberately accepts a private regular file, not JSON on argv.
func ReadTicket(name string) (Ticket, error) {
	var t Ticket
	f, err := os.Open(name)
	if err != nil {
		return t, errors.New("session_ticket_unavailable")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > 16384 || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return t, errors.New("session_ticket_not_private")
	}
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if d.Decode(&t) != nil {
		return Ticket{}, errors.New("session_ticket_invalid")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return Ticket{}, errors.New("session_ticket_invalid")
	}
	if t.Version != 1 || (t.Client != "codex" && t.Client != "claude") || (t.Auth != "api" && t.Auth != "account") || len(t.GatewayToken) != 64 || len(t.ControlToken) != 64 || t.SessionID == "" || !localOrigin(t.GatewayURL) || !localOrigin(t.ControlURL) || time.Now().After(t.ExpiresAt) {
		return Ticket{}, errors.New("session_ticket_invalid")
	}
	if err := f.Close(); err != nil {
		return Ticket{}, errors.New("session_ticket_unavailable")
	}
	if err := os.Remove(name); err != nil {
		return Ticket{}, errors.New("session_ticket_cannot_be_consumed")
	}
	return t, nil
}
