package api

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
)

// Ports the SPA static-file serving from gateway/src/main.rs: serve the built
// frontend, falling back to index.html for client-side routes. When the static
// dir is absent, serve a dev placeholder (API-only mode).

const devPlaceholderHTML = `<h1>Pulse Gateway API</h1>` +
	`<p>Running in API-only / development mode.</p>` +
	`<p>Use the Vite dev server at <code>http://localhost:9010</code> to access the UI.</p>`

func (st *State) handleStatic(w http.ResponseWriter, r *http.Request) {
	dir := st.StaticDir
	index := filepath.Join(dir, "index.html")
	if dir == "" || !fileExists(index) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(devPlaceholderHTML))
		return
	}

	// Serve the requested file if it exists under the static dir; else fall back
	// to index.html so client-side routes resolve.
	clean := path.Clean(r.URL.Path)
	full := filepath.Join(dir, filepath.FromSlash(clean))
	if clean != "/" && withinDir(dir, full) && fileExists(full) {
		http.ServeFile(w, r, full)
		return
	}
	http.ServeFile(w, r, index)
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}

// withinDir guards against path traversal: full must be inside dir.
func withinDir(dir, full string) bool {
	rel, err := filepath.Rel(dir, full)
	if err != nil {
		return false
	}
	return rel != ".." && !filepath.IsAbs(rel) &&
		!hasDotDotPrefix(rel)
}

func hasDotDotPrefix(rel string) bool {
	return len(rel) >= 2 && rel[0] == '.' && rel[1] == '.'
}
