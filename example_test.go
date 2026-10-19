package rrule_test

import (
	"fmt"
	"time"

	"github.com/elojah/rrule"
)

func ExampleParse() {
	// The last Friday of each month, 3 times.
	r, err := rrule.Parse("FREQ=MONTHLY;BYDAY=-1FR;COUNT=3")
	if err != nil {
		fmt.Println(err)

		return
	}

	for t := range r.All(time.Date(2026, 1, 1, 17, 0, 0, 0, time.UTC)) {
		fmt.Println(t.Format("Mon 2006-01-02 15:04"))
	}

	// Output:
	// Fri 2026-01-30 17:00
	// Fri 2026-02-27 17:00
	// Fri 2026-03-27 17:00
}

func ExampleParse_xFreq() {
	// Every quarter of a second, 5 times: readers that do not know X-FREQ read every second.
	r, err := rrule.Parse("FREQ=SECONDLY;X-FREQ=PT0.25S;COUNT=5")
	if err != nil {
		fmt.Println(err)

		return
	}

	fmt.Println(r.XFreq)

	for t := range r.All(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)) {
		fmt.Println(t.Format("15:04:05.000"))
	}

	// Output:
	// 250ms
	// 12:00:00.000
	// 12:00:00.250
	// 12:00:00.500
	// 12:00:00.750
	// 12:00:01.000
}

func ExampleRule_All() {
	// The first Monday of each quarter, forever: the loop ends it.
	r := rrule.Rule{Freq: rrule.Monthly, Interval: 3, ByDay: []rrule.WeekdayNum{rrule.Monday.Nth(1)}}

	for t := range r.All(time.Date(2026, 1, 1, 9, 0, 0, 0, time.UTC)) {
		if t.Year() > 2026 {
			break
		}

		fmt.Println(t.Format("Mon 2006-01-02"))
	}

	// Output:
	// Mon 2026-01-05
	// Mon 2026-04-06
	// Mon 2026-07-06
	// Mon 2026-10-05
}

func ExampleParseSet() {
	// Floating times, without a zone, are read in WithLocation's location.
	set, err := rrule.ParseSet("DTSTART:20260105T090000\nRRULE:FREQ=DAILY;COUNT=5\nEXDATE:20260107T090000",
		rrule.WithLocation(time.UTC))
	if err != nil {
		fmt.Println(err)

		return
	}

	for t := range set.All() {
		fmt.Println(t.Format("Mon 2006-01-02 15:04"))
	}

	// Output:
	// Mon 2026-01-05 09:00
	// Tue 2026-01-06 09:00
	// Thu 2026-01-08 09:00
	// Fri 2026-01-09 09:00
}

func ExampleSet() {
	// New Year's Day, then the first Monday of each January, twice: Start is an occurrence, whether the rule matches it
	// or not.
	s := rrule.Set{
		Start: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Form:  rrule.Date,
		RRules: []rrule.Rule{{
			Freq: rrule.Yearly, Count: 2, ByMonth: []time.Month{time.January},
			ByDay: []rrule.WeekdayNum{rrule.Monday.Nth(1)},
		}},
	}

	fmt.Println(s)

	for t := range s.All() {
		fmt.Println(t.Format("Mon 2006-01-02"))
	}

	// Output:
	// DTSTART;VALUE=DATE:20260101
	// RRULE:FREQ=YEARLY;COUNT=2;BYDAY=1MO;BYMONTH=1
	// Thu 2026-01-01
	// Mon 2026-01-05
	// Mon 2027-01-04
}

func ExampleSet_Between() {
	s := rrule.Set{
		Start:  time.Date(2026, 1, 5, 12, 0, 0, 0, time.UTC),
		RRules: []rrule.Rule{{Freq: rrule.Weekly, ByDay: []rrule.WeekdayNum{{Weekday: rrule.Monday}}}},
	}

	// February, the 1st included, March's excluded.
	for t := range s.Between(time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC), time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)) {
		fmt.Println(t.Format("Mon 2006-01-02"))
	}

	// Output:
	// Mon 2026-02-02
	// Mon 2026-02-09
	// Mon 2026-02-16
	// Mon 2026-02-23
}
