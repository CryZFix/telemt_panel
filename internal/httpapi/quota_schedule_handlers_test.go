package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/amirotin/telemt_panel/internal/config"
	"github.com/amirotin/telemt_panel/internal/hub"
	"github.com/amirotin/telemt_panel/internal/quotareset"
	"github.com/amirotin/telemt_panel/internal/store"
	"github.com/amirotin/telemt_panel/internal/telemt"
	"github.com/amirotin/telemt_panel/internal/telemt/telemttest"
)

func TestQuotaScheduleHTTPContract(t *testing.T) {
	mock := telemttest.New(telemttest.Scenario{})
	upstream := httptest.NewServer(mock.Handler())
	defer upstream.Close()
	c := telemt.New(upstream.URL, "")
	st, e := store.NewMemory(filepath.Join(t.TempDir(), "state.json"))
	if e != nil {
		t.Fatal(e)
	}
	defer st.Close()
	hb := hub.New(hub.Config{}, c, st)
	defer hb.Close()
	s := New(&config.Config{Auth: config.AuthConfig{Username: "admin", PasswordHash: testPasswordHash}}, c, st, hb, "test")
	defer s.quotaResets.Close()
	defer s.limiter.Stop()
	defer s.subLimiter.Stop()
	h := s.Handler()
	_, cookie := login(t, h, "admin", testPassword)
	request := func(method, path, rev string, body any) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r := mutatingJSON(t, method, path, cookie, body)
		if rev != "" {
			r.Header.Set("If-Match", rev)
		}
		h.ServeHTTP(w, r)
		return w
	}
	const common = "/api/settings/quota-schedule"
	for _, path := range []string{common, "/api/users/alice/quota-schedule"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 401 {
			t.Fatal("unauthenticated", w.Code)
		}
		r := mutatingJSON(t, "PUT", path, cookie, nil)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		w = httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("csrf", w.Code)
		}
	}
	w := request("GET", common, "", nil)
	var view quotareset.ScheduleView
	if json.Unmarshal(w.Body.Bytes(), &view) != nil || w.Code != 200 || view.Effective || !view.Durable {
		t.Fatal(w.Code, w.Body.String())
	}
	before, _, _ := c.QuotaList(context.Background())
	rule := quotareset.Rule{Kind: "interval", Days: 30, Start: "2026-01-31", Time: "00:00"}
	g := quotareset.GlobalSchedule{Enabled: true, Timezone: "UTC", Rule: rule}
	w = request("PUT", common, "", g)
	if w.Code != 409 {
		t.Fatal("missing revision", w.Code)
	}
	w = request("PUT", common, view.Revision, g)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &view)
	w = request("PUT", "/api/users/missing/quota-schedule", view.Revision, quotareset.UserSchedule{Mode: "off"})
	if w.Code != 404 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("PUT", "/api/users/alice/quota-schedule", view.Revision, quotareset.UserSchedule{Mode: "custom", Rule: rule})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	json.Unmarshal(w.Body.Bytes(), &view)
	w = request("GET", "/api/users/alice/quota-schedule", "", nil)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
	w = request("POST", common+"/preview", "", map[string]any{"rule": quotareset.Rule{Kind: "cron", Cron: "0 0 * * MON"}, "timezone": "Europe/Berlin"})
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = request("POST", common+"/preview", "", map[string]any{"rule": quotareset.Rule{Kind: "cron", Cron: "@daily"}, "timezone": "UTC"})
	if w.Code != 400 {
		t.Fatal(w.Code)
	}
	after, _, _ := c.QuotaList(context.Background())
	if !reflect.DeepEqual(before, after) {
		t.Fatal("editing schedule reset quotas")
	}
	recovered := quotareset.NewScheduler(st, s.quotaResets)
	v, err := recovered.View("alice")
	if err != nil || v.Policy.Mode != "custom" || !v.Global.Enabled {
		t.Fatal(v, err)
	}
	if auditTarget("quota.schedule.user", "alice") != "alice" || auditTarget("quota.schedule.completed", "") != "users" || auditOutcome("quota.schedule.interrupted") != "unknown" {
		t.Fatal("audit classification")
	}
}
