package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/amirotin/telemt_panel/internal/auth"
	"github.com/amirotin/telemt_panel/internal/quotareset"
	"github.com/amirotin/telemt_panel/internal/telemt"
)

func (s *Server) handlePrepareQuotaReset(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), subpageRequestTimeout)
	defer cancel()
	confirmation, err := s.quotaResets.Prepare(ctx)
	if err != nil {
		writeQuotaResetError(w, err)
		return
	}
	writeJSON(w, 200, confirmation)
}

func (s *Server) handleStartQuotaReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Token string `json:"token"`
	}
	if err := decodeJSONBody(w, r, &body, jsonBodyOptions{MaxBytes: 1024}); err != nil || len(body.Token) != 32 {
		auth.WriteError(w, 400, "bad_request", "a quota confirmation token is required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), subpageRequestTimeout)
	defer cancel()
	actor, _ := auth.UsernameFromContext(r.Context())
	status, err := s.quotaResets.Start(ctx, body.Token, actor, auth.ClientIP(r, s.cfg.TrustedProxyPrefixes))
	if err != nil {
		writeQuotaResetError(w, err)
		return
	}
	writeJSON(w, 202, status)
}

func (s *Server) handleQuotaResetStatus(w http.ResponseWriter, r *http.Request) {
	offset := 0
	if raw := r.URL.Query().Get("offset"); raw != "" {
		var err error
		offset, err = strconv.Atoi(raw)
		if err != nil || offset < 0 {
			auth.WriteError(w, 400, "bad_request", "offset must be a non-negative integer")
			return
		}
	}
	id := r.URL.Query().Get("id")
	if len(id) > 32 {
		auth.WriteError(w, 400, "bad_request", "invalid operation id")
		return
	}
	status, err := s.quotaResets.Status(id, offset)
	if err != nil {
		writeQuotaResetError(w, err)
		return
	}
	writeJSON(w, 200, struct {
		Operation *quotareset.Status `json:"operation"`
	}{status})
}

func writeQuotaResetError(w http.ResponseWriter, err error) {
	var api *telemt.APIError
	if errors.As(err, &api) && (api.Code == "capability_absent" || api.Code == "capability_unavailable") {
		auth.WriteError(w, api.Status, api.Code, "required quota API or configuration revision is unavailable")
		return
	}
	switch {
	case errors.Is(err, quotareset.ErrBusy):
		auth.WriteError(w, 409, "quota_reset_busy", "a quota operation is already in progress")
	case errors.Is(err, quotareset.ErrConfirmation):
		auth.WriteError(w, 409, "quota_confirmation_expired", "prepare and confirm the complete user list again")
	case errors.Is(err, quotareset.ErrRevision):
		auth.WriteError(w, 409, "revision_conflict", "the users configuration changed; confirm the new list")
	case errors.Is(err, quotareset.ErrEmpty):
		auth.WriteError(w, 400, "no_changes", "there are no users to reset")
	case errors.Is(err, quotareset.ErrClosed):
		auth.WriteError(w, 503, "quota_operation_unavailable", "panel is stopping; the quota operation will not resume automatically")
	case errors.Is(err, quotareset.ErrMissing):
		auth.WriteError(w, 404, "quota_operation_unavailable", "operation tracking was lost or replaced; do not retry resets automatically")
	default:
		writeTelemtError(w, err, false)
	}
}

func (s *Server) quotaResetEvent(status quotareset.Status, actor, ip string) {
	action := "quota.reset_all.completed"
	if status.State == "running" {
		action = "quota.reset_all.started"
	} else if status.Unknown > 0 {
		action = "quota.reset_all.interrupted"
	} else if status.State == "stopped" {
		action = "quota.reset_all.stopped"
	} else if status.Rejected > 0 {
		action = "quota.reset_all.partial"
	}
	if status.Trigger == "schedule" {
		action = strings.Replace(action, "quota.reset_all.", "quota.schedule.", 1)
	}
	detail := fmt.Sprintf("operation=%s total=%d confirmed=%d rejected=%d unknown=%d not_sent=%d reason=%s", status.ID, status.Total, status.Confirmed, status.Rejected, status.Unknown, status.Remaining, status.Reason)
	s.appendAuditIdentity(actor, ip, action, "", detail)
	if status.State != "running" {
		s.pokeUsersAfterMutation()
	}
}
