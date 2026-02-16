package templatestatic

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"text/template/parse"
)

// Parse clones t, extracts templates named static-css-* and static-js-*,
// writes them as files to outputDir, and returns a new template with
// <link>/<script> tags injected before </head> (CSS first, then JS).
//
// If a static definition has an explicit {{template "static-css-*"}} call
// in the template tree, the tag appears there instead of being auto-injected.
//
// The original template t is not modified.
func Parse(t *template.Template, data any, outputDir, urlPrefix string) (*template.Template, error) {
	// Use one clone to render template content (Execute prevents later Parse).
	renderClone, err := t.Clone()
	if err != nil {
		return nil, err
	}

	// Collect static definitions and their rendered content.
	type staticDef struct {
		name, filename, tag string
		content             []byte
		isCSS               bool
	}
	var statics []staticDef

	for _, tmpl := range renderClone.Templates() {
		name := tmpl.Name()

		var suffix, ext, tag string
		var isCSS bool
		switch {
		case strings.HasPrefix(name, "static-css-"):
			suffix = strings.TrimPrefix(name, "static-css-")
			ext = ".css"
			tag = `<link rel="stylesheet" href="` + urlPrefix + "/" + suffix + ext + `">`
			isCSS = true
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

		statics = append(statics, staticDef{
			name:     name,
			filename: suffix + ext,
			tag:      tag,
			content:  buf.Bytes(),
			isCSS:    isCSS,
		})
	}

	// Write files on a second clone (never Executed).
	resultClone, err := t.Clone()
	if err != nil {
		return nil, err
	}

	// Find which static names have explicit {{template "static-*"}} calls.
	placed := findPlacedTemplates(resultClone)

	var redefs []string
	var autoCSS, autoJS []string
	for _, s := range statics {
		if err := writeIfChanged(filepath.Join(outputDir, s.filename), s.content); err != nil {
			return nil, err
		}
		if placed[s.name] {
			// Explicit call exists — redefine to output the tag there.
			redefs = append(redefs, `{{define "`+s.name+`"}}`+s.tag+`{{end}}`)
		} else {
			// No explicit call — redefine to empty, collect for auto-injection.
			redefs = append(redefs, `{{define "`+s.name+`"}}{{end}}`)
			if s.isCSS {
				autoCSS = append(autoCSS, s.tag)
			} else {
				autoJS = append(autoJS, s.tag)
			}
		}
	}

	if len(redefs) > 0 {
		if _, err := resultClone.Parse(strings.Join(redefs, "")); err != nil {
			return nil, err
		}
	}

	// Inject auto tags before </head> (CSS first, then JS).
	autoTags := strings.Join(append(autoCSS, autoJS...), "")
	if autoTags != "" {
		injectBeforeCloseHead(resultClone, autoTags)
	}

	return resultClone, nil
}

// findPlacedTemplates walks all templates in t and returns a set of names
// that are explicitly invoked via {{template "name"}} calls.
func findPlacedTemplates(t *template.Template) map[string]bool {
	placed := make(map[string]bool)
	for _, tmpl := range t.Templates() {
		if tmpl.Tree == nil {
			continue
		}
		walkTree(tmpl.Tree.Root, func(n parse.Node) {
			if tn, ok := n.(*parse.TemplateNode); ok {
				if strings.HasPrefix(tn.Name, "static-css-") || strings.HasPrefix(tn.Name, "static-js-") {
					placed[tn.Name] = true
				}
			}
		})
	}
	return placed
}

// walkTree recursively visits every node in the parse tree.
func walkTree(n parse.Node, fn func(parse.Node)) {
	if n == nil {
		return
	}
	fn(n)
	switch n := n.(type) {
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			walkTree(child, fn)
		}
	case *parse.IfNode:
		walkTree(n.List, fn)
		walkTree(n.ElseList, fn)
	case *parse.RangeNode:
		walkTree(n.List, fn)
		walkTree(n.ElseList, fn)
	case *parse.WithNode:
		walkTree(n.List, fn)
		walkTree(n.ElseList, fn)
	}
}

// injectBeforeCloseHead finds the first </head> in any text node across
// all templates and splices tags immediately before it.
func injectBeforeCloseHead(t *template.Template, tags string) {
	for _, tmpl := range t.Templates() {
		if tmpl.Tree == nil {
			continue
		}
		if injectInList(tmpl.Tree.Root, tags) {
			return
		}
	}
}

func injectInList(list *parse.ListNode, tags string) bool {
	if list == nil {
		return false
	}
	for _, n := range list.Nodes {
		switch n := n.(type) {
		case *parse.TextNode:
			i := bytes.Index(n.Text, []byte("</head>"))
			if i >= 0 {
				n.Text = append(n.Text[:i], append([]byte(tags), n.Text[i:]...)...)
				return true
			}
		case *parse.IfNode:
			if injectInList(n.List, tags) || injectInList(n.ElseList, tags) {
				return true
			}
		case *parse.RangeNode:
			if injectInList(n.List, tags) || injectInList(n.ElseList, tags) {
				return true
			}
		case *parse.WithNode:
			if injectInList(n.List, tags) || injectInList(n.ElseList, tags) {
				return true
			}
		}
	}
	return false
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
