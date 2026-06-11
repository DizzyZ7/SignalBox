package httpapi

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed admin/*
var adminUIFiles embed.FS

func WithAdminUI(next http.Handler) http.Handler {
	adminFS, err := fs.Sub(adminUIFiles, "admin")
	if err != nil {
		panic(err)
	}
	admin := http.StripPrefix("/admin/", http.FileServer(http.FS(adminFS)))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin" {
			http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/admin/") {
			setAdminUIHeaders(w)
			admin.ServeHTTP(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func setAdminUIHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=()")
	h.Set("Cache-Control", "no-store")
}
