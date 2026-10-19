package rrule

import (
	"strconv"
	"strings"
	"time"
)

const (
	layoutDate     = "20060102"
	layoutDateTime = "20060102T150405"
)

// String returns the rule as RFC 5545 writes it, its parts in the grammar's order, UNTIL in UTC: Parse reads it back
// to the same rule.
func (r Rule) String() string {
	return r.format(func(t time.Time) string {
		return t.UTC().Format(layoutDateTime) + "Z"
	})
}

// MarshalText returns the rule's String, or Validate's error.
func (r Rule) MarshalText() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}

	return []byte(r.String()), nil
}

// UnmarshalText sets the rule to what Parse returns, without options.
func (r *Rule) UnmarshalText(text []byte) error {
	v, err := Parse(string(text))
	if err != nil {
		return err
	}

	*r = v

	return nil
}

// format writes the rule, UNTIL as until writes it.
func (r Rule) format(until func(time.Time) string) string {
	var b strings.Builder

	b.WriteString("FREQ=")
	b.WriteString(r.Freq.String())

	if r.XFreq != 0 {
		b.WriteString(";X-FREQ=")
		b.WriteString(formatXFreq(r.XFreq))
	}

	if !r.Until.IsZero() {
		b.WriteString(";UNTIL=")
		b.WriteString(until(r.Until))
	}

	writeInt(&b, "COUNT", r.Count)
	writeInt(&b, "INTERVAL", r.Interval)
	writeInts(&b, "BYSECOND", r.BySecond)
	writeInts(&b, "BYMINUTE", r.ByMinute)
	writeInts(&b, "BYHOUR", r.ByHour)

	if len(r.ByDay) > 0 {
		b.WriteString(";BYDAY=")

		for i, d := range r.ByDay {
			if i > 0 {
				b.WriteByte(',')
			}

			b.WriteString(d.String())
		}
	}

	writeInts(&b, "BYMONTHDAY", r.ByMonthDay)
	writeInts(&b, "BYYEARDAY", r.ByYearDay)
	writeInts(&b, "BYWEEKNO", r.ByWeekNo)

	if len(r.ByMonth) > 0 {
		months := make([]int, len(r.ByMonth))
		for i, m := range r.ByMonth {
			months[i] = int(m)
		}

		writeInts(&b, "BYMONTH", months)
	}

	writeInts(&b, "BYSETPOS", r.BySetPos)

	if r.WeekStart != Monday {
		b.WriteString(";WKST=")
		b.WriteString(r.WeekStart.String())
	}

	return b.String()
}

// String returns the set as iCalendar lines, joined by LF, not folded: DTSTART, then RRULE, EXRULE, RDATE and EXDATE,
// in its Form. ParseSet reads them back to the same occurrences. A Zoned set's TZID is the name of Start's location,
// so that a location from time.LoadLocation round trips, but neither time.Local, named Local, nor a time.FixedZone.
func (s Set) String() string {
	f := s.formatter()
	lines := []string{"DTSTART" + f.params + ":" + f.value(s.Start)}

	for _, r := range s.RRules {
		lines = append(lines, "RRULE:"+r.format(f.until))
	}

	for _, r := range s.ExRules {
		lines = append(lines, "EXRULE:"+r.format(f.until))
	}

	for _, d := range []struct {
		name  string
		times []time.Time
	}{{"RDATE", s.RDates}, {"EXDATE", s.ExDates}} {
		if len(d.times) == 0 {
			continue
		}

		values := make([]string, len(d.times))
		for i, t := range d.times {
			values[i] = f.value(t)
		}

		lines = append(lines, d.name+f.params+":"+strings.Join(values, ","))
	}

	return strings.Join(lines, "\n")
}

// MarshalText returns the set's String, or Validate's error.
func (s Set) MarshalText() ([]byte, error) {
	if err := s.Validate(); err != nil {
		return nil, err
	}

	return []byte(s.String()), nil
}

// UnmarshalText sets the set to what ParseSet returns, without options.
func (s *Set) UnmarshalText(text []byte) error {
	v, err := ParseSet(string(text))
	if err != nil {
		return err
	}

	*s = v

	return nil
}

// setFormatter writes a set's times in its Form.
type setFormatter struct {
	params string // of DTSTART, RDATE and EXDATE
	value  func(time.Time) string
	until  func(time.Time) string
}

func (s Set) formatter() setFormatter {
	loc := s.Start.Location()
	in := func(layout, suffix string) func(time.Time) string {
		return func(t time.Time) string {
			return t.In(loc).Format(layout) + suffix
		}
	}
	utc := func(t time.Time) string {
		return t.UTC().Format(layoutDateTime) + "Z"
	}

	switch s.Form {
	case Floating:
		return setFormatter{value: in(layoutDateTime, ""), until: in(layoutDateTime, "")}
	case Date:
		return setFormatter{params: ";VALUE=DATE", value: in(layoutDate, ""), until: in(layoutDate, "")}
	case Zoned:
	}

	if loc == time.UTC {
		return setFormatter{value: utc, until: utc}
	}

	name := loc.String()
	if strings.ContainsAny(name, ";:,") {
		name = strconv.Quote(name)
	}

	return setFormatter{params: ";TZID=" + name, value: in(layoutDateTime, ""), until: utc}
}

// formatXFreq writes a positive duration as ISO 8601 does, in hours, minutes and seconds, such as PT1H30M or PT0.5S.
func formatXFreq(d time.Duration) string {
	if d < 0 {
		return d.String() // which Validate refuses
	}

	h, m, ns := d/time.Hour, d%time.Hour/time.Minute, d%time.Minute
	s := "PT"

	if h > 0 {
		s += strconv.FormatInt(int64(h), 10) + "H"
	}

	if m > 0 {
		s += strconv.FormatInt(int64(m), 10) + "M"
	}

	if ns > 0 {
		s += strconv.FormatInt(int64(ns/time.Second), 10)

		if frac := ns % time.Second; frac > 0 {
			s += "." + strings.TrimRight(strconv.FormatInt(int64(frac+time.Second), 10)[1:], "0")
		}

		s += "S"
	}

	return s
}

func writeInt(b *strings.Builder, name string, v int) {
	if v == 0 {
		return
	}

	b.WriteString(";" + name + "=")
	b.WriteString(strconv.Itoa(v))
}

func writeInts(b *strings.Builder, name string, values []int) {
	if len(values) == 0 {
		return
	}

	b.WriteString(";" + name + "=")

	for i, v := range values {
		if i > 0 {
			b.WriteByte(',')
		}

		b.WriteString(strconv.Itoa(v))
	}
}
