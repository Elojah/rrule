// Payday is the last weekday of each month: a rule read from RFC 5545's text, in which BYSETPOS=-1 picks the last of
// each month's weekdays.
//
// `go run ./examples/02-parse`.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/elojah/rrule"
)

func main() {
	payday, err := rrule.Parse("RRULE:FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1;COUNT=12")
	if err != nil {
		log.Fatal(err)
	}

	for t := range payday.All(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)) {
		fmt.Println(t.Format("Mon 2006-01-02"))
	}

	// A rule part the RFC does not allow with the frequency is an error that says which.
	if _, err := rrule.Parse("FREQ=WEEKLY;BYMONTHDAY=1"); err != nil {
		fmt.Println(err)
	}
}
