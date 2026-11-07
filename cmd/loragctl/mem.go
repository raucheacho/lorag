package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// memCmd groups the problem-solution memory commands.
func memCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mem",
		Short: "Memory: recall and save fixes and answers",
		Long: `Recall what is already known before solving a problem or searching the web again,
and save what is verified and worth reusing: a fix, or an answer with its source.

  npm run dev 2>&1 | loragctl mem recall -        recall from a log on stdin
  loragctl mem save --error "$ERR" --fix "lsof -ti :3000 | xargs kill" --project shop
  loragctl mem save --kind knowledge --problem "GitHub API rate limit" --answer "5,000/h per user" --source https://docs.github.com/...
  loragctl mem feedback 3f2a9c41b7-1 --worked`,
	}
	cmd.AddCommand(memRecallCmd(), memSaveCmd(), memFeedbackCmd(), memListCmd(), memShowCmd(), memStatsCmd(), memExportCmd())
	return cmd
}

// argOrStdin joins the arguments, or reads stdin when the only one is "-".
func argOrStdin(args []string) (string, error) {
	if len(args) == 1 && args[0] == "-" {
		b, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
		return string(b), err
	}
	return strings.Join(args, " "), nil
}

func memRecallCmd() *cobra.Command {
	var project, query string
	var k, budget int
	var ctx []string
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "recall <error message | ->",
		Short: "Look up a known fix or answer (\"-\" reads an error from stdin)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			errText, err := argOrStdin(args)
			if err != nil {
				return err
			}
			context := map[string]string{}
			for _, c := range ctx {
				pkg, ver, ok := strings.Cut(c, "@")
				if !ok {
					return fmt.Errorf("--ctx expects package@version, got %q", c)
				}
				context[pkg] = ver
			}
			resp, err := apiPost("/memory/recall", map[string]any{
				"error": errText, "query": query, "project": project, "context": context, "k": k, "budget_tokens": budget,
			})
			if err != nil {
				return err
			}
			if asJSON {
				return printJSON(resp)
			}
			fmt.Println(resp["text"])
			return nil
		},
	}
	cmd.Flags().StringVarP(&project, "project", "p", "", "current project (its entries and global ones are searched)")
	cmd.Flags().StringVarP(&query, "query", "q", "", "the problem in words, in addition to the error")
	cmd.Flags().StringSliceVar(&ctx, "ctx", nil, "versions in use, e.g. --ctx expo@52.0.1,node@22")
	cmd.Flags().IntVarP(&k, "k", "k", 1, "number of fixes")
	cmd.Flags().IntVar(&budget, "budget", 0, "token budget of the answer (default 60)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print the full JSON answer")
	return cmd
}

func memSaveCmd() *cobra.Command {
	var kind, problem, fix, answer, source, errText, project, space, platform, replaces string
	var versions, tags []string
	var cost int
	cmd := &cobra.Command{
		Use:   "save",
		Short: "Save a fix or an answer (--error - reads the error from stdin)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if fix == "" {
				fix = answer
			}
			if fix == "" {
				return fmt.Errorf("--fix or --answer is required")
			}
			if errText == "-" {
				b, err := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
				if err != nil {
					return err
				}
				errText = string(b)
			}
			resp, err := apiPost("/memory/", map[string]any{
				"kind": kind, "problem": problem, "solution": fix, "source": source, "error": errText, "project": project, "space": space,
				"platform": platform, "versions": versions, "tags": tags, "solve_tokens": cost, "origin": "loragctl", "replaces": replaces,
			})
			if err != nil {
				return err
			}
			e, _ := resp["entry"].(map[string]any)
			fmt.Printf("%s %v [%v] %v\n", resp["action"], e["kind"], e["id"], e["problem"])
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "fix, knowledge or fact (default: fix with --error, else knowledge)")
	cmd.Flags().StringVar(&problem, "problem", "", "the problem, the question, or what the fact is about, in one line")
	cmd.Flags().StringVar(&fix, "fix", "", "the fix")
	cmd.Flags().StringVar(&answer, "answer", "", "the answer or the fact (same as --fix, for knowledge and facts)")
	cmd.Flags().StringVar(&replaces, "replaces", "", "id of an entry this one makes obsolete")
	cmd.Flags().StringVar(&source, "source", "", "where the answer comes from (URL, document)")
	cmd.Flags().StringVar(&errText, "error", "", "the error message, or - for stdin")
	cmd.Flags().StringVarP(&project, "project", "p", "", "project name")
	cmd.Flags().StringVar(&space, "space", "", "global, or a project (default: the project)")
	cmd.Flags().StringVar(&platform, "platform", "", "e.g. macos-arm64")
	cmd.Flags().StringSliceVar(&versions, "versions", nil, "e.g. expo@52,node@22")
	cmd.Flags().StringSliceVar(&tags, "tags", nil, "tags")
	cmd.Flags().IntVar(&cost, "solve-tokens", 0, "rough tokens the fix cost to find")
	return cmd
}

func memFeedbackCmd() *cobra.Command {
	var worked, failed bool
	var note string
	cmd := &cobra.Command{
		Use:   "feedback <id> --worked|--failed",
		Short: "Report whether a recalled entry helped",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if worked == failed {
				return fmt.Errorf("pass exactly one of --worked or --failed")
			}
			resp, err := apiPost("/memory/"+strings.Trim(args[0], "[]")+"/feedback", map[string]any{"worked": worked, "note": note})
			if err != nil {
				return err
			}
			fmt.Printf("[%v] %v (+%v/-%v)\n", resp["id"], resp["status"], resp["successes"], resp["failures"])
			return nil
		},
	}
	cmd.Flags().BoolVar(&worked, "worked", false, "the fix worked, or the answer was right")
	cmd.Flags().BoolVar(&failed, "failed", false, "it did not work, or the answer was wrong")
	cmd.Flags().StringVar(&note, "note", "", "what happened")
	return cmd
}

func memListCmd() *cobra.Command {
	var kind, status, space, query string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List memory entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			q := fmt.Sprintf("/memory/?kind=%s&status=%s&space=%s&q=%s&limit=%d", url.QueryEscape(kind), url.QueryEscape(status), url.QueryEscape(space), url.QueryEscape(query), limit)
			resp, err := apiGet(q)
			if err != nil {
				return err
			}
			entries, _ := resp["entries"].([]any)
			for _, x := range entries {
				e := x.(map[string]any)
				fmt.Printf("%-14v %-9v %-8v +%v/-%v  %-10v %v\n", e["id"], e["kind"], e["status"], e["successes"], e["failures"], e["space"], e["problem"])
			}
			fmt.Printf("%d entries\n", len(entries))
			return nil
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "fix or knowledge")
	cmd.Flags().StringVar(&status, "status", "", "valid, recheck or obsolete")
	cmd.Flags().StringVar(&space, "space", "", "global or a project")
	cmd.Flags().StringVarP(&query, "query", "q", "", "search entries by meaning and words")
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum entries")
	return cmd
}

func memShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show an entry and its history",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := apiGet("/memory/" + strings.Trim(args[0], "[]"))
			if err != nil {
				return err
			}
			return printJSON(resp)
		},
	}
}

func memStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Entries, recall hit rate and tokens saved",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := apiGet("/memory/stats")
			if err != nil {
				return err
			}
			fb, _ := s["feedback"].(map[string]any)
			fmt.Printf("entries      %v %v\n", s["entries"], s["by_status"])
			fmt.Printf("recalls      %v (hits %v, misses %v, hit rate %.0f%%)\n", s["recalls"], s["hits"], s["misses"], 100*num(s["hit_rate"]))
			fmt.Printf("tokens       %.0f per recall on average, about %v saved\n", num(s["avg_recall_tokens"]), s["tokens_saved_estimate"])
			fmt.Printf("feedback     %v worked, %v failed\n", fb["worked"], fb["failed"])
			return nil
		},
	}
}

func memExportCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "export",
		Short: "Write every entry as JSON Lines to stdout",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := http.Get(apiBaseURL + "/memory/export")
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("export failed: %s", resp.Status)
			}
			_, err = io.Copy(os.Stdout, resp.Body)
			return err
		},
	}
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func num(v any) float64 {
	f, _ := v.(float64)
	return f
}
