package rrule_test

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/elojah/rrule"
)

// TestParse checks rules that Parse reads, and that String writes them back in the grammar's order.
func TestParse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want rrule.Rule
		out  string
	}{
		{
			in:   "FREQ=DAILY",
			want: rrule.Rule{Freq: rrule.Daily},
		},
		{
			in: "RRULE:freq=monthly;byday=mo,+2tu,-1fr;wkst=su",
			want: rrule.Rule{Freq: rrule.Monthly, ByDay: []rrule.WeekdayNum{
				{Weekday: rrule.Monday}, rrule.Tuesday.Nth(2), rrule.Friday.Nth(-1),
			}, WeekStart: rrule.Sunday},
			out: "FREQ=MONTHLY;BYDAY=MO,2TU,-1FR;WKST=SU",
		},
		{
			in: "WKST=MO;BYSETPOS=1,-1;BYMONTH=1,12;BYWEEKNO=-53,1;BYYEARDAY=366,-1;BYMONTHDAY=01,-31;BYDAY=SU;" +
				"BYHOUR=0,23;BYMINUTE=0,59;BYSECOND=0,60;INTERVAL=2;COUNT=10;FREQ=YEARLY",
			want: rrule.Rule{
				Freq: rrule.Yearly, Count: 10, Interval: 2, BySecond: []int{0, 60}, ByMinute: []int{0, 59},
				ByHour: []int{0, 23}, ByDay: []rrule.WeekdayNum{{Weekday: rrule.Sunday}}, ByMonthDay: []int{1, -31},
				ByYearDay: []int{366, -1}, ByWeekNo: []int{-53, 1}, ByMonth: []time.Month{time.January, time.December},
				BySetPos: []int{1, -1},
			},
			out: "FREQ=YEARLY;COUNT=10;INTERVAL=2;BYSECOND=0,60;BYMINUTE=0,59;BYHOUR=0,23;BYDAY=SU;" +
				"BYMONTHDAY=1,-31;BYYEARDAY=366,-1;BYWEEKNO=-53,1;BYMONTH=1,12;BYSETPOS=1,-1",
		},
		{
			in:   "FREQ=MONTHLY;UNTIL=20261231T235959Z",
			want: rrule.Rule{Freq: rrule.Monthly, Until: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
		},
		{
			in:   "FREQ=MONTHLY;UNTIL=20261231T235959",
			want: rrule.Rule{Freq: rrule.Monthly, Until: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
			out:  "FREQ=MONTHLY;UNTIL=20261231T235959Z",
		},
		{
			in:   "FREQ=MONTHLY;UNTIL=20261231",
			want: rrule.Rule{Freq: rrule.Monthly, Until: time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)},
			out:  "FREQ=MONTHLY;UNTIL=20261231T000000Z",
		},
		{
			in:   "FREQ=HOURLY;INTERVAL=1",
			want: rrule.Rule{Freq: rrule.Hourly, Interval: 1},
		},
		{
			in:   "FREQ=SECONDLY;X-FREQ=PT0.5S",
			want: rrule.Rule{Freq: rrule.Secondly, XFreq: 500 * time.Millisecond},
		},
		{
			in:   "freq=hourly;interval=2;x-freq=pt120m",
			want: rrule.Rule{Freq: rrule.Hourly, Interval: 2, XFreq: 2 * time.Hour},
			out:  "FREQ=HOURLY;X-FREQ=PT2H;INTERVAL=2",
		},
		{
			in:   "FREQ=MINUTELY;X-FREQ=PT1,5M",
			want: rrule.Rule{Freq: rrule.Minutely, XFreq: 90 * time.Second},
			out:  "FREQ=MINUTELY;X-FREQ=PT1M30S",
		},
		{
			in:   "FREQ=SECONDLY;X-FREQ=PT1H0.000000001S",
			want: rrule.Rule{Freq: rrule.Secondly, XFreq: time.Hour + 1},
		},
		{
			in:   "FREQ=YEARLY;X-FREQ=PT0.25H;BYWEEKNO=1",
			want: rrule.Rule{Freq: rrule.Yearly, XFreq: 15 * time.Minute, ByWeekNo: []int{1}},
			out:  "FREQ=YEARLY;X-FREQ=PT15M;BYWEEKNO=1",
		},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			t.Parallel()

			r, err := rrule.Parse(c.in)
			if err != nil {
				t.Fatal(err)
			}

			out := c.out
			if out == "" {
				out = c.in
			}

			if s := r.String(); s != out {
				t.Fatalf("String %s, want %s", s, out)
			}

			if !r.Until.Equal(c.want.Until) {
				t.Fatalf("UNTIL %s, want %s", r.Until, c.want.Until)
			}

			r.Until, c.want.Until = time.Time{}, time.Time{}
			if !reflect.DeepEqual(r, c.want) {
				t.Fatalf("got %+v, want %+v", r, c.want)
			}
		})
	}
}

// TestParse_Location checks that floating UNTILs and dates are read in WithLocation's location.
func TestParse_Location(t *testing.T) {
	t.Parallel()

	paris := zone(t, "Europe/Paris")

	for in, want := range map[string]time.Time{
		"FREQ=DAILY;UNTIL=20260701T120000":  time.Date(2026, 7, 1, 12, 0, 0, 0, paris),
		"FREQ=DAILY;UNTIL=20260701":         time.Date(2026, 7, 1, 0, 0, 0, 0, paris),
		"FREQ=DAILY;UNTIL=20260701T120000Z": time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
	} {
		r, err := rrule.Parse(in, rrule.WithLocation(paris))
		if err != nil {
			t.Fatal(err)
		}

		if !r.Until.Equal(want) {
			t.Fatalf("%s: UNTIL %s, want %s", in, r.Until, want)
		}
	}
}

// TestParse_Errors checks what Parse refuses, and which error it says it is.
func TestParse_Errors(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]error{
		"":                                                rrule.ErrSyntax,
		"RRULE:":                                          rrule.ErrSyntax,
		"FREQ=DAILY;":                                     rrule.ErrSyntax,
		"FREQ=DAILY;;COUNT=1":                             rrule.ErrSyntax,
		"FREQ":                                            rrule.ErrSyntax,
		"FREQ=":                                           rrule.ErrSyntax,
		"FREQ=FORTNIGHTLY":                                rrule.ErrSyntax,
		"FREQ=DAILY;FREQ=DAILY":                           rrule.ErrSyntax,
		"FREQ=DAILY;FOO=1":                                rrule.ErrSyntax,
		"FREQ=DAILY;COUNT=-1":                             rrule.ErrSyntax,
		"FREQ=DAILY;COUNT=+1":                             rrule.ErrSyntax,
		"FREQ=DAILY;COUNT=1234567890":                     rrule.ErrSyntax,
		"FREQ=DAILY;BYHOUR=1,,2":                          rrule.ErrSyntax,
		"FREQ=DAILY;BYHOUR=123":                           rrule.ErrSyntax,
		"FREQ=DAILY;BYHOUR=-1":                            rrule.ErrSyntax,
		"FREQ=DAILY;BYYEARDAY=1234":                       rrule.ErrSyntax,
		"FREQ=MONTHLY;BYDAY=XX":                           rrule.ErrSyntax,
		"FREQ=MONTHLY;BYDAY=0MO":                          rrule.ErrSyntax,
		"FREQ=MONTHLY;BYDAY=100MO":                        rrule.ErrSyntax,
		"FREQ=MONTHLY;BYDAY=M":                            rrule.ErrSyntax,
		"FREQ=DAILY;WKST=XX":                              rrule.ErrSyntax,
		"FREQ=DAILY;UNTIL=2026":                           rrule.ErrSyntax,
		"FREQ=DAILY;UNTIL=20260101X000000":                rrule.ErrSyntax,
		"FREQ=DAILY;UNTIL=2026010aT000000":                rrule.ErrSyntax,
		"COUNT=1":                                         rrule.ErrInvalid,
		"FREQ=DAILY;COUNT=0":                              rrule.ErrInvalid,
		"FREQ=DAILY;INTERVAL=0":                           rrule.ErrInvalid,
		"FREQ=DAILY;COUNT=1;UNTIL=20260101":               rrule.ErrInvalid,
		"FREQ=DAILY;UNTIL=20260230":                       rrule.ErrInvalid,
		"FREQ=DAILY;UNTIL=20260101T240000":                rrule.ErrInvalid,
		"FREQ=DAILY;BYSECOND=61":                          rrule.ErrInvalid,
		"FREQ=DAILY;BYMINUTE=60":                          rrule.ErrInvalid,
		"FREQ=DAILY;BYHOUR=24":                            rrule.ErrInvalid,
		"FREQ=DAILY;BYMONTHDAY=0":                         rrule.ErrInvalid,
		"FREQ=DAILY;BYMONTHDAY=32":                        rrule.ErrInvalid,
		"FREQ=YEARLY;BYYEARDAY=367":                       rrule.ErrInvalid,
		"FREQ=YEARLY;BYWEEKNO=54":                         rrule.ErrInvalid,
		"FREQ=YEARLY;BYMONTH=13":                          rrule.ErrInvalid,
		"FREQ=YEARLY;BYMONTH=0":                           rrule.ErrInvalid,
		"FREQ=YEARLY;BYDAY=54MO":                          rrule.ErrInvalid,
		"FREQ=YEARLY;BYMONTH=1;BYSETPOS=0":                rrule.ErrInvalid,
		"FREQ=YEARLY;BYMONTH=1;BYSETPOS=367":              rrule.ErrInvalid,
		"FREQ=DAILY;BYSETPOS=1":                           rrule.ErrInvalid,
		"FREQ=WEEKLY;BYMONTHDAY=1":                        rrule.ErrInvalid,
		"FREQ=MONTHLY;BYYEARDAY=1":                        rrule.ErrInvalid,
		"FREQ=DAILY;BYYEARDAY=1":                          rrule.ErrInvalid,
		"FREQ=MONTHLY;BYWEEKNO=1":                         rrule.ErrInvalid,
		"FREQ=WEEKLY;BYDAY=1MO":                           rrule.ErrInvalid,
		"FREQ=YEARLY;BYWEEKNO=1;BYDAY=1MO":                rrule.ErrInvalid,
		"FREQ=DAILY;UNTIL=20260101T000060":                rrule.ErrUnsupported,
		"FREQ=DAILY;RSCALE=GREGORIAN":                     rrule.ErrUnsupported,
		"FREQ=MONTHLY;BYMONTHDAY=31;SKIP=OMIT":            rrule.ErrUnsupported,
		"FREQ=SECONDLY;X-FREQ=1S":                         rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=P":                          rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT":                         rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PTS":                        rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT1":                        rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT.5S":                      rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT1.S":                      rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT1.5H30M":                  rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT1S1M":                     rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT1H1H":                     rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT-1S":                      rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=R/PT1S":                     rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FREQ=PT1S;X-FREQ=PT1S":           rrule.ErrSyntax,
		"FREQ=SECONDLY;X-FOO=1":                           rrule.ErrSyntax,
		"X-FREQ=PT1S":                                     rrule.ErrInvalid,
		"FREQ=SECONDLY;X-FREQ=PT0S":                       rrule.ErrInvalid,
		"FREQ=SECONDLY;X-FREQ=PT0.0S":                     rrule.ErrInvalid,
		"FREQ=SECONDLY;X-FREQ=P1D":                        rrule.ErrUnsupported,
		"FREQ=SECONDLY;X-FREQ=P1W":                        rrule.ErrUnsupported,
		"FREQ=SECONDLY;X-FREQ=P1M":                        rrule.ErrUnsupported,
		"FREQ=SECONDLY;X-FREQ=P1DT1H":                     rrule.ErrUnsupported,
		"FREQ=SECONDLY;X-FREQ=PT0.0000000001S":            rrule.ErrUnsupported,
		"FREQ=SECONDLY;X-FREQ=PT2562048H":                 rrule.ErrUnsupported,
		"FREQ=SECONDLY;X-FREQ=PT2562047H47M16.854775808S": rrule.ErrUnsupported,
	} {
		_, err := rrule.Parse(in)
		if !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", in, err, want)
		}

		if pe := (*rrule.PartError)(nil); err != nil && !errors.As(err, &pe) {
			t.Errorf("%q: %T, want a *rrule.PartError", in, err)
		}
	}
}

// TestPartError checks how errors read.
func TestPartError(t *testing.T) {
	t.Parallel()

	for in, want := range map[string]string{
		"FREQ=DAILY;BYMONTHDAY=32":   "rrule: invalid, BYMONTHDAY=32",
		"FREQ=WEEKLY;BYMONTHDAY=1":   "rrule: invalid, BYMONTHDAY: not with FREQ=WEEKLY",
		"FREQ=DAILY;BYDAY=1MO":       "rrule: invalid, BYDAY=1MO: a number only with FREQ=MONTHLY or YEARLY",
		"COUNT=2":                    "rrule: invalid, FREQ: missing",
		"FREQ=DAILY;RSCALE=HEBREW":   "rrule: unsupported, RSCALE=HEBREW",
		"FREQ=DAILY;COUNT=1;COUNT=2": "rrule: syntax error, COUNT=2: more than once",
		"FREQ=HOURLY;X-FREQ=P1D":     "rrule: unsupported, X-FREQ=P1D: only hours, minutes and seconds",
	} {
		_, err := rrule.Parse(in)
		if err == nil || err.Error() != want {
			t.Errorf("%q: %v, want %s", in, err, want)
		}
	}
}

// TestRule_Validate checks what Validate refuses in a rule built in Go, which Parse could not make.
func TestRule_Validate(t *testing.T) {
	t.Parallel()

	for _, r := range []rrule.Rule{
		{},
		{Freq: rrule.Yearly + 1},
		{Freq: rrule.Daily, Interval: -1},
		{Freq: rrule.Secondly, XFreq: -time.Nanosecond},
		{Freq: rrule.Daily, Count: -1},
		{Freq: rrule.Daily, WeekStart: rrule.Sunday + 1},
		{Freq: rrule.Daily, BySecond: []int{-1}},
		{Freq: rrule.Monthly, ByDay: []rrule.WeekdayNum{{Weekday: -1}}},
		{Freq: rrule.Monthly, ByDay: []rrule.WeekdayNum{rrule.Monday.Nth(-54)}},
	} {
		if err := r.Validate(); !errors.Is(err, rrule.ErrInvalid) {
			t.Errorf("%+v: %v, want %v", r, err, rrule.ErrInvalid)
		}
	}
}

// TestRule_Text checks that a rule goes through JSON as its text, and that MarshalText refuses an invalid rule.
func TestRule_Text(t *testing.T) {
	t.Parallel()

	type event struct {
		Rule rrule.Rule `json:"rule"`
	}

	in := event{Rule: rrule.Rule{Freq: rrule.Monthly, ByDay: []rrule.WeekdayNum{rrule.Friday.Nth(-1)}}}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	if string(b) != `{"rule":"FREQ=MONTHLY;BYDAY=-1FR"}` {
		t.Fatalf("got %s", b)
	}

	var out event
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(in, out) {
		t.Fatalf("got %+v, want %+v", out, in)
	}

	if err := json.Unmarshal([]byte(`{"rule":"FREQ=NEVER"}`), &out); !errors.Is(err, rrule.ErrSyntax) {
		t.Fatalf("got %v, want %v", err, rrule.ErrSyntax)
	}

	if _, err := json.Marshal(event{}); !errors.Is(err, rrule.ErrInvalid) {
		t.Fatalf("got %v, want %v", err, rrule.ErrInvalid)
	}
}

// TestStrings checks how the enums print.
func TestStrings(t *testing.T) {
	t.Parallel()

	for got, want := range map[string]string{
		rrule.Secondly.String():       "SECONDLY",
		rrule.Frequency(0).String():   "Frequency(0)",
		rrule.Sunday.String():         "SU",
		rrule.Weekday(7).String():     "Weekday(7)",
		rrule.Friday.Nth(-1).String(): "-1FR",
		rrule.Monday.Nth(0).String():  "MO",
		rrule.Rule{Freq: 9}.String():  "FREQ=Frequency(9)",

		rrule.Rule{Freq: rrule.Secondly, XFreq: -1}.String(): "FREQ=SECONDLY;X-FREQ=-1ns",
	} {
		if got != want {
			t.Errorf("got %s, want %s", got, want)
		}
	}
}

// TestWeekdayOf checks the conversions from and to time.Weekday by the names RFC 5545 writes, the first two letters
// of time's, and that a weekday out of range stays so.
func TestWeekdayOf(t *testing.T) {
	t.Parallel()

	for d := time.Sunday; d <= time.Saturday; d++ {
		w := rrule.WeekdayOf(d)
		if want := strings.ToUpper(d.String()[:2]); w.String() != want {
			t.Errorf("%s: %s, want %s", d, w, want)
		}

		if back := w.Time(); back != d {
			t.Errorf("%s: %s, back to %s", d, w, back)
		}
	}

	for _, d := range []time.Weekday{-1, 7} {
		if w := rrule.WeekdayOf(d); w != -1 {
			t.Errorf("%d: %s, want -1", d, w)
		}
	}

	for _, w := range []rrule.Weekday{-1, 7} {
		if d := w.Time(); d != -1 {
			t.Errorf("%s: %d, want -1", w, d)
		}
	}
}
