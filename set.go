package rrule

import (
	"iter"
	"slices"
	"strconv"
	"time"
)

// Form is how a Set writes its times.
type Form int

// Forms.
const (
	// Zoned times are in Start's location, with a TZID, or in UTC, with a Z.
	Zoned Form = iota

	// Floating times have no zone: the same wall clock wherever they are read. A Set reads them in Start's location.
	Floating

	// Date times are days, VALUE=DATE, such as birthdays: midnight in Start's location.
	Date
)

// Set is a recurrence set, RFC 5545's: a start, DTSTART, and the rules and dates that add occurrences to it or take
// them away. Its occurrences are Start, those of RRules and RDates, without duplicates, less those of ExRules and
// ExDates, which take precedence.
//
// Rules repeat from Start, and in its location, which every occurrence is returned in. RFC 5545 calls a set with
// several RRules undefined, and has no ExRules any more, RFC 2445 had: Set takes both, as python-dateutil does.
type Set struct {
	Start   time.Time
	Form    Form
	RRules  []Rule
	ExRules []Rule
	RDates  []time.Time
	ExDates []time.Time
}

// Validate returns a *PartError for a set without a Start, with a rule that Rule.Validate refuses, or, in the Date
// form, with times that are not midnight in Start's location, or a rule with BYHOUR, BYMINUTE or BYSECOND.
func (s Set) Validate() error {
	if s.Start.IsZero() {
		return partError(ErrInvalid, "DTSTART", "", "missing")
	}

	if s.Form < Zoned || s.Form > Date {
		return partError(ErrInvalid, "Form", strconv.Itoa(int(s.Form)), "")
	}

	for _, rules := range [][]Rule{s.RRules, s.ExRules} {
		for _, r := range rules {
			if err := r.Validate(); err != nil {
				return err
			}

			if s.Form == Date && (len(r.ByHour) > 0 || len(r.ByMinute) > 0 || len(r.BySecond) > 0) {
				return partError(ErrInvalid, "RRULE", r.String(), "BYHOUR, BYMINUTE or BYSECOND with a DATE DTSTART")
			}
		}
	}

	if s.Form != Date {
		return nil
	}

	loc := s.Start.Location()

	for _, t := range append([]time.Time{s.Start}, append(s.RDates, s.ExDates...)...) {
		if h, m, sec := t.In(loc).Clock(); h != 0 || m != 0 || sec != 0 || t.Nanosecond() != 0 {
			return partError(ErrInvalid, "DATE", t.String(), "not midnight in DTSTART's location")
		}
	}

	return nil
}

// All returns the set's occurrences, in order, in Start's location. It yields nothing for a set that Validate
// refuses.
//
// The next occurrence may be far: All goes through the occurrences that ExRules and ExDates take away to find it, and
// through every period to 9999 of an RRule whose BYxxx parts nothing matches. Between ends instead.
func (s Set) All() iter.Seq[time.Time] {
	return s.merged(time.Time{}, time.Time{})
}

// Between returns the set's occurrences at or after from and before to. Unlike All's, its iteration ends at to,
// however few occurrences RRules have, and however many ExRules and ExDates take away. Rules expand from the period
// holding from, as Rule.Between's do.
func (s Set) Between(from, to time.Time) iter.Seq[time.Time] {
	return func(yield func(time.Time) bool) {
		for t := range s.merged(from, to) {
			if !t.Before(from) && !yield(t) {
				return
			}
		}
	}
}

// After returns the set's first occurrence after t, excluded, or false if there is none. Rules expand from the period
// holding t, as in Between, and as far as in All.
func (s Set) After(t time.Time) (time.Time, bool) {
	for o := range s.merged(t, time.Time{}) {
		if o.After(t) {
			return o, true
		}
	}

	return time.Time{}, false
}

// merged returns the set's occurrences before end, or all of them if end is zero. It may leave out those before from,
// if from is not zero.
func (s Set) merged(from, end time.Time) iter.Seq[time.Time] {
	return func(yield func(time.Time) bool) {
		m := s.merge(from, end)
		if m == nil {
			return
		}

		for {
			t, ok := m.next()
			if !ok || !yield(t) {
				return
			}
		}
	}
}

// source yields instants in increasing order.
type source interface {
	next() (time.Time, bool)
}

// times is a source of sorted instants.
type times []time.Time

func (t *times) next() (time.Time, bool) {
	if len(*t) == 0 {
		return time.Time{}, false
	}

	v := (*t)[0]
	*t = (*t)[1:]

	return v, true
}

// merger merges its sources, in, without duplicates and without the instants of out, until end if not zero.
type merger struct {
	loc     *time.Location
	end     time.Time
	in      []head
	out     []head
	prev    time.Time
	yielded bool
}

type head struct {
	src source
	t   time.Time
	ok  bool
}

func (h *head) advance() {
	h.t, h.ok = h.src.next()
}

// merge returns the merger of the set's sources, which end at end if it is not zero, and may start at from if it is
// not zero.
func (s Set) merge(from, end time.Time) *merger {
	if s.Validate() != nil {
		return nil
	}

	m := &merger{loc: s.Start.Location(), end: end}

	add := func(heads []head, src source) []head {
		h := head{src: src}
		h.advance()

		return append(heads, h)
	}

	m.in = add(m.in, sorted(append([]time.Time{s.Start}, s.RDates...), from))
	for _, r := range s.RRules {
		m.in = add(m.in, newExpansion(r, s.Start, from, end))
	}

	m.out = add(m.out, sorted(s.ExDates, from))
	for _, r := range s.ExRules {
		m.out = add(m.out, newExpansion(r, s.Start, from, end))
	}

	return m
}

func (m *merger) next() (time.Time, bool) {
	for {
		var best *head

		for i := range m.in {
			if h := &m.in[i]; h.ok && (best == nil || h.t.Before(best.t)) {
				best = h
			}
		}

		if best == nil {
			return time.Time{}, false
		}

		t := best.t
		if !m.end.IsZero() && !t.Before(m.end) {
			return time.Time{}, false
		}

		best.advance()

		if m.yielded && !t.After(m.prev) {
			continue
		}

		m.prev, m.yielded = t, true

		if !m.excluded(t) {
			return t.In(m.loc), true
		}
	}
}

// excluded reports whether t is an instant of out. Since the instants asked for increase, so do out's heads.
func (m *merger) excluded(t time.Time) bool {
	for i := range m.out {
		h := &m.out[i]
		for h.ok && h.t.Before(t) {
			h.advance()
		}

		if h.ok && h.t.Equal(t) {
			return true
		}
	}

	return false
}

// sorted returns a sorted copy of ts, which it leaves as they are, without those before from if it is not zero.
func sorted(ts []time.Time, from time.Time) *times {
	c := times(slices.Clone(ts))
	slices.SortFunc(c, time.Time.Compare)

	if !from.IsZero() {
		i, _ := slices.BinarySearchFunc(c, from, time.Time.Compare)
		c = c[i:]
	}

	return &c
}
