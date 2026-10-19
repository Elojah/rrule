package rrule_test

import (
	"testing"
	"time"

	"github.com/elojah/rrule"
)

// TestAll_DST checks occurrences around daylight saving time transitions: a wall clock that happens twice is its first
// instant, one that never happens is read with the offset from before the gap, and instants always increase.
func TestAll_DST(t *testing.T) {
	t.Parallel()

	const layout = "2006-01-02 15:04 -0700"

	cases := []struct {
		name  string
		zone  string
		start string // in zone, as time.DateTime
		rule  string
		want  []string // UTC
	}{
		{
			name:  "daily into a gap moves forward by the gap",
			zone:  "America/New_York",
			start: "2026-03-07 02:30:00",
			rule:  "FREQ=DAILY;COUNT=3",
			want:  []string{"2026-03-07 07:30 +0000", "2026-03-08 07:30 +0000", "2026-03-09 06:30 +0000"},
		},
		{
			name:  "daily on a repeated wall clock is its first instant",
			zone:  "America/New_York",
			start: "2026-10-31 01:30:00",
			rule:  "FREQ=DAILY;COUNT=3",
			want:  []string{"2026-10-31 05:30 +0000", "2026-11-01 05:30 +0000", "2026-11-02 06:30 +0000"},
		},
		{
			name:  "east of UTC too, where time.Date picks the second",
			zone:  "Europe/Paris",
			start: "2026-10-24 02:30:00",
			rule:  "FREQ=DAILY;COUNT=3",
			want:  []string{"2026-10-24 00:30 +0000", "2026-10-25 00:30 +0000", "2026-10-26 01:30 +0000"},
		},
		{
			name:  "a gap east of UTC",
			zone:  "Europe/Paris",
			start: "2026-03-28 02:30:00",
			rule:  "FREQ=DAILY;COUNT=3",
			want:  []string{"2026-03-28 01:30 +0000", "2026-03-29 01:30 +0000", "2026-03-30 00:30 +0000"},
		},
		{
			name:  "a 30 minute gap",
			zone:  "Australia/Lord_Howe",
			start: "2026-10-03 02:15:00",
			rule:  "FREQ=DAILY;COUNT=3",
			want:  []string{"2026-10-02 15:45 +0000", "2026-10-03 15:45 +0000", "2026-10-04 15:15 +0000"},
		},
		{
			name:  "a 30 minute repeat",
			zone:  "Australia/Lord_Howe",
			start: "2026-04-04 01:45:00",
			rule:  "FREQ=DAILY;COUNT=3",
			want:  []string{"2026-04-03 14:45 +0000", "2026-04-04 14:45 +0000", "2026-04-05 15:15 +0000"},
		},
		{
			// 02:00 moves to 03:00, which the next hour is too: each instant comes once, and COUNT counts it once.
			name:  "hourly across a gap",
			zone:  "America/New_York",
			start: "2026-03-08 00:00:00",
			rule:  "FREQ=HOURLY;COUNT=4",
			want: []string{
				"2026-03-08 05:00 +0000", "2026-03-08 06:00 +0000", "2026-03-08 07:00 +0000", "2026-03-08 08:00 +0000",
			},
		},
		{
			// 02:00 and 02:30 move to 03:00 and 03:30, before the wall clock gets there: the instants stay in order.
			name:  "half-hourly across a gap",
			zone:  "America/New_York",
			start: "2026-03-08 01:30:00",
			rule:  "FREQ=MINUTELY;INTERVAL=30;COUNT=4",
			want: []string{
				"2026-03-08 06:30 +0000", "2026-03-08 07:00 +0000", "2026-03-08 07:30 +0000", "2026-03-08 08:00 +0000",
			},
		},
		{
			// The wall clock goes 01:00 and 02:00: the second 01:00, 06:00 UTC, is not one.
			name:  "hourly across a repeat",
			zone:  "America/New_York",
			start: "2026-11-01 00:00:00",
			rule:  "FREQ=HOURLY;COUNT=4",
			want: []string{
				"2026-11-01 04:00 +0000", "2026-11-01 05:00 +0000", "2026-11-01 07:00 +0000", "2026-11-01 08:00 +0000",
			},
		},
		{
			name:  "UNTIL is an instant",
			zone:  "Europe/Paris",
			start: "2026-10-24 09:00:00",
			rule:  "FREQ=DAILY;UNTIL=20261026T080000Z",
			want:  []string{"2026-10-24 07:00 +0000", "2026-10-25 08:00 +0000", "2026-10-26 08:00 +0000"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			loc := zone(t, c.zone)

			start, err := time.ParseInLocation(time.DateTime, c.start, loc)
			if err != nil {
				t.Fatal(err)
			}

			r, err := rrule.Parse(c.rule)
			if err != nil {
				t.Fatal(err)
			}

			got := take(r.All(start), -1)
			for _, o := range got {
				if o.Location() != loc {
					t.Fatalf("%s in %s, want %s", o, o.Location(), loc)
				}
			}

			utc := make([]time.Time, len(got))
			for i, o := range got {
				utc[i] = o.UTC()
			}

			equal(t, format(utc, layout), c.want)
		})
	}
}

// TestAll_Ends checks that rules nothing matches, or that a period never reaches, end, and quickly.
func TestAll_Ends(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for _, rule := range []string{
		"FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30",
		"FREQ=MONTHLY;BYMONTH=2;BYMONTHDAY=31",
		"FREQ=DAILY;BYMONTH=4;BYMONTHDAY=31",
		"FREQ=WEEKLY;INTERVAL=2;BYMONTH=2;BYDAY=MO;BYSETPOS=6",
		"FREQ=HOURLY;BYMONTH=2;BYMONTHDAY=30",
		"FREQ=SECONDLY;BYMONTH=2;BYMONTHDAY=30",
		"FREQ=SECONDLY;BYMONTH=2;BYMONTHDAY=30;UNTIL=20270101T000000Z",
		"FREQ=HOURLY;INTERVAL=24;BYHOUR=1",
		"FREQ=HOURLY;INTERVAL=6;BYHOUR=3,5",
		"FREQ=MINUTELY;INTERVAL=1440;BYHOUR=5",
		"FREQ=MINUTELY;INTERVAL=120;BYMINUTE=30",
		"FREQ=SECONDLY;INTERVAL=86400;BYSECOND=1",
		"FREQ=SECONDLY;BYSECOND=60",
		"FREQ=DAILY;BYSECOND=60",
		"FREQ=DAILY;UNTIL=20251231T000000Z",
		"FREQ=SECONDLY;BYMONTH=1;BYSETPOS=2",
		"FREQ=HOURLY;X-FREQ=PT24H;BYHOUR=1",
		"FREQ=SECONDLY;X-FREQ=PT2.5S;BYSECOND=1,3,4",
		"FREQ=SECONDLY;X-FREQ=PT0.000000001S;BYMONTH=2;BYMONTHDAY=30",
		"FREQ=SECONDLY;X-FREQ=PT0.000000001S;BYSECOND=60",
		"FREQ=SECONDLY;X-FREQ=PT0.5S;BYMONTH=1;BYSETPOS=2",
	} {
		t.Run(rule, func(t *testing.T) {
			t.Parallel()

			r, err := rrule.Parse(rule)
			if err != nil {
				t.Fatal(err)
			}

			if got := take(r.All(start), -1); len(got) != 0 {
				t.Fatalf("got %s", got)
			}
		})
	}
}

// TestAll_XFreq checks that an X-FREQ of whole seconds, minutes or hours yields what FREQ and INTERVAL do, which
// INTERVAL then does not change, across both changes of daylight saving time, with BYxxx parts that limit under FREQ.
func TestAll_XFreq(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 5, 9, 0, 0, 0, zone(t, "Europe/Paris"))
	end := start.AddDate(1, 0, 0)

	for rule, xfreq := range map[string]string{
		"FREQ=HOURLY;INTERVAL=5;BYHOUR=3,8,13":                             "PT5H",
		"FREQ=HOURLY;INTERVAL=7;BYMONTHDAY=-1":                             "PT7H",
		"FREQ=MINUTELY;INTERVAL=20;BYHOUR=1,2,3;BYDAY=SU":                  "PT20M",
		"FREQ=SECONDLY;INTERVAL=90;BYMINUTE=0,1,2;BYHOUR=2,3;BYMONTH=3,10": "PT1M30S",
	} {
		t.Run(rule, func(t *testing.T) {
			t.Parallel()

			r, err := rrule.Parse(rule)
			if err != nil {
				t.Fatal(err)
			}

			x, err := rrule.Parse(rule + ";X-FREQ=" + xfreq)
			if err != nil {
				t.Fatal(err)
			}

			want := take(r.Between(start, start, end), -1)
			if len(want) == 0 {
				t.Fatal("no occurrences")
			}

			equal(t, format(take(x.Between(start, start, end), -1), time.RFC3339), format(want, time.RFC3339))
		})
	}
}

// TestAll_XFreqDST checks that an X-FREQ keeps to the wall clock across a gap, as FREQ does: a wall clock in the gap
// moves forward by it, and the wall clocks after it, the same instants, do not come again.
func TestAll_XFreqDST(t *testing.T) {
	t.Parallel()

	r, err := rrule.Parse("FREQ=SECONDLY;X-FREQ=PT0.5S;BYMINUTE=59;BYSECOND=59;COUNT=6")
	if err != nil {
		t.Fatal(err)
	}

	got := take(r.All(time.Date(2026, 3, 8, 1, 59, 59, 0, zone(t, "America/New_York"))), -1)
	for i, o := range got {
		got[i] = o.UTC()
	}

	equal(t, format(got, "15:04:05.0"), []string{
		"06:59:59.0", "06:59:59.5", "07:59:59.0", "07:59:59.5", "08:59:59.0", "08:59:59.5",
	})
}

// TestAll checks the rest of what All promises.
func TestAll(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 5, 9, 30, 15, 500, time.UTC) // a Monday

	cases := []struct {
		name  string
		rule  rrule.Rule
		start time.Time
		n     int
		want  []string
	}{
		{
			name:  "the start only if it matches",
			rule:  rrule.Rule{Freq: rrule.Weekly, ByDay: []rrule.WeekdayNum{{Weekday: rrule.Tuesday}}},
			start: start,
			n:     2,
			want:  []string{"2026-01-06 09:30:15.0000005", "2026-01-13 09:30:15.0000005"},
		},
		{
			name:  "BYSETPOS among times",
			rule:  rrule.Rule{Freq: rrule.Daily, ByHour: []int{17, 9}, ByMinute: []int{0}, BySetPos: []int{-1}},
			start: start,
			n:     2,
			want:  []string{"2026-01-05 17:00:15.0000005", "2026-01-06 17:00:15.0000005"},
		},
		{
			name:  "no leap second",
			rule:  rrule.Rule{Freq: rrule.Minutely, BySecond: []int{60, 0}, Count: 2},
			start: start,
			n:     -1,
			want:  []string{"2026-01-05 09:31:00.0000005", "2026-01-05 09:32:00.0000005"},
		},
		{
			name:  "ends in 9999",
			rule:  rrule.Rule{Freq: rrule.Yearly},
			start: time.Date(9998, 3, 1, 0, 0, 0, 0, time.UTC),
			n:     -1,
			want:  []string{"9998-03-01 00:00:00", "9999-03-01 00:00:00"},
		},
		{
			name: "weeks into 10000 too",
			rule: rrule.Rule{
				Freq: rrule.Weekly, ByDay: []rrule.WeekdayNum{{Weekday: rrule.Friday}, {Weekday: rrule.Saturday}},
			},
			start: time.Date(9999, 12, 20, 0, 0, 0, 0, time.UTC),
			n:     -1,
			want:  []string{"9999-12-24 00:00:00", "9999-12-25 00:00:00", "9999-12-31 00:00:00"},
		},
		{
			name:  "none after 9999",
			rule:  rrule.Rule{Freq: rrule.Daily},
			start: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
			n:     -1,
		},
		{
			name:  "an interval too large to reach a second period",
			rule:  rrule.Rule{Freq: rrule.Secondly, Interval: 1 << 62, Count: 2},
			start: start,
			n:     -1,
			want:  []string{"2026-01-05 09:30:15.0000005"},
		},
		{
			name:  "weeks of an interval too large",
			rule:  rrule.Rule{Freq: rrule.Weekly, Interval: 1 << 61, Count: 2},
			start: start,
			n:     -1,
			want:  []string{"2026-01-05 09:30:15.0000005"},
		},
		{
			name:  "nothing for an invalid rule",
			rule:  rrule.Rule{Freq: rrule.Daily, ByMonthDay: []int{0}},
			start: start,
			n:     -1,
		},
		{
			name:  "nth weekday of the year without BYMONTH",
			rule:  rrule.Rule{Freq: rrule.Yearly, ByDay: []rrule.WeekdayNum{rrule.Monday.Nth(1), rrule.Friday.Nth(-1)}},
			start: start,
			n:     3,
			want:  []string{"2026-01-05 09:30:15.0000005", "2026-12-25 09:30:15.0000005", "2027-01-04 09:30:15.0000005"},
		},
		{
			name: "plain and numbered weekdays together",
			rule: rrule.Rule{
				Freq: rrule.Monthly, ByDay: []rrule.WeekdayNum{{Weekday: rrule.Sunday}, rrule.Monday.Nth(1)},
			},
			start: start,
			n:     3,
			want:  []string{"2026-01-05 09:30:15.0000005", "2026-01-11 09:30:15.0000005", "2026-01-18 09:30:15.0000005"},
		},
		{
			name:  "the last week of the year before, by BYWEEKNO=-1",
			rule:  rrule.Rule{Freq: rrule.Yearly, ByWeekNo: []int{-1}, ByDay: []rrule.WeekdayNum{{Weekday: rrule.Friday}}},
			start: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), // a Friday, in week 53 of 2026
			n:     2,
			want:  []string{"2027-01-01 00:00:00", "2027-12-31 00:00:00"},
		},
		{
			name:  "X-FREQ, from the start's nanoseconds",
			rule:  rrule.Rule{Freq: rrule.Secondly, XFreq: 500 * time.Millisecond},
			start: start,
			n:     3,
			want:  []string{"2026-01-05 09:30:15.0000005", "2026-01-05 09:30:15.5000005", "2026-01-05 09:30:16.0000005"},
		},
		{
			name:  "X-FREQ without FREQ's interval, nor its minute from the start",
			rule:  rrule.Rule{Freq: rrule.Hourly, Interval: 5, XFreq: 90 * time.Minute},
			start: start,
			n:     3,
			want:  []string{"2026-01-05 09:30:15.0000005", "2026-01-05 11:00:15.0000005", "2026-01-05 12:30:15.0000005"},
		},
		{
			name:  "X-FREQ within the seconds BYSECOND limits to",
			rule:  rrule.Rule{Freq: rrule.Secondly, XFreq: 400 * time.Millisecond, BySecond: []int{0}},
			start: time.Date(2026, 1, 5, 9, 30, 0, 0, time.UTC),
			n:     5,
			want: []string{
				"2026-01-05 09:30:00", "2026-01-05 09:30:00.4", "2026-01-05 09:30:00.8", "2026-01-05 09:31:00",
				"2026-01-05 09:31:00.4",
			},
		},
		{
			name: "X-FREQ, a period of which has one candidate for BYSETPOS",
			rule: rrule.Rule{
				Freq: rrule.Yearly, ByMonth: []time.Month{time.January}, BySetPos: []int{-1}, XFreq: time.Hour,
			},
			start: start,
			n:     2,
			want:  []string{"2026-01-05 09:30:15.0000005", "2026-01-05 10:30:15.0000005"},
		},
		{
			name:  "X-FREQ, with a numbered weekday of the year as FREQ says",
			rule:  rrule.Rule{Freq: rrule.Yearly, ByDay: []rrule.WeekdayNum{rrule.Monday.Nth(2)}, XFreq: 12 * time.Hour},
			start: start,
			n:     3,
			want:  []string{"2026-01-12 09:30:15.0000005", "2026-01-12 21:30:15.0000005", "2027-01-11 09:30:15.0000005"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			equal(t, format(take(c.rule.All(c.start), c.n), "2006-01-02 15:04:05.999999999"), c.want)
		})
	}
}

// TestRule_Between checks that Between yields All's occurrences at or after from and before to, and without to, all
// of them after from.
func TestRule_Between(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	from, to := start.AddDate(0, 2, 0), start.AddDate(1, 0, 0)

	for _, rule := range []string{
		"FREQ=DAILY",
		"FREQ=WEEKLY;BYDAY=MO,FR;COUNT=40",
		"FREQ=MONTHLY;BYDAY=-1FR",
		"FREQ=HOURLY;INTERVAL=5;BYHOUR=3,8,13",
		"FREQ=YEARLY;BYMONTH=2;BYMONTHDAY=30",
		"FREQ=DAILY;UNTIL=20260201T000000Z",
	} {
		t.Run(rule, func(t *testing.T) {
			t.Parallel()

			r, err := rrule.Parse(rule)
			if err != nil {
				t.Fatal(err)
			}

			var want []time.Time

			for o := range r.All(start) {
				if !o.Before(to) {
					break
				}

				if !o.Before(from) {
					want = append(want, o)
				}
			}

			const layout = time.DateTime

			equal(t, format(take(r.Between(start, from, to), -1), layout), format(want, layout))

			if len(want) > 0 {
				equal(t, format(take(r.Between(start, from, time.Time{}), len(want)), layout), format(want, layout))
			}
		})
	}
}
