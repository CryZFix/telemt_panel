package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/amirotin/telemt_panel/internal/config"
	"github.com/amirotin/telemt_panel/internal/hub"
	"github.com/amirotin/telemt_panel/internal/quotareset"
	"github.com/amirotin/telemt_panel/internal/store"
	"github.com/amirotin/telemt_panel/internal/telemt"
	"github.com/amirotin/telemt_panel/internal/telemt/telemttest"
)

func TestBulkQuotaHTTPContract(t *testing.T) {
	mock := telemttest.New(telemttest.Scenario{})
	upstream := httptest.NewServer(mock.Handler())
	defer upstream.Close()
	c := telemt.New(upstream.URL, "")
	st, err := store.NewMemory("")
	if err != nil {
		t.Fatal(err)
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
	const prefix = "/api/users/operations/quota-reset"
	request := func(method, path string, body any) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, mutatingJSON(t, method, path, cookie, body))
		return w
	}
	for _, path := range []string{prefix, prefix + "/prepare"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", path, nil)
		r.Header.Set("Sec-Fetch-Site", "same-origin")
		h.ServeHTTP(w, r)
		if w.Code != 401 {
			t.Fatalf("unauthed %s: %d", path, w.Code)
		}
		w = httptest.NewRecorder()
		r = httptest.NewRequest("POST", path, nil)
		r.AddCookie(cookie)
		r.Header.Set("Sec-Fetch-Site", "cross-site")
		h.ServeHTTP(w, r)
		if w.Code != 403 {
			t.Fatal("CSRF bypass", w.Code)
		}
	}
	w := request("POST", prefix+"/prepare", nil)
	var p quotareset.Confirmation
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &p) != nil || p.Total != 1 {
		t.Fatalf("prepare: %d %s", w.Code, w.Body.String())
	}
	before, _, err := c.QuotaList(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	w = request("POST", prefix, map[string]string{"token": p.Token})
	if w.Code != 202 {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}
	deadline := time.Now().Add(3 * time.Second)
	var status quotareset.Status
	for time.Now().Before(deadline) {
		w = request("GET", prefix+"?id="+p.Token, nil)
		var body struct {
			Operation quotareset.Status `json:"operation"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &body) != nil {
			t.Fatal(w.Body.String())
		}
		status = body.Operation
		if status.State != "running" {
			break
		}
		time.Sleep(time.Millisecond)
	}
	if status.Confirmed != 1 || status.Remaining != 0 || status.State != "completed" {
		t.Fatalf("status: %+v", status)
	}
	after, _, err := c.QuotaList(context.Background())
	if err != nil || after["alice"].UsedBytes != 0 || after["alice"].DataQuotaBytes != before["alice"].DataQuotaBytes {
		t.Fatal("quota limit changed", err)
	}
	w = request("POST", prefix, map[string]string{"token": p.Token})
	if w.Code != 202 {
		t.Fatal(w.Body.String())
	}
	w = request("GET", prefix+"?id=old-operation", nil)
	if w.Code != 404 {
		t.Fatal("missing operation must not report success", w.Code)
	}
	if auditTarget("quota.reset_all.partial", "") != "users" || auditOutcome("quota.reset_all.partial") != "partial" || auditOutcome("quota.reset_all.interrupted") != "unknown" {
		t.Fatal("misleading audit classification")
	}
}
