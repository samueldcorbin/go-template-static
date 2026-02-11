package templatestatic

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
)

// Parse clones t, extracts blocks named static-css-* and static-js-*,
// writes them as files to outputDir, and returns a new template where
// those blocks produce <link>/<script> tags using urlPrefix.
// The original template t is not modified.
func Parse(t *template.Template, data any, outputDir, urlPrefix string) (*template.Template, error) {
	// Use one clone to render block content (Execute prevents later Parse).
	renderClone, err := t.Clone()
	if err != nil {
		return nil, err
	}

	// Collect static blocks and their rendered content.
	type staticBlock struct {
		name, filename, tag string
		content             []byte
	}
	var blocks []staticBlock

	for _, tmpl := range renderClone.Templates() {
		name := tmpl.Name()

		var suffix, ext, tag string
		switch {
		case strings.HasPrefix(name, "static-css-"):
			suffix = strings.TrimPrefix(name, "static-css-")
			ext = ".css"
			tag = `<link rel="stylesheet" href="` + urlPrefix + "/" + suffix + ext + `">`
		case strings.HasPrefix(name, "static-js-"):
			suffix = strings.TrimPrefix(name, "static-js-")
			ext = ".js"
			tag = `<script src="` + urlPrefix + "/" + suffix + ext + `"></script>`
		default:
			continue
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return nil, err
		}

		blocks = append(blocks, staticBlock{
			name:     name,
			filename: suffix + ext,
			tag:      tag,
			content:  buf.Bytes(),
		})
	}

	// Write files and build redefinitions on a second clone (never Executed).
	resultClone, err := t.Clone()
	if err != nil {
		return nil, err
	}

	var redefs []string
	for _, b := range blocks {
		if err := writeIfChanged(filepath.Join(outputDir, b.filename), b.content); err != nil {
			return nil, err
		}
		redefs = append(redefs, `{{define "`+b.name+`"}}`+b.tag+`{{end}}`)
	}

	if len(redefs) > 0 {
		if _, err := resultClone.Parse(strings.Join(redefs, "")); err != nil {
			return nil, err
		}
	}

	return resultClone, nil
}

// writeIfChanged writes content to path only if the file doesn't exist or its
// content differs. This preserves mtime for stable caching.
func writeIfChanged(path string, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, content) {
		return nil
	}
	return os.WriteFile(path, content, 0o644)
}
