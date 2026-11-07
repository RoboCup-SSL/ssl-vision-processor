package api

import (
	_ "embed"
	"net/http"
)

// redoc is Redoc's standalone bundle (MIT, viewer/REDOC-LICENSE), embedded so
// the docs work on a venue network with no internet. Refresh it with
// `make redoc`.
//
//go:embed viewer/redoc.standalone.js
var redoc []byte

// viewerPage loads the spec into Redoc with system fonts, so it needs nothing
// from the internet.
const viewerPage = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>vision_processor GUI host API</title>
<style>body { margin: 0; }</style>
</head>
<body>
<div id="redoc"></div>
<script src="/api/docs/redoc.standalone.js"></script>
<script>
Redoc.init("/api/openapi.json", {
  theme: { typography: { fontFamily: "system-ui, sans-serif", headings: { fontFamily: "system-ui, sans-serif" } } },
}, document.getElementById("redoc"));
</script>
</body>
</html>
`

// docsCSP allows the viewer page only this host's resources, plus the inline
// init script and the blob: worker and images Redoc creates.
const docsCSP = "default-src 'self'; script-src 'self' 'unsafe-inline' blob:; worker-src blob:; " +
	"style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; connect-src 'self'"

// ServeDocs registers GET /api/openapi.json (the spec) and GET /api/docs
// (a viewer for it). Call it after every Route, since the spec is built
// once, here.
func (a *API) ServeDocs() {
	spec, err := a.Spec()
	if err != nil {
		panic("api: marshalling the spec: " + err.Error())
	}

	a.mux.HandleFunc("GET /api/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(spec)
	})

	a.mux.HandleFunc("GET /api/docs", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Everything from this host: no request leaves the venue network,
		// not even Redoc's badge logo.
		w.Header().Set("Content-Security-Policy", docsCSP)
		_, _ = w.Write([]byte(viewerPage))
	})

	a.mux.HandleFunc("GET /api/docs/redoc.standalone.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/javascript")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write(redoc)
	})
}
