// Package routing owns reversible, private changes to harness routing settings.
// It never reads a separate harness authentication store. Recovery snapshots
// can contain sensitive values already present in user settings.
package routing

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

var (
	ErrChanged     = errors.New("routing_settings_changed")
	ErrUnavailable = errors.New("routing_settings_unavailable")
	ErrBackup      = errors.New("routing_backup_unavailable")
	ErrCustomRoute = errors.New("routing_custom_provider_present")
)

const maxConfig = 4 << 20

type journal struct {
	Path     string `json:"path"`
	Existed  bool   `json:"existed"`
	Mode     uint32 `json:"mode"`
	Original []byte `json:"original"`
	Applied  string `json:"applied_sha256"`
	Written  []byte `json:"written,omitempty"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func readConfig(path string) ([]byte, os.FileMode, bool, error) {
	st, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, 0600, false, nil
	}
	if err != nil || !st.Mode().IsRegular() || st.Size() > maxConfig {
		return nil, 0, false, ErrUnavailable
	}
	b, err := os.ReadFile(path)
	if err != nil || len(b) > maxConfig {
		return nil, 0, false, ErrUnavailable
	}
	return b, st.Mode().Perm(), true, nil
}
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".dmz-routing-")
	if err != nil {
		return ErrUnavailable
	}
	defer os.Remove(f.Name())
	if f.Chmod(mode) != nil {
		f.Close()
		return ErrUnavailable
	}
	if _, err = f.Write(data); err != nil {
		f.Close()
		return ErrUnavailable
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if syncErr != nil || closeErr != nil {
		return ErrUnavailable
	}
	if os.Rename(f.Name(), path) != nil {
		return ErrUnavailable
	}
	return nil
}

// Install records recovery state before atomically changing the settings. A nil
// expected value means the file must not exist; an empty slice means empty file.
// The journal can contain existing user settings, so it is owner-only and must
// never be included in diagnostics or publications.
func Install(path, state string, expected, replacement []byte) error {
	if !filepath.IsAbs(path) || !filepath.IsAbs(state) || path == state || len(replacement) > maxConfig {
		return ErrUnavailable
	}
	original, mode, exists, err := readConfig(path)
	if err != nil {
		return err
	}
	if exists != (expected != nil) || !bytes.Equal(original, expected) {
		return ErrChanged
	}
	if os.MkdirAll(filepath.Dir(state), 0700) != nil || os.Chmod(filepath.Dir(state), 0700) != nil {
		return ErrBackup
	}
	f, err := os.OpenFile(state, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ErrBackup
	}
	record := journal{Path: path, Existed: exists, Mode: uint32(mode), Original: original, Applied: digest(replacement), Written: replacement}
	err = json.NewEncoder(f).Encode(record)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		os.Remove(state)
		return ErrBackup
	}
	if os.MkdirAll(filepath.Dir(path), 0700) != nil {
		return ErrUnavailable
	}
	// Detect an edit made while the backup was being persisted.
	current, _, currentExists, err := readConfig(path)
	if err != nil || currentExists != exists || !bytes.Equal(current, original) {
		return ErrChanged
	}
	return writeAtomic(path, replacement, 0600)
}

// Restore also recovers a crash between journal creation and settings mutation.
// Drift is never overwritten; the backup remains for a selective adapter merge.
func Restore(state string) error { return RestoreMerge(state, nil) }

// RestoreMerge removes only owned routing fields when unrelated settings drift.
func RestoreMerge(state string, merge func([]byte, []byte, []byte) ([]byte, error)) error {
	info, err := os.Lstat(state)
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return ErrBackup
	}
	f, err := os.Open(state)
	if err != nil {
		return ErrBackup
	}
	defer f.Close()
	var record journal
	decoder := json.NewDecoder(io.LimitReader(f, 12<<20))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&record) != nil || decoder.Decode(new(any)) != io.EOF || !filepath.IsAbs(record.Path) || len(record.Original) > maxConfig {
		return ErrBackup
	}
	current, _, exists, err := readConfig(record.Path)
	if err != nil {
		return err
	}
	if exists == record.Existed && bytes.Equal(current, record.Original) {
		return os.Remove(state)
	}
	if !exists {
		return os.Remove(state)
	} // preserve a user's deletion
	if digest(current) != record.Applied {
		if merge == nil {
			return ErrChanged
		}
		if len(record.Written) == 0 || digest(record.Written) != record.Applied {
			return ErrBackup
		}
		result, mergeErr := merge(record.Original, record.Written, current)
		if mergeErr != nil || len(result) > maxConfig {
			return ErrChanged
		}
		if result == nil {
			err = os.Remove(record.Path)
		} else {
			err = writeAtomic(record.Path, result, os.FileMode(record.Mode).Perm())
		}
		if err != nil {
			return ErrUnavailable
		}
		if os.Remove(state) != nil {
			return ErrBackup
		}
		return nil
	}
	if record.Existed {
		err = writeAtomic(record.Path, record.Original, os.FileMode(record.Mode).Perm())
	} else {
		err = os.Remove(record.Path)
	}
	if err != nil {
		return ErrUnavailable
	}
	if os.Remove(state) != nil {
		return ErrBackup
	}
	return nil
}

// ClaudeSettings changes only the two routing environment keys. Existing
// model, tool, permission and sandbox settings retain their JSON values.
func ClaudeSettings(original []byte, origin, token string) ([]byte, error) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.Port() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") || len(token) != 64 {
		return nil, ErrUnavailable
	}
	port, portErr := strconv.Atoi(u.Port())
	if portErr != nil || port < 1 || port > 65535 {
		return nil, ErrUnavailable
	}
	if _, err := hex.DecodeString(token); err != nil {
		return nil, ErrUnavailable
	}
	root := map[string]json.RawMessage{}
	if original != nil && (json.Unmarshal(original, &root) != nil || root == nil) {
		return nil, ErrUnavailable
	}
	env := map[string]json.RawMessage{}
	if raw, ok := root["env"]; ok && (json.Unmarshal(raw, &env) != nil || env == nil) {
		return nil, ErrUnavailable
	}
	for _, key := range []string{"ANTHROPIC_BASE_URL", "CLAUDE_CODE_USE_BEDROCK", "CLAUDE_CODE_USE_VERTEX", "CLAUDE_CODE_USE_FOUNDRY"} {
		if raw, ok := env[key]; ok && string(raw) != "null" && string(raw) != `""` {
			return nil, ErrCustomRoute
		}
	}
	headers := ""
	if raw, ok := env["ANTHROPIC_CUSTOM_HEADERS"]; ok && json.Unmarshal(raw, &headers) != nil {
		return nil, ErrUnavailable
	}
	for _, line := range strings.Split(headers, "\n") {
		if strings.EqualFold(strings.TrimSpace(strings.SplitN(line, ":", 2)[0]), "X-Dmz-Token") {
			return nil, ErrCustomRoute
		}
	}
	if headers != "" {
		headers += "\n"
	}
	headers += "X-Dmz-Token: " + token
	env["ANTHROPIC_BASE_URL"], _ = json.Marshal(strings.TrimRight(origin, "/"))
	env["ANTHROPIC_CUSTOM_HEADERS"], _ = json.Marshal(headers)
	root["env"], _ = json.Marshal(env)
	result, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, ErrUnavailable
	}
	return append(result, '\n'), nil
}
