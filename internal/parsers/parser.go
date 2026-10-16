package parsers

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode"
)

// Parser is the interface for document parsers that work with io.Reader
type Parser interface {
	Parse(reader io.Reader) (string, error)
	Supports(ext string) bool
}

// FileParser is the interface for parsers that need a file path
type FileParser interface {
	ParseFile(path string) (string, error)
	Supports(ext string) bool
}

// Registry holds all available parsers
type Registry struct {
	parsers     []Parser
	fileParsers []FileParser
	cfg         ParserConfig
}

// ParserConfig holds configuration for parsers
type ParserConfig struct {
	OcrEnabled           bool
	OcrModel             string
	OllamaURL            string
	TranscriptionEnabled bool
	WhisperModel         WhisperModel // tiny, base, or small
	DataDir              string       // Directory for models
}

// NewRegistry creates a new parser registry
func NewRegistry() *Registry {
	return NewRegistryWithConfig(ParserConfig{
		OcrEnabled:           false,
		OcrModel:             "llava:7b",
		OllamaURL:            "http://localhost:11434",
		TranscriptionEnabled: false,
		WhisperModel:         WhisperTiny,
		DataDir:              "",
	})
}

// NewRegistryWithConfig creates a new parser registry with custom config
func NewRegistryWithConfig(cfg ParserConfig) *Registry {
	return &Registry{
		cfg: cfg,
		parsers: []Parser{
			&TextParser{},
			&MarkdownParser{},
			NewCodeParser(),
		},
		fileParsers: []FileParser{
			&PDFParser{},
			&DOCXParser{},
			&PPTXParser{},
			NewImageParserWithConfig(cfg.OllamaURL, cfg.OcrModel),
			NewAudioParserWithConfig(cfg.DataDir, cfg.WhisperModel),
			NewVideoParserWithConfig(cfg.DataDir, cfg.WhisperModel),
		},
	}
}

// Register adds a parser to the registry
func (r *Registry) Register(p Parser) {
	r.parsers = append(r.parsers, p)
}

// RegisterFileParser adds a file parser to the registry
func (r *Registry) RegisterFileParser(p FileParser) {
	r.fileParsers = append(r.fileParsers, p)
}

// GetParser returns a parser for the given file extension
func (r *Registry) GetParser(ext string) (Parser, error) {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	
	for _, p := range r.parsers {
		if p.Supports(ext) {
			return p, nil
		}
	}
	
	return nil, fmt.Errorf("no parser found for extension: %s", ext)
}

// GetFileParser returns a file parser for the given file extension
func (r *Registry) GetFileParser(ext string) (FileParser, error) {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	
	for _, p := range r.fileParsers {
		if p.Supports(ext) {
			return p, nil
		}
	}
	
	return nil, fmt.Errorf("no file parser found for extension: %s", ext)
}

// GetParserForPath returns appropriate parser for the given file path
func (r *Registry) GetParserForPath(path string) (interface{}, error) {
	ext := strings.ToLower(filepath.Ext(path))
	
	// First check file parsers (PDF, DOCX)
	for _, p := range r.fileParsers {
		if p.Supports(ext) {
			return p, nil
		}
	}
	
	// Then check regular parsers
	for _, p := range r.parsers {
		if p.Supports(ext) {
			return p, nil
		}
	}
	
	return nil, fmt.Errorf("no parser found for path: %s", path)
}

// Supports checks if a file extension is supported
func (r *Registry) Supports(ext string) bool {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	
	for _, p := range r.parsers {
		if p.Supports(ext) {
			return true
		}
	}
	
	for _, p := range r.fileParsers {
		if p.Supports(ext) {
			return true
		}
	}
	
	return false
}

// TextParser parses plain text files
type TextParser struct{}

func (t *TextParser) Parse(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read text: %w", err)
	}
	return string(data), nil
}

func (t *TextParser) Supports(ext string) bool {
	return ext == ".txt" || ext == ".text"
}

// MarkdownParser parses markdown files
type MarkdownParser struct{}

func (m *MarkdownParser) Parse(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read markdown: %w", err)
	}
	return string(data), nil
}

func (m *MarkdownParser) Supports(ext string) bool {
	return ext == ".md" || ext == ".markdown" || ext == ".mdx"
}

// PDFParser parses PDF files
type PDFParser struct{}

func (p *PDFParser) ParseFile(path string) (string, error) {
	// First, try to use pdftotext if available (best quality)
	if text, err := p.extractWithPdftotext(path); err == nil && len(text) > 100 {
		return text, nil
	}
	
	// Fallback: try to use pdfplumber or PyMuPDF via Python
	if text, err := p.extractWithPython(path); err == nil && len(text) > 100 {
		return text, nil
	}
	
	// Last resort: manual heuristic extraction
	return p.extractWithHeuristics(path)
}

func (p *PDFParser) extractWithPdftotext(path string) (string, error) {
	// Check if pdftotext (poppler-utils) is available
	if _, err := exec.LookPath("pdftotext"); err != nil {
		return "", fmt.Errorf("pdftotext not found")
	}
	
	// Create temp output file
	tmpFile, err := os.CreateTemp("", "pdf_extract_*.txt")
	if err != nil {
		return "", err
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())
	
	// Run pdftotext
	cmd := exec.Command("pdftotext", "-layout", path, tmpFile.Name())
	if err := cmd.Run(); err != nil {
		return "", err
	}
	
	// Read output
	data, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		return "", err
	}
	
	return string(data), nil
}

func (p *PDFParser) extractWithPython(path string) (string, error) {
	// Try pdfplumber first, then PyMuPDF
	script := `
import sys
try:
    import pdfplumber
    with pdfplumber.open(sys.argv[1]) as pdf:
        text = "\n".join(page.extract_text() or "" for page in pdf.pages)
        print(text)
except:
    try:
        import fitz  # PyMuPDF
        doc = fitz.open(sys.argv[1])
        text = "\n".join(page.get_text() for page in doc)
        print(text)
    except:
        sys.exit(1)
`
	cmd := exec.Command("python3", "-c", script, path)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func (p *PDFParser) extractWithHeuristics(path string) (string, error) {
	const maxSize = 10 * 1024 * 1024
	
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open pdf: %w", err)
	}
	defer file.Close()
	
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat pdf: %w", err)
	}
	
	size := info.Size()
	if size > maxSize {
		size = maxSize
	}
	
	data := make([]byte, size)
	_, err = file.Read(data)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read pdf: %w", err)
	}
	
	content := string(data)
	
	// Check if it's actually a PDF
	if !strings.HasPrefix(content, "%PDF") {
		// Not a valid PDF, return metadata only
		return fmt.Sprintf("[PDF Document: %s]\nFile size: %d bytes\n[Unable to extract text - install pdftotext or python-pdfplumber for better extraction]", 
			filepath.Base(path), info.Size()), nil
	}
	
	// Try to extract text using heuristics
	extracted := extractPDFText(content)
	extracted = filterPDFNoise(extracted)
	extracted = strings.Join(strings.Fields(extracted), " ")
	
	if len(extracted) < 200 {
		extracted = extractPDFMetadata(content, path, info.Size())
	}
	
	if len(extracted) > 200000 {
		extracted = extracted[:200000] + "\n[Content truncated...]"
	}
	
	return extracted, nil
}

// extractPDFText extracts text from PDF content using parenthesis parsing
func extractPDFText(content string) string {
	var result strings.Builder
	inString := false
	stringStart := 0
	
	for i := 0; i < len(content); i++ {
		if content[i] == '(' {
			// Check if this is likely a text string (not part of binary data)
			printableCount := 0
			totalCount := 0
			for j := i + 1; j < len(content) && j < i+100 && content[j] != ')'; j++ {
				if content[j] == '\\' && j+1 < len(content) {
					j++
					continue
				}
				totalCount++
				if unicode.IsPrint(rune(content[j])) || content[j] == ' ' || content[j] == '\n' {
					printableCount++
				}
			}
			// Accept if more than 60% printable and reasonable length
			if totalCount > 0 && float64(printableCount)/float64(totalCount) > 0.6 && totalCount < 500 {
				inString = true
				stringStart = i + 1
			}
		} else if content[i] == ')' && inString {
			text := content[stringStart:i]
			text = cleanPDFEscaped(text)
			if len(text) > 0 && isValidPDFText(text) {
				result.WriteString(text)
				result.WriteString(" ")
			}
			inString = false
		}
	}
	
	return strings.TrimSpace(result.String())
}

// cleanPDFEscaped cleans escaped characters in PDF strings
func cleanPDFEscaped(s string) string {
	var result strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result.WriteByte(' ')
			case 'r':
				result.WriteByte(' ')
			case 't':
				result.WriteByte(' ')
			case 'b', 'f':
				// Skip
			case '(', ')', '\\':
				result.WriteByte(s[i+1])
			default:
				// Skip unknown escape sequences
			}
			i++
		} else if s[i] >= 32 || s[i] == '\n' || s[i] == '\t' {
			result.WriteByte(s[i])
		}
	}
	return strings.TrimSpace(result.String())
}

// isValidPDFText checks if extracted text is valid (not just numbers/operators)
func isValidPDFText(s string) bool {
	if len(s) == 0 {
		return false
	}
	
	// Check if it has at least some letters
	hasLetter := false
	for _, r := range s {
		if unicode.IsLetter(r) {
			hasLetter = true
			break
		}
	}
	if !hasLetter {
		return false
	}
	
	// Check printable ratio
	printable := 0
	for _, r := range s {
		if unicode.IsPrint(r) || unicode.IsSpace(r) {
			printable++
		}
	}
	return float64(printable)/float64(len(s)) > 0.7
}

// filterPDFNoise removes PDF operators and other noise
func filterPDFNoise(s string) string {
	// Common PDF operators to remove
	noise := []string{
		"Tf", "Tj", "TJ", "Td", "TD", "Tm", "T*", "Tc", "Tw", "Tz", "TL", "Tr", "Ts",
		"g", "rg", "k", "K", "G", "RG", "W", "W*", "n", "f", "f*", "B", "B*", "b", "b*",
		"S", "s", "h", "re", "m", "l", "c", "v", "y", "Q", "q", "cm", "w", "J", "j",
		"M", "d", "i", "ri", "gs", "CS", "cs", "SC", "SCN", "sc", "scn",
		"BI", "ID", "EI", "sh", "Do", "MP", "DP", "BMC", "BDC", "EMC", "BX", "EX",
		"BT", "ET",
	}
	
	words := strings.Fields(s)
	var filtered []string
	
	for _, word := range words {
		// Skip empty words
		if len(word) == 0 {
			continue
		}
		
		// Skip PDF names (start with /)
		if strings.HasPrefix(word, "/") {
			continue
		}
		
		// Skip hex strings
		if isHexString(word) && len(word) > 8 {
			continue
		}
		
		// Skip pure numbers
		if isPureNumber(word) {
			continue
		}
		
		// Skip PDF operators
		isNoise := false
		for _, n := range noise {
			if word == n {
				isNoise = true
				break
			}
		}
		if isNoise {
			continue
		}
		
		// Skip date patterns like D:20211103112043
		if strings.HasPrefix(word, "D:") && len(word) > 10 {
			continue
		}
		
		// Skip UUID-like patterns
		if isUUIDLike(word) {
			continue
		}
		
		filtered = append(filtered, word)
	}
	
	return strings.Join(filtered, " ")
}

func isHexString(s string) bool {
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return len(s) > 0
}

func isPureNumber(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) && c != '.' && c != '-' {
			return false
		}
	}
	return len(s) > 0
}

func isUUIDLike(s string) bool {
	// Simple check for UUID patterns
	if len(s) < 30 {
		return false
	}
	dashCount := strings.Count(s, "-")
	return dashCount >= 4
}

// extractPDFMetadata extracts metadata and creates a text representation
func extractPDFMetadata(content, path string, size int64) string {
	var result strings.Builder
	
	result.WriteString(fmt.Sprintf("PDF Document: %s\n", filepath.Base(path)))
	result.WriteString(fmt.Sprintf("File size: %d bytes\n\n", size))
	
	// Extract common metadata fields
	keywords := []string{"/Title", "/Subject", "/Author", "/Creator", "/Producer"}
	for _, kw := range keywords {
		if val := extractPDFValue(content, kw); val != "" {
			result.WriteString(fmt.Sprintf("%s: %s\n", strings.TrimPrefix(kw, "/"), val))
		}
	}
	
	return result.String()
}

// extractPDFValue extracts a value after a PDF keyword
func extractPDFValue(content, keyword string) string {
	idx := strings.Index(content, keyword)
	if idx == -1 || idx+len(keyword) >= len(content) {
		return ""
	}
	
	start := idx + len(keyword)
	// Skip whitespace
	for start < len(content) && unicode.IsSpace(rune(content[start])) {
		start++
	}
	
	// Check if it's a string in parentheses
	if start < len(content) && content[start] == '(' {
		end := start + 1
		for end < len(content) && content[end] != ')' {
			if content[end] == '\\' && end+1 < len(content) {
				end += 2
			} else {
				end++
			}
		}
		if end < len(content) {
			text := content[start+1 : end]
			text = cleanPDFEscaped(text)
			return text
		}
	}
	
	return ""
}

func (p *PDFParser) Supports(ext string) bool {
	return ext == ".pdf"
}

// DOCXParser parses Word documents
type DOCXParser struct{}

func (d *DOCXParser) ParseFile(path string) (string, error) {
	// DOCX files are ZIP archives containing XML files
	// The main text is in word/document.xml
	
	r, err := zip.OpenReader(path)
	if err != nil {
		// Fallback: try to extract as raw text
		return extractRawDOCX(path)
	}
	defer r.Close()
	
	var textContent strings.Builder
	
	for _, f := range r.File {
		// Extract text from word/document.xml
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			defer rc.Close()
			
			data, err := io.ReadAll(rc)
			if err != nil {
				continue
			}
			
			text := extractTextFromDOCXXML(string(data))
			textContent.WriteString(text)
		}
		
		// Also check for headers and footers
		if strings.HasPrefix(f.Name, "word/header") || strings.HasPrefix(f.Name, "word/footer") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			text := extractTextFromDOCXXML(string(data))
			if len(text) > 0 {
				textContent.WriteString("\n")
				textContent.WriteString(text)
			}
		}
	}
	
	result := strings.TrimSpace(textContent.String())
	if len(result) == 0 {
		return extractRawDOCX(path)
	}
	
	if len(result) > 100000 {
		result = result[:100000] + "\n[Content truncated...]"
	}
	
	return result, nil
}

// extractTextFromDOCXXML extracts text from DOCX XML content
func extractTextFromDOCXXML(xmlContent string) string {
	// Simple XML tag stripping - look for <w:t> tags which contain text
	var result strings.Builder
	
	// Use a simple state machine to extract text from w:t tags
	inTextTag := false
	var textBuffer strings.Builder
	
	for i := 0; i < len(xmlContent)-4; i++ {
		if xmlContent[i:i+4] == "<w:t" {
			// Found start of text tag
			inTextTag = true
			// Find the closing >
			for j := i + 4; j < len(xmlContent) && xmlContent[j] != '>'; j++ {
				if xmlContent[j] == '/' && j+1 < len(xmlContent) && xmlContent[j+1] == '>' {
					// Empty tag
					inTextTag = false
					break
				}
				i = j
			}
			textBuffer.Reset()
		} else if inTextTag && xmlContent[i:i+5] == "</w:t>" {
			// Found end of text tag
			inTextTag = false
			if textBuffer.Len() > 0 {
				result.WriteString(textBuffer.String())
			}
			i += 4
		} else if inTextTag {
			// Collect text content
			c := xmlContent[i]
			// Handle XML entities
			if c == '&' {
				if i+4 < len(xmlContent) && xmlContent[i:i+4] == "&lt;" {
					textBuffer.WriteByte('<')
					i += 3
				} else if i+4 < len(xmlContent) && xmlContent[i:i+4] == "&gt;" {
					textBuffer.WriteByte('>')
					i += 3
				} else if i+5 < len(xmlContent) && xmlContent[i:i+5] == "&amp;" {
					textBuffer.WriteByte('&')
					i += 4
				} else if i+6 < len(xmlContent) && xmlContent[i:i+6] == "&quot;" {
					textBuffer.WriteByte('"')
					i += 5
				} else if i+6 < len(xmlContent) && xmlContent[i:i+6] == "&apos;" {
					textBuffer.WriteByte('\'')
					i += 5
				} else if i+1 < len(xmlContent) && xmlContent[i+1] == '#' {
					// Numeric entity
					end := i + 2
					for end < len(xmlContent) && xmlContent[end] != ';' {
						end++
					}
					if end < len(xmlContent) {
						entity := xmlContent[i+2 : end]
						if val, err := parseNumericEntity(entity); err == nil {
							textBuffer.WriteRune(val)
						}
						i = end
					}
				} else {
					textBuffer.WriteByte(c)
				}
			} else {
				textBuffer.WriteByte(c)
			}
		}
	}
	
	return result.String()
}

func parseNumericEntity(entity string) (rune, error) {
	var val int
	if strings.HasPrefix(entity, "x") || strings.HasPrefix(entity, "X") {
		_, err := fmt.Sscanf(entity[1:], "%x", &val)
		if err != nil {
			return 0, err
		}
	} else {
		_, err := fmt.Sscanf(entity, "%d", &val)
		if err != nil {
			return 0, err
		}
	}
	return rune(val), nil
}

// extractRawDOCX extracts text by reading the DOCX file as binary and looking for text
func extractRawDOCX(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open docx: %w", err)
	}
	defer file.Close()
	
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat docx: %w", err)
	}
	
	const maxSize = 2 * 1024 * 1024
	size := info.Size()
	if size > maxSize {
		size = maxSize
	}
	
	data := make([]byte, size)
	_, err = file.Read(data)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read docx: %w", err)
	}
	
	// Try to decode as UTF-8 and extract readable text
	content := string(data)
	var result strings.Builder
	
	// Look for sequences of printable ASCII characters
	lastWasSpace := true
	for i := 0; i < len(content); i++ {
		c := content[i]
		// Accept printable ASCII and common Unicode
		if c >= 32 && c < 127 {
			result.WriteByte(c)
			lastWasSpace = false
		} else if !lastWasSpace && (c == 0 || c > 127) {
			result.WriteByte(' ')
			lastWasSpace = true
		}
	}
	
	extracted := strings.TrimSpace(result.String())
	extracted = strings.Join(strings.Fields(extracted), " ")
	
	if len(extracted) < 50 {
		return fmt.Sprintf("[DOCX content from %s - %d bytes]", filepath.Base(path), info.Size()), nil
	}
	
	return extracted, nil
}

func (d *DOCXParser) Supports(ext string) bool {
	return ext == ".docx" || ext == ".doc"
}

// PPTXParser parses PowerPoint presentations
type PPTXParser struct{}

func (p *PPTXParser) ParseFile(path string) (string, error) {
	// PPTX files are ZIP archives containing XML files
	// The slide content is in ppt/slides/slide*.xml
	
	r, err := zip.OpenReader(path)
	if err != nil {
		return p.extractRawPPTX(path)
	}
	defer r.Close()
	
	var textContent strings.Builder
	slideCount := 0
	
	for _, f := range r.File {
		// Extract text from slide files
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slideCount++
			rc, err := f.Open()
			if err != nil {
				continue
			}
			
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			
			text := extractTextFromPPTXXML(string(data))
			if len(text) > 0 {
				textContent.WriteString(fmt.Sprintf("\n=== Slide %d ===\n", slideCount))
				textContent.WriteString(text)
				textContent.WriteString("\n")
			}
		}
		
		// Also extract notes if present
		if strings.HasPrefix(f.Name, "ppt/notesSlides/notesSlide") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				continue
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				continue
			}
			text := extractTextFromPPTXXML(string(data))
			if len(text) > 0 {
				textContent.WriteString("\n[Notes]: ")
				textContent.WriteString(text)
				textContent.WriteString("\n")
			}
		}
	}
	
	result := strings.TrimSpace(textContent.String())
	if len(result) == 0 {
		return p.extractRawPPTX(path)
	}
	
	if len(result) > 100000 {
		result = result[:100000] + "\n[Content truncated...]"
	}
	
	return result, nil
}

// extractTextFromPPTXXML extracts text from PPTX XML content
func extractTextFromPPTXXML(xmlContent string) string {
	var result strings.Builder
	
	// PPTX uses <a:t> tags for text (DrawingML namespace)
	inTextTag := false
	
	i := 0
	for i < len(xmlContent) {
		if !inTextTag && i < len(xmlContent)-4 && xmlContent[i:i+4] == "<a:t" {
			// Found start of text tag, find the closing >
			i += 4
			for i < len(xmlContent) && xmlContent[i] != '>' {
				i++
			}
			// Check if it's a self-closing tag
			if i > 0 && xmlContent[i-1] == '/' {
				inTextTag = false
			} else {
				inTextTag = true
			}
			i++ // skip >
		} else if inTextTag && i < len(xmlContent)-5 && xmlContent[i:i+5] == "</a:t" {
			// Found end of text tag
			inTextTag = false
			result.WriteString(" ")
			i += 5
			// Skip to >
			for i < len(xmlContent) && xmlContent[i] != '>' {
				i++
			}
			i++ // skip >
		} else if inTextTag {
			// Collect text content
			c := xmlContent[i]
			if c == '&' && i+4 < len(xmlContent) {
				if xmlContent[i:i+4] == "&lt;" {
					result.WriteByte('<')
					i += 4
				} else if xmlContent[i:i+4] == "&gt;" {
					result.WriteByte('>')
					i += 4
				} else if i+5 < len(xmlContent) && xmlContent[i:i+5] == "&amp;" {
					result.WriteByte('&')
					i += 5
				} else if i+6 < len(xmlContent) && xmlContent[i:i+6] == "&quot;" {
					result.WriteByte('"')
					i += 6
				} else if i+6 < len(xmlContent) && xmlContent[i:i+6] == "&apos;" {
					result.WriteByte('\'')
					i += 6
				} else {
					result.WriteByte(c)
					i++
				}
			} else {
				result.WriteByte(c)
				i++
			}
		} else {
			i++
		}
	}
	
	return strings.TrimSpace(result.String())
}

// extractRawPPTX extracts text by reading the PPTX file as binary
func (p *PPTXParser) extractRawPPTX(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open pptx: %w", err)
	}
	defer file.Close()
	
	info, err := file.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat pptx: %w", err)
	}
	
	const maxSize = 2 * 1024 * 1024
	size := info.Size()
	if size > maxSize {
		size = maxSize
	}
	
	data := make([]byte, size)
	_, err = file.Read(data)
	if err != nil && err != io.EOF {
		return "", fmt.Errorf("failed to read pptx: %w", err)
	}
	
	// Try to decode as UTF-8 and extract readable text
	content := string(data)
	var result strings.Builder
	
	// Look for sequences of printable ASCII characters
	lastWasSpace := true
	for i := 0; i < len(content); i++ {
		c := content[i]
		if c >= 32 && c < 127 {
			result.WriteByte(c)
			lastWasSpace = false
		} else if !lastWasSpace && (c == 0 || c > 127) {
			result.WriteByte(' ')
			lastWasSpace = true
		}
	}
	
	extracted := strings.TrimSpace(result.String())
	extracted = strings.Join(strings.Fields(extracted), " ")
	
	if len(extracted) < 50 {
		return fmt.Sprintf("[PPTX content from %s - %d bytes]", filepath.Base(path), info.Size()), nil
	}
	
	return extracted, nil
}

func (p *PPTXParser) Supports(ext string) bool {
	return ext == ".pptx" || ext == ".ppt"
}

// CodeParser parses code files
type CodeParser struct {
	extensions []string
}

func NewCodeParser() *CodeParser {
	return &CodeParser{
		extensions: []string{
			".go", ".py", ".js", ".ts", ".jsx", ".tsx",
			".java", ".kt", ".scala",
			".c", ".cpp", ".cc", ".h", ".hpp",
			".rs",
			".rb",
			".php",
			".swift",
			".m", ".mm",
			".cs",
			".r", ".R",
			".sh", ".bash", ".zsh",
			".sql",
			".html", ".htm", ".xml",
			".css", ".scss", ".sass", ".less",
			".json", ".yaml", ".yml", ".toml",
		},
	}
}

func (c *CodeParser) Parse(reader io.Reader) (string, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read code: %w", err)
	}
	return string(data), nil
}

func (c *CodeParser) Supports(ext string) bool {
	for _, e := range c.extensions {
		if e == ext {
			return true
		}
	}
	return false
}
