package httpapi

import (
	"errors"
	"net/http"
	"strings"

	"github.com/amirotin/telemt_panel/internal/auth"
	"github.com/amirotin/telemt_panel/internal/branding"
)

func (s *Server) handlePublicBranding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, s.branding.Public())
}

func (s *Server) handleGetBranding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, http.StatusOK, s.branding.Settings())
}

func (s *Server) handlePutBranding(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	var request struct {
		Title    *string `json:"title"`
		LogoMode *string `json:"logo_mode"`
		LogoPath *string `json:"logo_path"`
	}
	if err := decodeJSONBody(w, r, &request, jsonBodyOptions{MaxBytes: 16 << 10, RejectUnknown: true}); err != nil || request.Title == nil || request.LogoMode == nil || request.LogoPath == nil {
		auth.WriteError(w, http.StatusBadRequest, "bad_request", "invalid branding configuration")
		return
	}
	cfg := branding.Config{Title: *request.Title, LogoMode: *request.LogoMode, LogoPath: *request.LogoPath}
	result, err := s.branding.Save(cfg)
	if err != nil {
		for _, known := range []error{branding.ErrTitle, branding.ErrMode, branding.ErrPath, branding.ErrRead, branding.ErrImage} {
			if errors.Is(err, known) {
				auth.WriteError(w, http.StatusBadRequest, known.Error(), "could not apply branding configuration")
				return
			}
		}
		auth.WriteError(w, http.StatusInternalServerError, "internal_error", "could not save branding configuration")
		return
	}
	s.appendAudit(r, "branding.settings_change", "panel", "")
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleBrandingAsset(w http.ResponseWriter, r *http.Request) {
	iconSize := 0
	if strings.HasSuffix(r.URL.Path, "/icon") {
		iconSize = 192
		if r.URL.Query().Get("size") == "512" {
			iconSize = 512
		}
	}
	body, mime, version := s.branding.Asset(iconSize)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if len(body) == 0 {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", mime)
	etag := `"` + version + `"`
	w.Header().Set("ETag", etag)
	if r.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}
