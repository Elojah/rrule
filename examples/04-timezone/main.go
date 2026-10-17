// A backup runs every night at 02:30 in Paris, where clocks go forward from 02:00 to 03:00 on 29 March 2026. 02:30 does
// not happen that night: RFC 5545 reads it with the offset from before, so the backup runs at 03:30, still 01:30 UTC.
// The night after, 02:30 is summer time, an hour earlier in UTC.
//
// Paris's zone is embedded, and given to ParseSet with WithLoader, as a microcontroller, which has no zoneinfo to
// load it from, needs. Paris is /usr/share/zoneinfo/Europe/Paris.
//
// `go run ./examples/04-timezone`.
package main

import (
	_ "embed"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/elojah/rrule"
)

//go:embed Paris
var paris []byte

var errUnknownZone = errors.New("unknown time zone")

func load(name string) (*time.Location, error) {
	if name != "Europe/Paris" {
		return nil, fmt.Errorf("%w %s", errUnknownZone, name)
	}

	return time.LoadLocationFromTZData(name, paris)
}

func main() {
	backup, err := rrule.ParseSet("DTSTART;TZID=Europe/Paris:20260327T023000\nRRULE:FREQ=DAILY;COUNT=4",
		rrule.WithLoader(load))
	if err != nil {
		log.Fatal(err)
	}

	for t := range backup.All() {
		fmt.Println(t.Format("Mon 2006-01-02 15:04 MST"), "is", t.UTC().Format("15:04 UTC"))
	}
}
