package rrule

import (
	"math"
	"time"
)

// Expansion works on civil time, the wall clock without a zone: days counted from 1970-01-01, and seconds of the day.
// Only an occurrence becomes an instant, by wall.

const secondsPerDay = 86400

// maxDay is the last day expansion reaches, 9999-12-31: RFC 5545 writes years with 4 digits.
var maxDay = daysFromCivil(9999, time.December, 31)

// daysFromCivil returns the day of y-m-d, counted from 1970-01-01, with Howard Hinnant's algorithm.
func daysFromCivil(y int, m time.Month, d int) int64 {
	yy := int64(y)
	if m <= time.February {
		yy--
	}

	era := floorDiv(yy, 400)
	yoe := yy - era*400
	mp := (int64(m) + 9) % 12
	doy := (153*mp+2)/5 + int64(d) - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy

	return era*146097 + doe - 719468
}

// civilFromDays is daysFromCivil's inverse.
func civilFromDays(z int64) (int, time.Month, int) {
	z += 719468
	era := floorDiv(z, 146097)
	doe := z - era*146097
	yoe := (doe - doe/1460 + doe/36524 - doe/146096) / 365
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := doy - (153*mp+2)/5 + 1
	m := (mp+2)%12 + 1
	y := yoe + era*400

	if m <= 2 {
		y++
	}

	return int(y), time.Month(m), int(d)
}

// weekdayOf returns the weekday of a day: 1970-01-01 was a Thursday.
func weekdayOf(day int64) Weekday {
	return Weekday((day%7 + 7 + 3) % 7)
}

func isLeap(y int) bool {
	return y%4 == 0 && (y%100 != 0 || y%400 == 0)
}

func yearLen(y int) int {
	if isLeap(y) {
		return 366
	}

	return 365
}

func monthLen(y int, m time.Month) int {
	switch m {
	case time.February:
		if isLeap(y) {
			return 29
		}

		return 28
	case time.April, time.June, time.September, time.November:
		return 30
	case time.January, time.March, time.May, time.July, time.August, time.October, time.December:
		return 31
	default:
		return 0
	}
}

// weekOne returns the first day of week 1 of year y, for weeks starting on wkst: the week holding January 4, which
// is the first with at least 4 days in the year.
func weekOne(y int, wkst Weekday) int64 {
	jan4 := daysFromCivil(y, time.January, 4)

	return jan4 - int64((weekdayOf(jan4)-wkst+7)%7)
}

// weekNo returns the week of a day, for weeks starting on wkst, as RFC 5545 numbers them: from 1, in the year holding
// at least 4 of the week's days, which may be the one before or after the day's. It also returns how many weeks that
// year has.
func weekNo(day int64, wkst Weekday) (int, int) {
	start := day - int64((weekdayOf(day)-wkst+7)%7)
	y, _, _ := civilFromDays(start + 3) // the week's year holds its fourth day
	first := weekOne(y, wkst)

	return int((start-first)/7) + 1, int((weekOne(y+1, wkst) - first) / 7)
}

// wall returns the instant of a civil time in loc. A wall clock that happens twice, when clocks go back, is its first
// instant, and one that never happens, when they go forward, is read with the offset from before the gap: RFC 5545,
// section 3.3.5. time.Date guarantees neither.
func wall(day int64, sec int, nsec int, loc *time.Location, z *zoneCache) time.Time {
	u := day*secondsPerDay + int64(sec) // the wall clock, read as if in UTC
	if loc == time.UTC {
		return time.Unix(u, int64(nsec)).UTC()
	}

	// Far from the bounds of the offset found last, there is no transition to care about: offsets change by hours.
	if z.ok {
		x := u - int64(z.offset)
		if x >= z.start+secondsPerDay && x < z.end-secondsPerDay {
			return time.Unix(x, int64(nsec)).In(loc)
		}
	}

	// The offsets a day either side of the wall clock: they differ only if a transition is near.
	before := offsetAt(u-secondsPerDay, loc)
	after := offsetAt(u+secondsPerDay, loc)
	t := time.Unix(u-int64(before), int64(nsec)).In(loc)

	if before != after {
		// The wall clock exists with the offset before, which is the first instant if both have it. Otherwise, either
		// it exists with the offset after, or it is in the gap, and keeps the offset before.
		if zoneOffset(t) != before {
			if b := time.Unix(u-int64(after), int64(nsec)).In(loc); zoneOffset(b) == after {
				t = b
			}
		}
	}

	z.update(t)

	return t
}

// seekKey returns the first civil time, as a key, whose instant may be at or after from in loc: from's wall clock, or,
// less than a gap after clocks go forward, its wall clock with the offset from before, which wall reads those in the
// gap with. No wall clock before it is at or after from.
func seekKey(from time.Time, loc *time.Location) int64 {
	t := from.In(loc)
	offset := zoneOffset(t)

	if start, _ := t.ZoneBounds(); !start.IsZero() {
		if before := offsetAt(start.Unix()-1, loc); before < offset && from.Unix()-start.Unix() < int64(offset-before) {
			offset = before
		}
	}

	return from.Unix() + int64(offset)
}

func offsetAt(unix int64, loc *time.Location) int {
	return zoneOffset(time.Unix(unix, 0).In(loc))
}

func zoneOffset(t time.Time) int {
	_, o := t.Zone()

	return o
}

// zoneCache holds the offset of the last instant wall returned, and the bounds of when its location has it.
type zoneCache struct {
	ok         bool
	offset     int
	start, end int64
}

func (z *zoneCache) update(t time.Time) {
	_, z.offset = t.Zone()
	start, end := t.ZoneBounds()
	z.start, z.end = math.MinInt64, math.MaxInt64

	if !start.IsZero() {
		z.start = start.Unix()
	}

	if !end.IsZero() {
		z.end = end.Unix()
	}

	z.ok = true
}

func floorDiv(a, b int64) int64 {
	q := a / b
	if (a%b != 0) && ((a < 0) != (b < 0)) {
		q--
	}

	return q
}
