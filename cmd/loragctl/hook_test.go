package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
