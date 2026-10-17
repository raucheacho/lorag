package parsers

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Segment is extracted text with the place it comes from, so a search
// result can point at a page, a line range or a minute.
type Segment struct {
	Text string
	// Page or slide, 1-based; 0 when the source has no pages.
	Page int
	// Line of the first character of Text in the file, 1-based; 0 when the
	// source is not line-addressable.
	Line int
	// Offset of Text in the file, in bytes; -1 when unknown.
	Offset int
	// Media time range; EndMs is 0 when the segment is not timed.
	StartMs, EndMs int64
	// Heading path, function signature, sheet name…
	Section string
}

// Timed reports whether the segment comes from audio or video.
func (s Segment) Timed() bool { return s.EndMs > 0 }

// Kind groups extensions by how they are extracted.
type Kind string

const (
	KindText     Kind = "text"
	KindMarkdown Kind = "markdown"
	KindCode     Kind = "code"
	KindPDF      Kind = "pdf"
	KindOffice   Kind = "office"
	KindSheet    Kind = "sheet"
	KindSlides   Kind = "slides"
	KindAudio    Kind = "audio"
	KindVideo    Kind = "video"
	KindImage    Kind = "image"
)

// KindOf returns the kind of a file extension (with the dot), or "" when
// it is not supported.
func KindOf(ext string) Kind {
	ext = strings.ToLower(ext)
	switch ext {
	case ".txt", ".text", ".log", ".csv", ".tsv", ".rst", ".org":
		return KindText
	case ".md", ".markdown", ".mdx":
		return KindMarkdown
	case ".pdf":
		return KindPDF
	case ".docx", ".doc", ".odt", ".rtf":
		return KindOffice
	case ".xlsx":
		return KindSheet
	case ".pptx", ".ppt":
		return KindSlides
	case ".wav", ".mp3", ".m4a", ".ogg", ".flac", ".aac", ".opus":
		return KindAudio
	case ".mp4", ".avi", ".mkv", ".mov", ".webm", ".m4v":
		return KindVideo
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".tif":
		return KindImage
	}
	if NewCodeParser().Supports(ext) {
		return KindCode
	}
	return ""
}

// Extract reads a file into positioned segments.
func (r *Registry) Extract(path string) ([]Segment, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch KindOf(ext) {
	case KindText, "":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return []Segment{{Text: string(data), Line: 1}}, nil
	case KindMarkdown:
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return MarkdownSegments(string(data)), nil
	case KindCode:
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return CodeSegments(string(data)), nil
	case KindPDF:
		return pdfSegments(path)
	case KindSlides:
		return slideSegments(path)
	case KindSheet:
		return sheetSegments(path)
	case KindAudio:
		if !r.cfg.TranscriptionEnabled {
			return nil, fmt.Errorf("TRANSCRIPTION_DISABLED: enable audio transcription in Settings to index %s files", ext)
		}
		return NewAudioParserWithConfig(r.cfg.DataDir, r.cfg.WhisperModel).Segments(path)
	case KindVideo:
		if !r.cfg.TranscriptionEnabled {
			return nil, fmt.Errorf("TRANSCRIPTION_DISABLED: enable audio transcription in Settings to index %s files", ext)
		}
		return NewVideoParserWithConfig(r.cfg.DataDir, r.cfg.WhisperModel).Segments(path)
	case KindImage:
		return r.imageSegments(path)
	}
	// Office documents without pages: one segment, no position.
	text, err := r.parseLegacy(path)
	if err != nil {
		return nil, err
	}
	return []Segment{{Text: text, Offset: -1}}, nil
}

func (r *Registry) parseLegacy(path string) (string, error) {
	parser, err := r.GetParserForPath(path)
	if err != nil {
		return "", err
	}
	switch p := parser.(type) {
	case FileParser:
		return p.ParseFile(path)
	case Parser:
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer f.Close()
		return p.Parse(f)
	}
	return "", fmt.Errorf("no parser for %s", path)
}

// ---- text, markdown, code ----------------------------------------------------

var fence = regexp.MustCompile("^\\s*(```|~~~)")

// MarkdownSegments splits a document at its headings. Each segment is a
// verbatim slice of the source, so line numbers stay exact.
func MarkdownSegments(src string) []Segment {
	var segs []Segment
	var stack []string // heading titles by level
	start, startLine := 0, 1
	section := ""
	inFence := false
	offset := 0
	for n, line := range strings.SplitAfter(src, "\n") {
		lineNo := n + 1
		if fence.MatchString(line) {
			inFence = !inFence
		}
		if !inFence {
			if level, title := heading(line); level > 0 {
				if offset > start {
					segs = append(segs, Segment{Text: src[start:offset], Line: startLine, Offset: start, Section: section})
					start, startLine = offset, lineNo
				}
				for len(stack) >= level {
					stack = stack[:len(stack)-1]
				}
				for len(stack) < level-1 {
					stack = append(stack, "")
				}
				stack = append(stack, title)
				section = joinSection(stack)
			}
		}
		offset += len(line)
	}
	if start < len(src) {
		segs = append(segs, Segment{Text: src[start:], Line: startLine, Offset: start, Section: section})
	}
	return segs
}

func heading(line string) (int, string) {
	t := strings.TrimRight(line, "\r\n")
	level := 0
	for level < len(t) && level < 6 && t[level] == '#' {
		level++
	}
	if level == 0 || level >= len(t) || t[level] != ' ' {
		return 0, ""
	}
	return level, strings.TrimSpace(strings.TrimRight(t[level:], "#"))
}

func joinSection(stack []string) string {
	parts := make([]string, 0, len(stack))
	for _, s := range stack {
		if s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " › ")
}

// declaration matches the first line of a top-level definition in common
// languages; the code is split before each one.
var declaration = regexp.MustCompile(`^(export\s+)?(default\s+)?(pub(\([a-z]+\))?\s+)?(async\s+)?(func|function|def|class|type|struct|impl|fn|interface|enum|trait|module|object|protocol|extension|CREATE|create)\b`)

// CodeSegments splits source code before each top-level declaration.
// Segments are verbatim slices of the source.
func CodeSegments(src string) []Segment {
	var segs []Segment
	start, startLine, offset := 0, 1, 0
	section := ""
	for n, line := range strings.SplitAfter(src, "\n") {
		if declaration.MatchString(line) {
			if offset > start {
				segs = append(segs, Segment{Text: src[start:offset], Line: startLine, Offset: start, Section: section})
			}
			start, startLine = offset, n+1
			section = strings.TrimSpace(strings.TrimRight(strings.TrimRight(line, "\r\n"), "{(: "))
			if len(section) > 120 {
				section = section[:120]
			}
		}
		offset += len(line)
	}
	if start < len(src) {
		segs = append(segs, Segment{Text: src[start:], Line: startLine, Offset: start, Section: section})
	}
	return segs
}

// ---- PDF -----------------------------------------------------------------------

// pdfSegments returns one segment per page: pdftotext separates pages with
// a form feed.
func pdfSegments(path string) ([]Segment, error) {
	if _, err := exec.LookPath("pdftotext"); err == nil {
		out, err := exec.Command("pdftotext", "-layout", "-enc", "UTF-8", path, "-").Output()
		if err == nil && len(bytes.TrimSpace(out)) > 0 {
			var segs []Segment
			for i, page := range strings.Split(string(out), "\f") {
				if strings.TrimSpace(page) != "" {
					segs = append(segs, Segment{Text: page, Page: i + 1, Offset: -1})
				}
			}
			if len(segs) > 0 {
				return segs, nil
			}
		}
	}
	// Without pdftotext the pages are lost: fall back to the whole text.
	text, err := (&PDFParser{}).ParseFile(path)
	if err != nil {
		return nil, err
	}
	return []Segment{{Text: text, Offset: -1}}, nil
}

// ---- slides and sheets -----------------------------------------------------------

var slideName = regexp.MustCompile(`^ppt/(slides/slide|notesSlides/notesSlide)(\d+)\.xml$`)

// slideSegments returns one segment per slide, notes included.
func slideSegments(path string) ([]Segment, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		text, perr := (&PPTXParser{}).ParseFile(path)
		if perr != nil {
			return nil, perr
		}
		return []Segment{{Text: text, Offset: -1}}, nil
	}
	defer zr.Close()
	slides := map[int]string{}
	notes := map[int]string{}
	for _, f := range zr.File {
		m := slideName.FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[2])
		data, err := readZip(f)
		if err != nil {
			continue
		}
		text := extractTextFromPPTXXML(string(data))
		if strings.HasPrefix(m[1], "slides") {
			slides[n] = text
		} else {
			notes[n] = text
		}
	}
	nums := make([]int, 0, len(slides))
	for n := range slides {
		nums = append(nums, n)
	}
	sort.Ints(nums)
	var segs []Segment
	for _, n := range nums {
		text := strings.TrimSpace(slides[n])
		if note := strings.TrimSpace(notes[n]); note != "" {
			text += "\n\nNotes: " + note
		}
		if text != "" {
			segs = append(segs, Segment{Text: text, Page: n, Offset: -1, Section: fmt.Sprintf("Slide %d", n)})
		}
	}
	return segs, nil
}

func readZip(f *zip.File) ([]byte, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 64<<20))
}

// sheetSegments returns one segment per worksheet, one line per row with
// cells separated by " | ".
func sheetSegments(path string) ([]Segment, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("not an xlsx file: %w", err)
	}
	defer zr.Close()
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}

	var shared []string
	if f := files["xl/sharedStrings.xml"]; f != nil {
		data, err := readZip(f)
		if err == nil {
			var sst struct {
				SI []struct {
					T string `xml:"t"`
					R []struct {
						T string `xml:"t"`
					} `xml:"r"`
				} `xml:"si"`
			}
			if xml.Unmarshal(data, &sst) == nil {
				for _, si := range sst.SI {
					s := si.T
					for _, r := range si.R {
						s += r.T
					}
					shared = append(shared, s)
				}
			}
		}
	}

	// Sheet names in workbook order.
	type sheetRef struct{ name, target string }
	var sheets []sheetRef
	if wb, rels := files["xl/workbook.xml"], files["xl/_rels/workbook.xml.rels"]; wb != nil && rels != nil {
		var w struct {
			Sheets []struct {
				Name string `xml:"name,attr"`
				RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
			} `xml:"sheets>sheet"`
		}
		var r struct {
			Rel []struct {
				ID     string `xml:"Id,attr"`
				Target string `xml:"Target,attr"`
			} `xml:"Relationship"`
		}
		wd, _ := readZip(wb)
		rd, _ := readZip(rels)
		if xml.Unmarshal(wd, &w) == nil && xml.Unmarshal(rd, &r) == nil {
			targets := map[string]string{}
			for _, rel := range r.Rel {
				targets[rel.ID] = "xl/" + strings.TrimPrefix(rel.Target, "/xl/")
			}
			for _, s := range w.Sheets {
				sheets = append(sheets, sheetRef{s.Name, targets[s.RID]})
			}
		}
	}

	var segs []Segment
	for i, s := range sheets {
		f := files[s.target]
		if f == nil {
			continue
		}
		data, err := readZip(f)
		if err != nil {
			continue
		}
		var ws struct {
			Rows []struct {
				Cells []struct {
					T  string `xml:"t,attr"`
					V  string `xml:"v"`
					IS struct {
						T string `xml:"t"`
					} `xml:"is"`
				} `xml:"c"`
			} `xml:"sheetData>row"`
		}
		if xml.Unmarshal(data, &ws) != nil {
			continue
		}
		var b strings.Builder
		for _, row := range ws.Rows {
			var cells []string
			for _, c := range row.Cells {
				v := c.V
				switch c.T {
				case "s":
					if n, err := strconv.Atoi(c.V); err == nil && n < len(shared) {
						v = shared[n]
					}
				case "inlineStr":
					v = c.IS.T
				}
				if v = strings.TrimSpace(v); v != "" {
					cells = append(cells, v)
				}
			}
			if len(cells) > 0 {
				b.WriteString(strings.Join(cells, " | "))
				b.WriteByte('\n')
			}
		}
		if b.Len() > 0 {
			segs = append(segs, Segment{Text: b.String(), Page: i + 1, Offset: -1, Section: "Sheet " + s.name})
		}
	}
	return segs, nil
}

// ---- images ----------------------------------------------------------------------

// imageSegments reads the text in an image (tesseract) and, when the vision
// model is enabled, adds its description.
func (r *Registry) imageSegments(path string) ([]Segment, error) {
	var parts []string
	if text := ocr(path); text != "" {
		parts = append(parts, "Text in the image:\n"+text)
	}
	if r.cfg.OcrEnabled {
		desc, err := NewImageParserWithConfig(r.cfg.OllamaURL, r.cfg.OcrModel).ParseFile(path)
		if err != nil {
			return nil, err
		}
		parts = append(parts, desc)
	}
	if len(parts) == 0 {
		return nil, fmt.Errorf("NO_IMAGE_TEXT: no text found in the image (install tesseract for OCR, or enable image description in Settings)")
	}
	return []Segment{{Text: strings.Join(parts, "\n\n"), Offset: -1}}, nil
}

// ocr returns the text tesseract reads in an image, or "" when tesseract is
// missing or finds nothing.
func ocr(path string) string {
	if _, err := exec.LookPath("tesseract"); err != nil {
		return ""
	}
	out, err := exec.Command("tesseract", path, "stdout", "--psm", "3").Output()
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(out))
	if len(strings.Fields(text)) < 2 {
		return ""
	}
	return text
}
