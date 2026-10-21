# CLAUDE.md

Commands, fragile invariants and settled decisions. Behaviour is in godoc and the README, plans in TODO.md.

## Overview

`github.com/elojah/rrule`, RFC 5545 recurrence rules and sets. Go 1.26.1, no dependency, one package.

Files: `rule.go` (`Rule`, `Frequency`, `Weekday`, `WeekdayNum`, `Validate`), `parse.go` (`Parse`, `ParseSet`,
options, content lines, X-FREQ's durations), `format.go` (`String`s, text marshalers), `set.go` (`Set`, `Form`, the
merge), `expand.go` (the expansion of a rule), `calendar.go` (civil days, week numbers, `wall`), `errors.go`.
`examples/` are programs.

`testdata/dateutil.txt` is what python-dateutil expands random rules to: `make vectors` writes it with
`tools/vectors/gen.py`, never edit it. The same on any machine: `gen.py` lowers dateutil's `datetime.MAXYEAR` to the
year after a case's END, and a case over its timeout stops it rather than being left out. `testdata/zoneinfo` holds
the TZif files tests load their zones from.

## Commands

```sh
go test -race ./...                 # always -race
go test -race -run TestName ./...
make check                          # golangci-lint + tests (-shuffle=on -cpu 1,4) + each benchmark once
make fuzz                           # FuzzParse, FuzzParseSet, FuzzAll for FUZZ_TIME (30s)
make vectors                        # testdata/dateutil.txt, with pinned dateutil and six wheels, checked by sha256
make bench                          # BENCH=regexp
make benchcmp                       # main (BENCH_BASE) vs the working tree, with benchstat
make tinygo                         # tinygo-test, tinygo-run (examples, output checked), tinygo-build (Pico)
```

Lint (`.golangci.yml`) is `default: all` minus a disable list, tests included. Mind `nlreturn`, `paralleltest`
(`t.Parallel()` first), `err113`, `godot`, `funcorder` (exported methods first), `exhaustive` (a switch on an enum
lists every constant, even with a default), `gocyclo` (split a function above 30), `maintidx` (a long table goes in a
package-level var), and `gosec`'s G115 on integer conversions. CI runs `make check` with a pinned golangci-lint in
`./bin`.

## Conventions

- Comments are short: invariants, non-obvious whys, the public contract. What RFC 5545 leaves to interpretation gets a
  fuller explanation where it is handled, with the section.
- Errors are a `*PartError` (part, value, reason) wrapping one of `ErrSyntax`, `ErrInvalid`, `ErrUnsupported`, or the
  loader's error, so `errors.Is` works; ParseSet wraps it in a `*LineError`. Messages start with `rrule: `.
- `ErrSyntax` is what the grammar refuses, `ErrInvalid` what it allows but the RFC's MUSTs or a range refuse,
  `ErrUnsupported` valid iCalendar not handled.
- Options have no prefix: `WithLocation`, `WithLoader`.
- `Weekday` counts from Monday, so that the zero `WeekStart` is the RFC's default.

## Invariants

- **Expansion works on civil time** (days from 1970-01-01, seconds of the day, as a key `day*86400+sec`), and only an
  occurrence becomes an instant, through `wall`. Never compute on `time.Time` in a location in between. Under X-FREQ a
  period is a number of X-FREQs from the start's civil time (`civilAt`, `periodAt`, in 128 bits), and `nsec` is each
  period's; otherwise every candidate has the start's nanoseconds.
- **`wall` implements RFC 5545 3.3.5**: a repeated wall clock is its first instant, a skipped one keeps the offset from
  before the gap. `time.Date` picks either, depending on the zone (`TestAll_DST`, Paris against New York). Its
  `zoneCache` fast path trusts an offset only more than a day from its zone's bounds.
- **A rule's instants strictly increase**: `next` drops any not after the last (`prev`). A gap moves wall clocks onto or
  before later ones under HOURLY, MINUTELY and X-FREQ. COUNT and UNTIL apply after; `merger` relies on sorted sources.
- **Expansion always ends**: `periods` bounds the last period by 9999-12-31 or the day after UNTIL; `reachable` ends a
  rule whose interval never lands on a BYHOUR, BYMINUTE or BYSECOND (a gcd check); periods shorter than a day jump
  over a filtered day, hour, minute or second (`fillTime`, `nextBit`). `nextDay` jumps over the months, days of the
  month and weekdays that BYMONTH, BYMONTHDAY and BYDAY leave out (`monthMask`, which keeps every weekday a numbered
  BYDAY has), to the day after `lastDay` at most: DAILY and shorter on a filtered day, and WEEKLY, MONTHLY and YEARLY
  on a period without days (`skip`). A SECONDLY or X-FREQ period has one candidate, so a BYSETPOS without 1 or -1 ends
  the rule at once. `TestAll_Ends` holds the cases.
- **`jump` aligns to the interval**: it lands on the first period from its target that `first + k*step` reaches, so
  jumping never skips a period that could match. `step` is capped at `maxStep` so that its arithmetic cannot
  overflow, and so are X-FREQ's periods: one under 69ns ends 2^62 periods after the start, not in 9999.
- **BYSETPOS indexes the period's candidates without building them**: days by times, by `i/len(times)` and
  `i%len(times)`. A day can have 86400 times.
- **No `iter.Pull`**: sources are pull iterators (`source.next`), merged directly, so that TinyGo builds it and nothing
  needs a goroutine. `iter.Seq` only wraps them at the API.
- **Inputs are never changed**: `Set.All` sorts copies of RDates and ExDates.
- **`Between` bounds the expansion, not what it yields**: `newExpansion`'s `end` caps the last period, and
  `merger.end` the merge, so that a rule nothing matches, or a set whose exclusions take everything away, ends at
  `to`. Tests that only look at a window use `Between` (`TestDateutil`, `FuzzAll`): `All` going on to 9999 takes
  minutes under `-race -cover`, whose atomic counters parallel tests contend on.
- **`Between` and `Set.After` seek**, unless a rule has COUNT: `newExpansion`'s `from` jumps to the period holding
  `seekKey`, from's wall clock, or, less than a gap after clocks go forward, its wall clock with the offset from before
  the gap, which `wall` reads the gap's wall clocks with: they are at or after from, although before its wall clock.
  Skipped periods hold only instants before from, so what `prev` drops at or after it is the same.
  `TestRule_Between_DST` seeks every 10 minutes around changes; `TestDateutil` and `FuzzAll` seek too.

## Decisions not to revert

- **python-dateutil is the reference implementation.** Filters intersect, and the parts a rule lacks come from the
  start (YEARLY: BYMONTH and BYMONTHDAY; MONTHLY: BYMONTHDAY; WEEKLY: BYDAY; DAILY and longer: BYHOUR; HOURLY and
  longer: BYMINUTE; MINUTELY and longer: BYSECOND), which is the RFC's expand/limit table. BYSETPOS applies to a
  period's candidates before the start, UNTIL and COUNT. Where dateutil departs from the RFC, the library follows the
  RFC, and `gen.py` keeps such rules out of the vectors: a BYDAY with numbered and plain days (dateutil requires both),
  BYSECOND=60, BYWEEKNO=52, 53, -52 or -53 (dateutil leaves out the end of a year in the next one's week -52 or -53,
  and miscounts the weeks of the year before at its start, for any WKST).
- **DST gaps move the wall clock, they do not drop it.** RFC 5545 3.3.10 says both: that a recurrence's nonexistent
  local time is read as 3.3.5 reads a DATE-TIME, and that such an instance is ignored. Calendars move it; so does this.
  Invalid dates (February 30) are dropped and not counted.
- **A Set's Start is always an occurrence** ("DTSTART always counts as the first occurrence"), unless an EXDATE or
  EXRULE takes it away. `Rule.All(start)` yields only the rule's matches, as dateutil.
- **Sets take several RRULEs and EXRULEs**, which RFC 5545 calls undefined and dropped: their union, as dateutil.
- **Strict parsing**: unknown properties, parameters and rule parts are errors (X-FREQ is the one `X-` part read);
  value types must match DTSTART's (a DATE UNTIL, RDATE or EXDATE with a DATE-TIME DTSTART, and the reverse, are
  `ErrInvalid`). A floating UNTIL, RDATE or EXDATE is read in Start's location. A lenient mode is in TODO.md.
- **X-FREQ**, this package's rule part, is an ISO 8601 duration of hours, minutes and seconds, to the nanosecond:
  days and longer are `ErrUnsupported`, `R/…` is `ErrSyntax`. It replaces FREQ and INTERVAL, which any valid values
  keep for other readers. It expands as SECONDLY: every BYxxx part limits, none takes the start's value, BYSETPOS sees
  one candidate, but a numbered BYDAY counts as FREQ says (`nthYear`). It steps on the wall clock, so that
  `X-FREQ=PT1H` is `FREQ=HOURLY` (`TestAll_XFreq`). `gen.py` has none: dateutil does not know it.
- **Floating times default to UTC**, not `time.Local`, so that results do not depend on the host.
- **The library does not keep a rule's text**: `String` writes the grammar's order, X-FREQ after FREQ, UNTIL in UTC,
  INTERVAL whenever set, WKST unless Monday. `Parse(r.String())` gives back `r` (`FuzzParse`).
- **Leap seconds**: BYSECOND=60 parses and never matches; a DATE-TIME at second 60 is `ErrUnsupported`.

## Tests

- External test packages only (`_test`); `export_test.go` exports the calendar functions.
- Zones come from `testdata/zoneinfo` through `loadZone`, never the host's: the TinyGo container may have none.
- `TestRFC` runs every example of RFC 5545 3.8.5.3 through ParseSet. The RFC lists 15:00 for "every 3 hours until
  17:00Z", an erratum: 15:00 EDT is 19:00Z.
- `TestDateutil` compares each vector's first 50 occurrences, up to its END, and again from the middle one. Change the
  generator or its seed only with `make vectors`, and check the diff makes sense: new cases go after the others, which
  keep their random numbers.
- `FuzzAll` checks occurrences against an oracle of its own, `mismatch`, not the expansion's code, and against its own
  from just after the middle one.
- Each example has an `Example()` with `// Output:`, and `tinygo-run` diffs `tinygo run`'s output with it.
