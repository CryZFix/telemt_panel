package quotareset

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	_ "time/tzdata"

	"github.com/adhocore/gronx"
)

// Rule is either a calendar-day interval or a five-field cron expression.
type Rule struct {
	Kind  string `json:"kind"`
	Days  int    `json:"days,omitempty"`
	Start string `json:"start,omitempty"`
	Time  string `json:"time,omitempty"`
	Cron  string `json:"cron,omitempty"`
}

var cronField = regexp.MustCompile(`^[0-9*,/\-]+$`)
var cronName = regexp.MustCompile(`\b[A-Z]{3}\b`)
var monthNames = strings.NewReplacer("JAN", "1", "FEB", "2", "MAR", "3", "APR", "4", "MAY", "5", "JUN", "6", "JUL", "7", "AUG", "8", "SEP", "9", "OCT", "10", "NOV", "11", "DEC", "12")
var dayNames = strings.NewReplacer("SUN", "0", "MON", "1", "TUE", "2", "WED", "3", "THU", "4", "FRI", "5", "SAT", "6")

func cronExpression(expression string) (string, error) {
	f := strings.Fields(strings.ToUpper(strings.TrimSpace(expression)))
	if len(f) != 5 || len(expression) > 160 {
		return "", errors.New("cron requires five fields without seconds or a command")
	}
	f[3] = cronName.ReplaceAllStringFunc(f[3], monthNames.Replace)
	f[4] = cronName.ReplaceAllStringFunc(f[4], dayNames.Replace)
	for _, v := range f {
		if !cronField.MatchString(v) {
			return "", errors.New("unsupported cron field; use standard numbers, names, lists, ranges and steps")
		}
	}
	expr := strings.Join(f, " ")
	if !gronx.IsValid(expr) {
		return "", errors.New("invalid cron ranges or steps")
	}
	return expr, nil
}

// ServerZone resolves the process timezone without trusting a browser's zone.
func ServerZone() string {
	candidates := []string{strings.TrimPrefix(os.Getenv("TZ"), ":"), time.Local.String()}
	if data, err := os.ReadFile("/etc/timezone"); err == nil {
		candidates = append(candidates, strings.TrimSpace(string(data)))
	}
	if path, err := filepath.EvalSymlinks("/etc/localtime"); err == nil {
		if _, zone, ok := strings.Cut(path, "/zoneinfo/"); ok {
			candidates = append(candidates, zone)
		}
	}
	for _, zone := range candidates {
		if zone != "" && zone != "Local" {
			if _, err := time.LoadLocation(zone); err == nil {
				return zone
			}
		}
	}
	return "Local"
}

func location(zone string) (*time.Location, error) {
	if zone == "server" {
		zone = ServerZone()
	}
	if len(zone) > 128 || strings.TrimSpace(zone) != zone || zone == "" {
		return nil, errors.New("invalid timezone")
	}
	return time.LoadLocation(zone)
}

// wallTime resolves gaps by skipping them and folds to the first occurrence.
func wallTime(day time.Time, hour, minute int, loc *time.Location) (time.Time, bool) {
	value := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, loc)
	matches := func(t time.Time) bool {
		v := t.In(loc)
		return v.Year() == day.Year() && v.Month() == day.Month() && v.Day() == day.Day() && v.Hour() == hour && v.Minute() == minute
	}
	if !matches(value) {
		return time.Time{}, false
	}
	// A fold may have a non-hour offset change (e.g. Australia/Lord_Howe).
	for _, sample := range []time.Time{value.Add(-48 * time.Hour), value.Add(48 * time.Hour)} {
		_, offset := value.Zone()
		_, other := sample.In(loc).Zone()
		candidate := value.Add(time.Duration(offset-other) * time.Second)
		if candidate.Before(value) && matches(candidate) {
			value = candidate
		}
	}
	return value, true
}

// Next returns the next strictly future slot, independent of previous resets.
func (r Rule) Next(after time.Time, zone string) (time.Time, error) {
	loc, err := location(zone)
	if err != nil {
		return time.Time{}, err
	}
	after = after.In(loc)
	if r.Kind == "cron" {
		expr, err := cronExpression(r.Cron)
		if err != nil {
			return time.Time{}, err
		}
		cursor := after
		// Even minute-level rules must skip the whole repeated wall-time range.
		// IANA's historical date-line transitions can repeat a full day.
		for i := 0; i < 1501; i++ {
			next, err := gronx.NextTickAfter(expr, cursor, false)
			if err != nil || !next.After(cursor) {
				return time.Time{}, errors.New("cron has no next run")
			}
			canonical, ok := wallTime(next, next.Hour(), next.Minute(), loc)
			if ok && canonical.After(after) {
				return canonical, nil
			}
			cursor = next
		}
		return time.Time{}, errors.New("cron cannot resolve local time")
	}
	if r.Kind != "interval" || r.Days < 1 || r.Days > 3650000 {
		return time.Time{}, errors.New("invalid day interval")
	}
	anchor, err := time.Parse("2006-01-02", r.Start)
	if err != nil {
		return time.Time{}, errors.New("invalid first date")
	}
	clock, err := time.Parse("15:04", r.Time)
	if err != nil {
		return time.Time{}, errors.New("invalid time")
	}
	current := time.Date(after.Year(), after.Month(), after.Day(), 0, 0, 0, 0, time.UTC)
	days := int(current.Unix()/86400 - anchor.Unix()/86400)
	index := max(0, days/r.Days)
	for i := 0; i < 16; i++ {
		day := anchor.AddDate(0, 0, index*r.Days)
		if day.Year() > 9999 {
			return time.Time{}, errors.New("date out of range")
		}
		if candidate, ok := wallTime(day, clock.Hour(), clock.Minute(), loc); ok && candidate.After(after) {
			return candidate, nil
		}
		index++
	}
	return time.Time{}, errors.New("interval has no next run")
}

// Preview uses the same calculator as execution; it never creates a job.
func (r Rule) Preview(after time.Time, zone string) ([]time.Time, error) {
	result := make([]time.Time, 0, 3)
	for i := 0; i < 3; i++ {
		next, err := r.Next(after, zone)
		if err != nil {
			return nil, fmt.Errorf("schedule: %w", err)
		}
		result = append(result, next)
		after = next
	}
	return result, nil
}
