package rrule_test

import (
	"testing"
	"time"

	"github.com/elojah/rrule"
)

// TestCivil checks the day arithmetic against time's, for each day of 1600 to 2400 and around year 0.
func TestCivil(t *testing.T) {
	t.Parallel()

	for _, span := range [][2]int{{1600, 2400}, {-2, 2}, {9998, 9999}} {
		for d := time.Date(span[0], 1, 1, 0, 0, 0, 0, time.UTC); d.Year() <= span[1]; d = d.AddDate(0, 0, 1) {
			day := rrule.DaysFromCivil(d.Date())
			if want := d.Unix() / 86400; day != want {
				t.Fatalf("%s: day %d, want %d", d.Format(time.DateOnly), day, want)
			}

			if y, m, dd := rrule.CivilFromDays(day); y != d.Year() || m != d.Month() || dd != d.Day() {
				t.Fatalf("day %d: %d-%d-%d, want %s", day, y, m, dd, d.Format(time.DateOnly))
			}

			if wd := rrule.WeekdayOfDay(day); (int(wd)+1)%7 != int(d.Weekday()) {
				t.Fatalf("%s: %s, want %s", d.Format(time.DateOnly), wd, d.Weekday())
			}
		}
	}
}

// TestWeekNo checks week numbers with weeks from Monday against ISO 8601's, which time.ISOWeek returns, and with
// weeks from every day against the definition: week 1 is the first with at least 4 days in the year.
func TestWeekNo(t *testing.T) {
	t.Parallel()

	for d := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() <= 2100; d = d.AddDate(0, 0, 1) {
		day := rrule.DaysFromCivil(d.Date())

		y, w := d.ISOWeek()
		_, weeks := time.Date(y, 12, 28, 0, 0, 0, 0, time.UTC).ISOWeek() // December 28 is in the last week

		if n, total := rrule.WeekNo(day, rrule.Monday); n != w || total != weeks {
			t.Fatalf("%s: week %d of %d, want %d of %d", d.Format(time.DateOnly), n, total, w, weeks)
		}

		for wkst := rrule.Monday; wkst <= rrule.Sunday; wkst++ {
			n, total := rrule.WeekNo(day, wkst)
			if wantN, wantTotal := weekByDefinition(d, wkst); n != wantN || total != wantTotal {
				t.Fatalf("%s, WKST=%s: week %d of %d, want %d of %d", d.Format(time.DateOnly), wkst, n, total, wantN,
					wantTotal)
			}
		}
	}
}

// weekByDefinition counts weeks starting on wkst from the first with at least 4 days in its year.
func weekByDefinition(d time.Time, wkst rrule.Weekday) (int, int) {
	weekStart := func(d time.Time) time.Time {
		return d.AddDate(0, 0, -((int(d.Weekday()) + 6 - int(wkst) + 7) % 7))
	}
	weekOne := func(y int) time.Time {
		w := weekStart(time.Date(y, 1, 1, 0, 0, 0, 0, time.UTC))
		if w.Year() < y && w.AddDate(0, 0, 6).Day() < 4 { // fewer than 4 days in y
			w = w.AddDate(0, 0, 7)
		}

		return w
	}

	start := weekStart(d)
	y := start.AddDate(0, 0, 3).Year()
	first := weekOne(y)

	return int(start.Sub(first).Hours()/24/7) + 1, int(weekOne(y+1).Sub(first).Hours() / 24 / 7)
}
