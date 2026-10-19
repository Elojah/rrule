package rrule_test

import (
	"errors"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/elojah/rrule"
)

// seedRules returns rules to seed the fuzz targets with: the RFC's examples', and some of testdata/dateutil.txt's.
func seedRules(f *testing.F) []string {
	f.Helper()

	rules := []string{
		"FREQ=DAILY;COUNT=10",
		"FREQ=WEEKLY;INTERVAL=2;UNTIL=19971224T000000Z;WKST=SU;BYDAY=MO,WE,FR",
		"FREQ=MONTHLY;INTERVAL=2;COUNT=10;BYDAY=1SU,-1SU",
		"FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-2",
		"FREQ=YEARLY;BYWEEKNO=20;BYDAY=MO",
		"FREQ=YEARLY;INTERVAL=3;COUNT=10;BYYEARDAY=1,100,200",
		"FREQ=MINUTELY;INTERVAL=20;BYHOUR=9,10,11,12,13,14,15,16",
		"FREQ=HOURLY;INTERVAL=3;UNTIL=19970902T170000Z",
		"FREQ=SECONDLY;BYSECOND=0,60;COUNT=3",
		"FREQ=SECONDLY;X-FREQ=PT0.5S;COUNT=10",
		"FREQ=HOURLY;INTERVAL=3;X-FREQ=PT1H30M;BYDAY=MO,FR",
		"FREQ=MINUTELY;X-FREQ=PT0.001S;BYHOUR=12;BYSECOND=0,30",
	}

	b, err := os.ReadFile("testdata/dateutil.txt")
	if err != nil {
		f.Fatal(err)
	}

	for line := range strings.SplitSeq(string(b), "\n") {
		if r, ok := strings.CutPrefix(line, "RRULE:"); ok && len(rules) < 100 {
			rules = append(rules, r)
		}
	}

	return rules
}

// FuzzParse checks that Parse refuses with a *PartError, and that what it reads, String writes back to the same rule.
func FuzzParse(f *testing.F) {
	for _, r := range seedRules(f) {
		f.Add(r)
	}

	f.Fuzz(func(t *testing.T, s string) {
		r, err := rrule.Parse(s)
		if err != nil {
			if pe := (*rrule.PartError)(nil); !errors.As(err, &pe) {
				t.Fatalf("%T, want a *rrule.PartError", err)
			}

			return
		}

		again, err := rrule.Parse(r.String())
		if err != nil {
			t.Fatalf("%s: %v", r, err)
		}

		if !again.Until.Equal(r.Until) {
			t.Fatalf("UNTIL %s, was %s", again.Until, r.Until)
		}

		again.Until, r.Until = time.Time{}, time.Time{}
		if !reflect.DeepEqual(again, r) {
			t.Fatalf("got %+v, was %+v", again, r)
		}
	})
}

// FuzzParseSet checks that what ParseSet reads, String writes back to the same occurrences.
func FuzzParseSet(f *testing.F) {
	for _, r := range seedRules(f)[:20] {
		f.Add("DTSTART;TZID=America/New_York:19970902T090000\nRRULE:" + r + "\nEXDATE;TZID=Europe/Paris:19970904T150000")
	}

	f.Add("DTSTART;VALUE=DATE:20260101\nRRULE:FREQ=YEARLY;UNTIL=20300101\nRDATE;VALUE=DATE:20260704")
	f.Add("DTSTART:20260105T090000\nRRULE:FREQ=WEEKLY;BYDAY=MO,\n FR\nEXRULE:FREQ=MONTHLY\r\nRDATE:20260106T090000Z")

	f.Fuzz(func(t *testing.T, in string) {
		s, err := rrule.ParseSet(in, rrule.WithLoader(loadZone))
		if err != nil {
			return
		}

		again, err := rrule.ParseSet(s.String(), rrule.WithLoader(loadZone))
		if err != nil {
			t.Fatalf("%s: %v", s, err)
		}

		// A week at most, so that a set whose exclusions take everything away ends.
		end := s.Start.AddDate(0, 0, 7)
		got, want := take(again.Between(again.Start, end), 50), take(s.Between(s.Start, end), 50)

		if !slices.EqualFunc(got, want, func(a, b time.Time) bool {
			return a.Equal(b) && a.Location().String() == b.Location().String()
		}) {
			t.Fatalf("%s:\ngot %s\nwant %s", s, got, want)
		}
	})
}

// FuzzAll checks what any rule's occurrences hold to: they increase, from start, to UNTIL and COUNT, each passes the
// rule's BYxxx parts, checked here without the expansion's code, and is a number of X-FREQs from start, which is in
// UTC. It looks 30 years ahead at most, with Between, since All goes on to 9999 for a rule nothing matches.
func FuzzAll(f *testing.F) {
	for i, r := range seedRules(f) {
		f.Add(r, int64(i)*7919*86400+int64(i)*3607)
	}

	f.Fuzz(func(t *testing.T, s string, at int64) {
		r, err := rrule.Parse(s)
		if err != nil {
			return
		}

		const span = 200 * 365 * 86400 // from 1970

		start := time.Unix((at%span+span)%span, 0).UTC()
		n := 0

		var prev time.Time

		for o := range r.Between(start, start, start.AddDate(30, 0, 0)) {
			if n == 200 {
				break
			}

			switch {
			case o.Before(start):
				t.Fatalf("%s before the start", o)
			case n > 0 && !o.After(prev):
				t.Fatalf("%s after %s", o, prev)
			case !r.Until.IsZero() && o.After(r.Until):
				t.Fatalf("%s after UNTIL", o)
			case r.Count > 0 && n == r.Count:
				t.Fatalf("%s past COUNT", o)
			case r.XFreq > 0 && o.Sub(start)%r.XFreq != 0:
				t.Fatalf("%s not a number of X-FREQs from the start", o)
			}

			if why := mismatch(r, start, o); why != "" {
				t.Fatalf("%s: %s", o, why)
			}

			prev = o
			n++
		}
	})
}

// mismatch returns which of r's BYxxx parts, or the defaults it takes from start, o does not match, or "".
func mismatch(r rrule.Rule, start, o time.Time) string {
	freq := r.Freq
	if r.XFreq > 0 {
		freq = rrule.Secondly // whose parts all limit, and take nothing from start
	}

	clock := func(part string, values []int, v, def int, from rrule.Frequency) string {
		if len(values) > 0 && !slices.Contains(values, v) || len(values) == 0 && freq >= from && v != def {
			return part
		}

		return ""
	}

	for _, why := range []string{
		mismatchDay(r, freq, start, o),
		clock("BYHOUR", r.ByHour, o.Hour(), start.Hour(), rrule.Daily),
		clock("BYMINUTE", r.ByMinute, o.Minute(), start.Minute(), rrule.Hourly),
		clock("BYSECOND", r.BySecond, o.Second(), start.Second(), rrule.Minutely),
	} {
		if why != "" {
			return why
		}
	}

	return ""
}

// mismatchDay is mismatch for the day parts, with the defaults of freq.
func mismatchDay(r rrule.Rule, freq rrule.Frequency, start, o time.Time) string {
	daysIn := func(y int, m time.Month) int { return time.Date(y, m+1, 0, 0, 0, 0, 0, time.UTC).Day() }
	signed := func(values []int, n, total int) bool {
		return slices.Contains(values, n) || slices.Contains(values, n-total-1)
	}
	weekday := rrule.Weekday((int(o.Weekday()) + 6) % 7)
	yearLen := time.Date(o.Year(), 12, 31, 0, 0, 0, 0, time.UTC).YearDay()

	switch {
	case len(r.ByMonth) > 0 && !slices.Contains(r.ByMonth, o.Month()):
		return "BYMONTH"
	case len(r.ByMonthDay) > 0 && !signed(r.ByMonthDay, o.Day(), daysIn(o.Year(), o.Month())):
		return "BYMONTHDAY"
	case len(r.ByYearDay) > 0 && !signed(r.ByYearDay, o.YearDay(), yearLen):
		return "BYYEARDAY"
	case len(r.ByDay) > 0 && !slices.ContainsFunc(r.ByDay, func(d rrule.WeekdayNum) bool { return d.Weekday == weekday }):
		return "BYDAY"
	}

	if len(r.ByWeekNo) > 0 && r.WeekStart == rrule.Monday {
		y, w := o.ISOWeek()
		if _, weeks := time.Date(y, 12, 28, 0, 0, 0, 0, time.UTC).ISOWeek(); !signed(r.ByWeekNo, w, weeks) {
			return "BYWEEKNO"
		}
	}

	if len(r.ByWeekNo) > 0 || len(r.ByYearDay) > 0 || len(r.ByMonthDay) > 0 || len(r.ByDay) > 0 {
		return ""
	}

	switch {
	case freq == rrule.Yearly && len(r.ByMonth) == 0 && o.Month() != start.Month():
		return "the start's month"
	case (freq == rrule.Yearly || freq == rrule.Monthly) && o.Day() != start.Day():
		return "the start's day"
	case freq == rrule.Weekly && o.Weekday() != start.Weekday():
		return "the start's weekday"
	}

	return ""
}
