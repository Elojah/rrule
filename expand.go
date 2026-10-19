package rrule

import (
	"iter"
	"math/bits"
	"slices"
	"time"
)

// All returns the rule's occurrences from start, in order, as instants in start's location. start is the first only
// if the rule matches it: a Set's Start is always its first.
//
// The rule repeats on start's wall clock: a daily rule at 09:00 stays at 09:00 across daylight saving time. A wall
// clock that happens twice is its first instant, and one that never happens is read with the offset from before the
// gap, so 02:30 becomes 03:30 when clocks go forward at 02:00, and the instants yielded always increase. Invalid
// dates, such as February 30, are skipped and not counted. Expansion ends in 9999 at the latest.
//
// All yields nothing for a rule that Validate refuses.
func (r Rule) All(start time.Time) iter.Seq[time.Time] {
	return r.Between(start, time.Time{}, time.Time{})
}

// Between returns the rule's occurrences from start, as All does, at or after from and before to. Unlike All's, its
// iteration ends at to, however few occurrences the rule has. A zero to is no end.
func (r Rule) Between(start, from, to time.Time) iter.Seq[time.Time] {
	return func(yield func(time.Time) bool) {
		e := newExpansion(r, start, to)
		if e == nil {
			return
		}

		for {
			t, ok := e.next()
			if !ok {
				return
			}

			if !t.Before(from) && !yield(t) {
				return
			}
		}
	}
}

// expansion yields a rule's occurrences. It goes through the rule's periods, a year, a month, a week, a day, an hour,
// a minute or a second, Interval apart, from start's. Each period's candidates are the days that pass every BYxxx
// filter, by the times of day: those of BYHOUR, BYMINUTE and BYSECOND for DAILY and longer, or the period's own for
// shorter ones. Rule parts the rule lacks take start's, as python-dateutil does, which is what RFC 5545's table of
// expanding and limiting parts amounts to.
type expansion struct {
	freq     Frequency
	wkst     Weekday
	step     int64 // in the period's unit: years, months, days, hours, minutes or seconds
	first    int64 // start's period, which the others are counted from
	period   int64
	last     int64 // the last period that may hold an occurrence
	every    int64 // X-FREQ in nanoseconds, the step of a SECONDLY rule's periods if not 0
	nsec     int   // of the current period's candidates: start's, but under X-FREQ
	startKey int64 // start's civil time, as day*secondsPerDay + second of the day
	startNs  int   // and its nanoseconds, which X-FREQ's periods count from
	remain   int   // occurrences left, -1 for no COUNT
	until    time.Time
	end      time.Time // the first instant not to yield, if not zero
	loc      *time.Location
	zone     zoneCache

	// Filters, as bit sets: bit n of the positive set for n, of the negative set for -n.
	months      uint16
	weekNos     [2]uint64
	yearDays    [2][6]uint64
	monthDays   [2]uint32
	weekdays    uint8
	nth         [7][2]uint64 // per weekday
	hasWeekNo   bool
	hasYearDay  bool
	hasMonthDay bool
	hasNth      bool
	nthYear     bool // nth weekdays count in the year, not in the month
	hours       uint32
	minutes     uint64
	seconds     uint64
	setPos      []int

	// times holds the seconds of the day of each occurrence of a day for DAILY and longer, or for shorter ones the
	// offsets from the period's own second of the day, base.
	times []int32

	// The current period's candidates: days by times, or, with BYSETPOS, the keys it picks.
	days []int64
	base int64
	keys []int64
	i    int

	// What the last checks found: the last day dayOK checked, and whether it passed; the year of the last day
	// checked, its first day and its length; and for BYWEEKNO, the first days of week 1 of the year of the last week
	// checked, and of the year after.
	cache struct {
		day            int64
		pass, dayOK    bool
		year           int
		start          int64
		length         int
		yearOK         bool
		week1, nextWk1 int64
		weekOK         bool
	}

	yielded bool
	prev    time.Time
	done    bool
}

// newExpansion returns the expansion of r from start, to end excluded if it is not zero, or nil if r is not valid.
func newExpansion(r Rule, start, end time.Time) *expansion {
	if r.Validate() != nil {
		return nil
	}

	loc := start.Location()
	y, m, d := start.Date()
	h, mi, s := start.Clock()
	startDay := daysFromCivil(y, m, d)
	startSec := h*3600 + mi*60 + s

	e := &expansion{
		freq:     r.Freq,
		wkst:     r.WeekStart,
		every:    int64(r.XFreq),
		nsec:     start.Nanosecond(),
		startKey: startDay*secondsPerDay + int64(startSec),
		startNs:  start.Nanosecond(),
		remain:   -1,
		until:    r.Until,
		end:      end,
		loc:      loc,
		setPos:   r.BySetPos,
		nthYear:  r.Freq == Yearly && len(r.ByMonth) == 0, // as the rule's FREQ counts, under X-FREQ too
	}

	if e.every > 0 {
		// X-FREQ's periods are those of a SECONDLY rule, of another length: every BYxxx part limits, and none takes
		// start's value.
		r.Freq, e.freq = Secondly, Secondly
	}

	if r.Count > 0 {
		e.remain = r.Count
	}

	e.filters(r, start)
	e.timesOfDay(r, h, mi, s)
	e.periods(r, startDay, startSec)

	switch {
	case y < 0 || y > 9999, !r.Until.IsZero() && r.Until.Before(start), len(e.times) == 0, !e.reachable(startSec):
		e.done = true
	case e.freq == Secondly && len(e.setPos) > 0 && !slices.Contains(e.setPos, 1) && !slices.Contains(e.setPos, -1):
		e.done = true // a SECONDLY period has one candidate, which BYSETPOS leaves out
	default:
		e.fill()
	}

	return e
}

// next returns the next occurrence, or false once there are none.
func (e *expansion) next() (time.Time, bool) {
	for !e.done {
		key, ok := e.candidate()
		if !ok {
			e.advance()
			e.fill()

			continue
		}

		if key < e.startKey {
			continue
		}

		day := floorDiv(key, secondsPerDay)
		if day > maxDay {
			break
		}

		t := wall(day, int(key-day*secondsPerDay), e.nsec, e.loc, &e.zone)
		if e.yielded && !t.After(e.prev) {
			continue // a wall clock that a gap moved onto or before the last
		}

		if !e.until.IsZero() && t.After(e.until) || !e.end.IsZero() && !t.Before(e.end) {
			break
		}

		if e.remain == 0 {
			break
		}

		if e.remain > 0 {
			e.remain--
		}

		e.yielded, e.prev = true, t

		return t, true
	}

	e.done = true

	return time.Time{}, false
}

// filters sets the BYxxx day filters, with dateutil's defaults from start.
func (e *expansion) filters(r Rule, start time.Time) {
	for _, m := range r.ByMonth {
		e.months |= 1 << m
	}

	for _, n := range r.ByWeekNo {
		setSigned(e.weekNos[:], n)
	}

	e.hasWeekNo = len(r.ByWeekNo) > 0

	for _, n := range r.ByYearDay {
		if n > 0 {
			e.yearDays[0][n/64] |= 1 << (n % 64)
		} else {
			e.yearDays[1][-n/64] |= 1 << (-n % 64)
		}
	}

	for _, n := range r.ByMonthDay {
		if n > 0 {
			e.monthDays[0] |= 1 << n
		} else {
			e.monthDays[1] |= 1 << -n
		}
	}

	for _, d := range r.ByDay {
		if d.N == 0 {
			e.weekdays |= 1 << d.Weekday
		} else {
			setSigned(e.nth[d.Weekday][:], d.N)
			e.hasNth = true
		}
	}

	if len(r.ByWeekNo) == 0 && len(r.ByYearDay) == 0 && len(r.ByMonthDay) == 0 && len(r.ByDay) == 0 {
		_, month, day := start.Date()

		switch r.Freq {
		case Yearly:
			if len(r.ByMonth) == 0 {
				e.months = 1 << month
			}

			e.monthDays[0] = 1 << day
		case Monthly:
			e.monthDays[0] = 1 << day
		case Weekly:
			e.weekdays = 1 << weekdayOf(daysFromCivil(start.Date()))
		case Secondly, Minutely, Hourly, Daily:
		}
	}

	e.hasYearDay = e.yearDays != [2][6]uint64{}
	e.hasMonthDay = e.monthDays != [2]uint32{}
}

// timesOfDay sets the hours, minutes and seconds, those that expand defaulting to start's, and the times they make.
func (e *expansion) timesOfDay(r Rule, h, mi, s int) {
	for _, v := range r.ByHour {
		e.hours |= 1 << v
	}

	for _, v := range r.ByMinute {
		e.minutes |= 1 << v
	}

	for _, v := range r.BySecond {
		e.seconds |= 1 << v
	}

	if e.hours == 0 && r.Freq >= Daily {
		e.hours = 1 << h
	}

	if e.minutes == 0 && r.Freq >= Hourly {
		e.minutes = 1 << mi
	}

	if e.seconds == 0 && r.Freq >= Minutely {
		e.seconds = 1 << s
	}

	seconds := e.seconds &^ (1 << 60) // no leap second on Go's time line

	switch r.Freq {
	case Secondly:
		e.times = []int32{0}
	case Minutely:
		for sec := range eachBit(seconds) {
			e.times = append(e.times, int32(sec)) //nolint:gosec // in 0..59
		}
	case Hourly:
		for m := range eachBit(e.minutes) {
			for sec := range eachBit(seconds) {
				e.times = append(e.times, int32(m*60+sec)) //nolint:gosec // within an hour
			}
		}
	case Daily, Weekly, Monthly, Yearly:
		for hour := range eachBit(uint64(e.hours)) {
			for m := range eachBit(e.minutes) {
				for sec := range eachBit(seconds) {
					e.times = append(e.times, int32(hour*3600+m*60+sec)) //nolint:gosec // within a day
				}
			}
		}
	}
}

// periods sets the first period, the step between periods, and the last period: the one holding 9999-12-31, or the
// day after Until's or end's, which a day covers whatever the offset.
func (e *expansion) periods(r Rule, startDay int64, startSec int) {
	interval := min(int64(max(r.Interval, 1)), maxStep)
	lastDay := maxDay

	for _, t := range []time.Time{r.Until, e.end} {
		if !t.IsZero() {
			lastDay = min(lastDay, daysFromCivil(t.In(e.loc).Date())+1)
		}
	}

	if e.every > 0 {
		// The periods of X-FREQ count from start's, 0, to the last before the day after lastDay. Interval is FREQ's.
		e.first, e.step, e.period = 0, 1, 0
		e.last = e.periodAt((lastDay+1)*secondsPerDay) - 1

		return
	}

	y, m, _ := civilFromDays(startDay)
	ly, lm, _ := civilFromDays(lastDay)
	sec := int64(startSec)

	switch e.freq {
	case Yearly:
		e.first, e.last, e.step = int64(y), int64(ly), interval
	case Monthly:
		e.first, e.last, e.step = int64(y)*12+int64(m)-1, int64(ly)*12+int64(lm)-1, interval
	case Weekly:
		e.first = startDay - int64((weekdayOf(startDay)-e.wkst+7)%7)
		e.last, e.step = lastDay, mulSat(interval, 7)
	case Daily:
		e.first, e.last, e.step = startDay, lastDay, interval
	case Hourly:
		e.first, e.last, e.step = startDay*24+sec/3600, lastDay*24+23, interval
	case Minutely:
		e.first, e.last, e.step = startDay*1440+sec/60, lastDay*1440+1439, interval
	case Secondly:
		e.first, e.last, e.step = startDay*secondsPerDay+sec, lastDay*secondsPerDay+secondsPerDay-1, interval
	}

	e.period = e.first
}

// reachable reports whether a period shorter than a day can have one of the hours, minutes and seconds the rule
// limits to: those it lands on repeat with the greatest common divisor of the step and the day, from start's. Under
// X-FREQ, they are nanoseconds, and a second is reachable if one of its nanoseconds is.
func (e *expansion) reachable(startSec int) bool {
	var units, unit int64

	switch e.freq {
	case Hourly:
		units, unit = 24, 3600
	case Minutely:
		units, unit = 1440, 60
	case Secondly:
		units, unit = secondsPerDay, 1
	case Daily, Weekly, Monthly, Yearly:
		return true
	}

	limited := e.hours != 0 || (e.freq <= Minutely && e.minutes != 0) || (e.freq == Secondly && e.seconds != 0)
	if !limited {
		return true
	}

	g := gcd(e.step%units, units)
	want := (int64(startSec) / unit) % g
	width := int64(1) // of an hour, a minute or a second, in the units of the step

	if e.every > 0 {
		units, width = secondsPerDay*int64(time.Second), int64(time.Second)
		g = gcd(e.every%units, units)
		want = (int64(startSec)*width + int64(e.startNs)) % g
	}

	// hit reports whether the step lands in the nth hour, minute or second of the day.
	hit := func(n int) bool {
		return ((want-int64(n)*width)%g+g)%g < width
	}

	for h := range 24 {
		if e.hours != 0 && e.hours&(1<<h) == 0 {
			continue
		}

		if e.freq == Hourly {
			if hit(h) {
				return true
			}

			continue
		}

		for m := range 60 {
			if e.minutes != 0 && e.minutes&(1<<m) == 0 {
				continue
			}

			if e.freq == Minutely {
				if hit(h*60 + m) {
					return true
				}

				continue
			}

			for s := range 60 {
				if (e.seconds == 0 || e.seconds&(1<<s) != 0) && hit(h*3600+m*60+s) {
					return true
				}
			}
		}
	}

	return false
}

// candidate returns the current period's next candidate, as a key, or false once there are none.
func (e *expansion) candidate() (int64, bool) {
	if len(e.setPos) > 0 {
		if e.i >= len(e.keys) {
			return 0, false
		}

		e.i++

		return e.keys[e.i-1], true
	}

	if e.i >= len(e.days)*len(e.times) {
		return 0, false
	}

	d, t := e.i/len(e.times), e.i%len(e.times)
	e.i++

	return e.days[d]*secondsPerDay + e.base + int64(e.times[t]), true
}

// advance moves to the next period.
func (e *expansion) advance() {
	e.jump(e.period + 1)
}

// jump moves to the first period from target on, which may be past the last.
func (e *expansion) jump(target int64) {
	if target > e.last {
		e.done = true

		return
	}

	k := (target - e.first + e.step - 1) / e.step
	e.period = e.first + k*e.step

	if e.period > e.last {
		e.done = true
	}
}

// fill finds the candidates of the current period, moving on to the next until one has some.
func (e *expansion) fill() {
	for !e.done {
		e.i = 0
		e.days = e.days[:0]
		e.base = 0

		if e.fillDays() {
			nd, nt := len(e.days), len(e.times)
			if nd == 0 {
				e.advance()

				continue
			}

			if len(e.setPos) == 0 {
				return
			}

			e.keys = e.keys[:0]
			n := nd * nt

			for _, p := range e.setPos {
				i := p - 1
				if p < 0 {
					i = n + p
				}

				if i < 0 || i >= n {
					continue
				}

				if k := e.days[i/nt]*secondsPerDay + e.base + int64(e.times[i%nt]); !slices.Contains(e.keys, k) {
					e.keys = append(e.keys, k)
				}
			}

			if len(e.keys) > 0 {
				slices.Sort(e.keys)

				return
			}

			e.advance()
		}
	}
}

// fillDays sets the days of the current period that pass the filters, and the base of its times. It returns false
// if it moved to another period instead, as periods shorter than a day do to skip a day, an hour or a minute that
// cannot match.
func (e *expansion) fillDays() bool {
	switch e.freq {
	case Yearly:
		y := int(e.period)
		for m := time.January; m <= time.December; m++ {
			if e.months == 0 || e.months&(1<<m) != 0 {
				e.monthDays1(y, m)
			}
		}
	case Monthly:
		y := floorDiv(e.period, 12)
		if m := time.Month(e.period - y*12 + 1); e.months == 0 || e.months&(1<<m) != 0 {
			e.monthDays1(int(y), m)
		}
	case Weekly:
		for day := e.period; day < e.period+7; day++ {
			if e.dayOK(day) {
				e.days = append(e.days, day)
			}
		}
	case Daily:
		if !e.dayOK(e.period) {
			e.jump(e.nextDay(e.period))

			return false
		}

		e.days = append(e.days, e.period)
	case Hourly, Minutely, Secondly:
		return e.fillTime()
	}

	return true
}

// monthDays1 appends the days of a month that pass the filters.
func (e *expansion) monthDays1(y int, m time.Month) {
	first := daysFromCivil(y, m, 1)
	for d := range monthLen(y, m) {
		if day := first + int64(d); e.checkDay(day, y, m, d+1) {
			e.days = append(e.days, day)
		}
	}
}

// fillTime is fillDays for periods shorter than a day.
func (e *expansion) fillTime() bool {
	var perDay int64

	switch e.freq {
	case Hourly:
		perDay = 24
	case Minutely:
		perDay = 1440
	case Secondly:
		perDay = secondsPerDay
	case Daily, Weekly, Monthly, Yearly:
		return true
	}

	unit := secondsPerDay / perDay
	day := floorDiv(e.period, perDay)
	sec := (e.period - day*perDay) * unit

	if e.every > 0 {
		var key int64

		key, e.nsec = e.civilAt(e.period)
		day = floorDiv(key, secondsPerDay)
		sec = key - day*secondsPerDay
	}

	h, m, s := int(sec/3600), int(sec/60%60), int(sec%60)

	// Every period before the next day, hour, minute or second the rule has is filtered: jump lands on the first of
	// them that the interval reaches.
	to := func(day int64, sec int) {
		if key := day*secondsPerDay + int64(sec); e.every > 0 {
			e.jump(e.periodAt(key))
		} else {
			e.jump(key / unit)
		}
	}

	switch {
	case !e.dayOK(day):
		to(e.nextDay(day), 0)
	case e.hours != 0 && e.hours&(1<<h) == 0:
		to(day, nextBit(uint64(e.hours), h, 24)*3600)
	case e.freq <= Minutely && e.minutes != 0 && e.minutes&(1<<m) == 0:
		to(day, h*3600+nextBit(e.minutes, m, 60)*60)
	case e.freq == Secondly && e.seconds != 0 && e.seconds&(1<<s) == 0:
		to(day, h*3600+m*60+nextBit(e.seconds, s, 60))
	default:
		e.days = append(e.days, day)
		e.base = sec

		return true
	}

	return false
}

// civilAt returns the civil time of X-FREQ period k, k X-FREQs after start's, as a key and nanoseconds.
func (e *expansion) civilAt(k int64) (int64, int) {
	// k X-FREQs over 1e9, in 128 bits: k is at most the last period, so the high word is below 1e9.
	hi, lo := bits.Mul64(uint64(k), uint64(e.every)) //nolint:gosec // both positive
	sec, ns := bits.Div64(hi, lo, uint64(time.Second))
	ns += uint64(e.startNs) //nolint:gosec // below 1e9
	sec += ns / uint64(time.Second)

	return e.startKey + int64(sec), int(ns % uint64(time.Second)) //nolint:gosec // seconds to 9999, below 1e9
}

// periodAt returns the first X-FREQ period whose civil time is at or after the second key, from 0, start's, to
// maxStep at most, which no expansion reaches.
func (e *expansion) periodAt(key int64) int64 {
	sec := key - e.startKey
	if sec <= 0 {
		return 0
	}

	// The nanoseconds from start's, rounded up to a period, in 128 bits: from year 0 to 9999, they reach 2^69.
	every := uint64(e.every) //nolint:gosec // positive
	hi, lo := bits.Mul64(uint64(sec), uint64(time.Second))
	lo, borrow := bits.Sub64(lo, uint64(e.startNs), 0) //nolint:gosec // below 1e9
	hi -= borrow
	lo, carry := bits.Add64(lo, every-1, 0)
	hi += carry

	if hi >= every {
		return maxStep
	}

	k, _ := bits.Div64(hi, lo, every)

	return int64(min(k, maxStep)) //nolint:gosec // at most maxStep
}

// dayOK reports whether a day passes the day filters.
func (e *expansion) dayOK(day int64) bool {
	c := &e.cache
	if !c.dayOK || c.day != day {
		y, m, d := civilFromDays(day)
		c.day, c.pass, c.dayOK = day, e.checkDay(day, y, m, d), true
	}

	return c.pass
}

// nextDay returns the day after day, or, if BYMONTH leaves out day's month, the first of the next month it has.
func (e *expansion) nextDay(day int64) int64 {
	y, m, _ := civilFromDays(day)
	if e.months == 0 || e.months&(1<<m) != 0 {
		return day + 1
	}

	for {
		if m++; m > time.December {
			y, m = y+1, time.January
		}

		if e.months&(1<<m) != 0 {
			return daysFromCivil(y, m, 1)
		}
	}
}

// checkDay is dayOK for day, which is y-m-d.
func (e *expansion) checkDay(day int64, y int, m time.Month, d int) bool {
	if e.months != 0 && e.months&(1<<m) == 0 {
		return false
	}

	if e.hasWeekNo {
		n, weeks := e.weekNo(day)
		if !hasSigned(e.weekNos[:], n, weeks) {
			return false
		}
	}

	c := &e.cache
	if !c.yearOK || c.year != y {
		c.year, c.start, c.length, c.yearOK = y, daysFromCivil(y, time.January, 1), yearLen(y), true
	}

	yd := int(day-c.start) + 1
	yl := c.length

	if e.hasYearDay {
		neg := yl + 1 - yd
		if e.yearDays[0][yd/64]&(1<<(yd%64)) == 0 && e.yearDays[1][neg/64]&(1<<(neg%64)) == 0 {
			return false
		}
	}

	ml := monthLen(y, m)

	if e.hasMonthDay && e.monthDays[0]&(1<<d) == 0 && e.monthDays[1]&(1<<(ml+1-d)) == 0 {
		return false
	}

	if e.weekdays == 0 && !e.hasNth {
		return true
	}

	wd := weekdayOf(day)
	if e.weekdays&(1<<wd) != 0 {
		return true
	}

	if !e.hasNth {
		return false
	}

	// Which such weekday of the month, or of the year, the day is, from the start and from the end.
	n, total := d, ml
	if e.nthYear {
		n, total = yd, yl
	}

	set := e.nth[wd]

	return set[0]&(1<<((n-1)/7+1)) != 0 || set[1]&(1<<((total-n)/7+1)) != 0
}

// weekNo is the function weekNo, which it calls only for a week of another year than the last.
func (e *expansion) weekNo(day int64) (int, int) {
	c := &e.cache
	start := day - int64((weekdayOf(day)-e.wkst+7)%7)

	if !c.weekOK || start < c.week1 || start >= c.nextWk1 {
		y, _, _ := civilFromDays(start + 3)
		c.week1, c.nextWk1, c.weekOK = weekOne(y, e.wkst), weekOne(y+1, e.wkst), true
	}

	return int((start-c.week1)/7) + 1, int((c.nextWk1 - c.week1) / 7)
}

// nextBit returns the lowest bit of set above n and below limit, or limit if there is none.
func nextBit(set uint64, n, limit int) int {
	if i := bits.TrailingZeros64(set >> (n + 1) << (n + 1)); i < limit {
		return i
	}

	return limit
}

// setSigned sets n in a pair of positive and negative bit sets.
func setSigned(set []uint64, n int) {
	if n > 0 {
		set[0] |= 1 << n
	} else {
		set[1] |= 1 << -n
	}
}

// hasSigned reports whether a pair of bit sets holds n, counted from 1 of total, or n counted from the end: for the
// first of 52, 1 or -52.
func hasSigned(set []uint64, n, total int) bool {
	return set[0]&(1<<n) != 0 || set[1]&(1<<(total+1-n)) != 0
}

// eachBit yields the bits set in b, from the lowest.
func eachBit(b uint64) iter.Seq[int] {
	return func(yield func(int) bool) {
		for b != 0 {
			i := bits.TrailingZeros64(b)
			if !yield(i) {
				return
			}

			b &= b - 1
		}
	}
}

func gcd(a, b int64) int64 {
	for b != 0 {
		a, b = b, a%b
	}

	return a
}

// maxStep is more periods than years 0 to 9999 have seconds, small enough that jump's arithmetic does not overflow.
// X-FREQ's periods stop there too: under 69 nanoseconds, it ends 2^62 periods after the start, 146 years at least.
const maxStep = 1 << 62

// mulSat returns a×b for positive a and b, or maxStep if it is more.
func mulSat(a, b int64) int64 {
	if a > maxStep/b {
		return maxStep
	}

	return a * b
}
