package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/lorag/lorag/internal/memory"
)

func TestNoMemoryMatchesTheMemory(t *testing.T) {
	if noMemory != memory.NoMemory {
		t.Fatalf("noMemory = %q, memory.NoMemory = %q", noMemory, memory.NoMemory)
	}
}

func TestClaudeCodeHook(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/memory/recall" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
		if strings.Contains(got["error"].(string), "EADDRINUSE") {
			_, _ = w.Write([]byte(`{"text":"[fix 3f2a] kill the process on :3000","matches":[{"by":"signature"}]}`))
			return
		}
		_, _ = w.Write([]byte(`{"text":"","matches":[]}`))
	}))
	defer srv.Close()
	old := apiBaseURL
	apiBaseURL = srv.URL + "/api/v1"
	defer func() { apiBaseURL = old }()

	event := `{"hook_event_name":"PostToolUseFailure","tool_name":"Bash","cwd":"/home/me/shop","is_interrupt":false,
		"error":"Exit code 1\nError: listen EADDRINUSE: address already in use :::3000"}`
	out := claudeCodeHook(strings.NewReader(event), "")
	if out == nil {
		t.Fatal("expected a recalled fix")
	}
	if got["project"] != "shop" || strings.HasPrefix(got["error"].(string), "Exit code") {
		t.Errorf("recall request = %v", got)
	}
	hso := out["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "PostToolUseFailure" || !strings.Contains(hso["additionalContext"].(string), "kill the process") {
		t.Errorf("output = %v", hso)
	}

	for name, event := range map[string]string{
		"miss":      `{"error":"Exit code 2\nsomething unknown"}`,
		"bare exit": `{"error":"Exit code 1"}`,
		"interrupt": `{"error":"Exit code 130\nEADDRINUSE","is_interrupt":true}`,
		"bad json":  `not json`,
	} {
		if out := claudeCodeHook(strings.NewReader(event), ""); out != nil {
			t.Errorf("%s: expected no output, got %v", name, out)
		}
	}

	// Lorag not running: said, not silent (TestHookSaysOnceWhenLoragIsUnreachable).
	apiBaseURL = "http://127.0.0.1:1/api/v1"
	if out := claudeCodeHook(strings.NewReader(event), ""); out == nil {
		t.Error("server down: expected an unreachable notice")
	}
}

func TestUnsavedMiss(t *testing.T) {
	miss := memoryCall{Tool: "memory_recall", Result: "No memory for this problem. Once you have solved it..."}
	hit := memoryCall{Tool: "memory_recall", Result: "[3f2a-1] +1/-0 2026-10-02\nFix: kill it"}
	save := memoryCall{Tool: "memory_save", Result: "Saved fix [3f2a-2] in shop."}
	for name, tc := range map[string]struct {
		calls []memoryCall
		want  bool
	}{
		"miss":           {[]memoryCall{miss}, true},
		"miss then save": {[]memoryCall{miss, save}, false},
		"save then miss": {[]memoryCall{save, miss}, true},
		"hit":            {[]memoryCall{hit}, false},
		"miss then hit":  {[]memoryCall{miss, hit}, true},
		"nothing":        {nil, false},
	} {
		if got := unsavedMiss(tc.calls); got != tc.want {
			t.Errorf("%s: got %v", name, got)
		}
	}
	for name, want := range map[string]string{
		"memory_save": "memory_save", "mcp__plugin_lorag_lorag__memory_recall": "memory_recall",
		"lorag.memory_feedback": "memory_feedback", "search": "", "my_memory_save_tool": "",
	} {
		if got := memoryTool(name); got != want {
			t.Errorf("memoryTool(%q) = %q", name, got)
		}
	}
}

// claudeTranscript builds a Claude Code transcript in its JSONL format.
func claudeTranscript(t *testing.T, lines ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestClaudeCodeStopHook(t *testing.T) {
	prompt := func(text string) string { return `{"type":"user","message":{"role":"user","content":"` + text + `"}}` }
	use := func(id, name string) string {
		return `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"` + id + `","name":"` + name + `","input":{}}]}}`
	}
	result := func(id, text string) string {
		return `{"type":"user","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"` + id + `","content":[{"type":"text","text":"` + text + `"}]}]}}`
	}
	const recall, save = "mcp__plugin_lorag_lorag__memory_recall", "mcp__plugin_lorag_lorag__memory_save"
	const missText = "No memory for this problem. Once you have solved it, save it."

	stop := func(path string, active bool) map[string]any {
		ev, _ := json.Marshal(map[string]any{"hook_event_name": "Stop", "transcript_path": path, "stop_hook_active": active})
		return claudeCodeHook(strings.NewReader(string(ev)), "")
	}

	unsaved := claudeTranscript(t, prompt("fix the build"), use("a", recall), result("a", missText), use("b", "Bash"), result("b", "ok"))
	out := stop(unsaved, false)
	if out == nil || out["decision"] != "block" || !strings.Contains(out["reason"].(string), "memory_save") {
		t.Fatalf("unsaved miss: %v", out)
	}
	if out := stop(unsaved, true); out != nil {
		t.Errorf("stop_hook_active: expected no output, got %v", out)
	}

	for name, path := range map[string]string{
		"saved": claudeTranscript(t, prompt("fix"), use("a", recall), result("a", missText), use("b", save), result("b", "Saved fix [x] in shop.")),
		"hit":   claudeTranscript(t, prompt("fix"), use("a", recall), result("a", "[x-1] +1/-0 2026-10-02\nFix: y")),
		// The miss belongs to an earlier turn.
		"earlier turn": claudeTranscript(t, prompt("fix"), use("a", recall), result("a", missText), prompt("thanks")),
		// A subagent's calls are not this turn's.
		"sidechain": claudeTranscript(t, prompt("fix"), strings.Replace(use("a", recall), `{"type"`, `{"isSidechain":true,"type"`, 1), result("a", missText)),
		"no file":   filepath.Join(t.TempDir(), "missing.jsonl"),
	} {
		if out := stop(path, false); out != nil {
			t.Errorf("%s: expected no output, got %v", name, out)
		}
	}
}

// fakeMemory answers recalls with the given body, whatever was asked, and
// records the last request.
func fakeMemory(t *testing.T, status int, body string) *map[string]any {
	t.Helper()
	got := map[string]any{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	old := apiBaseURL
	apiBaseURL = srv.URL + "/api/v1"
	t.Cleanup(func() { apiBaseURL = old; srv.Close() })
	return &got
}

func TestHookKeepsOnlyConfidentMatches(t *testing.T) {
	// 0.51: accepted by the memory thanks to shared words, below its 0.55
	// threshold. A hook must not inject it.
	fakeMemory(t, 200, `{"text":"[fix a] firebase","matches":[{"by":"similarity","similarity":0.51}],"thresholds":{"similarity":0.55}}`)
	if out := claudeCodeHook(strings.NewReader(`{"error":"Exit code 128\nworkflow scope"}`), ""); out != nil {
		t.Errorf("weak match injected: %v", out)
	}

	fakeMemory(t, 200, `{"text":"[fix b] scene lifecycle","matches":[{"by":"similarity","similarity":0.76}],"thresholds":{"similarity":0.55}}`)
	if out := claudeCodeHook(strings.NewReader(`{"error":"Exit code 1\nNoSceneLifecycleAdoption"}`), ""); out == nil {
		t.Error("confident match dropped")
	}
}

func TestHookRecallsOnPrompt(t *testing.T) {
	got := fakeMemory(t, 200, `{"text":"[fix c] scene lifecycle","matches":[{"by":"similarity","similarity":0.7}],"thresholds":{"similarity":0.55}}`)
	event := `{"hook_event_name":"UserPromptSubmit","cwd":"/home/me/goul","prompt":"sur ios l'app quitte brutalement au lancement"}`
	out := claudeCodeHook(strings.NewReader(event), "")
	if out == nil {
		t.Fatal("expected a recalled entry")
	}
	if (*got)["query"] != "sur ios l'app quitte brutalement au lancement" || (*got)["project"] != "goul" {
		t.Errorf("recall request = %v", *got)
	}
	hso := out["hookSpecificOutput"].(map[string]any)
	if hso["hookEventName"] != "UserPromptSubmit" || !strings.Contains(hso["additionalContext"].(string), "scene lifecycle") {
		t.Errorf("output = %v", hso)
	}

	for _, prompt := range []string{"ok merge", "/code-review high"} {
		ev, _ := json.Marshal(map[string]any{"hook_event_name": "UserPromptSubmit", "prompt": prompt})
		if out := claudeCodeHook(strings.NewReader(string(ev)), ""); out != nil {
			t.Errorf("%q: expected no recall, got %v", prompt, out)
		}
	}
}

func TestHookSaysOnceWhenLoragIsUnreachable(t *testing.T) {
	// Another program on the port: the 404 that hid a port conflict.
	fakeMemory(t, 404, `404 page not found`)
	session := "test-" + strings.ReplaceAll(t.Name(), "/", "-") + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { os.Remove(filepath.Join(os.TempDir(), "lorag-unreachable-"+session)) })
	event := `{"hook_event_name":"PostToolUseFailure","error":"Exit code 1\nboom","session_id":"` + session + `"}`

	out := claudeCodeHook(strings.NewReader(event), "")
	if out == nil {
		t.Fatal("expected an unreachable notice")
	}
	ctx := out["hookSpecificOutput"].(map[string]any)["additionalContext"].(string)
	if !strings.Contains(ctx, "unreachable") || !strings.Contains(ctx, "HTTP 404") {
		t.Errorf("notice = %q", ctx)
	}
	if out := claudeCodeHook(strings.NewReader(event), ""); out != nil {
		t.Errorf("second notice in the same session: %v", out)
	}
}

func TestHookStaysQuietWhenARecallFails(t *testing.T) {
	// Lorag is up and answers in JSON: the embedder refused this text. Not a
	// reason to tell the user to start a server that runs.
	got := fakeMemory(t, 400, `{"error":"embed query: ollama returned status 500"}`)
	session := "test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { os.Remove(filepath.Join(os.TempDir(), "lorag-unreachable-"+session)) })
	long := strings.Repeat("Failed to update prerender cache for /uwsgi.ini ", 200)
	ev, _ := json.Marshal(map[string]any{"hook_event_name": "UserPromptSubmit", "prompt": long, "session_id": session})

	if out := claudeCodeHook(strings.NewReader(string(ev)), ""); out != nil {
		t.Errorf("notice for a failed recall: %v", out)
	}
	if n := len([]rune((*got)["query"].(string))); n != maxQueryRunes {
		t.Errorf("query sent with %d runes, want %d", n, maxQueryRunes)
	}
}
