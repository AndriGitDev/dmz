package routing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	toml "github.com/pelletier/go-toml/v2"
)

// Byte-level tests for in-place Codex edits. Synthetic configurations only; no
// real Codex configuration, endpoint or credential is read.

// codexBlock is the owned table exactly as install appends it.
const codexBlock = `[model_providers.dmz_background]
name = "dmz background gateway"
base_url = "http://127.0.0.1:43123/v1"
wire_api = "responses"
requires_openai_auth = true
supports_websockets = false
http_headers = { X-Dmz-Token = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef" }
env_http_headers = { X-Dmz-OpenAI-Key = "OPENAI_API_KEY" }
`

const codexSelector = `model_provider = "dmz_background"` + "\n"

// codexCommentedFixture has comments and layout that a re-encoder would lose,
// including text inside strings that looks like owned keys and headers.
const codexCommentedFixture = `# Synthetic Codex settings; comments must survive.
model = "synthetic-model"     # aligned comment
model_provider = 'openai'   # user note on the selector
approval_policy = "on-request"
notes = """
[model_providers.dmz_background]
model_provider = "not a key"
"""
"quoted.key" = { a = 1, b = [ 1, 2 ] }
mixed = [
  1, # inside array
  "two",
]

# Model providers the user defined.
[model_providers.inactive_custom]
name = "Inactive synthetic provider"
base_url = "http://127.0.0.1:9/v1"

[projects."/home/notandi/verk efni/ástæða.d"]
trust_level = "trusted"

[profiles.fast]
model = "synthetic-small"   # keep alignment
`

func codexMustInstall(t *testing.T, original []byte) []byte {
	t.Helper()
	out, err := CodexSettings(original, codexOrigin, codexToken)
	if err != nil {
		t.Fatalf("install: %v\n%s", err, original)
	}
	return out
}

func codexMustMerge(t *testing.T, original, written, current []byte) []byte {
	t.Helper()
	out, err := MergeCodexSettings(original, written, current)
	if err != nil {
		t.Fatalf("merge: %v\n--- current ---\n%s", err, current)
	}
	return out
}

func wantBytes(t *testing.T, label string, got, want []byte) {
	t.Helper()
	if !bytes.Equal(got, want) {
		t.Fatalf("%s bytes differ\n--- got ---\n%q\n--- want ---\n%q", label, got, want)
	}
}

func TestCodexSettingsEditsInPlace(t *testing.T) {
	block := codexBlock
	cases := []struct{ name, original, want string }{
		{"no file", "", codexSelector + "\n" + block},
		{"comment only", "# comment only\n\n", "# comment only\n\n" + codexSelector + "\n" + block},
		{
			"after last root key, before table comments",
			"# head\nmodel = \"m\" # note\n\n# MCP\n[mcp_servers.x]\ncommand = \"c\"\n",
			"# head\nmodel = \"m\" # note\n" + codexSelector + "\n# MCP\n[mcp_servers.x]\ncommand = \"c\"\n\n" + block,
		},
		{
			"no root key: before the first table's comment block",
			"# file header\n\n# Servers\n  [mcp_servers.x]\ncommand = \"c\"\n",
			"# file header\n\n" + codexSelector + "# Servers\n  [mcp_servers.x]\ncommand = \"c\"\n\n" + block,
		},
		{
			"replace selector value, keep its comment and quoting of others",
			"model_provider   =   'openai'   # keep me\n[tui]\ntheme = 'dark'\n",
			"model_provider   =   \"dmz_background\"   # keep me\n[tui]\ntheme = 'dark'\n\n" + block,
		},
		{
			"quoted and escaped selector key",
			"\"model\\u005fprovider\" = \"openai\"\n",
			"\"model\\u005fprovider\" = \"dmz_background\"\n\n" + block,
		},
		{"missing final newline", "model = \"m\"", "model = \"m\"\n" + codexSelector + "\n" + block},
		{
			"missing final newline after a table",
			"[tui]\ntheme = \"dark\"",
			codexSelector + "[tui]\ntheme = \"dark\"\n\n" + block,
		},
		{
			"multiline string as last root value",
			"notes = \"\"\"\n[not.a.table]\n\"\"\"\n[tui]\n",
			"notes = \"\"\"\n[not.a.table]\n\"\"\"\n" + codexSelector + "[tui]\n\n" + block,
		},
		{
			"providers defined by dotted root keys",
			"model_providers.local.name = \"x\"\n",
			"model_providers.local.name = \"x\"\n" + codexSelector + "\n" + block,
		},
		{
			"providers outer table with inline entry",
			"[model_providers]\nlocal = { name = \"x\" }\n",
			codexSelector + "[model_providers]\nlocal = { name = \"x\" }\n\n" + block,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wantBytes(t, "install", codexMustInstall(t, []byte(c.original)), []byte(c.want))
		})
	}
	wantBytes(t, "nil original", codexMustInstall(t, nil), []byte(codexSelector+"\n"+block))

	crlf := "model = \"m\"\r\n[tui]\r\ntheme = \"dark\"\r\n"
	want := "model = \"m\"\r\n" + strings.ReplaceAll(codexSelector, "\n", "\r\n") + "[tui]\r\ntheme = \"dark\"\r\n\r\n" + strings.ReplaceAll(block, "\n", "\r\n")
	wantBytes(t, "crlf", codexMustInstall(t, []byte(crlf)), []byte(want))
}

func TestCodexSettingsKeepsCommentedFixtureBytes(t *testing.T) {
	out := codexMustInstall(t, []byte(codexCommentedFixture))
	want := strings.Replace(codexCommentedFixture, "model_provider = 'openai'", `model_provider = "dmz_background"`, 1) + "\n" + codexBlock
	wantBytes(t, "install", out, []byte(want))
	cfg := parseCodex(t, out)
	wantDmzProvider(t, cfg)
	if !strings.Contains(cfg["notes"].(string), "[model_providers.dmz_background]") {
		t.Fatal("string contents changed")
	}
}

func TestCodexSettingsRefusesInlineProviders(t *testing.T) {
	for _, cfg := range []string{
		"model_providers = { local = { name = \"x\" } }\n",
		"model_providers = {}\n",
	} {
		if out, err := CodexSettings([]byte(cfg), codexOrigin, codexToken); err != ErrUnavailable || out != nil {
			t.Errorf("%q: out=%q err=%v, want ErrUnavailable", cfg, out, err)
		}
	}
}

// Codex appends a trusted project as a new table after existing ones.
const codexTrustAppend = "\n[projects.\"/home/synthetic/verkefni\"]\ntrust_level = \"trusted\"\n"

func TestMergeCodexAfterProjectTrustRestoresOriginalBytes(t *testing.T) {
	for _, original := range []string{codexCommentedFixture, codexRichFixture, "model = \"m\"\n", "# only a comment\n"} {
		written := codexMustInstall(t, []byte(original))
		current := append(bytes.Clone(written), codexTrustAppend...)
		got := codexMustMerge(t, []byte(original), written, current)
		wantBytes(t, "merge", got, []byte(original+codexTrustAppend))
	}
}

func TestMergeCodexAfterModelChangeKeepsCurrentBytes(t *testing.T) {
	original := []byte(codexCommentedFixture)
	written := codexMustInstall(t, original)
	// A /model style edit: change one value in place and add a root key next
	// to the existing root keys; also edit inside a later table.
	current := bytes.Replace(written, []byte(`model = "synthetic-model"`), []byte(`model = "synthetic-model-2"`), 1)
	current = bytes.Replace(current, []byte("approval_policy = \"on-request\"\n"), []byte("approval_policy = \"on-request\"\nmodel_reasoning_effort = \"high\"\n"), 1)
	current = bytes.Replace(current, []byte("model = \"synthetic-small\"   # keep alignment\n"), []byte("model = \"synthetic-small\"   # keep alignment\nnew_key = 1 # added inside a table\n"), 1)
	got := codexMustMerge(t, original, written, current)
	want := strings.Replace(codexCommentedFixture, `model = "synthetic-model"`, `model = "synthetic-model-2"`, 1)
	want = strings.Replace(want, "approval_policy = \"on-request\"\n", "approval_policy = \"on-request\"\nmodel_reasoning_effort = \"high\"\n", 1)
	want = strings.Replace(want, "# keep alignment\n", "# keep alignment\nnew_key = 1 # added inside a table\n", 1)
	wantBytes(t, "merge", got, []byte(want))
}

func TestMergeCodexLineEndingsAndFinalNewline(t *testing.T) {
	crlf := "# c\r\nmodel = \"m\"\r\n[tui]\r\ntheme = \"dark\"\r\n"
	written := codexMustInstall(t, []byte(crlf))
	appended := strings.ReplaceAll(codexTrustAppend, "\n", "\r\n")
	got := codexMustMerge(t, []byte(crlf), written, append(bytes.Clone(written), appended...))
	wantBytes(t, "crlf", got, []byte(crlf+appended))

	for _, original := range []string{"model = \"m\"", "[tui]\ntheme = \"dark\"", "# only comment"} {
		written := codexMustInstall(t, []byte(original))
		current := bytes.Replace(written, []byte(`"m"`), []byte(`"m2"`), 1)
		got := codexMustMerge(t, []byte(original), written, bytes.Replace(current, []byte(`theme = "dark"`), []byte(`theme = "light"`), 1))
		want := strings.Replace(strings.Replace(original, `"m"`, `"m2"`, 1), `"dark"`, `"light"`, 1)
		wantBytes(t, "no final newline "+original, got, []byte(want))
	}
}

func TestMergeCodexNoFileKeepsLaterEdits(t *testing.T) {
	written := codexMustInstall(t, nil)
	got := codexMustMerge(t, nil, written, append(bytes.Clone(written), codexTrustAppend...))
	wantBytes(t, "merge", got, []byte(strings.TrimPrefix(codexTrustAppend, "\n")))

	// Comments the user added to a file dmz created are kept.
	got = codexMustMerge(t, nil, written, append(bytes.Clone(written), "# user note\n"...))
	wantBytes(t, "comment", got, []byte("# user note\n"))

	// Only whitespace left: the file is removed.
	out, err := MergeCodexSettings(nil, written, append(bytes.Clone(written), "\n\n"...))
	if err != nil || out != nil {
		t.Fatalf("whitespace only: out=%q err=%v", out, err)
	}
}

func TestMergeCodexEditedOwnedValuesKeepTheirBytes(t *testing.T) {
	original := []byte(codexCommentedFixture)
	written := codexMustInstall(t, original)

	// Owned table edited: kept byte for byte; the selector is still restored.
	current := bytes.Replace(written, []byte("supports_websockets = false\n"), []byte("supports_websockets = false\nstream_idle_timeout_ms = 1 # user\n"), 1)
	got := codexMustMerge(t, original, written, current)
	want := strings.Replace(string(current), `model_provider = "dmz_background"`, "model_provider = 'openai'", 1)
	wantBytes(t, "owned table edited", got, []byte(want))

	// Selector changed by the user: kept; the unchanged owned table is removed.
	current = bytes.Replace(written, []byte(`model_provider = "dmz_background"`), []byte(`model_provider = "inactive_custom"`), 1)
	got = codexMustMerge(t, original, written, current)
	want = strings.Replace(codexCommentedFixture, "model_provider = 'openai'", `model_provider = "inactive_custom"`, 1)
	wantBytes(t, "selector changed", got, []byte(want))

	// A comment the user put on the selector line survives its removal.
	plain := []byte("model = \"m\"\n")
	written = codexMustInstall(t, plain)
	current = bytes.Replace(written, []byte(codexSelector), []byte("model_provider = \"dmz_background\" # why\n"), 1)
	got = codexMustMerge(t, plain, written, current)
	wantBytes(t, "selector comment", got, []byte("model = \"m\"\n# why\n"))
}

func TestMergeCodexKeepsCommentsAroundOwnedTable(t *testing.T) {
	original := []byte("model = \"m\"\n")
	written := codexMustInstall(t, original)
	current := append(bytes.Clone(written), "# appended after the owned table\n"...)
	got := codexMustMerge(t, original, written, current)
	wantBytes(t, "merge", got, []byte("model = \"m\"\n# appended after the owned table\n"))
}

// codexOldWritten reproduces the whole-file encoding used by earlier versions,
// whose journals may still be on disk.
func codexOldWritten(t *testing.T, original []byte) []byte {
	t.Helper()
	root := map[string]any{}
	if err := toml.Unmarshal(original, &root); err != nil {
		t.Fatal(err)
	}
	providers, _ := root["model_providers"].(map[string]any)
	if providers == nil {
		providers = map[string]any{}
	}
	providers[codexProvider] = map[string]any{
		"name": "dmz background gateway", "base_url": "http://127.0.0.1:43123/v1", "wire_api": "responses",
		"requires_openai_auth": true, "supports_websockets": false,
		"http_headers":     map[string]any{"X-Dmz-Token": codexToken},
		"env_http_headers": map[string]any{"X-Dmz-OpenAI-Key": "OPENAI_API_KEY"},
	}
	root["model_providers"] = providers
	root["model_provider"] = codexProvider
	out, err := toml.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestMergeCodexOldJournalLayouts(t *testing.T) {
	for _, original := range []string{"model = \"m\"\n", codexRichFixture, ""} {
		written := codexOldWritten(t, []byte(original))
		current := append(bytes.Clone(written), codexTrustAppend...)
		got := codexMustMerge(t, []byte(original), written, current)
		if bytes.Contains(got, []byte(codexProvider)) || !bytes.HasSuffix(got, []byte(strings.TrimPrefix(codexTrustAppend, "\n"))) {
			t.Fatalf("old layout not cleaned:\n%s", got)
		}
		want := parseCodex(t, []byte(original+codexTrustAppend))
		if gotCfg := parseCodex(t, got); !sameTOML(gotCfg, want) {
			t.Fatalf("old layout semantics: %#v\nwant %#v", gotCfg, want)
		}
	}
}

func TestMergeCodexOwnedEntryInsideOuterTable(t *testing.T) {
	original := []byte("model = \"m\" # keep\n")
	written := codexMustInstall(t, original)
	// The owned entry rewritten as one inline value under [model_providers].
	current := []byte("model = \"m2\" # keep\nmodel_provider = \"dmz_background\"\n\n[model_providers]\ndmz_background = { name = \"dmz background gateway\", base_url = \"http://127.0.0.1:43123/v1\", wire_api = \"responses\", requires_openai_auth = true, supports_websockets = false, http_headers = { X-Dmz-Token = \"" + codexToken + "\" }, env_http_headers = { X-Dmz-OpenAI-Key = \"OPENAI_API_KEY\" } }\n\n[tui] # user\ntheme = \"dark\"\n")
	got := codexMustMerge(t, original, written, current)
	wantBytes(t, "merge", got, []byte("model = \"m2\" # keep\n\n\n[tui] # user\ntheme = \"dark\"\n"))
}

func TestMergeCodexRefusesUneditableOwnedLayout(t *testing.T) {
	original := []byte("# keep\nmodel = \"m\"\n")
	written := codexMustInstall(t, original)
	// Owned values rewritten into an inline table equal to what was written.
	current := []byte("# keep\nmodel = \"m\"\nmodel_provider = \"dmz_background\"\nmodel_providers = { dmz_background = { name = \"dmz background gateway\", base_url = \"http://127.0.0.1:43123/v1\", wire_api = \"responses\", requires_openai_auth = true, supports_websockets = false, http_headers = { X-Dmz-Token = \"" + codexToken + "\" }, env_http_headers = { X-Dmz-OpenAI-Key = \"OPENAI_API_KEY\" } } }\n")
	if out, err := MergeCodexSettings(original, written, current); err != ErrChanged || out != nil {
		t.Fatalf("out=%q err=%v, want ErrChanged", out, err)
	}
	// Even with no original values left, an unsafe layout must not turn into
	// file deletion and silently discard comments the user added.
	newFile := append([]byte("# a new user comment\n"), bytes.TrimPrefix(current, original)...)
	if out, err := MergeCodexSettings(nil, codexMustInstall(t, nil), newFile); err != ErrChanged || out != nil {
		t.Fatal("uneditable new-file layout was treated as a removable empty file")
	}

	// Through the journal the current file and the recovery record remain.
	dir := t.TempDir()
	path, state := filepath.Join(dir, "config.toml"), filepath.Join(dir, "state", "codex.json")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Install(path, state, original, written); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, current, 0600); err != nil {
		t.Fatal(err)
	}
	if err := RestoreMerge(state, MergeCodexSettings); !errors.Is(err, ErrChanged) {
		t.Fatalf("restore: %v", err)
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, current) {
		t.Fatal("refused merge changed the settings file")
	}
	if _, err := os.Stat(state); err != nil {
		t.Fatal("refused merge removed the recovery record")
	}
}

func TestMergeCodexRestoreMergeKeepsAppendedTrust(t *testing.T) {
	dir := t.TempDir()
	path, state := filepath.Join(dir, "config.toml"), filepath.Join(dir, "state", "codex.json")
	original := []byte(codexCommentedFixture)
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	written := codexMustInstall(t, original)
	if err := Install(path, state, original, written); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(codexTrustAppend); err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := RestoreMerge(state, MergeCodexSettings); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	wantBytes(t, "restored file", got, []byte(codexCommentedFixture+codexTrustAppend))
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("journal not removed")
	}
}

func TestScanTOMLSpans(t *testing.T) {
	src := "a = 1 # c\n  [ t . \"q.k\" ]  # h\nb.c = \"\"\"\nx\n\"\"\"\r\n[[arr]]\nd = [\n 1,\n]"
	doc, ok := scanTOML([]byte(src))
	if !ok {
		t.Fatal("scan failed")
	}
	type span struct {
		path            string
		header, root    bool
		line, val, tail string
	}
	var got []span
	for _, e := range doc.exprs {
		s := span{path: strings.Join(e.path, "|"), header: e.header, root: e.root, line: src[e.start:e.end]}
		if !e.header {
			s.val = src[e.valStart:e.valEnd]
		}
		if e.comment >= 0 {
			s.tail = src[e.comment:e.end]
		}
		got = append(got, s)
	}
	want := []span{
		{"a", false, true, "a = 1 # c\n", "1", "# c\n"},
		{"t|q.k", true, false, "  [ t . \"q.k\" ]  # h\n", "", "# h\n"},
		{"t|q.k|b|c", false, false, "b.c = \"\"\"\nx\n\"\"\"\r\n", "\"\"\"\nx\n\"\"\"", ""},
		{"arr", true, false, "[[arr]]\n", "", ""},
		{"arr|d", false, false, "d = [\n 1,\n]", "[\n 1,\n]", ""},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d expressions: %#v", len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expr %d = %#v, want %#v", i, got[i], want[i])
		}
	}
	if doc.eol != "\n" {
		t.Fatalf("eol %q", doc.eol)
	}
}

func TestSameTOML(t *testing.T) {
	a, _ := decodeTOML([]byte("n = nan\nt = 1979-05-27T07:32:00-08:00\nx = [1, { y = 0.5 }]\n"))
	b, _ := decodeTOML([]byte("n = nan\nt = 1979-05-27T07:32:00-08:00\nx = [1, { y = 0.5 }]\n"))
	if !sameTOML(a, b) {
		t.Fatal("equal documents differ")
	}
	c, _ := decodeTOML([]byte("n = nan\nt = 1979-05-27T15:32:00Z\nx = [1, { y = 0.5 }]\n"))
	if sameTOML(a, c) {
		t.Fatal("offset change not detected")
	}
	if sameTOML(map[string]any{"f": 0.0}, map[string]any{"f": math.Copysign(0, -1)}) || sameTOML(map[string]any{"t": time.Time{}}, map[string]any{"t": "x"}) {
		t.Fatal("distinct values compare equal")
	}
}

// TestCodexMergeAfterInstalledCodexEdits is opt-in: set DMZ_CODEX_BIN to a
// Codex CLI. In temporary CODEX_HOME directories with synthetic settings only,
// Codex itself edits both the installed config and the untouched original. A
// disconnect must produce exactly the bytes Codex wrote for the original, and
// Codex must load the result.
func TestCodexMergeAfterInstalledCodexEdits(t *testing.T) {
	bin := os.Getenv("DMZ_CODEX_BIN")
	if bin == "" {
		t.Skip("set DMZ_CODEX_BIN to check merges after edits made by an installed Codex CLI")
	}
	edit := func(config []byte, steps ...[]string) []byte {
		home := t.TempDir()
		path := filepath.Join(home, "config.toml")
		if err := os.WriteFile(path, config, 0600); err != nil {
			t.Fatal(err)
		}
		for _, args := range steps {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			cmd := exec.CommandContext(ctx, bin, args...)
			cmd.Dir = home
			cmd.Env = []string{"CODEX_HOME=" + home, "HOME=" + home, "PATH=" + os.Getenv("PATH"), "NO_COLOR=1"}
			out, err := cmd.CombinedOutput()
			cancel()
			if err != nil || bytes.Contains(out, []byte("failed to load")) {
				b, _ := os.ReadFile(path)
				t.Fatalf("codex %v: %v\n%s\n--- config ---\n%s", args, err, out, b)
			}
		}
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	steps := [][]string{
		{"features", "list"},
		{"mcp", "add", "synthetic", "--", "echo", "synthetic"},
		{"features", "disable", "unified_exec"},
	}
	for _, original := range [][]byte{
		[]byte(codexCommentedFixture + "\n[features]\nunified_exec = true # user feature\n"),
		[]byte(codexRichFixture),
		[]byte("model = \"synthetic-model\""),
	} {
		written := codexMustInstall(t, original)
		current := edit(written, steps...)
		if bytes.Equal(current, written) {
			t.Fatal("Codex made no edit; the check would not exercise a merge")
		}
		want := edit(original, steps...)
		got := codexMustMerge(t, original, written, current)
		if !bytes.Equal(got, want) {
			evidence, _ := json.Marshal(map[string]string{"written": string(written), "current": string(current), "merged": string(got), "codex_on_original": string(want)})
			t.Fatalf("merge differs from Codex editing the original: %s", evidence)
		}
		edit(got, []string{"features", "list"})
	}
}
