package quotareset

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"strconv"
	"strings"
	"sync"
	"time"
)

const scheduleKey = "quota_schedule_v1"

var (
	ErrScheduleInvalid     = errors.New("invalid quota schedule")
	ErrScheduleConflict    = errors.New("quota schedule changed")
	ErrScheduleStorage     = errors.New("durable quota schedule state unavailable")
	ErrScheduleUnavailable = errors.New("quota schedule state unreadable")
)

// ScheduleStore is technical state, never the optional history database.
type ScheduleStore interface {
	GetSetting(string) (string, bool, error)
	SetSetting(string, string) error
	StateDurable() bool
}

// GlobalSchedule applies only to users inheriting the common rule.
type GlobalSchedule struct {
	Enabled  bool   `json:"enabled"`
	Timezone string `json:"timezone"`
	Rule     Rule   `json:"rule"`
}

// UserSchedule represents the sparse per-user override.
type UserSchedule struct {
	Mode  string    `json:"mode"`
	Rule  Rule      `json:"rule"`
	Since time.Time `json:"since,omitempty"`
}
type checkpoint struct {
	Next       time.Time `json:"next"`
	Generation uint64    `json:"generation"`
}

// ScheduledRun separates an unconfirmed reservation from known final counts.
type ScheduledRun struct {
	ID        string     `json:"id"`
	State     string     `json:"state"`
	Started   time.Time  `json:"started_at"`
	Finished  *time.Time `json:"finished_at,omitempty"`
	Total     int        `json:"total"`
	Confirmed *int       `json:"confirmed,omitempty"`
	Unknown   *int       `json:"unknown,omitempty"`
	Rejected  *int       `json:"rejected,omitempty"`
	Remaining *int       `json:"remaining,omitempty"`
	Reason    string     `json:"reason,omitempty"`
}
type scheduleRecord struct {
	Version  int                     `json:"version"`
	Revision uint64                  `json:"revision"`
	Global   GlobalSchedule          `json:"global"`
	Users    map[string]UserSchedule `json:"users"`
	Cursors  map[string]checkpoint   `json:"cursors"`
	Last     *ScheduledRun           `json:"last,omitempty"`
}

// ScheduleView exposes only configuration summaries, not all per-user rules.
type ScheduleView struct {
	Revision      string         `json:"revision"`
	Global        GlobalSchedule `json:"global"`
	Policy        *UserSchedule  `json:"policy,omitempty"`
	Effective     bool           `json:"effective"`
	ServerZone    string         `json:"server_timezone"`
	EffectiveZone string         `json:"effective_timezone"`
	Durable       bool           `json:"durable"`
	NextRuns      []time.Time    `json:"next_runs"`
	NextDue       *time.Time     `json:"next_due,omitempty"`
	Custom        int            `json:"custom_count"`
	Excluded      int            `json:"excluded_count"`
	Last          *ScheduledRun  `json:"last,omitempty"`
	Error         string         `json:"error,omitempty"`
}

// Scheduler serializes edits and reservations; there is one loop, not one
// timer per user. All resets still pass through Manager.
type Scheduler struct {
	mu            sync.Mutex
	tickMu        sync.Mutex
	store         ScheduleStore
	manager       *Manager
	record        scheduleRecord
	loadErr       error
	lastError     string
	now           func() time.Time
	pending       *ScheduledRun
	pendingClaims map[string]checkpoint
}

func defaultGlobal(now time.Time) GlobalSchedule {
	return GlobalSchedule{Timezone: ServerZone(), Rule: Rule{Kind: "interval", Days: 30, Start: now.In(time.Local).AddDate(0, 0, 1).Format("2006-01-02"), Time: "00:00", Cron: "0 0 1 * *"}}
}

// NewScheduler reads policy without starting tasks or enabling defaults.
func NewScheduler(st ScheduleStore, m *Manager) *Scheduler {
	s := &Scheduler{store: st, manager: m, now: time.Now, record: scheduleRecord{Version: 1, Revision: 1, Global: defaultGlobal(time.Now()), Users: map[string]UserSchedule{}, Cursors: map[string]checkpoint{}}}
	raw, ok, err := st.GetSetting(scheduleKey)
	if err != nil {
		s.loadErr = err
		return s
	}
	if ok {
		if err = json.Unmarshal([]byte(raw), &s.record); err != nil || s.record.Version != 1 || s.record.Users == nil || s.record.Cursors == nil {
			s.loadErr = ErrScheduleUnavailable
			return s
		}
		if s.record.Revision == 0 || s.validate(s.record.Global.Rule, s.record.Global.Timezone) != nil {
			s.loadErr = ErrScheduleUnavailable
			return s
		}
		for name, p := range s.record.Users {
			if name == "" || (p.Mode != "inherit" && p.Mode != "off" && p.Mode != "custom") || (p.Mode == "custom" && s.validate(p.Rule, s.record.Global.Timezone) != nil) {
				s.loadErr = ErrScheduleUnavailable
				return s
			}
			if p.Mode == "custom" {
				if _, exists := s.record.Cursors["user:"+name]; !exists {
					s.loadErr = ErrScheduleUnavailable
					return s
				}
			}
		}
		if s.record.Global.Enabled {
			if _, exists := s.record.Cursors["global"]; !exists {
				s.loadErr = ErrScheduleUnavailable
				return s
			}
		}
		for key, c := range s.record.Cursors {
			if _, enabled := s.rule(key); !enabled || c.Next.IsZero() || c.Generation == 0 {
				s.loadErr = ErrScheduleUnavailable
				return s
			}
		}
	}
	if s.record.Last != nil && s.record.Last.State == "running" {
		last := *s.record.Last
		last.State = "interrupted"
		last.Reason = "panel_restarted"
		s.record.Last = &last
	}
	return s
}
func (s *Scheduler) clone() scheduleRecord {
	r := s.record
	r.Users = maps.Clone(r.Users)
	r.Cursors = maps.Clone(r.Cursors)
	return r
}
func (s *Scheduler) save(r scheduleRecord) error {
	data, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err = s.store.SetSetting(scheduleKey, string(data)); err != nil {
		s.lastError = "quota_schedule_storage"
		return ErrScheduleStorage
	}
	s.record = r
	s.lastError = ""
	return nil
}
func (s *Scheduler) revision() string { return strconv.FormatUint(s.record.Revision, 10) }
func (s *Scheduler) rule(key string) (Rule, bool) {
	if key == "global" {
		return s.record.Global.Rule, s.record.Global.Enabled
	}
	if !strings.HasPrefix(key, "user:") || len(key) <= 5 {
		return Rule{}, false
	}
	p, ok := s.record.Users[key[5:]]
	return p.Rule, ok && p.Mode == "custom"
}

func (s *Scheduler) validate(rule Rule, zone string) error {
	if _, err := rule.Preview(s.now(), zone); err != nil {
		return fmt.Errorf("%w: %s", ErrScheduleInvalid, err)
	}
	return nil
}

// View returns immutable configuration and server-calculated upcoming times.
func (s *Scheduler) View(username string) (ScheduleView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return ScheduleView{}, ErrScheduleUnavailable
	}
	r := s.record
	v := ScheduleView{Revision: s.revision(), Global: r.Global, ServerZone: ServerZone(), EffectiveZone: r.Global.Timezone, Durable: s.store.StateDurable(), Effective: r.Global.Enabled, NextRuns: []time.Time{}, Error: s.lastError}
	if r.Last != nil {
		last := *r.Last
		v.Last = &last
	}
	for _, p := range r.Users {
		if p.Mode == "custom" {
			v.Custom++
		}
		if p.Mode == "off" {
			v.Excluded++
		}
	}
	rule, key := r.Global.Rule, "global"
	if username != "" {
		p, ok := r.Users[username]
		if !ok {
			p.Mode = "inherit"
		}
		if p.Mode != "custom" {
			p.Rule = r.Global.Rule
		}
		v.Policy = &p
		if p.Mode == "off" {
			v.Effective = false
		}
		if p.Mode == "custom" {
			v.Effective = true
			rule = p.Rule
			key = "user:" + username
		}
	}
	if v.Effective {
		after := s.now()
		if c, ok := r.Cursors[key]; ok {
			next := c.Next
			if v.Policy != nil && next.Before(v.Policy.Since) {
				if candidate, err := rule.Next(v.Policy.Since, r.Global.Timezone); err == nil {
					next = candidate
				}
			}
			v.NextDue = &next
			if next.After(after) {
				after = next.Add(-time.Nanosecond)
			}
		}
		runs, err := rule.Preview(after, r.Global.Timezone)
		if err != nil {
			v.Error = "invalid_quota_schedule"
		} else {
			v.NextRuns = runs
		}
	}
	return v, nil
}

// SaveGlobal affects future slots only; timezone changes rebase all rules.
func (s *Scheduler) SaveGlobal(expected string, g GlobalSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return ErrScheduleUnavailable
	}
	if expected != s.revision() {
		return ErrScheduleConflict
	}
	if s.pending != nil {
		return ErrScheduleStorage
	}
	if g.Timezone == "server" {
		g.Timezone = ServerZone()
	}
	if err := s.validate(g.Rule, g.Timezone); err != nil {
		return err
	}
	if g.Enabled && !s.store.StateDurable() {
		return ErrScheduleStorage
	}
	r := s.clone()
	r.Revision++
	old := r.Global
	r.Global = g
	if g != old {
		delete(r.Cursors, "global")
		if g.Enabled {
			next, _ := g.Rule.Next(s.now(), g.Timezone)
			r.Cursors["global"] = checkpoint{next, r.Revision}
		}
	}
	if old.Timezone != g.Timezone {
		for name, p := range r.Users {
			p.Since = s.now().UTC()
			r.Users[name] = p
			if p.Mode == "custom" {
				next, err := p.Rule.Next(s.now(), g.Timezone)
				if err != nil {
					return ErrScheduleInvalid
				}
				r.Cursors["user:"+name] = checkpoint{next, r.Revision}
			}
		}
	}
	return s.save(r)
}

// SaveUser stores only exceptions or a changed inheritance activation boundary.
func (s *Scheduler) SaveUser(expected, name string, p UserSchedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return ErrScheduleUnavailable
	}
	if expected != s.revision() {
		return ErrScheduleConflict
	}
	if s.pending != nil {
		return ErrScheduleStorage
	}
	if p.Mode != "inherit" && p.Mode != "off" && p.Mode != "custom" {
		return ErrScheduleInvalid
	}
	if p.Mode == "custom" {
		if !s.store.StateDurable() {
			return ErrScheduleStorage
		}
		if err := s.validate(p.Rule, s.record.Global.Timezone); err != nil {
			return err
		}
	} else {
		p.Rule = s.record.Global.Rule
	}
	r := s.clone()
	r.Revision++
	p.Since = s.now().UTC()
	r.Users[name] = p
	delete(r.Cursors, "user:"+name)
	if p.Mode == "custom" {
		next, _ := p.Rule.Next(s.now(), r.Global.Timezone)
		r.Cursors["user:"+name] = checkpoint{next, r.Revision}
	}
	return s.save(r)
}

// ForgetUser removes a confirmed deletion's override without changing others.
func (s *Scheduler) ForgetUser(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadErr != nil {
		return ErrScheduleUnavailable
	}
	if _, ok := s.record.Users[name]; !ok {
		return nil
	}
	r := s.clone()
	delete(r.Users, name)
	delete(r.Cursors, "user:"+name)
	r.Revision++
	return s.save(r)
}

func (s *Scheduler) finish(run *ScheduledRun, claims map[string]checkpoint) error {
	r := s.clone()
	r.Last = run
	// Slots elapsed during this operation do not form a continuous backlog.
	for key, claim := range claims {
		if current, ok := r.Cursors[key]; ok && current.Generation == claim.Generation {
			if rule, on := s.rule(key); on {
				next, err := rule.Next(s.now(), r.Global.Timezone)
				if err == nil && next.After(current.Next) {
					current.Next = next
					r.Cursors[key] = current
				}
			}
		}
	}
	if err := s.save(r); err != nil {
		s.pending = run
		s.pendingClaims = claims
		return err
	}
	s.pending = nil
	s.pendingClaims = nil
	return nil
}

// Tick coalesces unstarted missed slots. Reserved windows are never replayed.
func (s *Scheduler) Tick(ctx context.Context) {
	if !s.tickMu.TryLock() {
		return
	}
	defer s.tickMu.Unlock()
	s.mu.Lock()
	if s.loadErr != nil {
		s.mu.Unlock()
		return
	}
	if s.pending != nil {
		if s.finish(s.pending, s.pendingClaims) != nil {
			s.mu.Unlock()
			return
		}
	}
	if !s.store.StateDurable() {
		s.mu.Unlock()
		return
	}
	due := false
	now := s.now()
	for _, c := range s.record.Cursors {
		if !c.Next.After(now) {
			due = true
			break
		}
	}
	s.mu.Unlock()
	if !due {
		return
	}
	claims := map[string]checkpoint{}
	status, err := s.manager.ExecuteScheduled(ctx, func(names []string, id string) ([]string, error) {
		s.mu.Lock()
		defer s.mu.Unlock()
		now = s.now()
		r := s.clone()
		for key, c := range r.Cursors {
			if c.Next.After(now) {
				continue
			}
			rule, on := s.rule(key)
			if !on {
				continue
			}
			next, e := rule.Next(now, r.Global.Timezone)
			if e != nil {
				return nil, ErrScheduleInvalid
			}
			claims[key] = c
			c.Next = next
			r.Cursors[key] = c
		}
		if len(claims) == 0 {
			return nil, ErrBusy
		}
		selected := []string{}
		for _, name := range names {
			p := r.Users[name]
			key := "global"
			if p.Mode == "off" {
				continue
			}
			if p.Mode == "custom" {
				key = "user:" + name
			}
			if c, ok := claims[key]; ok {
				eligible := c.Next
				if eligible.Before(p.Since) {
					rule, _ := s.rule(key)
					next, e := rule.Next(p.Since, r.Global.Timezone)
					if e != nil {
						return nil, ErrScheduleInvalid
					}
					eligible = next
				}
				if !eligible.After(now) {
					selected = append(selected, name)
				}
			}
		}
		r.Last = &ScheduledRun{ID: id, State: "running", Total: len(selected), Started: now.UTC()}
		if len(selected) == 0 {
			r.Last.State = "skipped"
			r.Last.Reason = "no_users"
		}
		if err := s.save(r); err != nil {
			return nil, err
		}
		return selected, nil
	})
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if !errors.Is(err, ErrBusy) && !errors.Is(err, ErrEmpty) {
			s.lastError = "quota_schedule_unavailable"
			if errors.Is(err, ErrScheduleStorage) {
				s.lastError = "quota_schedule_storage"
			}
		}
		return
	}
	run := &ScheduledRun{ID: status.ID, State: status.State, Started: status.StartedAt, Finished: status.FinishedAt, Total: status.Total, Confirmed: &status.Confirmed, Unknown: &status.Unknown, Rejected: &status.Rejected, Remaining: &status.Remaining, Reason: status.Reason}
	_ = s.finish(run, claims)
}

// Run is the sole scheduler loop and exits with the panel lifecycle.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			tick, cancel := context.WithTimeout(ctx, 10*time.Second)
			s.Tick(tick)
			cancel()
		}
	}
}
