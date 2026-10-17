// A team holds its retrospective every other Thursday at 16:00, six times: a Rule built in Go, which String writes as
// RFC 5545 does, and All expands from the first.
//
// `go run ./examples/01-rule`.
package main

import (
	"fmt"
	"time"

	"github.com/elojah/rrule"
)

func main() {
	retro := rrule.Rule{
		Freq:     rrule.Weekly,
		Interval: 2,
		Count:    6,
		ByDay:    []rrule.WeekdayNum{{Weekday: rrule.Thursday}},
	}
	fmt.Println(retro)

	for t := range retro.All(time.Date(2026, 1, 8, 16, 0, 0, 0, time.UTC)) {
		fmt.Println(t.Format("Mon 2006-01-02 15:04"))
	}
}
