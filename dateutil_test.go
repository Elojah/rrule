package rrule_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/elojah/rrule"
)

// TestDateutil checks each rule of testdata/dateutil.txt against what python-dateutil, the reference implementation,
// expands it to: random rules, in UTC, which make vectors writes with tools/vectors/gen.py.
func TestDateutil(t *testing.T) {
	t.Parallel()

	b, err := os.ReadFile("testdata/dateutil.txt")
	if err != nil {
		t.Fatal(err)
	}

	const (
		limit  = 50 // gen.py's LIMIT
		layout = "20060102T150405"
	)

	// A case is 4 lines from DTSTART: RRULE, END, and the occurrences, which may be none.
	all := strings.Split(string(b), "\n")
	cases := 0

	for i, line := range all {
		if !strings.HasPrefix(line, "DTSTART:") {
			continue
		}

		if i+3 >= len(all) {
			t.Fatalf("line %d: not a case", i+1)
		}

		lines := all[i : i+4]
		cases++

		t.Run(lines[1], func(t *testing.T) {
			t.Parallel()

			start, err := time.Parse(layout, strings.TrimPrefix(lines[0], "DTSTART:"))
			if err != nil {
				t.Fatal(err)
			}

			end, err := time.Parse(layout, strings.TrimPrefix(lines[2], "END:"))
			if err != nil {
				t.Fatal(err)
			}

			r, err := rrule.Parse(lines[1])
			if err != nil {
				t.Fatal(err)
			}

			// END is included. Between ends there, where All would go on to 9999 for a rule without more.
			got := take(r.Between(start, start, end.Add(time.Second)), limit)

			equal(t, format(got, layout), strings.Fields(lines[3]))
		})
	}

	if cases < 500 {
		t.Fatalf("%d cases, want at least 500", cases)
	}
}
