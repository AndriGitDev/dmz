package routing

import (
	"bytes"
	"reflect"

	toml "github.com/pelletier/go-toml/v2"
)

// MergeCodexSettings removes the Codex route dmz still owns from a config the
// user edited after installation, keeping every other semantic value. original
// is the pre-install file (nil when none existed), written is the exact TOML
// CodexSettings produced and current is the file as it is now.
//
// Only two values are owned: root.model_provider and
// model_providers.dmz_background. Each is restored (or removed) only while it
// still deep-equals the written value; a value the user changed is kept. The
// result is nil only when no file existed before and nothing remains, so the
// caller can delete the generated file. Comments and formatting of a rewritten
// file are not retained; when no owned value remains, current is returned
// unchanged.
func MergeCodexSettings(original, written, current []byte) ([]byte, error) {
	if len(original) > maxConfig || len(written) > maxConfig || len(current) > maxConfig || written == nil || current == nil {
		return nil, ErrUnavailable
	}
	oldRoot, appliedRoot, root := map[string]any{}, map[string]any{}, map[string]any{}
	if original != nil && (toml.Unmarshal(original, &oldRoot) != nil || oldRoot == nil) {
		return nil, ErrUnavailable
	}
	if toml.Unmarshal(written, &appliedRoot) != nil || appliedRoot == nil || toml.Unmarshal(current, &root) != nil || root == nil {
		return nil, ErrUnavailable
	}

	appliedDefault, ok := appliedRoot["model_provider"]
	if !ok || appliedDefault != codexProvider {
		return nil, ErrUnavailable
	}
	appliedProviders, ok := appliedRoot["model_providers"].(map[string]any)
	if !ok {
		return nil, ErrUnavailable
	}
	appliedOwn, ok := appliedProviders[codexProvider].(map[string]any)
	if !ok {
		return nil, ErrUnavailable
	}
	var oldProviders map[string]any
	if raw, had := oldRoot["model_providers"]; had {
		if oldProviders, ok = raw.(map[string]any); !ok {
			return nil, ErrUnavailable
		}
	}

	changed := false
	if value, present := root["model_provider"]; present && reflect.DeepEqual(value, appliedDefault) {
		if previous, had := oldRoot["model_provider"]; had {
			root["model_provider"] = previous
		} else {
			delete(root, "model_provider")
		}
		changed = true
	}

	if raw, present := root["model_providers"]; present {
		providers, isTable := raw.(map[string]any)
		if !isTable {
			return nil, ErrUnavailable
		}
		if own, present := providers[codexProvider]; present && reflect.DeepEqual(own, appliedOwn) {
			if previous, had := oldProviders[codexProvider]; had {
				providers[codexProvider] = previous
			} else {
				delete(providers, codexProvider)
			}
			changed = true
		}
		if _, had := oldRoot["model_providers"]; !had && len(providers) == 0 {
			delete(root, "model_providers")
			changed = true
		}
	}

	if original == nil && len(root) == 0 {
		return nil, nil
	}
	if !changed {
		return bytes.Clone(current), nil
	}
	result, err := toml.Marshal(root)
	if err != nil || len(result) > maxConfig {
		return nil, ErrUnavailable
	}
	if result == nil {
		result = []byte{}
	}
	return result, nil
}
