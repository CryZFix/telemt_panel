package quotareset

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/amirotin/telemt_panel/internal/telemt"
)

type scheduleMemory struct {
	raw           string
	durable, fail bool
}

func (s *scheduleMemory) GetSetting(string) (string, bool, error) { return s.raw, s.raw != "", nil }
func (s *scheduleMemory) SetSetting(_, v string) error {
	if s.fail {
		return errors.New("disk")
	}
	s.raw = v
	return nil
}
func (s *scheduleMemory) StateDurable() bool { return s.durable }

func scheduleFixture(t *testing.T, n int) (*Scheduler, *scheduleMemory, *fakeClient, *time.Time) {
	t.Helper()
	st := &scheduleMemory{durable: true}
	f := fixture(n)
	m := New(f, nil)
	t.Cleanup(m.Close)
	s := NewScheduler(st, m)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	return s, st, f, &now
}
func daily() Rule { return Rule{Kind: "interval", Days: 1, Start: "2026-09-01", Time: "00:00"} }
func saveGlobal(t *testing.T, s *Scheduler) {
	t.Helper()
	if e := s.SaveGlobal(s.revision(), GlobalSchedule{Enabled: true, Timezone: "UTC", Rule: daily()}); e != nil {
		t.Fatal(e)
	}
}
func saveUser(t *testing.T, s *Scheduler, n, mode string) {
	t.Helper()
	if e := s.SaveUser(s.revision(), n, UserSchedule{Mode: mode, Rule: daily()}); e != nil {
		t.Fatal(e)
	}
}

func TestScheduleCatchupModesAndRestart(t *testing.T) {
	s, st, f, now := scheduleFixture(t, 4)
	calls := []string{}
	f.reset = func(_ context.Context, n, _ string) (telemt.QuotaEntry, error) {
		calls = append(calls, n)
		return telemt.QuotaEntry{}, nil
	}
	saveGlobal(t, s)
	saveUser(t, s, "user_0001", "off")
	saveUser(t, s, "user_0002", "custom")
	s.Tick(context.Background())
	if len(calls) != 0 {
		t.Fatal("save caused reset")
	}
	*now = now.AddDate(0, 0, 12)
	s.Tick(context.Background())
	if !reflect.DeepEqual(calls, []string{"user_0000", "user_0002", "user_0003"}) {
		t.Fatal(calls)
	}
	s.Tick(context.Background())
	restarted := NewScheduler(st, s.manager)
	restarted.now = s.now
	restarted.Tick(context.Background())
	if len(calls) != 3 {
		t.Fatal("replayed", calls)
	}
	v, e := restarted.View("")
	if e != nil || v.Last.Confirmed == nil || *v.Last.Confirmed != 3 {
		t.Fatal(v, e)
	}
	*now = now.AddDate(0, 0, 1)
	restarted.Tick(context.Background())
	if len(calls) != 6 {
		t.Fatal(calls)
	}
}

func TestScheduleCustomIndependentAndDurability(t *testing.T) {
	s, st, f, now := scheduleFixture(t, 2)
	calls := 0
	f.reset = func(context.Context, string, string) (telemt.QuotaEntry, error) {
		calls++
		return telemt.QuotaEntry{}, nil
	}
	st.durable = false
	if e := s.SaveUser(s.revision(), "user_0001", UserSchedule{Mode: "custom", Rule: daily()}); !errors.Is(e, ErrScheduleStorage) {
		t.Fatal(e)
	}
	st.durable = true
	saveUser(t, s, "user_0001", "custom")
	*now = now.AddDate(0, 0, 1)
	st.fail = true
	s.Tick(context.Background())
	if calls != 0 {
		t.Fatal("write without durable claim")
	}
	st.fail = false
	s.Tick(context.Background())
	if calls != 1 {
		t.Fatal(calls)
	}
}

func TestScheduleUnknownAndInterruptedNeverReplay(t *testing.T) {
	s, st, f, now := scheduleFixture(t, 3)
	calls := 0
	reserved := ""
	f.reset = func(context.Context, string, string) (telemt.QuotaEntry, error) {
		calls++
		reserved = st.raw
		return telemt.QuotaEntry{}, context.DeadlineExceeded
	}
	saveGlobal(t, s)
	*now = now.AddDate(0, 0, 1)
	s.Tick(context.Background())
	s.Tick(context.Background())
	if calls != 1 || s.record.Last.Unknown == nil || *s.record.Last.Unknown != 1 {
		t.Fatal(calls, s.record.Last)
	}
	st.raw = reserved
	restarted := NewScheduler(st, s.manager)
	restarted.now = s.now
	restarted.Tick(context.Background())
	if calls != 1 || restarted.record.Last.State != "interrupted" || restarted.record.Last.Confirmed != nil {
		t.Fatal("unsafe recovered result", restarted.record.Last)
	}
}

func TestScheduleFinalSaveFailureAndConcurrentPolicyEdit(t *testing.T) {
	s, st, f, now := scheduleFixture(t, 2)
	calls := 0
	f.reset = func(context.Context, string, string) (telemt.QuotaEntry, error) {
		calls++
		if calls == 1 {
			saveUser(t, s, "user_0001", "off")
		}
		st.fail = true
		return telemt.QuotaEntry{}, nil
	}
	saveGlobal(t, s)
	*now = now.AddDate(0, 0, 1)
	s.Tick(context.Background())
	if s.pending == nil {
		t.Fatal("lost pending result")
	}
	s.Tick(context.Background())
	if calls != 2 {
		t.Fatal("retried", calls)
	}
	st.fail = false
	s.Tick(context.Background())
	if s.pending != nil || s.record.Users["user_0001"].Mode != "off" {
		t.Fatal("lost concurrent edit")
	}
	r := NewScheduler(st, s.manager)
	r.now = s.now
	r.Tick(context.Background())
	if calls != 2 {
		t.Fatal("replayed after recovery")
	}
}

func TestScheduleUnavailableBusyAndNewInheritance(t *testing.T) {
	s, _, f, now := scheduleFixture(t, 2)
	calls := []string{}
	f.reset = func(_ context.Context, n, _ string) (telemt.QuotaEntry, error) {
		calls = append(calls, n)
		return telemt.QuotaEntry{}, nil
	}
	saveGlobal(t, s)
	*now = now.AddDate(0, 0, 5)
	saveUser(t, s, "user_0001", "inherit")
	f.readOnly = true
	s.Tick(context.Background())
	if len(calls) != 0 {
		t.Fatal(calls)
	}
	f.readOnly = false
	s.manager.checking = true
	s.Tick(context.Background())
	s.manager.checking = false
	if len(calls) != 0 {
		t.Fatal(calls)
	}
	s.Tick(context.Background())
	if !reflect.DeepEqual(calls, []string{"user_0000"}) {
		t.Fatal(calls)
	}
	*now = now.AddDate(0, 0, 2)
	s.Tick(context.Background())
	if len(calls) != 3 {
		t.Fatal(calls)
	}
}

func TestScheduleEditsAndCorruptState(t *testing.T) {
	s, st, _, now := scheduleFixture(t, 2)
	saveGlobal(t, s)
	if e := s.SaveGlobal("0", s.record.Global); !errors.Is(e, ErrScheduleConflict) {
		t.Fatal(e)
	}
	*now = now.AddDate(0, 0, 10)
	g := s.record.Global
	g.Rule.Days = 7
	if e := s.SaveGlobal(s.revision(), g); e != nil {
		t.Fatal(e)
	}
	if !s.record.Cursors["global"].Next.After(*now) {
		t.Fatal("retroactive edit")
	}
	saveUser(t, s, "user_0001", "custom")
	if e := s.ForgetUser("user_0001"); e != nil {
		t.Fatal(e)
	}
	if len(s.record.Users) != 0 || len(s.record.Cursors) != 1 {
		t.Fatal("override leak")
	}
	r := s.clone()
	r.Cursors["global"] = checkpoint{}
	data, _ := json.Marshal(r)
	st.raw = string(data)
	broken := NewScheduler(st, s.manager)
	if _, e := broken.View(""); !errors.Is(e, ErrScheduleUnavailable) {
		t.Fatal(e)
	}
}

func TestScheduleLargeBatch(t *testing.T) {
	s, _, f, now := scheduleFixture(t, 2501)
	calls := 0
	f.reset = func(context.Context, string, string) (telemt.QuotaEntry, error) {
		calls++
		return telemt.QuotaEntry{}, nil
	}
	saveGlobal(t, s)
	*now = now.AddDate(0, 0, 1)
	s.Tick(context.Background())
	if calls != 2501 {
		t.Fatal(calls)
	}
}

func TestScheduleTimezoneChangeAndClockRollback(t *testing.T) {
	s, _, _, now := scheduleFixture(t, 2)
	saveGlobal(t, s)
	saveUser(t, s, "user_0001", "custom")
	g := s.record.Global
	g.Timezone = "Asia/Tokyo"
	if e := s.SaveGlobal(s.revision(), g); e != nil {
		t.Fatal(e)
	}
	for _, c := range s.record.Cursors {
		if !c.Next.After(*now) || c.Next.Hour() != 0 {
			t.Fatal("timezone not applied", c)
		}
	}
	first := s.record.Cursors["global"].Next
	*now = now.AddDate(0, 0, -10)
	v, e := s.View("")
	if e != nil || !v.NextRuns[0].Equal(first) {
		t.Fatal("preview precedes persisted checkpoint", v, e)
	}
}

func TestScheduleInheritanceJoinsLatestMissedWindow(t *testing.T) {
	s, _, f, now := scheduleFixture(t, 2)
	calls := 0
	f.reset = func(context.Context, string, string) (telemt.QuotaEntry, error) {
		calls++
		return telemt.QuotaEntry{}, nil
	}
	saveGlobal(t, s)
	*now = now.AddDate(0, 0, 5)
	saveUser(t, s, "user_0001", "inherit")
	*now = now.AddDate(0, 0, 2)
	s.Tick(context.Background())
	if calls != 2 {
		t.Fatal("lost missed window after joining", calls)
	}
}
