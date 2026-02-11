package templatestatic

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"testing"
)

const testTemplate = `{{define "page"}}
<html>
<head>{{block "static-css-main" .}}body { color: red; }{{end}}</head>
<body>
{{block "greeting" .}}Hello{{end}}
{{block "static-js-app" .}}console.log("hi");{{end}}
</body>
</html>
{{end}}`

func TestParse(t *testing.T) {
	tmpl := template.Must(template.New("test").Parse(testTemplate))
	outDir := t.TempDir()

	rt, err := Parse(tmpl, nil, outDir, "/static")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// 1. Check static files were written with correct content.
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

	// 2. Returned template renders <link>/<script> tags.
	var buf bytes.Buffer
	if err := rt.ExecuteTemplate(&buf, "page", nil); err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	out := buf.String()

	wantCSS := `<link rel="stylesheet" href="/static/main.css">`
	if !bytes.Contains([]byte(out), []byte(wantCSS)) {
		t.Errorf("output missing CSS tag %q\ngot: %s", wantCSS, out)
	}

	wantJS := `<script src="/static/app.js"></script>`
	if !bytes.Contains([]byte(out), []byte(wantJS)) {
		t.Errorf("output missing JS tag %q\ngot: %s", wantJS, out)
	}

	// 3. Non-static blocks are unchanged.
	if !bytes.Contains([]byte(out), []byte("Hello")) {
		t.Errorf("output missing non-static block content 'Hello'\ngot: %s", out)
	}

	// 4. Original template is not modified.
	var origBuf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&origBuf, "page", nil); err != nil {
		t.Fatalf("original ExecuteTemplate: %v", err)
	}
	origOut := origBuf.String()
	if bytes.Contains([]byte(origOut), []byte("<link")) {
		t.Errorf("original template was modified — contains <link> tag:\n%s", origOut)
	}
	if !bytes.Contains([]byte(origOut), []byte("body { color: red; }")) {
		t.Errorf("original template lost CSS content:\n%s", origOut)
	}
}

func TestParseWithData(t *testing.T) {
	const tmplStr = `{{define "page"}}{{block "static-css-theme" .}}/* {{.Theme}} */{{end}}{{end}}`
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
