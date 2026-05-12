package server

import (
	"net/http"
)

const defaultPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Authorized access only</title>
<style>
body{font-family:system-ui,sans-serif;max-width:42rem;margin:2rem auto;padding:0 1rem;line-height:1.5;color:#222}
.banner{background:#e8eef5;border:1px solid #8a9bb0;border-left:4px solid #2c5282;color:#1a365d;padding:1rem 1.25rem;border-radius:8px;margin-bottom:1.5rem;font-weight:600}
code{background:#f4f4f4;padding:.15em .4em;border-radius:4px}
.muted{color:#555;font-size:.95rem}
</style>
</head>
<body>
<p class="banner">This system is for <strong>authorized personnel only</strong>. If you are not an authorized user, do not attempt to access or use this service.</p>
<h1>Authorized access only</h1>
</body>
</html>
`

// DefaultPage serves a small HTML landing page for non-WebSocket HTTP requests (authorized-access notice).
func DefaultPage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = w.Write([]byte(defaultPageHTML))
	}
}
