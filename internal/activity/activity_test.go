package activity

import "testing"

func TestRingBufferOrderAndFilter(t *testing.T) {
	l := NewLog(3)
	for i, src := range []Source{SourceMCP, SourceAPI, SourceMCP, SourceStudio} {
		l.Record(Query{Text: string(rune('a' + i)), Source: src, DurationMs: float64(i)})
	}
	all := l.List(Filter{})
	if len(all) != 3 || all[0].Text != "d" || all[2].Text != "b" {
		t.Fatalf("unexpected order: %+v", all)
	}
	mcp := l.List(Filter{Source: SourceMCP})
	if len(mcp) != 1 || mcp[0].Text != "c" {
		t.Fatalf("filter failed: %+v", mcp)
	}
	if _, ok := l.Get(all[1].ID); !ok {
		t.Fatal("Get failed")
	}
	if s := l.Summarize(); s.Total != 3 || s.P50Ms != 2 {
		t.Fatalf("summary %+v", s)
	}
}
