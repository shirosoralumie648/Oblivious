package deepdoc

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// --- Markdown ---------------------------------------------------------------

func TestParseMarkdownBuildsHeadingTreeTablesListsAndCode(t *testing.T) {
	md := strings.Join([]string{
		"# Guide",
		"",
		"Intro paragraph.",
		"",
		"## Setup",
		"",
		"- install go",
		"- run tests",
		"",
		"```bash",
		"go test ./...",
		"```",
		"",
		"## Pricing",
		"",
		"| Plan | Price |",
		"| --- | --- |",
		"| Free | 0 |",
		"| Pro | 99 |",
	}, "\n")

	doc := ParseMarkdown(md)
	if doc.Title != "Guide" {
		t.Fatalf("title = %q, want Guide", doc.Title)
	}
	if len(doc.Root.Children) != 1 {
		t.Fatalf("root children = %d, want 1", len(doc.Root.Children))
	}
	guide := doc.Root.Children[0]
	if guide.Title != "Guide" || len(guide.Children) != 2 {
		t.Fatalf("guide section: title=%q children=%d", guide.Title, len(guide.Children))
	}
	if guide.Blocks[0].Type != BlockParagraph || guide.Blocks[0].Text != "Intro paragraph." {
		t.Fatalf("intro block = %+v", guide.Blocks[0])
	}
	setup := guide.Children[0]
	if setup.Title != "Setup" || setup.Level != 2 {
		t.Fatalf("setup = %+v", setup)
	}
	if setup.Blocks[0].Type != BlockList || len(setup.Blocks[0].Items) != 2 {
		t.Fatalf("setup list = %+v", setup.Blocks[0])
	}
	if setup.Blocks[1].Type != BlockCode || setup.Blocks[1].Language != "bash" || setup.Blocks[1].Text != "go test ./..." {
		t.Fatalf("code block = %+v", setup.Blocks[1])
	}
	pricing := guide.Children[1]
	if pricing.Blocks[0].Type != BlockTable {
		t.Fatalf("pricing block = %+v", pricing.Blocks[0])
	}
	table := pricing.Blocks[0].Table
	if !table.HasHeader || len(table.Rows) != 3 || table.Rows[2][1] != "99" {
		t.Fatalf("pricing table = %+v", table)
	}
}

func TestParseMarkdownEmptyInput(t *testing.T) {
	doc := ParseMarkdown("")
	if !doc.IsEmpty() {
		t.Fatalf("expected empty structure, got %+v", doc.Root)
	}
}

// --- HTML -------------------------------------------------------------------

func TestParseHTMLExtractsStructureAndSkipsScript(t *testing.T) {
	html := `<!DOCTYPE html><html><head><title>Doc Title</title>
<script>alert("evil")</script><style>p{color:red}</style></head>
<body>
<h1>Report</h1>
<p>First &amp; second.</p>
<h2>Data</h2>
<table><tr><th>Name</th><th>Score</th></tr><tr><td>alice</td><td>10</td></tr></table>
<ul><li>one</li><li>two</li></ul>
<pre>raw code</pre>
</body></html>`

	doc := ParseHTML(html)
	if doc.Title != "Doc Title" {
		t.Fatalf("title = %q, want Doc Title", doc.Title)
	}
	if len(doc.Root.Children) != 1 || doc.Root.Children[0].Title != "Report" {
		t.Fatalf("root children = %+v", doc.Root.Children)
	}
	report := doc.Root.Children[0]
	if report.Blocks[0].Text != "First & second." {
		t.Fatalf("paragraph = %+v", report.Blocks[0])
	}
	for _, b := range report.Blocks {
		if strings.Contains(b.Text, "alert") || strings.Contains(b.Text, "color:red") {
			t.Fatalf("script/style leaked into output: %+v", b)
		}
	}
	data := report.Children[0]
	if data.Title != "Data" {
		t.Fatalf("data section = %+v", data)
	}
	var table *Table
	var items []string
	var code string
	for _, b := range data.Blocks {
		switch b.Type {
		case BlockTable:
			table = b.Table
		case BlockList:
			items = b.Items
		case BlockCode:
			code = b.Text
		}
	}
	if table == nil || !table.HasHeader || len(table.Rows) != 2 || table.Rows[1][0] != "alice" {
		t.Fatalf("table = %+v", table)
	}
	if len(items) != 2 || items[0] != "one" {
		t.Fatalf("list = %v", items)
	}
	if code != "raw code" {
		t.Fatalf("pre = %q", code)
	}
}

func TestParseHTMLMalformedInputDoesNotPanic(t *testing.T) {
	inputs := []string{
		"<html><h1>Unclosed",
		"<table><tr><td>orphan cell",
		"<p>text < not a tag",
		"<<<>>>",
		"<script>never closed",
		"<a href='broken>text</a>",
	}
	for _, input := range inputs {
		doc := ParseHTML(input) // must not panic
		_ = doc.IsEmpty()
	}
	doc := ParseHTML("<table><tr><td>orphan</td></tr>")
	var found bool
	for _, b := range doc.Root.Blocks {
		if b.Type == BlockTable && len(b.Table.Rows) == 1 && b.Table.Rows[0][0] == "orphan" {
			found = true
		}
	}
	if !found {
		t.Fatalf("unterminated table not recovered: %+v", doc.Root.Blocks)
	}
}

// --- DOCX -------------------------------------------------------------------

func buildDocxFixture(t *testing.T, documentXML string) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"word/document.xml":   documentXML,
	} {
		f, err := writer.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

const docxFixtureXML = `<?xml version="1.0"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:pPr><w:pStyle w:val="Heading1"/></w:pPr><w:r><w:t>Annual Report</w:t></w:r></w:p>
    <w:p><w:r><w:t>Revenue grew </w:t></w:r><w:r><w:t>strongly.</w:t></w:r></w:p>
    <w:p><w:pPr><w:pStyle w:val="Heading2"/></w:pPr><w:r><w:t>Figures</w:t></w:r></w:p>
    <w:p><w:pPr><w:numPr><w:ilvl w:val="0"/></w:numPr></w:pPr><w:r><w:t>first item</w:t></w:r></w:p>
    <w:p><w:pPr><w:numPr><w:ilvl w:val="0"/></w:numPr></w:pPr><w:r><w:t>second item</w:t></w:r></w:p>
    <w:tbl>
      <w:tr><w:tc><w:p><w:r><w:t>Quarter</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Revenue</w:t></w:r></w:p></w:tc></w:tr>
      <w:tr><w:tc><w:p><w:r><w:t>Q1</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>1000</w:t></w:r></w:p></w:tc></w:tr>
    </w:tbl>
  </w:body>
</w:document>`

func TestParseDOCXExtractsHeadingsListsAndTables(t *testing.T) {
	data := buildDocxFixture(t, docxFixtureXML)
	doc, err := ParseDOCX(data)
	if err != nil {
		t.Fatalf("ParseDOCX: %v", err)
	}
	if doc.Title != "Annual Report" {
		t.Fatalf("title = %q", doc.Title)
	}
	report := doc.Root.Children[0]
	if report.Blocks[0].Text != "Revenue grew strongly." {
		t.Fatalf("paragraph = %+v", report.Blocks[0])
	}
	figures := report.Children[0]
	if figures.Title != "Figures" || figures.Level != 2 {
		t.Fatalf("figures = %+v", figures)
	}
	var list []string
	var table *Table
	for _, b := range figures.Blocks {
		switch b.Type {
		case BlockList:
			list = b.Items
		case BlockTable:
			table = b.Table
		}
	}
	if len(list) != 2 || list[1] != "second item" {
		t.Fatalf("list = %v", list)
	}
	if table == nil || len(table.Rows) != 2 || table.Rows[1][1] != "1000" || !table.HasHeader {
		t.Fatalf("table = %+v", table)
	}
}

func TestParseDOCXRejectsInvalidArchives(t *testing.T) {
	if _, err := ParseDOCX([]byte("not a zip")); err == nil {
		t.Fatal("expected error for non-zip input")
	}
	// Valid zip without word/document.xml.
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	f, _ := writer.Create("other.txt")
	f.Write([]byte("hello"))
	writer.Close()
	if _, err := ParseDOCX(buf.Bytes()); err == nil {
		t.Fatal("expected error for zip without document.xml")
	}
	// Malformed XML inside.
	data := buildDocxFixture(t, "<w:document><unclosed")
	if _, err := ParseDOCX(data); err == nil {
		t.Fatal("expected error for malformed xml")
	}
}

// --- CSV / plain text ---------------------------------------------------------

func TestParseCSVDetectsHeader(t *testing.T) {
	doc := ParseCSV("name,score\nalice,10\nbob,12\n", ',')
	if len(doc.Root.Blocks) != 1 || doc.Root.Blocks[0].Type != BlockTable {
		t.Fatalf("blocks = %+v", doc.Root.Blocks)
	}
	table := doc.Root.Blocks[0].Table
	if !table.HasHeader || len(table.Rows) != 3 || table.Rows[1][0] != "alice" {
		t.Fatalf("table = %+v", table)
	}

	noHeader := ParseCSV("1,2\n3,4\n", ',')
	if noHeader.Root.Blocks[0].Table.HasHeader {
		t.Fatal("numeric first row must not be a header")
	}
}

func TestParsePlainTextSectionsAndParagraphs(t *testing.T) {
	text := "INTRODUCTION\n\nThis is the intro paragraph.\nIt spans lines.\n\nDETAILS\n\nMore content here."
	doc := ParsePlainText(text)
	if len(doc.Root.Children) != 2 {
		t.Fatalf("children = %d, want 2", len(doc.Root.Children))
	}
	if doc.Root.Children[0].Title != "INTRODUCTION" {
		t.Fatalf("first section = %+v", doc.Root.Children[0])
	}
	intro := doc.Root.Children[0]
	if len(intro.Blocks) != 1 || intro.Blocks[0].Text != "This is the intro paragraph. It spans lines." {
		t.Fatalf("intro blocks = %+v", intro.Blocks)
	}
}

// --- Detection ----------------------------------------------------------------

func TestDetectFormat(t *testing.T) {
	docx := buildDocxFixture(t, docxFixtureXML)
	cases := []struct {
		name     string
		filename string
		content  string
		want     string
	}{
		{"md extension", "notes.md", "anything", FormatMarkdown},
		{"html extension", "page.html", "anything", FormatHTML},
		{"docx extension", "report.docx", "anything", FormatDOCX},
		{"csv extension", "data.csv", "a,b", FormatCSV},
		{"tsv extension", "data.tsv", "a\tb", FormatTSV},
		{"html sniff", "upload.bin", "<!DOCTYPE html><html><body><p>x</p></body></html>", FormatHTML},
		{"markdown sniff", "upload.bin", "# Title\n\n- item\n- item", FormatMarkdown},
		{"csv sniff", "upload.bin", "a,b,c\n1,2,3\n4,5,6", FormatCSV},
		{"tsv sniff", "upload.bin", "a\tb\n1\t2", FormatTSV},
		{"plain fallback", "upload.bin", "just some plain sentences. nothing special here.", FormatText},
	}
	for _, tc := range cases {
		got := DetectFormat(tc.filename, []byte(tc.content))
		if got != tc.want {
			t.Fatalf("%s: DetectFormat = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := DetectFormat("upload.bin", docx); got != FormatDOCX {
		t.Fatalf("zip sniff = %q, want docx", got)
	}
}

func TestParseDispatchesAllFormats(t *testing.T) {
	docx := buildDocxFixture(t, docxFixtureXML)
	for _, tc := range []struct {
		filename string
		content  []byte
		format   string
	}{
		{"a.md", []byte("# T\n\nbody"), FormatMarkdown},
		{"a.html", []byte("<h1>T</h1><p>body</p>"), FormatHTML},
		{"a.docx", docx, FormatDOCX},
		{"a.csv", []byte("x,y\n1,2"), FormatCSV},
		{"a.tsv", []byte("x\ty\n1\t2"), FormatTSV},
		{"a.txt", []byte("hello world."), FormatText},
	} {
		doc, err := Parse(tc.filename, tc.content)
		if err != nil {
			t.Fatalf("Parse(%s): %v", tc.filename, err)
		}
		if doc.Format != tc.format {
			t.Fatalf("Parse(%s) format = %q, want %q", tc.filename, doc.Format, tc.format)
		}
		if doc.IsEmpty() {
			t.Fatalf("Parse(%s) produced empty structure", tc.filename)
		}
	}
}
