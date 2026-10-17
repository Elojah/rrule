// Package rrule reads, writes and expands RFC 5545 recurrence rules, those of iCalendar's RRULE, and recurrence sets:
// a start, rules, and dates added and taken away, such as a weekly meeting but on holidays.
//
//	r, err := rrule.Parse("FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-1") // the last weekday of each month
//	...
//	for t := range r.All(start) {
//		...
//	}
//
//	set, err := rrule.ParseSet("DTSTART;TZID=Europe/Paris:20260105T093000\n" +
//		"RRULE:FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR\n" +
//		"EXDATE;TZID=Europe/Paris:20261225T093000")
//	...
//	next, ok := set.After(time.Now())
//
// # Rules
//
// A Rule is the value of an RRULE: a frequency, Freq, repeating every Interval periods, until Count occurrences or
// the instant Until, on the days and times its BYxxx parts give. Rule.All expands it from a start, which RFC 5545
// leaves to DTSTART: a rule's parts that it lacks take their values from the start, so that FREQ=MONTHLY repeats on
// the start's day of the month, at its time. Parse reads a rule, and String writes it.
//
// What a BYxxx part does depends on the frequency, as RFC 5545's table in section 3.3.10 says: BYDAY expands a
// weekly rule to each day it lists, and limits a daily one to them. Rules expand as python-dateutil, RFC 5545's
// reference implementation, does: tests check thousands of occurrences against it, and every example of the RFC.
//
// # Sets
//
// A Set is a recurrence set: its Start, DTSTART, is its first occurrence, whether its rules match it or not, then
// come the occurrences of its RRules and RDates, without duplicates, less those of its ExRules and ExDates. ParseSet
// reads one from the lines of an iCalendar component, and String writes it.
//
// # Time zones
//
// Rules repeat on the wall clock of the start's location: a daily rule at 09:00 in Paris stays at 09:00 when clocks
// change, and its instants move by an hour in UTC. Daylight saving time skips some wall clocks and repeats others:
// RFC 5545 reads a skipped one with the offset from before the gap, so that 02:30 becomes 03:30 when clocks go forward
// at 02:00, and a repeated one as its first instant. Occurrences are instants, and always increase.
//
// ParseSet loads the location of a TZID with time.LoadLocation, which needs the host's zoneinfo, or the program to
// import time/tzdata. WithLoader replaces it, such as with time.LoadLocationFromTZData for a zone that a
// microcontroller embeds: the package builds with TinyGo. Times without a zone, floating ones, are read in UTC, or in
// WithLocation's location.
package rrule
