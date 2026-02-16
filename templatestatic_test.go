package templatestatic

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"testing"
)

// Auto-injection: no explicit {{template}} calls, tags injected before </head>.
const testTemplateAuto = `{{define "static-css-main"}}body { color: red; }{{end}}
{{define "static-js-app"}}console.log("hi");{{end}}
{{define "greeting"}}Hello{{end}}
{{define "page"}}
<html>
<head>
<title>Test</title>
</head>
<body>
{{template "greeting" .}}
</body>
</html>
{{end}}`

func TestParseAutoInject(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(testTemplateAuto))
	outDir := t.TempDir()

	rt, err := Parse(tmpl, nil, outDir, "/static")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Check static files were written.
	css, err := os.ReadFile(filepath.Join(outDir, "main.css"))
	if err != nil {
		t.Fatalf("reading main.css: %v", err)
	}
	if string(css) != "body { color: red; }" {
		t.Errorf("main.css = %q, want %q", css, "body { color: red; }")
	}

	js, err := os.ReadFile(filepath.Join(outDir, "app.js"))
	if err != nil {
		t.Fatalf("reading app.js: %v", err)
	}
	if string(js) != `console.log("hi");` {
		t.Errorf("app.js = %q, want %q", js, `console.log("hi");`)
	}

	// Render and check tags were injected before </head>.
	var buf bytes.Buffer
	if err := rt.ExecuteTemplate(&buf, "page", nil); err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	out := buf.String()

	wantCSS := `<link rel="stylesheet" href="/static/main.css">`
	wantJS := `<script src="/static/app.js"></script>`
	if !bytes.Contains([]byte(out), []byte(wantCSS)) {
		t.Errorf("output missing CSS tag %q\ngot: %s", wantCSS, out)
	}
	if !bytes.Contains([]byte(out), []byte(wantJS)) {
		t.Errorf("output missing JS tag %q\ngot: %s", wantJS, out)
	}

	// Tags should appear before </head>.
	headClose := bytes.Index([]byte(out), []byte("</head>"))
	cssPos := bytes.Index([]byte(out), []byte(wantCSS))
	jsPos := bytes.Index([]byte(out), []byte(wantJS))
	if cssPos > headClose {
		t.Errorf("CSS tag should appear before </head>")
	}
	if jsPos > headClose {
		t.Errorf("JS tag should appear before </head>")
	}
	// CSS before JS.
	if cssPos > jsPos {
		t.Errorf("CSS tag should appear before JS tag")
	}

	// Non-static definitions are unchanged.
	if !bytes.Contains([]byte(out), []byte("Hello")) {
		t.Errorf("output missing non-static content 'Hello'\ngot: %s", out)
	}

	// Original template is not modified.
	var origBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&origBuf, "page", nil); err != nil {
		t.Fatalf("original ExecuteTemplate: %v", err)
	}
	origOut := origBuf.String()
	if bytes.Contains([]byte(origOut), []byte("<link")) {
		t.Errorf("original template was modified — contains <link> tag:\n%s", origOut)
	}
}

// Explicit placement: {{template "static-css-*"}} call controls where the tag goes.
const testTemplateExplicit = `{{define "static-css-critical"}}h1 { font-size: 2em; }{{end}}
{{define "static-js-app"}}console.log("hi");{{end}}
{{define "page"}}
<html>
<head>
{{template "static-css-critical"}}
</head>
<body></body>
</html>
{{end}}`

func TestParseExplicitPlacement(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(testTemplateExplicit))
	outDir := t.TempDir()

	rt, err := Parse(tmpl, nil, outDir, "/static")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var buf bytes.Buffer
	if err := rt.ExecuteTemplate(&buf, "page", nil); err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	out := buf.String()

	wantCSS := `<link rel="stylesheet" href="/static/critical.css">`
	wantJS := `<script src="/static/app.js"></script>`

	// Explicit CSS tag should appear exactly once (not also auto-injected).
	if c := bytes.Count([]byte(out), []byte(wantCSS)); c != 1 {
		t.Errorf("CSS tag should appear exactly once, got %d\noutput: %s", c, out)
	}

	// CSS tag should come before the auto-injected JS tag (explicit call is first).
	cssPos := bytes.Index([]byte(out), []byte(wantCSS))
	jsPos := bytes.Index([]byte(out), []byte(wantJS))
	if cssPos < 0 {
		t.Fatalf("output missing CSS tag %q\ngot: %s", wantCSS, out)
	}
	if jsPos < 0 {
		t.Fatalf("output missing JS tag %q\ngot: %s", wantJS, out)
	}
	if cssPos > jsPos {
		t.Errorf("explicit CSS tag should appear before auto-injected JS tag\noutput: %s", out)
	}

	// JS had no explicit call — should be auto-injected before </head>.
	headClose := bytes.Index([]byte(out), []byte("</head>"))
	if jsPos > headClose {
		t.Errorf("auto-injected JS tag should appear before </head>")
	}
}

func TestParseWithData(t *testing.T) {
	const tmplStr = `{{define "static-css-theme"}}/* {{.Theme}} */{{end}}
{{define "page"}}<html><head></head></html>{{end}}`
	tmpl := template.Must(template.New("test").Parse(tmplStr))
	outDir := t.TempDir()

	data := struct{ Theme string }{Theme: "dark"}
	rt, err := Parse(tmpl, data, outDir, "/assets")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	css, err := os.ReadFile(filepath.Join(outDir, "theme.css"))
	if err != nil {
		t.Fatalf("reading theme.css: %v", err)
	}
	if string(css) != "/* dark */" {
		t.Errorf("theme.css = %q, want %q", css, "/* dark */")
	}

	var buf bytes.Buffer
	if err := rt.ExecuteTemplate(&buf, "page", nil); err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	want := `<link rel="stylesheet" href="/assets/theme.css">`
	if !bytes.Contains(buf.Bytes(), []byte(want)) {
		t.Errorf("output missing %q\ngot: %s", want, buf.String())
	}
}

func TestWriteIfChangedPreservesMtime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.css")

	content := []byte("body{}")
	if err := writeIfChanged(path, content); err != nil {
		t.Fatal(err)
	}
	info1, _ := os.Stat(path)

	// Write same content again — mtime should not change.
	if err := writeIfChanged(path, content); err != nil {
		t.Fatal(err)
	}
	info2, _ := os.Stat(path)

	if !info1.ModTime().Equal(info2.ModTime()) {
		t.Error("mtime changed for identical content")
	}

	// Write different content — mtime should change.
	if err := writeIfChanged(path, []byte("div{}")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "div{}" {
		t.Errorf("content = %q, want %q", got, "div{}")
	}
}
