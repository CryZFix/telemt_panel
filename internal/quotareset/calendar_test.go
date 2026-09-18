package quotareset

import (
	"testing"
	"time"
)

func TestCalendarParity(t *testing.T) {
	for _, tc := range []struct {
		name        string
		rule        Rule
		zone, after string
		want        []string
	}{
		{"minute fold", Rule{Kind: "cron", Cron: "* * * * *"}, "Europe/Berlin", "2026-10-25T00:59:00Z", []string{"2026-10-25T02:00:00Z", "2026-10-25T02:01:00Z", "2026-10-25T02:02:00Z"}},
		{"cron spring", Rule{Kind: "cron", Cron: "30 2 * * *"}, "Europe/Berlin", "2026-03-27T03:00:00Z", []string{"2026-03-28T01:30:00Z", "2026-03-30T00:30:00Z", "2026-03-31T00:30:00Z"}},
		{"day OR weekday", Rule{Kind: "cron", Cron: "0 0 1 * MON"}, "UTC", "2026-09-01T01:00:00Z", []string{"2026-09-07T00:00:00Z", "2026-09-14T00:00:00Z", "2026-09-21T00:00:00Z"}},
		{"30 days", Rule{Kind: "interval", Days: 30, Start: "2026-01-31", Time: "00:00"}, "UTC", "2026-01-31T00:00:00Z", []string{"2026-03-02T00:00:00Z", "2026-04-01T00:00:00Z", "2026-05-01T00:00:00Z"}},
		{"month", Rule{Kind: "cron", Cron: "0 0 1 * *"}, "UTC", "2026-01-31T00:00:00Z", []string{"2026-02-01T00:00:00Z", "2026-03-01T00:00:00Z", "2026-04-01T00:00:00Z"}},
		{"spring", Rule{Kind: "interval", Days: 1, Start: "2026-03-28", Time: "02:30"}, "Europe/Berlin", "2026-03-27T03:00:00Z", []string{"2026-03-28T01:30:00Z", "2026-03-30T00:30:00Z", "2026-03-31T00:30:00Z"}},
		{"fold", Rule{Kind: "cron", Cron: "30 2 * * *"}, "Europe/Berlin", "2026-10-24T01:00:00Z", []string{"2026-10-25T00:30:00Z", "2026-10-26T01:30:00Z", "2026-10-27T01:30:00Z"}},
		{"after fold", Rule{Kind: "cron", Cron: "30 2 * * *"}, "Europe/Berlin", "2026-10-25T00:40:00Z", []string{"2026-10-26T01:30:00Z", "2026-10-27T01:30:00Z", "2026-10-28T01:30:00Z"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			at, _ := time.Parse(time.RFC3339, tc.after)
			out, err := tc.rule.Preview(at, tc.zone)
			if err != nil {
				t.Fatal(err)
			}
			for i, v := range out {
				if v.UTC().Format(time.RFC3339) != tc.want[i] {
					t.Fatalf("%d: %s want %s", i, v.UTC(), tc.want[i])
				}
			}
		})
	}
}
func TestCronValidation(t *testing.T) {
	for _, expr := range []string{"0 0 * * 7", "0 0 * * SUN", "*/15 8-18 * * MON-FRI", "0 0 1 * MON"} {
		if _, e := (Rule{Kind: "cron", Cron: expr}).Preview(time.Now(), "UTC"); e != nil {
			t.Error(expr, e)
		}
	}
	for _, expr := range []string{"0 0 31 2 *", "61 0 * * *", "@daily", "0 0 * * * *", "0 0 L * *", "0 0 * JANFEB *", "0 0 * * SUN7"} {
		if _, e := (Rule{Kind: "cron", Cron: expr}).Preview(time.Now(), "UTC"); e == nil {
			t.Error("accepted", expr)
		}
	}
}

func TestInvalidCalendarValues(t *testing.T) {
	for _, r := range []Rule{
		{Kind: "interval", Days: 0, Start: "2026-09-01", Time: "00:00"},
		{Kind: "interval", Days: 1, Start: "2026-02-29", Time: "00:00"},
		{Kind: "interval", Days: 1, Start: "2026-09-01", Time: "24:00"},
		{Kind: "interval", Days: 3650001, Start: "2026-09-01", Time: "00:00"},
	} {
		if _, e := r.Preview(time.Now(), "UTC"); e == nil {
			t.Fatal("accepted invalid calendar", r)
		}
	}
	if _, e := daily().Preview(time.Now(), "Unknown/Timezone"); e == nil {
		t.Fatal("accepted invalid timezone")
	}
}
