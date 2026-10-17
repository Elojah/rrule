package rrule

import (
	"strconv"
	"time"
)

// Frequency is how often a rule repeats, its FREQ. The zero value is none, which Validate refuses.
type Frequency int

// Frequencies, from the shortest period.
const (
	Secondly Frequency = iota + 1
	Minutely
	Hourly
	Daily
	Weekly
	Monthly
	Yearly
)

// String returns the frequency as RFC 5545 writes it, such as WEEKLY.
func (f Frequency) String() string {
	switch f {
	case Secondly:
		return "SECONDLY"
	case Minutely:
		return "MINUTELY"
	case Hourly:
		return "HOURLY"
	case Daily:
		return "DAILY"
	case Weekly:
		return "WEEKLY"
	case Monthly:
		return "MONTHLY"
	case Yearly:
		return "YEARLY"
	default:
		return "Frequency(" + strconv.Itoa(int(f)) + ")"
	}
}

// Weekday is a day of the week. It counts from Monday, unlike time.Weekday, so that the zero WeekStart is Monday,
// RFC 5545's default.
type Weekday int

// Weekdays.
const (
	Monday Weekday = iota
	Tuesday
	Wednesday
	Thursday
	Friday
	Saturday
	Sunday
)

// String returns the weekday as RFC 5545 writes it, such as MO.
func (d Weekday) String() string {
	if d < Monday || d > Sunday {
		return "Weekday(" + strconv.Itoa(int(d)) + ")"
	}

	return weekdayCodes[d]
}

// Nth returns the nth such weekday of the month or the year, from its end if n is negative: Friday.Nth(-1) is the
// last Friday, BYDAY=-1FR. Nth(0) is every such weekday.
func (d Weekday) Nth(n int) WeekdayNum {
	return WeekdayNum{N: n, Weekday: d}
}

// WeekdayNum is an entry of BYDAY: a weekday, and which one of the month or the year it is, N, from its end if
// negative. N is 0 for every such weekday.
type WeekdayNum struct {
	N       int
	Weekday Weekday
}

// String returns the entry as RFC 5545 writes it, such as -1FR or MO.
func (w WeekdayNum) String() string {
	if w.N == 0 {
		return w.Weekday.String()
	}

	return strconv.Itoa(w.N) + w.Weekday.String()
}

// Rule is a recurrence rule, the value of an RRULE. It holds no start: its occurrences begin at the time given to
// All, and a Set holds one, its DTSTART.
//
// A zero or nil field is a rule part left out. Validate checks the rest, as Parse does.
type Rule struct {
	Freq Frequency

	// Interval is how many periods of Freq separate those the rule repeats in, 1 if 0.
	Interval int

	// Count is how many occurrences there are, without bound if 0. A rule may have a Count or an Until, not both.
	Count int

	// Until is the last instant an occurrence may be at, without bound if zero.
	Until time.Time

	// WeekStart is the day weeks start on, for WEEKLY rules with an Interval above 1 and for ByWeekNo.
	WeekStart Weekday

	BySecond   []int // 0 to 60
	ByMinute   []int // 0 to 59
	ByHour     []int // 0 to 23
	ByDay      []WeekdayNum
	ByMonthDay []int // 1 to 31, or -31 to -1 from the end of the month
	ByYearDay  []int // 1 to 366, or -366 to -1
	ByWeekNo   []int // 1 to 53, or -53 to -1
	ByMonth    []time.Month
	BySetPos   []int // 1 to 366, or -366 to -1, among the occurrences of each period
}

// Validate returns a *PartError for a rule that RFC 5545 does not allow: no Freq, a value out of range, a Count with
// an Until, or a rule part that its Freq refuses, such as ByWeekNo with anything but YEARLY.
func (r Rule) Validate() error {
	if r.Freq < Secondly || r.Freq > Yearly {
		if r.Freq == 0 {
			return partError(ErrInvalid, "FREQ", "", "missing")
		}

		return partError(ErrInvalid, "FREQ", strconv.Itoa(int(r.Freq)), "")
	}

	if r.Interval < 0 {
		return partError(ErrInvalid, "INTERVAL", strconv.Itoa(r.Interval), "")
	}

	if r.Count < 0 {
		return partError(ErrInvalid, "COUNT", strconv.Itoa(r.Count), "")
	}

	if r.Count > 0 && !r.Until.IsZero() {
		return partError(ErrInvalid, "COUNT", strconv.Itoa(r.Count), "with UNTIL")
	}

	if r.WeekStart < Monday || r.WeekStart > Sunday {
		return partError(ErrInvalid, "WKST", strconv.Itoa(int(r.WeekStart)), "")
	}

	if err := r.validateTime(); err != nil {
		return err
	}

	if err := r.validateDays(); err != nil {
		return err
	}

	for _, m := range r.ByMonth {
		if m < time.January || m > time.December {
			return partError(ErrInvalid, "BYMONTH", strconv.Itoa(int(m)), "")
		}
	}

	if err := checkList("BYSETPOS", r.BySetPos, 366, true); err != nil {
		return err
	}

	if len(r.BySetPos) > 0 && !r.hasBy() {
		return partError(ErrInvalid, "BYSETPOS", "", "without another BYxxx rule part")
	}

	return nil
}

func (r Rule) validateTime() error {
	if err := checkList("BYSECOND", r.BySecond, 60, false); err != nil {
		return err
	}

	if err := checkList("BYMINUTE", r.ByMinute, 59, false); err != nil {
		return err
	}

	return checkList("BYHOUR", r.ByHour, 23, false)
}

func (r Rule) validateDays() error {
	for _, d := range r.ByDay {
		if d.Weekday < Monday || d.Weekday > Sunday || d.N < -53 || d.N > 53 {
			return partError(ErrInvalid, "BYDAY", d.String(), "")
		}

		if d.N != 0 && r.Freq != Monthly && r.Freq != Yearly {
			return partError(ErrInvalid, "BYDAY", d.String(), "a number only with FREQ=MONTHLY or YEARLY")
		}

		if d.N != 0 && len(r.ByWeekNo) > 0 {
			return partError(ErrInvalid, "BYDAY", d.String(), "a number not with BYWEEKNO")
		}
	}

	if err := checkList("BYMONTHDAY", r.ByMonthDay, 31, true); err != nil {
		return err
	}

	if len(r.ByMonthDay) > 0 && r.Freq == Weekly {
		return partError(ErrInvalid, "BYMONTHDAY", "", "not with FREQ=WEEKLY")
	}

	if err := checkList("BYYEARDAY", r.ByYearDay, 366, true); err != nil {
		return err
	}

	if len(r.ByYearDay) > 0 && (r.Freq == Daily || r.Freq == Weekly || r.Freq == Monthly) {
		return partError(ErrInvalid, "BYYEARDAY", "", "not with FREQ="+r.Freq.String())
	}

	if err := checkList("BYWEEKNO", r.ByWeekNo, 53, true); err != nil {
		return err
	}

	if len(r.ByWeekNo) > 0 && r.Freq != Yearly {
		return partError(ErrInvalid, "BYWEEKNO", "", "only with FREQ=YEARLY")
	}

	return nil
}

// hasBy reports whether the rule has a BYxxx rule part other than BYSETPOS.
func (r Rule) hasBy() bool {
	return len(r.BySecond) > 0 || len(r.ByMinute) > 0 || len(r.ByHour) > 0 || len(r.ByDay) > 0 ||
		len(r.ByMonthDay) > 0 || len(r.ByYearDay) > 0 || len(r.ByWeekNo) > 0 || len(r.ByMonth) > 0
}

// checkList checks that each value is in 0..limit, or in -limit..-1 and 1..limit if signed.
func checkList(part string, values []int, limit int, signed bool) error {
	for _, v := range values {
		if v > limit || (signed && (v == 0 || v < -limit)) || (!signed && v < 0) {
			return partError(ErrInvalid, part, strconv.Itoa(v), "")
		}
	}

	return nil
}

var weekdayCodes = [...]string{"MO", "TU", "WE", "TH", "FR", "SA", "SU"}
