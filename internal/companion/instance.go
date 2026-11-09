package companion

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

type instanceDescriptor struct {
	Origin string `json:"origin"`
	Token  string `json:"token"`
}

func DefaultInstanceFile() (string, error) {
	root, err := os.UserCacheDir()
	if err != nil {
		return "", errors.New("app_cache_unavailable")
	}
	dir := filepath.Join(root, "dmz-companion")
	if os.MkdirAll(dir, 0700) != nil || os.Chmod(dir, 0700) != nil {
		return "", errors.New("app_cache_unavailable")
	}
	return filepath.Join(dir, "instance.json"), nil
}

// TryReopen uses only DMZ's private local descriptor, never another client's
// settings or authentication store. Stale descriptors carry no provider key.
func TryReopen(name string, open func(string) error) (bool, error) {
	f, err := os.Open(name)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("app_state_unavailable")
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		f.Close()
		return false, errors.New("app_state_unavailable")
	}
	body, err := io.ReadAll(io.LimitReader(f, 4097))
	f.Close()
	if err != nil || len(body) > 4096 {
		return false, errors.New("app_state_unavailable")
	}
	var d instanceDescriptor
	if json.Unmarshal(body, &d) != nil {
		if time.Since(info.ModTime()) < 5*time.Second {
			return false, errors.New("app_is_starting")
		}
		_ = os.Remove(name)
		return false, nil
	}
	if !localOrigin(d.Origin) || len(d.Token) != 64 {
		_ = os.Remove(name)
		return false, nil
	}
	r, _ := http.NewRequest("POST", d.Origin+"/api/window", bytes.NewReader([]byte("{}")))
	r.Header.Set("Authorization", "Bearer "+d.Token)
	r.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("local_redirect_unsupported") }}
	response, err := client.Do(r)
	if err != nil {
		var network *net.OpError
		if errors.As(err, &network) && network.Op == "dial" {
			removeOwnInstance(name, d.Token)
			return false, nil
		}
		return false, errors.New("app_is_starting")
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		if response.StatusCode == 401 || response.StatusCode == 404 {
			removeOwnInstance(name, d.Token)
			return false, nil
		}
		return false, errors.New("app_is_busy")
	}
	var result struct {
		Address string `json:"address"`
	}
	if json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&result) != nil {
		return false, errors.New("app_state_unavailable")
	}
	u, err := url.Parse(result.Address)
	if err != nil || "http://"+u.Host != d.Origin || u.Path != "/" || u.RawQuery != "" || !strings.HasPrefix(u.Fragment, "open=") || len(strings.TrimPrefix(u.Fragment, "open=")) != 64 {
		return false, errors.New("app_state_unavailable")
	}
	if open(result.Address) != nil {
		return false, errors.New("app_window_unavailable")
	}
	return true, nil
}

// Reopen waits briefly when another double-click is still creating its private
// descriptor or starting the HTTP host. It never replaces a busy live app.
func Reopen(ctx context.Context, name string, open func(string) error) (bool, error) {
	for attempt := 0; ; attempt++ {
		ok, err := TryReopen(name, open)
		if err == nil || err.Error() != "app_is_starting" || attempt >= 9 {
			return ok, err
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, ctx.Err()
		case <-timer.C:
		}
	}
}

func removeOwnInstance(name, token string) {
	body, err := os.ReadFile(name)
	if err != nil {
		return
	}
	var d instanceDescriptor
	if json.Unmarshal(body, &d) == nil && d.Token == token {
		_ = os.Remove(name)
	}
}
