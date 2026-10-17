package rrule_test

import (
	"errors"
	"iter"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/elojah/rrule"
)

var errUnknownZone = errors.New("unknown time zone")

var (
	zonesOnce sync.Once
	zones     map[string]*time.Location
	errZones  error
)

// zone returns a location from testdata/zoneinfo, which tests use rather than the host's, which may lack them, as
// TinyGo's container may.
func zone(tb testing.TB, name string) *time.Location {
	tb.Helper()

	loc, err := loadZone(name)
	if err != nil {
		tb.Fatal(err)
	}

	return loc
}

// loadZone is ParseSet's loader in tests.
func loadZone(name string) (*time.Location, error) {
	zonesOnce.Do(func() {
		zones = map[string]*time.Location{}

		for _, n := range []string{"America/New_York", "Europe/Paris", "Australia/Lord_Howe"} {
			b, err := os.ReadFile(filepath.Join("testdata", "zoneinfo", filepath.FromSlash(n)))
			if err != nil {
				errZones = err

				return
			}

			if zones[n], err = time.LoadLocationFromTZData(n, b); err != nil {
				errZones = err

				return
			}
		}
	})

	if errZones != nil {
		return nil, errZones
	}

	if loc, ok := zones[name]; ok {
		return loc, nil
	}

	return nil, errUnknownZone
}

// parseSet is rrule.ParseSet with lines joined, and zones from testdata.
func parseSet(tb testing.TB, lines ...string) rrule.Set {
	tb.Helper()

	s, err := rrule.ParseSet(strings.Join(lines, "\n"), rrule.WithLoader(loadZone))
	if err != nil {
		tb.Fatal(err)
	}

	return s
}

// take returns the first n instants of seq, or all if n is negative.
func take(seq iter.Seq[time.Time], n int) []time.Time {
	var ts []time.Time

	for t := range seq {
		if n >= 0 && len(ts) == n {
			break
		}

		ts = append(ts, t)
	}

	return ts
}

// format returns times in layout.
func format(ts []time.Time, layout string) []string {
	s := make([]string, len(ts))
	for i, t := range ts {
		s[i] = t.Format(layout)
	}

	return s
}

// equal checks that got and want are the same strings, and says where they part.
func equal(t *testing.T, got, want []string) {
	t.Helper()

	for i := range max(len(got), len(want)) {
		switch {
		case i >= len(got):
			t.Fatalf("got %d, want %d: missing %s", len(got), len(want), want[i])
		case i >= len(want):
			t.Fatalf("got %d, want %d: extra %s", len(got), len(want), got[i])
		case got[i] != want[i]:
			t.Fatalf("at %d: got %s, want %s", i, got[i], want[i])
		}
	}
}

// days expands specs such as "1997-09 2-11,15 20-30/2" to those days of the month, at clock, as "2006-01-02 15:04".
func days(t *testing.T, clock string, specs ...string) []string {
	t.Helper()

	var out []string

	for _, spec := range specs {
		fields := strings.Fields(spec)
		for _, f := range fields[1:] {
			for r := range strings.SplitSeq(f, ",") {
				step := 1

				if a, b, ok := strings.Cut(r, "/"); ok {
					r, step = a, atoi(t, b)
				}

				from, to := r, r
				if a, b, ok := strings.Cut(r, "-"); ok {
					from, to = a, b
				}

				for d := atoi(t, from); d <= atoi(t, to); d += step {
					out = append(out, fields[0]+"-"+pad(d)+" "+clock)
				}
			}
		}
	}

	return out
}

func atoi(t *testing.T, s string) int {
	t.Helper()

	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}

	return n
}

func pad(n int) string {
	if n < 10 {
		return "0" + strconv.Itoa(n)
	}

	return strconv.Itoa(n)
}
