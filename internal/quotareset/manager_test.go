package quotareset

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/amirotin/telemt_panel/internal/telemt"
)

type fakeClient struct {
	users    []telemt.UserInfo
	revision string
	readOnly bool
	reset    func(context.Context, string, string) (telemt.QuotaEntry, error)
}

func (f *fakeClient) Health(context.Context) (telemt.HealthData, error) {
	return telemt.HealthData{ReadOnly: f.readOnly}, nil
}
func (f *fakeClient) UsersWithRevision(context.Context) ([]telemt.UserInfo, string, error) {
	return f.users, f.revision, nil
}
func (f *fakeClient) QuotaList(context.Context) (map[string]telemt.QuotaEntry, bool, error) {
	return nil, true, nil
}
func (f *fakeClient) ResetQuotaWithRevision(ctx context.Context, name, rev string) (telemt.QuotaEntry, error) {
	return f.reset(ctx, name, rev)
}

func fixture(n int) *fakeClient {
	f := &fakeClient{revision: "r1", reset: func(context.Context, string, string) (telemt.QuotaEntry, error) { return telemt.QuotaEntry{}, nil }}
	for i := 0; i < n; i++ {
		f.users = append(f.users, telemt.UserInfo{Username: fmt.Sprintf("user_%04d", i), Enabled: i%3 != 0})
	}
	return f
}
func done(t *testing.T, m *Manager) *Status {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s, e := m.Status("", 0)
		if e != nil {
			t.Fatal(e)
		}
		if s != nil && s.State != "running" {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("operation did not finish")
	return nil
}
func launch(t *testing.T, m *Manager) Confirmation {
	t.Helper()
	p, e := m.Prepare(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = m.Start(context.Background(), p.Token, "admin", "127.0.0.1"); e != nil {
		t.Fatal(e)
	}
	return p
}

func TestLargeSequentialAndReplay(t *testing.T) {
	f := fixture(2501)
	var active, peak, calls atomic.Int32
	f.reset = func(_ context.Context, _, rev string) (telemt.QuotaEntry, error) {
		if rev != "r1" {
			t.Errorf("revision %s", rev)
		}
		v := active.Add(1)
		if v > peak.Load() {
			peak.Store(v)
		}
		calls.Add(1)
		active.Add(-1)
		return telemt.QuotaEntry{}, nil
	}
	m := New(f, nil)
	defer m.Close()
	p := launch(t, m)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, e := m.Start(context.Background(), p.Token, "admin", ""); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	s := done(t, m)
	if s.Confirmed != 2501 || s.Remaining != 0 || peak.Load() != 1 || calls.Load() != 2501 {
		t.Fatalf("bad result %+v calls %d peak %d", s, calls.Load(), peak.Load())
	}
	if _, e := m.Start(context.Background(), p.Token, "admin", ""); e != nil {
		t.Fatal(e)
	}
	if calls.Load() != 2501 {
		t.Fatal("completed batch replayed")
	}
}

func TestUnconfirmedExpiredAndChangedNeverWrite(t *testing.T) {
	f := fixture(2)
	var calls int
	f.reset = func(context.Context, string, string) (telemt.QuotaEntry, error) {
		calls++
		return telemt.QuotaEntry{}, nil
	}
	m := New(f, nil)
	defer m.Close()
	if _, e := m.Start(context.Background(), "missing", "", ""); !errors.Is(e, ErrConfirmation) {
		t.Fatal(e)
	}
	p, e := m.Prepare(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	clock := time.Now().Add(3 * time.Minute)
	m.now = func() time.Time { return clock }
	if _, e = m.Start(context.Background(), p.Token, "", ""); !errors.Is(e, ErrConfirmation) {
		t.Fatal(e)
	}
	p, e = m.Prepare(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	f.revision = "r2"
	if _, e = m.Start(context.Background(), p.Token, "", ""); !errors.Is(e, ErrRevision) {
		t.Fatal(e)
	}
	p, e = m.Prepare(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	f.users[0].Username = "recreated"
	if _, e = m.Start(context.Background(), p.Token, "", ""); !errors.Is(e, ErrRevision) {
		t.Fatal(e)
	}
	f.readOnly = true
	if _, e = m.Prepare(context.Background()); e == nil {
		t.Fatal("read-only prepare accepted")
	}
	if calls != 0 {
		t.Fatal("unconfirmed mutation")
	}
}

func TestRefusalsUnknownAndStop(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		err                          error
		rejected, unknown, remaining int
	}{
		{"missing", &telemt.APIError{Status: 404, Code: "not_found"}, 1, 0, 0},
		{"revision", &telemt.APIError{Status: 409, Code: "revision_conflict"}, 1, 0, 2},
		{"read-only", &telemt.APIError{Status: 403, Code: "read_only"}, 1, 0, 2},
		{"auth", &telemt.APIError{Status: 401, Code: "unauthorized"}, 1, 0, 2},
		{"route", &telemt.APIError{Status: 404, Code: "http_error"}, 1, 0, 2},
		{"save-after-reset", &telemt.APIError{Status: 500, Code: "internal_error"}, 0, 1, 2},
		{"lost-response", context.DeadlineExceeded, 0, 1, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := fixture(5)
			calls := 0
			f.reset = func(context.Context, string, string) (telemt.QuotaEntry, error) {
				calls++
				if calls == 3 {
					return telemt.QuotaEntry{}, tc.err
				}
				return telemt.QuotaEntry{}, nil
			}
			m := New(f, nil)
			defer m.Close()
			launch(t, m)
			s := done(t, m)
			if s.Rejected != tc.rejected || s.Unknown != tc.unknown || s.Remaining != tc.remaining || s.Total != s.Confirmed+s.Rejected+s.Unknown+s.Remaining {
				t.Fatalf("%+v", s)
			}
			if tc.remaining > 0 && calls != 3 {
				t.Fatal("continued or retried ambiguous operation")
			}
		})
	}
}

func TestShutdownAndSingleResetExclusion(t *testing.T) {
	f := fixture(10)
	entered := make(chan struct{})
	var calls atomic.Int32
	f.reset = func(ctx context.Context, _, _ string) (telemt.QuotaEntry, error) {
		calls.Add(1)
		close(entered)
		<-ctx.Done()
		return telemt.QuotaEntry{}, ctx.Err()
	}
	m := New(f, nil)
	launch(t, m)
	<-entered
	if _, e := m.ResetSingle(context.Background(), "user_0001"); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	if _, e := m.Prepare(context.Background()); !errors.Is(e, ErrBusy) {
		t.Fatal(e)
	}
	m.Close()
	s := done(t, m)
	if s.Unknown != 1 || s.Remaining != 9 || calls.Load() != 1 {
		t.Fatalf("%+v", s)
	}
	if _, e := m.Prepare(context.Background()); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
}

func TestExceptionPagesAndCopies(t *testing.T) {
	f := fixture(130)
	f.reset = func(context.Context, string, string) (telemt.QuotaEntry, error) {
		return telemt.QuotaEntry{}, &telemt.APIError{Status: 404, Code: "not_found"}
	}
	m := New(f, nil)
	defer m.Close()
	p := launch(t, m)
	s := done(t, m)
	if len(s.Issues) != 50 || s.IssuesTotal != 130 || s.NextOffset == nil || *s.NextOffset != 50 {
		t.Fatalf("%+v", s)
	}
	s.Issues[0].Username = "tampered"
	first, _ := m.Status(p.Token, 0)
	if first.Issues[0].Username == "tampered" {
		t.Fatal("snapshot shares issue storage")
	}
	last, _ := m.Status(p.Token, 100)
	if len(last.Issues) != 30 || last.NextOffset != nil {
		t.Fatalf("%+v", last)
	}
	if _, e := m.Status("old", 0); !errors.Is(e, ErrMissing) {
		t.Fatal(e)
	}
}
