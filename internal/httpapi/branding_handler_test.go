package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrandingAuthorizationAndValidation(t *testing.T) {
	s := newTestServer(t)
	t.Cleanup(s.geoip.Close)
	h := s.Handler()
	for _, method := range []string{"GET", "PUT"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(method, "/api/settings/branding", nil)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatal(method, w.Code)
		}
	}
	_, cookie := login(t, h, "admin", testPassword)
	if cookie == nil {
		t.Fatal("login failed")
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/settings/branding", strings.NewReader(`{"title":"Private","logo_mode":"hidden","logo_path":""}`))
	r.AddCookie(cookie)
	r.Header.Set("Origin", "https://untrusted.example")
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal("csrf", w.Code)
	}
	for _, body := range []string{`null`, `{}`, `{"title":"x","logo_mode":"hidden"}`, `{"title":"x","logo_mode":"hidden","logo_path":null}`, `{"title":"x","logo_mode":"hidden","logo_path":"","extra":1}`, `{} {}`} {
		w = httptest.NewRecorder()
		r = httptest.NewRequest("PUT", "/api/settings/branding", strings.NewReader(body))
		r.AddCookie(cookie)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		h.ServeHTTP(w, r)
		if w.Code != 400 {
			t.Fatal(body, w.Code)
		}
	}
	w = httptest.NewRecorder()
	r = httptest.NewRequest("PUT", "/api/settings/branding", strings.NewReader(`{"title":"Private","logo_mode":"hidden","logo_path":""}`))
	r.AddCookie(cookie)
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	h.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/branding", nil))
	if w.Code != 200 || strings.Contains(w.Body.String(), "logo_path") || !strings.Contains(w.Body.String(), "Private") {
		t.Fatal(w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/api/branding/icon?path=/etc/passwd", nil))
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/png" || strings.Contains(w.Body.String(), "root:") {
		t.Fatal(w.Code, w.Header())
	}
	etag := w.Header().Get("ETag")
	w = httptest.NewRecorder()
	r = httptest.NewRequest("GET", "/api/branding/icon", nil)
	r.Header.Set("If-None-Match", etag)
	h.ServeHTTP(w, r)
	if w.Code != http.StatusNotModified {
		t.Fatal(w.Code)
	}
}
