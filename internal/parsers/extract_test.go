package parsers

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestMarkdownSegments(t *testing.T) {
	src := "intro line\n\n## Install\nrun make\n\n```sh\n# not a heading\n```\n## Usage\n### Flags\n-v verbose\n"
	segs := MarkdownSegments(src)
	var got []string
	joined := ""
	for _, s := range segs {
		got = append(got, fmt.Sprintf("%d:%s", s.Line, s.Section))
		if src[s.Offset:s.Offset+len(s.Text)] != s.Text {
			t.Fatalf("segment is not a verbatim slice: %q", s.Text)
		}
		joined += s.Text
	}
	want := []string{"1:", "3:Install", "9:Usage", "10:Usage › Flags"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("segments %v, want %v", got, want)
	}
	if joined != src {
		t.Fatal("segments do not cover the source")
	}
}

func TestCodeSegments(t *testing.T) {
	src := "package x\n\nimport \"fmt\"\n\nfunc A() {\n\tfmt.Println()\n}\n\ntype B struct{}\n\nfunc (b B) C() int {\n\treturn 1\n}\n"
	segs := CodeSegments(src)
	var got []string
	for _, s := range segs {
		got = append(got, fmt.Sprintf("%d:%s", s.Line, s.Section))
	}
	want := "1:|5:func A()|9:type B struct{}|11:func (b B) C() int"
	if strings.Join(got, "|") != want {
		t.Fatalf("got %v, want %s", got, want)
	}
}

// minimalPDF writes a PDF with one page per text, enough for pdftotext.
func minimalPDF(pages []string) []byte {
	var b bytes.Buffer
	var offsets []int
	obj := func(body string) {
		offsets = append(offsets, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(offsets), body)
	}
	b.WriteString("%PDF-1.4\n")
	n := len(pages)
	kids := make([]string, n)
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", 4+2*i)
	}
	obj("<< /Type /Catalog /Pages 2 0 R >>")
	obj(fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", strings.Join(kids, " "), n))
	obj("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>")
	for i, text := range pages {
		stream := fmt.Sprintf("BT /F1 18 Tf 72 720 Td (%s) Tj ET", text)
		obj(fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i))
		obj(fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(stream), stream))
	}
	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offsets)+1)
	for _, o := range offsets {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets)+1, xref)
	return b.Bytes()
}

func TestPDFPages(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed")
	}
	path := filepath.Join(t.TempDir(), "doc.pdf")
	os.WriteFile(path, minimalPDF([]string{"Introduction to the cluster", "Rollback procedure for deployments", "Appendix tables"}), 0o644)
	segs, err := NewRegistry().Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 {
		t.Fatalf("%d segments: %+v", len(segs), segs)
	}
	if segs[1].Page != 2 || !strings.Contains(segs[1].Text, "Rollback procedure") {
		t.Fatalf("page 2: %+v", segs[1])
	}
}

func writeZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for name, body := range files {
		w, _ := zw.Create(name)
		w.Write([]byte(body))
	}
	zw.Close()
	f.Close()
}

func TestSheetSegments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "book.xlsx")
	writeZip(t, path, map[string]string{
		"xl/workbook.xml": `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` +
			`<sheet name="Costs" sheetId="1" r:id="rId1"/><sheet name="Team" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Target="worksheets/sheet2.xml"/></Relationships>`,
		"xl/sharedStrings.xml":     `<sst><si><t>Service</t></si><si><t>Monthly</t></si><si><t>Storage</t></si></sst>`,
		"xl/worksheets/sheet1.xml": `<worksheet><sheetData><row><c t="s"><v>0</v></c><c t="s"><v>1</v></c></row><row><c t="s"><v>2</v></c><c><v>42</v></c></row></sheetData></worksheet>`,
		"xl/worksheets/sheet2.xml": `<worksheet><sheetData><row><c t="inlineStr"><is><t>Alice</t></is></c></row></sheetData></worksheet>`,
	})
	segs, err := NewRegistry().Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 || segs[0].Section != "Sheet Costs" || segs[0].Text != "Service | Monthly\nStorage | 42\n" || segs[1].Page != 2 {
		t.Fatalf("%+v", segs)
	}
}

func TestSlideSegments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deck.pptx")
	writeZip(t, path, map[string]string{
		"ppt/slides/slide1.xml":           `<p:sld><a:t>Roadmap</a:t></p:sld>`,
		"ppt/slides/slide2.xml":           `<p:sld><a:t>Budget 2027</a:t></p:sld>`,
		"ppt/slides/slide10.xml":          `<p:sld><a:t>Questions</a:t></p:sld>`,
		"ppt/notesSlides/notesSlide2.xml": `<p:notes><a:t>mention the freeze</a:t></p:notes>`,
	})
	segs, err := NewRegistry().Extract(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 3 || segs[1].Page != 2 || !strings.Contains(segs[1].Text, "Notes: mention the freeze") || segs[2].Page != 10 {
		t.Fatalf("%+v", segs)
	}
}

func TestWhisperJSON(t *testing.T) {
	segs, err := parseWhisperJSON([]byte(`{"transcription":[
		{"offsets":{"from":0,"to":4200},"text":" Welcome to the incident review."},
		{"offsets":{"from":4200,"to":4300},"text":" [BLANK_AUDIO]"},
		{"offsets":{"from":192000,"to":197500},"text":" The rollback took eleven minutes."}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 2 || segs[1].StartMs != 192000 || segs[1].EndMs != 197500 || segs[1].Text != "The rollback took eleven minutes." {
		t.Fatalf("%+v", segs)
	}
}

func TestKinds(t *testing.T) {
	for ext, want := range map[string]Kind{".MD": KindMarkdown, ".go": KindCode, ".pdf": KindPDF, ".mp3": KindAudio, ".mov": KindVideo, ".png": KindImage, ".xlsx": KindSheet, ".exe": ""} {
		if got := KindOf(ext); got != want {
			t.Errorf("KindOf(%s) = %q, want %q", ext, got, want)
		}
	}
}

func TestTranscriptionDisabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.mp3")
	os.WriteFile(path, []byte("ID3"), 0o644)
	if _, err := NewRegistry().Extract(path); err == nil || !strings.Contains(err.Error(), "TRANSCRIPTION_DISABLED") {
		t.Fatalf("err = %v", err)
	}
}
