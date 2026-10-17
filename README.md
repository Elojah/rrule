# rrule

[![CI](https://github.com/Elojah/rrule/actions/workflows/ci.yml/badge.svg)](https://github.com/Elojah/rrule/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/elojah/rrule.svg)](https://pkg.go.dev/github.com/elojah/rrule)

Recurrence rules from Go, RFC 5545's: read, write and expand iCalendar's RRULE, and whole recurrence sets.

- **Rules**: every frequency and BYxxx part, BYSETPOS, COUNT and UNTIL, WKST, expanded as python-dateutil, the
  reference implementation, does: tests check over 10,000 occurrences of random rules against it, and every example
  of the RFC.
- **Sets**: DTSTART, RRULE, RDATE, EXDATE and EXRULE, read from and written to iCalendar lines, with TZID, dates and
  floating times.
- **Time zones**: rules repeat on the wall clock, and daylight saving time's skipped and repeated wall clocks follow
  RFC 5545.
- **Iterators**: `iter.Seq[time.Time]`, which a `range` loop ends, for rules that repeat forever.
- **TinyGo**: builds for a microcontroller, with its time zones embedded.
- No dependency: the standard library only.

```sh
go get github.com/elojah/rrule
```

## Quick start

```go
r, err := rrule.Parse("FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1") // the last weekday of each month
if err != nil {
	return err
}
for t := range r.All(time.Date(2026, 1, 1, 18, 0, 0, 0, time.UTC)) {
	if t.Year() > 2026 {
		break
	}
	fmt.Println(t.Format("Mon 2006-01-02")) // Fri 2026-01-30, Fri 2026-02-27, Tue 2026-03-31…
}
```

A set, as an iCalendar event holds it:

```go
set, err := rrule.ParseSet(`DTSTART;TZID=Europe/Paris:20261201T093000
RRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR
EXDATE;TZID=Europe/Paris:20261224T093000,20261225T093000
RDATE;TZID=Europe/Paris:20261219T093000`)
if err != nil {
	return err
}
next, ok := set.After(time.Now())
for t := range set.Between(from, to) { // to excluded
	...
}
```

Or built in Go, and written as RFC 5545 does:

```go
retro := rrule.Rule{
	Freq:     rrule.Weekly,
	Interval: 2,
	ByDay:    []rrule.WeekdayNum{{Weekday: rrule.Thursday}},
}
fmt.Println(retro) // FREQ=WEEKLY;INTERVAL=2;BYDAY=TH
```

| Type | Holds |
| --- | --- |
| [`Rule`](https://pkg.go.dev/github.com/elojah/rrule#Rule) | An RRULE: `Parse`, `String`, `Validate`, `All(start)`, `Between(start, from, to)`. |
| [`Set`](https://pkg.go.dev/github.com/elojah/rrule#Set) | A start and the rules and dates added and taken away: `ParseSet`, `String`, `All`, `Between`, `After`. |
| [`Option`](https://pkg.go.dev/github.com/elojah/rrule#Option) | `WithLocation` for floating times, `WithLoader` for TZIDs. |
| [`PartError`](https://pkg.go.dev/github.com/elojah/rrule#PartError) | What was refused, matching `ErrSyntax`, `ErrInvalid` or `ErrUnsupported`; `LineError` says on which line. |

Both types are text marshalers, so they go into JSON or a database column as their RFC 5545 text.

## Examples

Each is a program, `go run ./examples/01-rule`, whose test checks what it prints.

| Example | Shows |
| --- | --- |
| [`01-rule`](examples/01-rule) | A Rule built in Go, written as text, expanded. |
| [`02-parse`](examples/02-parse) | BYSETPOS for the last weekday of each month, and an error. |
| [`03-set`](examples/03-set) | A set with EXDATE and RDATE, `Between` and `After`. |
| [`04-timezone`](examples/04-timezone) | A zone embedded with `go:embed` through `WithLoader`, and a wall clock that daylight saving time skips. |

## Behaviour worth knowing

- **DTSTART is a set's first occurrence**, as RFC 5545 says, even if its rules do not match it: the RFC's "Friday the
  13th" example excludes it with an EXDATE. `Rule.All(start)` yields only what the rule matches, as dateutil does.
- **A rule repeats on the start's wall clock.** A daily 09:00 stays at 09:00 across daylight saving time. A wall clock
  that does not exist moves forward by the gap (02:30 becomes 03:30), one that happens twice is its first instant, and
  each instant comes once: an hourly rule does not repeat the hour clocks go back over.
- **Invalid dates are skipped**, and not counted: `BYMONTHDAY=30` has no February.
- **Expansion ends**, in 9999 at the latest, even for a rule nothing matches. `Between` ends at its end, however few
  occurrences rules have and however many EXDATEs and EXRULEs take away; `All` and `Set.After` may go far to find the
  next.
- **Parsing is strict**: an unknown rule part, property or parameter is an error, and so is a rule part the RFC does
  not allow with its frequency. RFC 7529's RSCALE and SKIP, and RDATE periods, are `ErrUnsupported`.
- **Floating times are read in UTC** by default, so that results do not depend on the host: `WithLocation` changes it.

## Development

```sh
make check    # golangci-lint, and the tests with -race, shuffled, with GOMAXPROCS 1 and 4
make fuzz     # each fuzz target for 30s
make tinygo   # the tests, the examples and a Pico build with TinyGo
make vectors  # testdata/dateutil.txt from python-dateutil, pinned
```

## License

MIT.
