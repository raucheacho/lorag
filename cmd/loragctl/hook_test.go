package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

	apiBaseURL = "http://127.0.0.1:1/api/v1" // Lorag not running
	if out := claudeCodeHook(strings.NewReader(event), ""); out != nil {
		t.Errorf("server down: expected no output, got %v", out)
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
