package rrule

import (
	"math"
	"slices"
	"strings"
	"time"
)

// Option configures Parse and ParseSet.
type Option func(*options)

type options struct {
	location *time.Location
	loader   func(name string) (*time.Location, error)
}

// WithLocation sets the location floating times are read in, those without a zone, such as DTSTART:20260105T090000,
// and dates. The default is UTC, so that what Parse returns does not depend on the host.
func WithLocation(loc *time.Location) Option {
	return func(o *options) {
		o.location = loc
	}
}

// WithLoader sets how ParseSet finds the location of a TZID. The default is time.LoadLocation, which reads the
// host's zoneinfo, or Go's embedded copy if the program imports time/tzdata: a microcontroller has neither, but
// time.LoadLocationFromTZData reads a zone embedded with go:embed.
func WithLoader(load func(name string) (*time.Location, error)) Option {
	return func(o *options) {
		o.loader = load
	}
}

func newOptions(opts []Option) options {
	o := options{location: time.UTC, loader: time.LoadLocation}
	for _, opt := range opts {
		opt(&o)
	}

	return o
}

// Parse reads a recurrence rule as RFC 5545 writes it, such as FREQ=MONTHLY;BYDAY=-1FR, with or without the RRULE:
// before it, in any case. It returns a *PartError for a rule that does not follow the grammar, or that Validate
// refuses.
//
// A floating UNTIL, without Z, and a date are read in WithLocation's location. RFC 5545 wants UNTIL in UTC when the
// start has a zone: ParseSet checks it against the start.
func Parse(s string, opts ...Option) (Rule, error) {
	o := newOptions(opts)

	if len(s) >= 6 && strings.EqualFold(s[:6], "RRULE:") {
		s = s[6:]
	}

	r, _, err := parseRule(s, o.location)
	if err != nil {
		return Rule{}, err
	}

	return r, nil
}

// valueKind is what a DATE or DATE-TIME value was written as.
type valueKind int

const (
	kindNone valueKind = iota
	kindDate
	kindFloating
	kindUTC
	kindZoned
)

// ParseSet reads a recurrence set: the DTSTART, RRULE, EXRULE, RDATE and EXDATE lines of an iCalendar component,
// in any order, separated by CRLF or LF, folded or not. DTSTART is required, once; the others may repeat.
//
// DTSTART sets the Set's Form and the location of Start: its TZID, UTC with a Z, or WithLocation's for a floating
// time or a date. RDATE and EXDATE may have a TZID of their own. Floating values are read in Start's location, and
// their type, date or date and time, must match DTSTART's, as must UNTIL's.
//
// ParseSet returns a *LineError wrapping a *PartError for a line it refuses, such as a property other than those, or a
// parameter other than TZID and VALUE. Without a line to blame, for a missing DTSTART, it returns the *PartError.
func ParseSet(s string, opts ...Option) (Set, error) {
	o := newOptions(opts)

	lines, err := unfold(s)
	if err != nil {
		return Set{}, err
	}

	var (
		set   Set
		start *contentLine
	)

	for i := range lines {
		if lines[i].name != "DTSTART" {
			continue
		}

		if start != nil {
			return Set{}, lines[i].error(partError(ErrInvalid, "DTSTART", "", "more than once"))
		}

		start = &lines[i]
	}

	if start == nil {
		return Set{}, partError(ErrInvalid, "DTSTART", "", "missing")
	}

	if err := set.parseStart(start, o); err != nil {
		return Set{}, start.error(err)
	}

	for i := range lines {
		if err := set.parseLine(&lines[i], o); err != nil {
			return Set{}, lines[i].error(err)
		}
	}

	if err := set.Validate(); err != nil {
		return Set{}, err
	}

	return set, nil
}

func (s *Set) parseStart(l *contentLine, o options) error {
	if err := l.allow("TZID", "VALUE"); err != nil {
		return err
	}

	values, kind, err := parseTimes(l, o, nil)
	if err != nil {
		return err
	}

	if len(values) != 1 {
		return partError(ErrInvalid, "DTSTART", l.value, "one value")
	}

	s.Start = values[0]

	switch kind {
	case kindDate:
		s.Form = Date
	case kindFloating:
		s.Form = Floating
	case kindUTC, kindZoned, kindNone:
		s.Form = Zoned
	}

	return nil
}

func (s *Set) parseLine(l *contentLine, o options) error {
	switch l.name {
	case "DTSTART":
		return nil
	case "RRULE", "EXRULE":
		if err := l.allow(); err != nil {
			return err
		}

		r, kind, err := parseRule(l.value, s.Start.Location())
		if err != nil {
			return err
		}

		if kind != kindNone && (kind == kindDate) != (s.Form == Date) {
			return partError(ErrInvalid, "UNTIL", "", "of another type than DTSTART")
		}

		if l.name == "RRULE" {
			s.RRules = append(s.RRules, r)
		} else {
			s.ExRules = append(s.ExRules, r)
		}
	case "RDATE", "EXDATE":
		if err := l.allow("TZID", "VALUE"); err != nil {
			return err
		}

		values, kind, err := parseTimes(l, o, s.Start.Location())
		if err != nil {
			return err
		}

		if (kind == kindDate) != (s.Form == Date) {
			return partError(ErrInvalid, l.name, l.value, "of another type than DTSTART")
		}

		if l.name == "RDATE" {
			s.RDates = append(s.RDates, values...)
		} else {
			s.ExDates = append(s.ExDates, values...)
		}
	default:
		return partError(ErrUnsupported, l.name, "", "not a property of a recurrence set")
	}

	return nil
}

// parseTimes reads the values of DTSTART, RDATE or EXDATE, in their TZID's location, or floating ones in floating, or
// WithLocation's for DTSTART.
func parseTimes(l *contentLine, o options, floating *time.Location) ([]time.Time, valueKind, error) {
	date := false

	if v, ok := l.params["VALUE"]; ok {
		switch strings.ToUpper(v) {
		case "DATE":
			date = true
		case "DATE-TIME":
		case "PERIOD":
			if l.name == "RDATE" {
				return nil, kindNone, partError(ErrUnsupported, "VALUE", v, "")
			}

			return nil, kindNone, partError(ErrInvalid, "VALUE", v, "")
		default:
			return nil, kindNone, partError(ErrInvalid, "VALUE", v, "")
		}
	}

	loc := floating
	if loc == nil {
		loc = o.location
	}

	tzid, zoned := l.params["TZID"]
	if zoned {
		if date {
			return nil, kindNone, partError(ErrInvalid, "TZID", tzid, "with VALUE=DATE")
		}

		var err error
		if loc, err = o.loader(tzid); err != nil {
			return nil, kindNone, partError(err, "TZID", tzid, "")
		}
	}

	var (
		kind   valueKind
		values []time.Time
	)

	for v := range strings.SplitSeq(l.value, ",") {
		t, k, err := parseTime(l.name, v, loc)
		if err != nil {
			return nil, kindNone, err
		}

		if (k == kindDate) != date {
			return nil, kindNone, partError(ErrInvalid, l.name, v, "not of its VALUE type")
		}

		if k == kindUTC && zoned {
			return nil, kindNone, partError(ErrInvalid, l.name, v, "UTC with a TZID")
		}

		if k == kindFloating && zoned {
			k = kindZoned
		}

		kind = k
		values = append(values, t)
	}

	return values, kind, nil
}

// parseRule reads a rule, in floating for a floating UNTIL or a date, and returns how UNTIL was written.
func parseRule(s string, floating *time.Location) (Rule, valueKind, error) {
	if s == "" {
		return Rule{}, kindNone, partError(ErrSyntax, "RRULE", "", "empty")
	}

	var (
		r     Rule
		until valueKind
		seen  = map[string]bool{}
	)

	for part := range strings.SplitSeq(strings.ToUpper(s), ";") {
		name, value, ok := strings.Cut(part, "=")
		if !ok || name == "" || value == "" {
			return Rule{}, kindNone, partError(ErrSyntax, part, "", "not NAME=VALUE")
		}

		if seen[name] {
			return Rule{}, kindNone, partError(ErrSyntax, name, value, "more than once")
		}

		seen[name] = true

		var err error
		if name == "UNTIL" {
			r.Until, until, err = parseTime(name, value, floating)
		} else {
			err = r.parsePart(name, value)
		}

		if err != nil {
			return Rule{}, kindNone, err
		}
	}

	if err := r.Validate(); err != nil {
		return Rule{}, kindNone, err
	}

	return r, until, nil
}

// parsePart reads a rule part but UNTIL into r.
func (r *Rule) parsePart(name, value string) error {
	var err error

	switch name {
	case "FREQ":
		r.Freq, err = parseFreq(value)
	case "COUNT":
		r.Count, err = parsePositive(name, value)
	case "INTERVAL":
		r.Interval, err = parsePositive(name, value)
	case "X-FREQ":
		r.XFreq, err = parseXFreq(value)
	case "BYSECOND":
		r.BySecond, err = parseInts(name, value, 2, false)
	case "BYMINUTE":
		r.ByMinute, err = parseInts(name, value, 2, false)
	case "BYHOUR":
		r.ByHour, err = parseInts(name, value, 2, false)
	case "BYDAY":
		r.ByDay, err = parseByDay(value)
	case "BYMONTHDAY":
		r.ByMonthDay, err = parseInts(name, value, 2, true)
	case "BYYEARDAY":
		r.ByYearDay, err = parseInts(name, value, 3, true)
	case "BYWEEKNO":
		r.ByWeekNo, err = parseInts(name, value, 2, true)
	case "BYMONTH":
		var months []int

		months, err = parseInts(name, value, 2, false)
		for _, m := range months {
			r.ByMonth = append(r.ByMonth, time.Month(m))
		}
	case "BYSETPOS":
		r.BySetPos, err = parseInts(name, value, 3, true)
	case "WKST":
		var ok bool
		if r.WeekStart, ok = parseWeekday(value); !ok {
			err = partError(ErrSyntax, name, value, "")
		}
	case "RSCALE", "SKIP":
		err = partError(ErrUnsupported, name, value, "")
	default:
		err = partError(ErrSyntax, name, value, "unknown rule part")
	}

	return err
}

func parseFreq(value string) (Frequency, error) {
	for f := Secondly; f <= Yearly; f++ {
		if value == f.String() {
			return f, nil
		}
	}

	return 0, partError(ErrSyntax, "FREQ", value, "")
}

// parseXFreq reads X-FREQ, an ISO 8601 duration, PnYnMnWnDTnHnMnS, of which only hours, minutes and seconds are
// handled, to the nanosecond. The last component may have a fraction, after a dot or a comma.
func parseXFreq(value string) (time.Duration, error) {
	s, ok := strings.CutPrefix(value, "P")
	date, clock, timed := strings.Cut(s, "T")
	dates, okDate := durationParts(date, "YMWD")
	times, okTime := durationParts(clock, "HMS")
	parts := append(dates, times...)

	if !ok || !okDate || !okTime || len(parts) == 0 || timed && len(times) == 0 {
		return 0, partError(ErrSyntax, "X-FREQ", value, "not an ISO 8601 duration")
	}

	for _, p := range parts[:len(parts)-1] {
		if p.frac != "" {
			return 0, partError(ErrSyntax, "X-FREQ", value, "a fraction but on the last component")
		}
	}

	if len(dates) > 0 {
		return 0, partError(ErrUnsupported, "X-FREQ", value, "only hours, minutes and seconds")
	}

	var d int64

	for _, p := range times {
		ns, reason := p.nanoseconds()
		if reason == "" && ns > math.MaxInt64-d {
			reason = "longer than a time.Duration"
		}

		if reason != "" {
			return 0, partError(ErrUnsupported, "X-FREQ", value, reason)
		}

		d += ns
	}

	if d == 0 {
		return 0, partError(ErrInvalid, "X-FREQ", value, "")
	}

	return time.Duration(d), nil
}

// durationPart is a component of an ISO 8601 duration: its digits, those of its fraction, and its designator.
type durationPart struct {
	whole, frac string
	unit        byte
}

// durationParts splits s into components, whose designators come in the order of units, each once at most.
func durationParts(s, units string) ([]durationPart, bool) {
	var parts []durationPart

	digits := func(s string) int {
		i := 0
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}

		return i
	}

	for s != "" {
		i := digits(s)
		p := durationPart{whole: s[:i]}

		if i < len(s) && (s[i] == '.' || s[i] == ',') {
			n := digits(s[i+1:])
			p.frac, i = s[i+1:i+1+n], i+1+n

			if n == 0 {
				return nil, false
			}
		}

		if p.whole == "" || i == len(s) {
			return nil, false
		}

		u := strings.IndexByte(units, s[i])
		if u < 0 {
			return nil, false
		}

		p.unit = s[i]
		parts = append(parts, p)
		units, s = units[u+1:], s[i+1:]
	}

	return parts, true
}

// nanoseconds returns the duration of an hours, minutes or seconds component, or why it is not a time.Duration.
func (p durationPart) nanoseconds() (int64, string) {
	const long, fine = "longer than a time.Duration", "finer than a nanosecond"

	unit := int64(time.Second)

	switch p.unit {
	case 'H':
		unit = int64(time.Hour)
	case 'M':
		unit = int64(time.Minute)
	}

	whole, ok := parseUint(p.whole)
	if !ok || whole > math.MaxInt64/unit {
		return 0, long
	}

	ns := whole * unit

	frac := strings.TrimRight(p.frac, "0")
	if frac == "" {
		return ns, ""
	}

	if len(frac) > 18 {
		return 0, fine
	}

	// The fraction is f/10^k units, a whole number of nanoseconds only if 10^k/g divides f, g being gcd(unit, 10^k).
	f, _ := parseUint(frac)
	pow := int64(1)

	for range len(frac) {
		pow *= 10
	}

	g := gcd(unit, pow)
	if f%(pow/g) != 0 {
		return 0, fine
	}

	if extra := f / (pow / g) * (unit / g); ns <= math.MaxInt64-extra {
		return ns + extra, ""
	}

	return 0, long
}

// parseUint reads decimal digits, or returns false if they are more than an int64 holds.
func parseUint(s string) (int64, bool) {
	var n int64

	for _, c := range []byte(s) {
		d := int64(c - '0')
		if n > (math.MaxInt64-d)/10 {
			return 0, false
		}

		n = n*10 + d
	}

	return n, true
}

// parsePositive reads COUNT or INTERVAL, which the grammar allows to be 0, but which mean nothing then.
func parsePositive(name, value string) (int, error) {
	n, ok := parseInt(value, 9, false)
	if !ok {
		return 0, partError(ErrSyntax, name, value, "")
	}

	if n == 0 {
		return 0, partError(ErrInvalid, name, value, "")
	}

	return n, nil
}

func parseInts(name, value string, digits int, signed bool) ([]int, error) {
	var values []int

	for v := range strings.SplitSeq(value, ",") {
		n, ok := parseInt(v, digits, signed)
		if !ok {
			return nil, partError(ErrSyntax, name, v, "")
		}

		values = append(values, n)
	}

	return values, nil
}

func parseByDay(value string) ([]WeekdayNum, error) {
	var days []WeekdayNum

	for v := range strings.SplitSeq(value, ",") {
		if len(v) < 2 {
			return nil, partError(ErrSyntax, "BYDAY", v, "")
		}

		d, ok := parseWeekday(v[len(v)-2:])
		if !ok {
			return nil, partError(ErrSyntax, "BYDAY", v, "")
		}

		n := 0
		if ord := v[:len(v)-2]; ord != "" {
			if n, ok = parseInt(ord, 2, true); !ok || n == 0 {
				return nil, partError(ErrSyntax, "BYDAY", v, "")
			}
		}

		days = append(days, WeekdayNum{N: n, Weekday: d})
	}

	return days, nil
}

func parseWeekday(s string) (Weekday, bool) {
	for d, code := range weekdayCodes {
		if s == code {
			return Weekday(d), true
		}
	}

	return 0, false
}

// parseInt reads up to digits decimal digits, after a sign if signed. RFC 5545 allows leading zeros.
func parseInt(s string, digits int, signed bool) (int, bool) {
	neg := false

	if signed && s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}

	if s == "" || len(s) > digits {
		return 0, false
	}

	n := 0

	for _, c := range []byte(s) {
		if c < '0' || c > '9' {
			return 0, false
		}

		n = n*10 + int(c-'0')
	}

	if neg {
		n = -n
	}

	return n, true
}

// parseTime reads a DATE, such as 20260105, or a DATE-TIME, such as 20260105T090000 or 20260105T090000Z, floating
// ones in loc.
func parseTime(part, s string, loc *time.Location) (time.Time, valueKind, error) {
	kind := kindDate

	switch {
	case len(s) == 8:
	case len(s) == 15 && s[8] == 'T':
		kind = kindFloating
	case len(s) == 16 && s[8] == 'T' && s[15] == 'Z':
		kind, loc = kindUTC, time.UTC
	default:
		return time.Time{}, kindNone, partError(ErrSyntax, part, s, "not a DATE or DATE-TIME")
	}

	y, ok1 := parseInt(s[0:4], 4, false)
	m, ok2 := parseInt(s[4:6], 2, false)
	d, ok3 := parseInt(s[6:8], 2, false)
	hh, mm, ss, ok4, ok5, ok6 := 0, 0, 0, true, true, true

	if kind != kindDate {
		hh, ok4 = parseInt(s[9:11], 2, false)
		mm, ok5 = parseInt(s[11:13], 2, false)
		ss, ok6 = parseInt(s[13:15], 2, false)
	}

	if !ok1 || !ok2 || !ok3 || !ok4 || !ok5 || !ok6 {
		return time.Time{}, kindNone, partError(ErrSyntax, part, s, "not a DATE or DATE-TIME")
	}

	if ss == 60 {
		return time.Time{}, kindNone, partError(ErrUnsupported, part, s, "a leap second")
	}

	if m < 1 || m > 12 || d < 1 || d > monthLen(y, time.Month(m)) || hh > 23 || mm > 59 || ss > 59 {
		return time.Time{}, kindNone, partError(ErrInvalid, part, s, "")
	}

	day := daysFromCivil(y, time.Month(m), d)

	return wall(day, hh*3600+mm*60+ss, 0, loc, &zoneCache{}), kind, nil
}

// contentLine is a line of iCalendar, unfolded: NAME;PARAM=VALUE:VALUE.
type contentLine struct {
	number int
	name   string
	params map[string]string
	value  string
}

func (l *contentLine) error(err error) error {
	return &LineError{Line: l.number, Err: err}
}

// allow returns an error for a parameter other than those named.
func (l *contentLine) allow(names ...string) error {
	for p, v := range l.params {
		if !slices.Contains(names, p) {
			return partError(ErrUnsupported, p, v, "parameter of "+l.name)
		}
	}

	return nil
}

// unfold splits iCalendar into its content lines: a line starting with a space or a tab continues the one before.
func unfold(s string) ([]contentLine, error) {
	type raw struct {
		number int
		text   string
	}

	var lines []raw

	for i, text := range strings.Split(s, "\n") {
		text = strings.TrimSuffix(text, "\r")

		switch {
		case text == "":
		case text[0] == ' ' || text[0] == '\t':
			if len(lines) == 0 {
				return nil, &LineError{Line: i + 1, Err: partError(ErrSyntax, "line", text, "continues nothing")}
			}

			lines[len(lines)-1].text += text[1:]
		default:
			lines = append(lines, raw{number: i + 1, text: text})
		}
	}

	content := make([]contentLine, 0, len(lines))

	for _, r := range lines {
		l, err := parseContentLine(r.text)
		if err != nil {
			return nil, &LineError{Line: r.number, Err: err}
		}

		l.number = r.number
		content = append(content, l)
	}

	return content, nil
}

// parseContentLine reads NAME *(;PARAM=VALUE) :VALUE, a parameter's value quoted or not.
func parseContentLine(s string) (contentLine, error) {
	end := strings.IndexAny(s, ";:")
	if end <= 0 {
		return contentLine{}, partError(ErrSyntax, "line", s, "not NAME:VALUE")
	}

	l := contentLine{name: strings.ToUpper(s[:end]), params: map[string]string{}}
	s = s[end:]

	for s[0] == ';' {
		name, rest, ok := strings.Cut(s[1:], "=")
		if !ok || name == "" || rest == "" {
			return contentLine{}, partError(ErrSyntax, l.name, s, "not a parameter")
		}

		name = strings.ToUpper(name)

		var value string

		if rest[0] == '"' {
			q := strings.IndexByte(rest[1:], '"')
			if q < 0 {
				return contentLine{}, partError(ErrSyntax, name, rest, "unterminated quote")
			}

			value, s = rest[1:q+1], rest[q+2:]
		} else {
			e := strings.IndexAny(rest, ";:")
			if e < 0 {
				return contentLine{}, partError(ErrSyntax, l.name, s, "no value")
			}

			value, s = rest[:e], rest[e:]
		}

		if _, ok := l.params[name]; ok {
			return contentLine{}, partError(ErrSyntax, name, value, "more than once")
		}

		l.params[name] = value

		if s == "" {
			return contentLine{}, partError(ErrSyntax, l.name, "", "no value")
		}
	}

	if s[0] != ':' {
		return contentLine{}, partError(ErrSyntax, l.name, s, "not NAME:VALUE")
	}

	l.value = s[1:]

	return l, nil
}
