package companion

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
)

type backgroundPreferences struct {
	HighlightRestored bool `json:"highlight_restored"`
}

func loadBackgroundPreferences(path string) backgroundPreferences {
	var preferences backgroundPreferences
	f, err := os.Open(path)
	if err != nil {
		return preferences
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if err != nil || len(data) > 4096 || json.Unmarshal(data, &preferences) != nil {
		return backgroundPreferences{}
	}
	return preferences
}

func (a *App) highlightRestored() bool {
	a.mu.Lock()
	m := a.background
	a.mu.Unlock()
	if m == nil {
		return false
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.preferences.HighlightRestored
}

func (m *backgroundManager) setHighlightRestored(enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return errors.New("background_unavailable")
	}
	next := backgroundPreferences{HighlightRestored: enabled}
	if writeStateJSON(filepath.Join(m.cfg.StateDir, "preferences.json"), next) != nil {
		return errors.New("preferences_unavailable")
	}
	m.preferences = next
	for _, r := range m.routes {
		if r.handler != nil {
			r.handler.SetHighlightRestored(enabled)
		}
	}
	return nil
}
