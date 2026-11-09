package routing

import (
	"bytes"
	"encoding/json"
	"strings"
)

// MergeClaudeSettings preserves user edits while removing only the route and
// local header still owned by the recorded installation.
func MergeClaudeSettings(original, written, current []byte) ([]byte, error) {
	oldRoot := map[string]json.RawMessage{}
	appliedRoot := map[string]json.RawMessage{}
	root := map[string]json.RawMessage{}
	if original != nil && (json.Unmarshal(original, &oldRoot) != nil || oldRoot == nil) {
		return nil, ErrUnavailable
	}
	if json.Unmarshal(written, &appliedRoot) != nil || json.Unmarshal(current, &root) != nil || root == nil {
		return nil, ErrUnavailable
	}
	oldEnv := map[string]json.RawMessage{}
	appliedEnv := map[string]json.RawMessage{}
	env := map[string]json.RawMessage{}
	if raw, ok := oldRoot["env"]; ok {
		if json.Unmarshal(raw, &oldEnv) != nil {
			return nil, ErrUnavailable
		}
	}
	if json.Unmarshal(appliedRoot["env"], &appliedEnv) != nil {
		return nil, ErrUnavailable
	}
	raw, present := root["env"]
	if !present || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return current, nil
	}
	if json.Unmarshal(raw, &env) != nil || env == nil {
		return nil, ErrUnavailable
	}
	if bytes.Equal(env["ANTHROPIC_BASE_URL"], appliedEnv["ANTHROPIC_BASE_URL"]) {
		if value, ok := oldEnv["ANTHROPIC_BASE_URL"]; ok {
			env["ANTHROPIC_BASE_URL"] = value
		} else {
			delete(env, "ANTHROPIC_BASE_URL")
		}
	}
	if bytes.Equal(env["ANTHROPIC_CUSTOM_HEADERS"], appliedEnv["ANTHROPIC_CUSTOM_HEADERS"]) {
		if value, ok := oldEnv["ANTHROPIC_CUSTOM_HEADERS"]; ok {
			env["ANTHROPIC_CUSTOM_HEADERS"] = value
		} else {
			delete(env, "ANTHROPIC_CUSTOM_HEADERS")
		}
	} else {
		var appliedHeaders, currentHeaders string
		if json.Unmarshal(appliedEnv["ANTHROPIC_CUSTOM_HEADERS"], &appliedHeaders) == nil && json.Unmarshal(env["ANTHROPIC_CUSTOM_HEADERS"], &currentHeaders) == nil {
			ownedToken := ""
			for _, line := range strings.Split(appliedHeaders, "\n") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "X-Dmz-Token") {
					ownedToken = strings.TrimSpace(parts[1])
				}
			}
			var kept []string
			for _, line := range strings.Split(currentHeaders, "\n") {
				parts := strings.SplitN(line, ":", 2)
				if len(parts) == 2 && strings.EqualFold(strings.TrimSpace(parts[0]), "X-Dmz-Token") && strings.TrimSpace(parts[1]) == ownedToken && ownedToken != "" {
					continue
				}
				kept = append(kept, line)
			}
			restored := strings.Join(kept, "\n")
			if _, had := oldEnv["ANTHROPIC_CUSTOM_HEADERS"]; !had && restored == "" {
				delete(env, "ANTHROPIC_CUSTOM_HEADERS")
			} else {
				env["ANTHROPIC_CUSTOM_HEADERS"], _ = json.Marshal(restored)
			}
		}
	}
	if _, had := oldRoot["env"]; !had && len(env) == 0 {
		delete(root, "env")
	} else {
		root["env"], _ = json.Marshal(env)
	}
	if original == nil && len(root) == 0 {
		return nil, nil
	}
	result, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, ErrUnavailable
	}
	return append(result, '\n'), nil
}
