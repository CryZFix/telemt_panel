package telemt

import (
	"context"
	"net/http"
	"testing"
)

func TestQuotaResetSendsConfirmedRevision(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"ok":true,"data":[{"username":"alice"}],"revision":"r1"}`))
			return
		}
		if r.URL.Path != "/v1/users/alice/reset-quota" || r.Header.Get("If-Match") != "r1" {
			t.Errorf("unexpected reset %s %q", r.URL.Path, r.Header.Get("If-Match"))
		}
		w.Write([]byte(`{"ok":true,"data":{"username":"alice","used_bytes":0,"last_reset_epoch_secs":123},"revision":"r1"}`))
	})
	users, revision, err := client.UsersWithRevision(context.Background())
	if err != nil || len(users) != 1 || revision != "r1" {
		t.Fatal(users, revision, err)
	}
	q, err := client.ResetQuotaWithRevision(context.Background(), users[0].Username, revision)
	if err != nil || q.LastResetEpochSecs != 123 {
		t.Fatal(q, err)
	}
}
