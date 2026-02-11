# go-template-static

A Go library that extracts CSS and JS from Go HTML templates into standalone static files.

Keep your styles and scripts colocated with your templates during development. At startup, `Parse` pulls them out into `.css`/`.js` files and rewires the template to output `<link>`/`<script>` tags instead.

## Install

```
go get github.com/samueldcorbin/go-template-static@v0.1.0
```

## Usage

Name your template blocks with `static-css-` or `static-js-` prefixes:

```html
{{define "page"}}
<html>
<head>
  {{block "static-css-main" .}}
  body { margin: 0; }
  h1 { color: navy; }
  {{end}}
</head>
<body>
  <h1>Hello</h1>
  {{block "static-js-app" .}}
  console.log("ready");
  {{end}}
</body>
</html>
{{end}}
```

Then call `Parse` at startup:

```go
package main

import (
	"html/template"
	"net/http"

	templatestatic "github.com/samueldcorbin/go-template-static"
)

func main() {
	t := template.Must(template.ParseGlob("templates/*.html"))

	rt, err := templatestatic.Parse(t, nil, "./static", "/static")
	if err != nil {
		panic(err)
	}

	// Serve the extracted static files.
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Render using rt — static blocks now output <link>/<script> tags.
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rt.ExecuteTemplate(w, "page", nil)
	})

	http.ListenAndServe(":8080", nil)
}
```

This writes `./static/main.css` and `./static/app.js`, and renders the page as:

```html
<html>
<head>
  <link rel="stylesheet" href="/static/main.css">
</head>
<body>
  <h1>Hello</h1>
  <script src="/static/app.js"></script>
</body>
</html>
```

The original template `t` is never modified.

## API

```go
func Parse(t *template.Template, data any, outputDir, urlPrefix string) (*template.Template, error)
```

- **t** — the source template (not modified)
- **data** — passed to each static block during rendering (use for template variables in your CSS/JS)
- **outputDir** — directory to write static files into (created if needed)
- **urlPrefix** — URL path prefix for generated `<link>`/`<script>` tags
- Returns a new template ready for rendering

Files are only written when content changes, preserving mtime for stable caching.

## Editor Support

For syntax highlighting of CSS/JS inside static blocks, install the [samueldcorbin.go-template-static-syntax](https://marketplace.visualstudio.com/items?itemName=samueldcorbin.go-template-static-syntax) VS Code extension.

## License

MIT
