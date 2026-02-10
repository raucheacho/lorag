package indexer

import (
	"strings"
	"unicode/utf8"
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
