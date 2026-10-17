package indexer

import (
	"strings"
	"unicode/utf8"

	"github.com/lorag/lorag/internal/parsers"
)

// Chunker splits text into chunks
type Chunker struct {
	chunkSize    int
	chunkOverlap int
}

// NewChunker creates a new chunker
func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 4
	}
	return &Chunker{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

// Chunk splits text into chunks
func (c *Chunker) Chunk(text string) []string {
	if text == "" {
		return []string{}
	}

	// Try to chunk by paragraphs first
	paragraphs := c.splitParagraphs(text)

	var chunks []string
	var currentChunk strings.Builder

	for _, para := range paragraphs {
		paraLen := utf8.RuneCountInString(para)
		currentLen := utf8.RuneCountInString(currentChunk.String())

		// If paragraph is larger than chunk size, split it
		if paraLen > c.chunkSize {
			// Flush current chunk if not empty
			if currentChunk.Len() > 0 {
				chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
				currentChunk.Reset()
			}

			// Split large paragraph
			paraChunks := c.splitBySize(para)
			chunks = append(chunks, paraChunks...)
			continue
		}

		// Check if adding this paragraph would exceed chunk size
		if currentLen+paraLen > c.chunkSize && currentChunk.Len() > 0 {
			chunks = append(chunks, strings.TrimSpace(currentChunk.String()))

			// Keep overlap
			overlap := c.getOverlap(currentChunk.String())
			currentChunk.Reset()
			currentChunk.WriteString(overlap)
		}

		if currentChunk.Len() > 0 {
			currentChunk.WriteString("\n\n")
		}
		currentChunk.WriteString(para)
	}

	// Don't forget the last chunk
	if currentChunk.Len() > 0 {
		chunks = append(chunks, strings.TrimSpace(currentChunk.String()))
	}

	return chunks
}

// splitParagraphs splits text into paragraphs
func (c *Chunker) splitParagraphs(text string) []string {
	// Split by double newline or multiple single newlines
	text = strings.ReplaceAll(text, "\r\n", "\n")

	var paragraphs []string
	var current strings.Builder

	lines := strings.Split(text, "\n")
	emptyCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			emptyCount++
			if emptyCount >= 1 && current.Len() > 0 {
				paragraphs = append(paragraphs, strings.TrimSpace(current.String()))
				current.Reset()
			}
		} else {
			emptyCount = 0
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(line)
		}
	}

	if current.Len() > 0 {
		paragraphs = append(paragraphs, strings.TrimSpace(current.String()))
	}

	// If no paragraphs found, treat entire text as one paragraph
	if len(paragraphs) == 0 && strings.TrimSpace(text) != "" {
		paragraphs = append(paragraphs, strings.TrimSpace(text))
	}

	return paragraphs
}

// splitBySize splits text by fixed size with word boundary awareness
func (c *Chunker) splitBySize(text string) []string {
	var chunks []string

	runes := []rune(text)
	start := 0

	for start < len(runes) {
		end := start + c.chunkSize
		if end > len(runes) {
			end = len(runes)
		}

		// Try to find a better break point
		if end < len(runes) {
			// Look for sentence ending or whitespace
			for i := end - 1; i > start+c.chunkSize/2; i-- {
				if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
					end = i + 1
					break
				}
			}

			// If no sentence break, try whitespace
			if end == start+c.chunkSize {
				for i := end - 1; i > start+c.chunkSize/2; i-- {
					if runes[i] == ' ' || runes[i] == '\n' || runes[i] == '\t' {
						end = i
						break
					}
				}
			}
		}

		chunk := string(runes[start:end])
		chunks = append(chunks, strings.TrimSpace(chunk))

		// Move start with overlap, but ensure we make progress
		if end >= len(runes) {
			break
		}
		start = end - c.chunkOverlap
		if start < 0 {
			start = 0
		}
		// Ensure we advance at least a little to avoid infinite loop
		if start >= end {
			start = end
		}
	}

	return chunks
}

// getOverlap returns the overlap portion of a chunk
func (c *Chunker) getOverlap(chunk string) string {
	runes := []rune(chunk)
	if len(runes) <= c.chunkOverlap {
		return chunk
	}

	start := len(runes) - c.chunkOverlap
	return string(runes[start:])
}

// CountTokens estimates token count (rough approximation)
func (c *Chunker) CountTokens(text string) int {
	// Rough approximation: 1 token ≈ 4 characters for English
	return utf8.RuneCountInString(text) / 4
}

// Piece is a chunk of text with its position in the source.
type Piece struct {
	Text      string
	Page      int
	LineStart int
	LineEnd   int
	StartByte int // -1 when unknown
	EndByte   int
	StartMs   int64
	EndMs     int64
	Section   string
}

// ChunkSegments splits positioned segments into pieces that keep their page,
// line range or time range. Small adjacent segments of the same page are
// merged first, timed segments are grouped up to the chunk size.
func (c *Chunker) ChunkSegments(segs []parsers.Segment) []Piece {
	var out []Piece
	for _, s := range mergeSmall(segs, c.chunkSize) {
		if s.Timed() {
			continue
		}
		cursor := 0
		for _, text := range c.Chunk(s.Text) {
			if text == "" {
				continue
			}
			p := Piece{Text: text, Page: s.Page, Section: s.Section, StartByte: -1, EndByte: -1}
			if s.Line > 0 || s.Offset >= 0 {
				cursor = locate(&p, s, text, cursor)
			}
			out = append(out, p)
		}
	}
	return append(out, c.groupTimed(segs)...)
}

// mergeSmall joins consecutive untimed segments of the same page while they
// fit in size. Line-addressable segments are verbatim slices of the source,
// so a merged segment is still one.
func mergeSmall(segs []parsers.Segment, size int) []parsers.Segment {
	var out []parsers.Segment
	for _, s := range segs {
		if s.Timed() || strings.TrimSpace(s.Text) == "" {
			if s.Timed() {
				out = append(out, s)
			}
			continue
		}
		if n := len(out); n > 0 {
			last := &out[n-1]
			contiguous := last.Offset >= 0 && s.Offset == last.Offset+len(last.Text)
			if !last.Timed() && last.Page == s.Page && contiguous &&
				utf8.RuneCountInString(last.Text)+utf8.RuneCountInString(s.Text) <= size {
				last.Text += s.Text
				continue
			}
		}
		out = append(out, s)
	}
	return out
}

// locate sets the line and byte range of text inside segment s, searching
// from cursor (pieces come in order), and returns the next cursor. The
// chunker trims and joins paragraphs, so the first and last lines of the
// piece are searched rather than the whole text.
func locate(p *Piece, s parsers.Segment, text string, cursor int) int {
	first, last := edgeLines(text)
	start := -1
	if i := strings.Index(s.Text[cursor:], first); i >= 0 {
		start = cursor + i
	} else if i := strings.Index(s.Text, first); i >= 0 {
		start = i
	}
	if start < 0 {
		return cursor
	}
	end := start + len(first)
	if i := strings.Index(s.Text[start:], last); i >= 0 && last != first {
		end = start + i + len(last)
	}
	if s.Line > 0 {
		p.LineStart = s.Line + strings.Count(s.Text[:start], "\n")
		p.LineEnd = s.Line + strings.Count(s.Text[:end], "\n")
	}
	if s.Offset >= 0 {
		p.StartByte, p.EndByte = s.Offset+start, s.Offset+end
	}
	return start + 1
}

// edgeLines returns the first and last non-empty lines of text, capped so
// a long wrapped paragraph still matches.
func edgeLines(text string) (string, string) {
	lines := strings.Split(text, "\n")
	pick := func(l string, head bool) string {
		l = strings.TrimSpace(l)
		r := []rune(l)
		if len(r) > 60 {
			if head {
				return string(r[:60])
			}
			return string(r[len(r)-60:])
		}
		return l
	}
	first, last := "", ""
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			first = pick(l, true)
			break
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			last = pick(lines[i], false)
			break
		}
	}
	return first, last
}

// maxTimedSpan bounds the time range of one media piece, so a result points
// at the right minute even in slow speech.
const maxTimedSpan = 60_000

// groupTimed groups transcript segments, in time order, into pieces of at
// most the chunk size and one minute; a piece spans from its first start to
// its last end.
func (c *Chunker) groupTimed(segs []parsers.Segment) []Piece {
	var out []Piece
	var cur *Piece
	for _, s := range segs {
		if !s.Timed() {
			continue
		}
		text := strings.TrimSpace(s.Text)
		tooLong := cur != nil && s.EndMs-cur.StartMs > maxTimedSpan
		if cur != nil && (tooLong || utf8.RuneCountInString(cur.Text)+1+utf8.RuneCountInString(text) > c.chunkSize) {
			out = append(out, *cur)
			cur = nil
		}
		if cur == nil {
			cur = &Piece{Text: text, StartMs: s.StartMs, EndMs: s.EndMs, StartByte: -1, EndByte: -1, Section: s.Section}
			continue
		}
		cur.Text += " " + text
		if s.EndMs > cur.EndMs {
			cur.EndMs = s.EndMs
		}
		if cur.Section != s.Section {
			cur.Section = ""
		}
	}
	if cur != nil {
		out = append(out, *cur)
	}
	return out
}
