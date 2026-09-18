package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/amirotin/telemt_panel/internal/auth"
	"github.com/amirotin/telemt_panel/internal/quotareset"
)

func writeScheduleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, quotareset.ErrScheduleInvalid):
		auth.WriteError(w, 400, "invalid_quota_schedule", "invalid interval, cron expression or timezone")
	case errors.Is(err, quotareset.ErrScheduleConflict):
		auth.WriteError(w, 409, "quota_schedule_conflict", "schedule changed; reload before saving")
	case errors.Is(err, quotareset.ErrScheduleStorage):
		auth.WriteError(w, 503, "quota_schedule_storage", "durable technical state is required for automatic quota resets")
	default:
		auth.WriteError(w, 503, "quota_schedule_unavailable", "quota schedule state is unavailable")
	}
}

func (s *Server) handleQuotaSchedule(w http.ResponseWriter, r *http.Request) {
	v, err := s.quotaSchedules.View(r.PathValue("username"))
	if err != nil {
		writeScheduleError(w, err)
		return
	}
	w.Header().Set("ETag", `"`+v.Revision+`"`)
	writeJSON(w, 200, v)
}

func (s *Server) handleSaveQuotaSchedule(w http.ResponseWriter, r *http.Request) {
	var body quotareset.GlobalSchedule
	if err := decodeJSONBody(w, r, &body, jsonBodyOptions{MaxBytes: 4096}); err != nil {
		auth.WriteError(w, 400, "bad_request", "invalid schedule body")
		return
	}
	if err := s.quotaSchedules.SaveGlobal(strings.Trim(r.Header.Get("If-Match"), `"`), body); err != nil {
		writeScheduleError(w, err)
		return
	}
	s.appendAudit(r, "quota.schedule.global", "", "")
	s.handleQuotaSchedule(w, r)
}

func (s *Server) handleSaveUserQuotaSchedule(w http.ResponseWriter, r *http.Request) {
	var body quotareset.UserSchedule
	if err := decodeJSONBody(w, r, &body, jsonBodyOptions{MaxBytes: 4096}); err != nil {
		auth.WriteError(w, 400, "bad_request", "invalid schedule body")
		return
	}
	name := r.PathValue("username")
	ctx, cancel := context.WithTimeout(r.Context(), subpageRequestTimeout)
	defer cancel()
	users, err := s.tc.Users(ctx)
	if err != nil {
		writeTelemtError(w, err, false)
		return
	}
	found := false
	for _, user := range users {
		if user.Username == name {
			found = true
			break
		}
	}
	if !found {
		auth.WriteError(w, 404, "not_found", "user not found")
		return
	}
	if err := s.quotaSchedules.SaveUser(strings.Trim(r.Header.Get("If-Match"), `"`), name, body); err != nil {
		writeScheduleError(w, err)
		return
	}
	s.appendAudit(r, "quota.schedule.user", name, "")
	s.handleQuotaSchedule(w, r)
}

func (s *Server) handlePreviewQuotaSchedule(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Rule     quotareset.Rule `json:"rule"`
		Timezone string          `json:"timezone"`
	}
	if err := decodeJSONBody(w, r, &body, jsonBodyOptions{MaxBytes: 4096}); err != nil {
		auth.WriteError(w, 400, "bad_request", "invalid schedule body")
		return
	}
	if body.Timezone == "server" {
		body.Timezone = quotareset.ServerZone()
	}
	runs, err := body.Rule.Preview(time.Now(), body.Timezone)
	if err != nil {
		writeScheduleError(w, quotareset.ErrScheduleInvalid)
		return
	}
	writeJSON(w, 200, struct {
		NextRuns []time.Time `json:"next_runs"`
		Timezone string      `json:"effective_timezone"`
	}{runs, body.Timezone})
}
