package webui

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amirotin/telemt_panel/internal/branding"
)

func TestBrandedHTMLManifestAndRevalidation(t *testing.T) {
	fsys := fixtureFS()
	fsys["index.html"].Data = []byte(`<html><head><meta name="apple-mobile-web-app-title" content="Telemt Panel"><link rel="icon" href="icon.svg" type="image/svg+xml"><link rel="apple-touch-icon" href="apple-touch-icon.png"><title>Telemt Panel</title></head><body></body></html>`)
	h, err := New(fsys, "/private")
	if err != nil {
		t.Fatal(err)
	}
	p := branding.Public{Title: `Private </script><script>alert("x")</script>`, LogoMode: "hidden", IconURL: "/private/api/branding/icon?v=1"}
	h.SetBranding(func() branding.Public { return p })
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/login", nil))
	body := w.Body.String()
	if strings.Contains(body, "Telemt Panel") || strings.Contains(body, `<script>alert`) || !strings.Contains(body, `window.__PANEL_BRANDING__=`) || !strings.Contains(body, `href="/private/api/branding/icon?v=1"`) {
		t.Fatal(body)
	}
	etag := w.Header().Get("ETag")
	r := httptest.NewRequest("GET", "/login", nil)
	r.Header.Set("If-None-Match", etag)
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 304 {
		t.Fatal(w.Code)
	}
	p.Title = "New title"
	w = httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 200 || w.Header().Get("ETag") == etag || !strings.Contains(w.Body.String(), "<title>New title</title>") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/manifest.webmanifest", nil))
	var manifest map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &manifest); err != nil {
		t.Fatal(err)
	}
	if manifest["name"] != "New title" || strings.Contains(w.Body.String(), "Telemt") {
		t.Fatal(manifest)
	}
}
