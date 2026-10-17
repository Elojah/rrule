package rrule_test

import (
	"testing"
	"time"

	"github.com/elojah/rrule"
)

func BenchmarkParse(b *testing.B) {
	for b.Loop() {
		if _, err := rrule.Parse("FREQ=MONTHLY;INTERVAL=2;UNTIL=20301231T000000Z;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseSet(b *testing.B) {
	const in = "DTSTART;TZID=Europe/Paris:20260105T090000\n" +
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR;UNTIL=20261231T230000Z\n" +
		"EXDATE;TZID=Europe/Paris:20260302T090000,20260504T090000,20260803T090000\n" +
		"RDATE;TZID=Europe/Paris:20260704T090000"

	for b.Loop() {
		if _, err := rrule.ParseSet(in, rrule.WithLoader(loadZone)); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkRule_All expands 100 occurrences of each rule.
func BenchmarkRule_All(b *testing.B) {
	utc := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	paris := time.Date(2026, 1, 5, 9, 0, 0, 0, zone(b, "Europe/Paris"))

	for _, c := range []struct {
		name  string
		rule  string
		start time.Time
	}{
		{"daily", "FREQ=DAILY", utc},
		{"daily-zoned", "FREQ=DAILY", paris},
		{"weekly", "FREQ=WEEKLY;BYDAY=MO,WE,FR", utc},
		{"monthly-bysetpos", "FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1", utc},
		{"yearly-byweekno", "FREQ=YEARLY;BYWEEKNO=1,20,-1;BYDAY=MO", utc},
		{"yearly-bymonth", "FREQ=YEARLY;BYMONTH=3,6,9,12;BYDAY=-1FR", utc},
		{"hourly-zoned", "FREQ=HOURLY;INTERVAL=7", paris},
		{"minutely-byhour", "FREQ=MINUTELY;INTERVAL=15;BYHOUR=9,10,11,12,13,14,15,16,17", utc},
		{"secondly-filtered", "FREQ=SECONDLY;BYHOUR=12;BYMINUTE=0;BYSECOND=0", utc},
	} {
		r, err := rrule.Parse(c.rule)
		if err != nil {
			b.Fatal(err)
		}

		b.Run(c.name, func(b *testing.B) {
			for b.Loop() {
				if n := len(take(r.All(c.start), 100)); n != 100 {
					b.Fatalf("%d occurrences", n)
				}
			}
		})
	}
}

// BenchmarkRule_All_None goes through every day to 9999 for a rule nothing matches.
func BenchmarkRule_All_None(b *testing.B) {
	r := rrule.Rule{Freq: rrule.Daily, ByMonth: []time.Month{time.February}, ByMonthDay: []int{30}}
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	for b.Loop() {
		for range r.All(start) {
			b.Fatal("an occurrence")
		}
	}
}

// BenchmarkSet_All merges 100 occurrences of a set with every kind of source.
func BenchmarkSet_All(b *testing.B) {
	s := parseSet(b,
		"DTSTART;TZID=Europe/Paris:20260105T090000",
		"RRULE:FREQ=WEEKLY;BYDAY=MO,WE,FR",
		"RRULE:FREQ=MONTHLY;BYMONTHDAY=15",
		"EXRULE:FREQ=MONTHLY;BYDAY=1MO",
		"EXDATE;TZID=Europe/Paris:20260302T090000,20260504T090000,20260803T090000",
		"RDATE;TZID=Europe/Paris:20260704T090000,20260815T090000",
	)

	for b.Loop() {
		if n := len(take(s.All(), 100)); n != 100 {
			b.Fatalf("%d occurrences", n)
		}
	}
}

// BenchmarkSet_After finds the next occurrence five years after the start.
func BenchmarkSet_After(b *testing.B) {
	s := rrule.Set{
		Start:  time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC),
		RRules: []rrule.Rule{{Freq: rrule.Weekly, ByDay: []rrule.WeekdayNum{{Weekday: rrule.Tuesday}}}},
	}
	at := s.Start.AddDate(5, 0, 0)

	for b.Loop() {
		if _, ok := s.After(at); !ok {
			b.Fatal("none")
		}
	}
}
