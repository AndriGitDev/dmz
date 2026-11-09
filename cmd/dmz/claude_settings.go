package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// A command-line settings layer pins routing above ordinary user/project env
// settings. Its local token is kept out of process arguments in a private,
// temporary file. Managed policy may still take precedence and needs separate
// qualification; this is not an enforcement boundary against a local agent.
func claudeSettings(args []string, base, headers string) ([]string, func(), error) {
	settings := map[string]json.RawMessage{}
	remaining := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			remaining = append(remaining, args[i:]...)
			break
		}
		value := ""
		if arg == "--settings" {
			if i+1 == len(args) {
				return nil, nil, errors.New("missing Claude settings value")
			}
			i++
			value = args[i]
			if value == "" {
				return nil, nil, errors.New("empty Claude settings value")
			}
		} else if strings.HasPrefix(arg, "--settings=") {
			value = strings.TrimPrefix(arg, "--settings=")
			if value == "" {
				return nil, nil, errors.New("empty Claude settings value")
			}
		}
		if value == "" {
			remaining = append(remaining, arg)
			continue
		}
		parsed, err := readSettings(value)
		if err != nil {
			return nil, nil, err
		}
		settings = parsed
	}
	paths, err := claudeSettingsPaths()
	if err != nil {
		return nil, nil, err
	}
	for _, name := range paths {
		parsed, err := readSettingsFile(name, true)
		if err != nil {
			return nil, nil, err
		}
		if err := checkClaudeRoute(parsed); err != nil {
			return nil, nil, err
		}
	}
	if err := checkClaudeRoute(settings); err != nil {
		return nil, nil, err
	}
	env := map[string]json.RawMessage{}
	if raw, ok := settings["env"]; ok {
		if json.Unmarshal(raw, &env) != nil || env == nil {
			return nil, nil, errors.New("invalid Claude env settings")
		}
	}
	for key, value := range map[string]string{"ANTHROPIC_BASE_URL": base, "ANTHROPIC_CUSTOM_HEADERS": headers, "CLAUDE_CODE_USE_BEDROCK": "0", "CLAUDE_CODE_USE_VERTEX": "0", "CLAUDE_CODE_USE_FOUNDRY": "0", "CLAUDE_CODE_USE_MANTLE": "0"} {
		env[key], _ = json.Marshal(value)
	}
	settings["env"], _ = json.Marshal(env)
	encoded, err := json.Marshal(settings)
	if err != nil {
		return nil, nil, errors.New("cannot encode temporary Claude settings")
	}
	dir, err := os.MkdirTemp("", "dmz-launch-")
	if err != nil {
		return nil, nil, errors.New("cannot create private launch settings")
	}
	cleanup := func() { _ = os.RemoveAll(dir) }
	name := filepath.Join(dir, "settings.json")
	if err := os.WriteFile(name, encoded, 0600); err != nil {
		cleanup()
		return nil, nil, errors.New("cannot write private launch settings")
	}
	return append([]string{"--settings", name}, remaining...), cleanup, nil
}

func readSettings(value string) (map[string]json.RawMessage, error) {
	if strings.HasPrefix(strings.TrimSpace(value), "{") {
		return decodeSettings([]byte(value))
	}
	return readSettingsFile(value, false)
}

func readSettingsFile(name string, optional bool) (map[string]json.RawMessage, error) {
	info, err := os.Stat(name)
	if optional && errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Size() > 1<<20 {
		return nil, errors.New("Claude settings file cannot be checked for routing compatibility")
	}
	body, err := os.ReadFile(name)
	if err != nil {
		return nil, errors.New("Claude settings file cannot be read")
	}
	return decodeSettings(body)
}

func decodeSettings(body []byte) (map[string]json.RawMessage, error) {
	var result map[string]json.RawMessage
	if len(body) > 1<<20 || json.Unmarshal(body, &result) != nil || result == nil {
		return nil, errors.New("Claude settings must be a JSON object")
	}
	return result, nil
}

func checkClaudeRoute(settings map[string]json.RawMessage) error {
	var env map[string]json.RawMessage
	if raw, ok := settings["env"]; ok {
		if json.Unmarshal(raw, &env) != nil {
			return errors.New("invalid Claude env settings")
		}
	}
	for key, raw := range env {
		if key != "ANTHROPIC_BASE_URL" && key != "ANTHROPIC_CUSTOM_HEADERS" && key != "CLAUDE_CODE_USE_BEDROCK" && key != "CLAUDE_CODE_USE_VERTEX" && key != "CLAUDE_CODE_USE_FOUNDRY" && key != "CLAUDE_CODE_USE_MANTLE" {
			continue
		}
		var value string
		if json.Unmarshal(raw, &value) != nil {
			return errors.New("Claude routing settings must be strings")
		}
		if key == "ANTHROPIC_BASE_URL" {
			base := strings.TrimRight(value, "/")
			if base != "" && base != "https://api.anthropic.com" && base != "https://api.anthropic.com/v1" {
				return errors.New("Claude settings define an incompatible upstream; gateway chaining is not qualified")
			}
		} else if key == "ANTHROPIC_CUSTOM_HEADERS" {
			if value != "" {
				return errors.New("Claude settings define custom headers; move required headers to the launch environment for this prototype")
			}
		} else if value != "" && value != "0" && value != "false" {
			return errors.New("Claude settings define an alternate backend outside this adapter's route")
		}
	}
	return nil
}

func claudeSettingsPaths() ([]string, error) {
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	if configDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, errors.New("cannot resolve Claude settings directory")
		}
		configDir = filepath.Join(home, ".claude")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, errors.New("cannot resolve project settings directory")
	}
	project := cwd
	for current := cwd; ; current = filepath.Dir(current) {
		if _, err := os.Stat(filepath.Join(current, ".git")); err == nil {
			project = current
			break
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	paths := []string{filepath.Join(configDir, "settings.json"), filepath.Join(project, ".claude", "settings.json"), filepath.Join(project, ".claude", "settings.local.json")}
	if cwd != project {
		paths = append(paths, filepath.Join(cwd, ".claude", "settings.json"), filepath.Join(cwd, ".claude", "settings.local.json"))
	}
	return paths, nil
}
