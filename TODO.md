# TODO

The library reads, writes and expands RFC 5545 recurrence rules and sets, for calendars, schedulers and devices: a
server listing a user's next events, or a controller deciding when to run. The next steps make it faster to ask for
far occurrences, and read more of the iCalendar found in the wild, without a dependency, and keep it building with
TinyGo. Every change keeps the invariants in CLAUDE.md, a semantic change comes with the RFC's or dateutil's
occurrences in its test, and a performance change with `benchstat` against `main` (`make benchcmp`).

## Short-term: small, no new invariant

- [ ] Seek in `Between` and `Set.After`: a rule without COUNT can start at the period holding `from` rather than at
  the start's, which a SECONDLY rule from years ago needs. COUNT still needs every occurrence from the start.
- [ ] Skip over months that BYMONTH leaves out in WEEKLY rules too, and over days nothing in a period matches, so
  that a rule nothing matches ends sooner than going through every day to 9999 (`BenchmarkRule_All_None`).
- [ ] `WeekdayOf(time.Weekday) Weekday` and `Weekday.Time()`, for code holding a `time.Weekday`.
- [ ] More vectors: BYWEEKNO is in 26 cases of 573. Raise its share in `gen.py`, with WKST other than Monday.

## Mid-term: more iCalendar

- [ ] A lenient mode, `WithLenient()`: a DATE EXDATE or UNTIL with a DATE-TIME DTSTART, as some producers write,
  taking away or ending at the whole day, and a trailing `;` in a rule.
- [ ] `RDATE;VALUE=PERIOD`: the period's start as the occurrence, its duration kept so that `String` writes it back.
- [ ] VTIMEZONE: a loader from an iCalendar file's own definitions, for TZIDs that are not IANA names, such as
  Outlook's "W. Europe Standard Time". A `WithLoader` function, not a change to ParseSet.
- [ ] RFC 7529's RSCALE and SKIP: SKIP=BACKWARD or FORWARD for February 30 with the Gregorian calendar first, other
  calendars after, if anyone needs them.
- [ ] Text for people, such as "every other Thursday, 6 times", in English, from a Rule.
