package routing

import (
	"encoding/hex"
	"net/url"
	"strconv"
	"strings"
)

// codexProvider is the provider ID dmz owns inside the Codex settings file.
const codexProvider = "dmz_background"

// Keys that move the Codex model or account route away from the built-in
// "openai" provider. Their presence makes the effective route ambiguous, so
// dmz refuses to route rather than guess which endpoint the client would use.
var codexRouteKeys = []string{"model_provider", "openai_base_url", "chatgpt_base_url"}

// codexBaseURL accepts only a literal loopback root origin and returns the
// Responses API base URL dmz publishes to Codex.
func codexBaseURL(origin, token string) (string, bool) {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme != "http" || u.Opaque != "" || u.User != nil || u.Hostname() != "127.0.0.1" ||
		u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.RawFragment != "" || (u.Path != "" && u.Path != "/") || u.RawPath != "" {
		return "", false
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 || strconv.Itoa(port) != u.Port() {
		return "", false
	}
	if len(token) != 64 {
		return "", false
	}
	if _, err := hex.DecodeString(token); err != nil {
		return "", false
	}
	return "http://127.0.0.1:" + u.Port() + "/v1", true
}

// CodexSettings returns the Codex config.toml that routes the default session
// through the dmz loopback gateway. A nil original means no file exists. The
// input is parsed strictly first. The change is made in place: the top-level
// model_provider value is replaced (or one line is added before any table) and
// the owned [model_providers.dmz_background] table is appended, so unrelated
// bytes, comments and formatting are kept. A layout that cannot be extended
// that way, such as an inline model_providers table, is refused. Authentication
// stays with Codex: no credential, model, approval, sandbox or feature value is
// written.
func CodexSettings(original []byte, origin, token string) ([]byte, error) {
	baseURL, ok := codexBaseURL(origin, token)
	if !ok || len(original) > maxConfig {
		return nil, ErrUnavailable
	}
	root := map[string]any{}
	if original != nil {
		if root, ok = decodeTOML(original); !ok {
			return nil, ErrUnavailable
		}
	}
	if err := codexCheckRoute(root, true); err != nil {
		return nil, err
	}
	providers := map[string]any{}
	if raw, ok := root["model_providers"]; ok {
		if providers, ok = raw.(map[string]any); !ok {
			return nil, ErrUnavailable
		}
	}
	if _, ok := providers[codexProvider]; ok {
		return nil, ErrCustomRoute
	}
	if raw, ok := root["profiles"]; ok {
		profiles, ok := raw.(map[string]any)
		if !ok {
			return nil, ErrUnavailable
		}
		for _, raw := range profiles {
			profile, ok := raw.(map[string]any)
			if !ok {
				return nil, ErrUnavailable
			}
			// A profile-level provider or endpoint would bypass the gateway
			// whenever that profile is selected, including "openai".
			if err := codexCheckRoute(profile, false); err != nil {
				return nil, err
			}
		}
	}
	providers[codexProvider] = map[string]any{
		"name":                 "dmz background gateway",
		"base_url":             baseURL,
		"wire_api":             "responses",
		"requires_openai_auth": true,
		"supports_websockets":  false,
		"http_headers":         map[string]any{"X-Dmz-Token": token},
		"env_http_headers":     map[string]any{"X-Dmz-OpenAI-Key": "OPENAI_API_KEY"},
	}
	root["model_providers"] = providers
	root["model_provider"] = codexProvider

	doc, ok := scanTOML(original)
	if !ok {
		return nil, ErrUnavailable
	}
	for _, e := range doc.exprs {
		// An inline model_providers table cannot gain a table by header.
		if !e.header && pathIs(e.path, "model_providers") {
			return nil, ErrUnavailable
		}
	}
	var edit tomlEdit
	if e := doc.rootKey("model_provider"); e != nil {
		edit = tomlEdit{start: e.valStart, end: e.valEnd, text: `"` + codexProvider + `"`}
	} else {
		at := doc.rootInsertAt()
		edit = tomlEdit{start: at, end: at, text: doc.newLineAt(at) + `model_provider = "` + codexProvider + `"` + doc.eol}
	}
	result, ok := applyEdits(original, []tomlEdit{edit})
	if !ok {
		return nil, ErrUnavailable
	}
	// A table header at the end of the document cannot change the scope of
	// anything before it.
	if len(result) > 0 {
		if result[len(result)-1] != '\n' {
			result = append(result, doc.eol...)
		}
		result = append(result, doc.eol...)
	}
	result = append(result, codexProviderBlock(baseURL, token, doc.eol)...)
	if len(result) > maxConfig {
		return nil, ErrUnavailable
	}
	if got, ok := decodeTOML(result); !ok || !sameTOML(got, root) {
		return nil, ErrUnavailable
	}
	return result, nil
}

// codexProviderBlock is the owned provider table. Its values were validated as
// a loopback URL and a hex token, so they need no escaping.
func codexProviderBlock(baseURL, token, eol string) string {
	lines := []string{
		"[model_providers." + codexProvider + "]",
		`name = "dmz background gateway"`,
		`base_url = "` + baseURL + `"`,
		`wire_api = "responses"`,
		`requires_openai_auth = true`,
		`supports_websockets = false`,
		`http_headers = { X-Dmz-Token = "` + token + `" }`,
		`env_http_headers = { X-Dmz-OpenAI-Key = "OPENAI_API_KEY" }`,
	}
	return strings.Join(lines, eol) + eol
}

// codexCheckRoute rejects an active non-default route. At the top level an
// explicit "openai" provider is the default and is replaced; in profiles any
// provider or endpoint override is rejected.
func codexCheckRoute(table map[string]any, top bool) error {
	for _, key := range codexRouteKeys {
		raw, ok := table[key]
		if !ok {
			continue
		}
		value, isString := raw.(string)
		if !isString {
			return ErrUnavailable
		}
		if top && key == "model_provider" && value == "openai" {
			continue
		}
		return ErrCustomRoute
	}
	return nil
}
