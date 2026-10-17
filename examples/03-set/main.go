// A stand-up meets every weekday at 09:30 UTC from December: a recurrence set, as an iCalendar event holds it, from
// which EXDATE takes Christmas Eve and Christmas away, and to which RDATE adds a Saturday before a release.
//
// `go run ./examples/03-set`.
package main

import (
	"fmt"
	"log"
	"time"

	"github.com/elojah/rrule"
)

const standup = `DTSTART:20261201T093000Z
RRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR
EXDATE:20261224T093000Z,20261225T093000Z
RDATE:20261219T093000Z`

func main() {
	set, err := rrule.ParseSet(standup)
	if err != nil {
		log.Fatal(err)
	}

	from := time.Date(2026, 12, 18, 0, 0, 0, 0, time.UTC)
	for t := range set.Between(from, from.AddDate(0, 0, 10)) {
		fmt.Println(t.Format("Mon 2006-01-02 15:04"))
	}

	if next, ok := set.After(time.Date(2026, 12, 23, 12, 0, 0, 0, time.UTC)); ok {
		fmt.Println("after the 23rd:", next.Format("Mon 2006-01-02"))
	}
}
