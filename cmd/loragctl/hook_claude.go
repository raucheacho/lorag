package main

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Claude Code adapter: reads Claude Code hook events on stdin and answers in
// its hook output format.

// claudeHookInput is the part of a Claude Code hook event the adapter reads.
type claudeHookInput struct {
	HookEventName  string `json:"hook_event_name"`
	Error          string `json:"error"`
	IsInterrupt    bool   `json:"is_interrupt"`
	Cwd            string `json:"cwd"`
	TranscriptPath string `json:"transcript_path"`
	StopHookActive bool   `json:"stop_hook_active"`
}

func hookClaudeCodeCmd() *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "claude-code",
		Short: "Claude Code hook: adds a known fix to a failed command, reminds to save what was solved",
		Long: `Reads a Claude Code hook event on stdin.

PostToolUseFailure: recalls a fix for the error and returns it as additional
context when the memory has one.

Stop: when a memory recall in this turn found nothing and no memory_save
followed, asks Claude once to save the solution if it has one. It never
asks twice in a row (stop_hook_active).

It never fails the tool call or the turn: when Lorag is not running, the
error is unknown or nothing matches, it prints nothing and exits 0.

Register it in ~/.claude/settings.json:

  {
    "hooks": {
      "PostToolUseFailure": [
        {"matcher": "Bash", "hooks": [{"type": "command", "command": "loragctl hook claude-code", "timeout": 10}]}
      ],
      "Stop": [
        {"hooks": [{"type": "command", "command": "loragctl hook claude-code", "timeout": 10}]}
      ]
    }
  }`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			if out := claudeCodeHook(os.Stdin, project); out != nil {
				_ = json.NewEncoder(os.Stdout).Encode(out)
			}
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name (default: the name of the working directory)")
	return cmd
}

// claudeCodeHook returns the hook output, or nil when there is nothing to add.
func claudeCodeHook(stdin io.Reader, project string) map[string]any {
	var in claudeHookInput
	if err := json.NewDecoder(io.LimitReader(stdin, 1<<20)).Decode(&in); err != nil {
		return nil
	}
	if in.HookEventName == "Stop" {
		return claudeStop(in)
	}
	if in.IsInterrupt {
		return nil
	}
	if project == "" && in.Cwd != "" {
		project = filepath.Base(in.Cwd)
	}
	fix := recallFix(in.Error, project)
	if fix == "" {
		return nil
	}
	event := in.HookEventName
	if event == "" {
		event = "PostToolUseFailure"
	}
	return map[string]any{
		"hookSpecificOutput": map[string]any{
			"hookEventName":     event,
			"additionalContext": recalledFixContext(fix),
		},
	}
}

// claudeStop blocks the end of the turn once when it left an unsaved miss.
func claudeStop(in claudeHookInput) map[string]any {
	if in.StopHookActive || in.TranscriptPath == "" {
		return nil
	}
	f, err := os.Open(in.TranscriptPath)
	if err != nil {
		return nil
	}
	defer f.Close()
	if !unsavedMiss(claudeTurnCalls(f)) {
		return nil
	}
	return map[string]any{"decision": "block", "reason": saveReminder}
}

// claudeLine is the part of a Claude Code transcript line the adapter reads.
type claudeLine struct {
	Type        string `json:"type"`
	IsSidechain bool   `json:"isSidechain"`
	IsMeta      bool   `json:"isMeta"`
	Message     struct {
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

type claudeBlock struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

// claudeTurnCalls returns the memory calls of the last turn of a Claude Code
// transcript (JSONL): those after the user's last prompt, in order.
func claudeTurnCalls(r io.Reader) []memoryCall {
	var calls []memoryCall
	index := map[string]int{} // tool_use id -> position in calls
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 32<<20)
	for sc.Scan() {
		var l claudeLine
		if json.Unmarshal(sc.Bytes(), &l) != nil || l.IsSidechain {
			continue
		}
		var blocks []claudeBlock
		if json.Unmarshal(l.Message.Content, &blocks) != nil {
			// Plain string content: a prompt typed by the user.
			if l.Type == "user" && !l.IsMeta && len(l.Message.Content) > 0 {
				calls, index = nil, map[string]int{}
			}
			continue
		}
		switch l.Type {
		case "assistant":
			for _, b := range blocks {
				if t := memoryTool(b.Name); b.Type == "tool_use" && t != "" {
					index[b.ID] = len(calls)
					calls = append(calls, memoryCall{Tool: t})
				}
			}
		case "user":
			prompt := !l.IsMeta
			for _, b := range blocks {
				if b.Type != "tool_result" {
					continue
				}
				prompt = false
				if i, ok := index[b.ToolUseID]; ok {
					calls[i].Result = blockText(b.Content)
				}
			}
			if prompt {
				calls, index = nil, map[string]int{}
			}
		}
	}
	if sc.Err() != nil {
		return nil // unreadable transcript: no reminder rather than a wrong one
	}
	return calls
}

// blockText returns a tool result's text, given as a string or as text blocks.
func blockText(raw json.RawMessage) string {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) != nil {
		return ""
	}
	out := ""
	for _, p := range parts {
		if p.Type == "text" {
			out += p.Text
		}
	}
	return out
}
