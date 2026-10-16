package indexer

import (
	"fmt"
	"strings"
	"testing"

	"github.com/lorag/lorag/internal/parsers"
)

func TestChunkSegmentsLines(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Runbook\n\n")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&b, "Step %d: check the pods of service %d and restart them.\n\n", i, i)
	}
	b.WriteString("## Rollback\n\nRun helm rollback api 42 then watch the error rate.\n")
	src := b.String()
	lines := strings.Split(src, "\n")

	c := NewChunker(300, 40)
	pieces := c.ChunkSegments(parsers.MarkdownSegments(src))
	if len(pieces) < 5 {
		t.Fatalf("only %d pieces", len(pieces))
	}
	for _, p := range pieces {
		if p.LineStart == 0 || p.LineEnd < p.LineStart {
			t.Fatalf("no line range: %+v", p)
		}
		// The first and last lines of the piece are at the reported lines.
		first, last := edgeLines(p.Text)
		if !strings.Contains(lines[p.LineStart-1], first) {
			t.Fatalf("line %d is %q, piece starts with %q", p.LineStart, lines[p.LineStart-1], first)
		}
		if !strings.Contains(lines[p.LineEnd-1], last) {
			t.Fatalf("line %d is %q, piece ends with %q", p.LineEnd, lines[p.LineEnd-1], last)
		}
		if p.StartByte < 0 || !strings.HasPrefix(src[p.StartByte:], first) {
			t.Fatalf("byte offset %d does not point at %q", p.StartByte, first)
		}
	}
	lastPiece := pieces[len(pieces)-1]
	if lastPiece.Section != "Runbook › Rollback" || !strings.Contains(lastPiece.Text, "helm rollback") {
		t.Fatalf("last piece: %+v", lastPiece)
	}
	want := strings.Count(src[:strings.Index(src, "Run helm")], "\n") + 1
	if lastPiece.LineEnd != want {
		t.Fatalf("rollback line %d, want %d", lastPiece.LineEnd, want)
	}
}

func TestChunkSegmentsPagesAndTime(t *testing.T) {
	c := NewChunker(80, 10)
	segs := []parsers.Segment{
		{Text: "page one text", Page: 1, Offset: -1},
		{Text: "page two text", Page: 2, Offset: -1},
		{Text: "first sentence of the talk", StartMs: 0, EndMs: 3000, Offset: -1},
		{Text: "second sentence", StartMs: 3000, EndMs: 5000, Offset: -1},
		{Text: "a much later sentence that will not fit in the same piece anymore", StartMs: 61000, EndMs: 66000, Offset: -1},
	}
	pieces := c.ChunkSegments(segs)
	if len(pieces) != 4 {
		t.Fatalf("%d pieces: %+v", len(pieces), pieces)
	}
	if pieces[0].Page != 1 || pieces[1].Page != 2 {
		t.Fatalf("pages merged: %+v", pieces[:2])
	}
	if p := pieces[2]; p.StartMs != 0 || p.EndMs != 5000 || p.Text != "first sentence of the talk second sentence" {
		t.Fatalf("timed group: %+v", p)
	}
	if p := pieces[3]; p.StartMs != 61000 || p.EndMs != 66000 {
		t.Fatalf("second group: %+v", p)
	}
}
