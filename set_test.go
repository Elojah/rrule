package rrule_test

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/elojah/rrule"
)

// TestParseSet reads a set with every property, CRLF, a folded line, and a quoted TZID.
func TestParseSet(t *testing.T) {
	t.Parallel()

	s, err := rrule.ParseSet(strings.Join([]string{
		`DTSTART;TZID="Europe/Paris";VALUE=DATE-TIME:20260105T090000`, // a Monday
		"RRULE:FREQ=WEEKLY;BYDAY=MO,",
		" WE;COUNT=6",
		"rrule:FREQ=MONTHLY;BYMONTHDAY=15;COUNT=2",
		"EXRULE:FREQ=WEEKLY;BYDAY=WE;COUNT=1",         // January 7
		"EXDATE;TZID=Europe/Paris:20260112T090000",    // a Monday
		"RDATE;TZID=America/New_York:20260110T030000", // 09:00 in Paris
		"RDATE:20260105T080000Z,20260105T080000Z",     // Start
		"",
	}, "\r\n"), rrule.WithLoader(loadZone))
	if err != nil {
		t.Fatal(err)
	}

	paris := zone(t, "Europe/Paris")
	if s.Start.Location() != paris || s.Form != rrule.Zoned {
		t.Fatalf("start %s, form %d", s.Start, s.Form)
	}

	got := take(s.All(), -1)
	for _, o := range got {
		if o.Location() != paris {
			t.Fatalf("%s in %s", o, o.Location())
		}
	}

	equal(t, format(got, "2006-01-02 15:04"), days(t, "09:00", "2026-01 5,10,14,15,19,21", "2026-02 15"))
}

// TestSet_String checks that each Form writes back what ParseSet read.
func TestSet_String(t *testing.T) {
	t.Parallel()

	paris := zone(t, "Europe/Paris")

	cases := []struct {
		name  string
		in    []string
		opts  []rrule.Option
		form  rrule.Form
		want  []string // as time.DateTime, in Start's location
		where *time.Location
	}{
		{
			name: "UTC",
			in:   []string{"DTSTART:20260105T090000Z", "RRULE:FREQ=DAILY;COUNT=2", "RDATE:20260110T090000Z"},
			opts: []rrule.Option{rrule.WithLocation(paris)}, // for floating times, which there are none of
			form: rrule.Zoned,
			want: []string{"2026-01-05 09:00:00", "2026-01-06 09:00:00", "2026-01-10 09:00:00"},
		},
		{
			name: "TZID",
			in: []string{
				"DTSTART;TZID=Europe/Paris:20260105T090000", "RRULE:FREQ=DAILY;UNTIL=20260107T080000Z",
				"EXDATE;TZID=Europe/Paris:20260106T090000",
			},
			form:  rrule.Zoned,
			want:  []string{"2026-01-05 09:00:00", "2026-01-07 09:00:00"},
			where: paris,
		},
		{
			name: "floating",
			in: []string{
				"DTSTART:20260105T090000", "RRULE:FREQ=DAILY;UNTIL=20260106T090000",
				"EXRULE:FREQ=DAILY;COUNT=1",
			},
			opts:  []rrule.Option{rrule.WithLocation(paris)},
			form:  rrule.Floating,
			want:  []string{"2026-01-06 09:00:00"},
			where: paris,
		},
		{
			name: "dates",
			in: []string{
				"DTSTART;VALUE=DATE:20260101", "RRULE:FREQ=YEARLY;UNTIL=20280101",
				"RDATE;VALUE=DATE:20260704", "EXDATE;VALUE=DATE:20270101",
			},
			opts:  []rrule.Option{rrule.WithLocation(paris)},
			form:  rrule.Date,
			want:  []string{"2026-01-01 00:00:00", "2026-07-04 00:00:00", "2028-01-01 00:00:00"},
			where: paris,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			opts := append([]rrule.Option{rrule.WithLoader(loadZone)}, c.opts...)

			s, err := rrule.ParseSet(strings.Join(c.in, "\n"), opts...)
			if err != nil {
				t.Fatal(err)
			}

			if s.Form != c.form {
				t.Fatalf("form %d, want %d", s.Form, c.form)
			}

			if where := c.where; where != nil && s.Start.Location() != where {
				t.Fatalf("start in %s, want %s", s.Start.Location(), where)
			}

			equal(t, format(take(s.All(), -1), time.DateTime), c.want)

			if got := s.String(); got != strings.Join(c.in, "\n") {
				t.Fatalf("String:\n%s\nwant:\n%s", got, strings.Join(c.in, "\n"))
			}
		})
	}

	fixed := rrule.Set{Start: time.Date(2026, 1, 5, 9, 0, 0, 0, time.FixedZone("UTC+01:00", 3600))}
	if got := fixed.String(); got != `DTSTART;TZID="UTC+01:00":20260105T090000` {
		t.Fatalf("got %s", got)
	}
}

// TestParseSet_Errors checks what ParseSet refuses, which error it says it is, and on which line.
func TestParseSet_Errors(t *testing.T) {
	t.Parallel()

	const start = "DTSTART:20260101T000000Z\n"

	cases := []struct {
		in   string
		want error
		line int // 0 for none
	}{
		{"", rrule.ErrInvalid, 0},
		{"RRULE:FREQ=DAILY", rrule.ErrInvalid, 0},
		{start + "DTSTART:20260102T000000Z", rrule.ErrInvalid, 2},
		{start + "SUMMARY:Standup", rrule.ErrUnsupported, 2},
		{"DTSTART;X-FOO=1:20260101T000000Z", rrule.ErrUnsupported, 1},
		{"DTSTART;TZID=Europe/Paris:20260101T000000Z", rrule.ErrInvalid, 1},
		{"DTSTART;TZID=Mars/Olympus_Mons:20260101T000000", errUnknownZone, 1},
		{"DTSTART;VALUE=DATE:20260101T000000", rrule.ErrInvalid, 1},
		{"DTSTART:20260101", rrule.ErrInvalid, 1},
		{"DTSTART;VALUE=DATE;TZID=Europe/Paris:20260101", rrule.ErrInvalid, 1},
		{"DTSTART;VALUE=TEXT:x", rrule.ErrInvalid, 1},
		{"DTSTART:20260101T000000Z,20260102T000000Z", rrule.ErrInvalid, 1},
		{"DTSTART:2026", rrule.ErrSyntax, 1},
		{"DTSTART;VALUE=DATE:20260101\nRRULE:FREQ=DAILY;UNTIL=20260105T000000Z", rrule.ErrInvalid, 2},
		{start + "RRULE:FREQ=DAILY;UNTIL=20260105", rrule.ErrInvalid, 2},
		{start + "RDATE;VALUE=PERIOD:20260101T000000Z/PT1H", rrule.ErrUnsupported, 2},
		{start + "EXDATE;VALUE=PERIOD:20260101T000000Z/PT1H", rrule.ErrInvalid, 2},
		{start + "EXDATE;VALUE=DATE:20260101", rrule.ErrInvalid, 2},
		{"DTSTART;VALUE=DATE:20260101\nRDATE:20260102T000000Z", rrule.ErrInvalid, 2},
		{"DTSTART;VALUE=DATE:20260101\nRRULE:FREQ=DAILY;BYHOUR=1", rrule.ErrInvalid, 0},
		{start + "RRULE;X-FOO=1:FREQ=DAILY", rrule.ErrUnsupported, 2},
		{start + "RDATE;X-FOO=1:20260101T000000Z", rrule.ErrUnsupported, 2},
		{start + "RRULE:RRULE:FREQ=DAILY", rrule.ErrSyntax, 2},
		{start + "\nRRULE:FREQ=DAILY;", rrule.ErrSyntax, 3},
		{start + "RRULE:FREQ=\n DAILY;BYHOUR=99\nRRULE:FREQ=DAILY", rrule.ErrInvalid, 2},
		{" DTSTART:20260101T000000Z", rrule.ErrSyntax, 1},
		{"DTSTART", rrule.ErrSyntax, 1},
		{":20260101T000000Z", rrule.ErrSyntax, 1},
		{"DTSTART;TZID:x", rrule.ErrSyntax, 1},
		{"DTSTART;TZID=", rrule.ErrSyntax, 1},
		{"DTSTART;TZID=x", rrule.ErrSyntax, 1},
		{"DTSTART;TZID=x;", rrule.ErrSyntax, 1},
		{`DTSTART;TZID="Europe/Paris:20260101T000000`, rrule.ErrSyntax, 1},
		{`DTSTART;TZID="Europe/Paris"x:20260101T000000`, rrule.ErrSyntax, 1},
		{`DTSTART;TZID="Europe/Paris"`, rrule.ErrSyntax, 1},
		{"DTSTART;TZID=a;TZID=b:20260101T000000", rrule.ErrSyntax, 1},
	}

	for _, c := range cases {
		_, err := rrule.ParseSet(c.in, rrule.WithLoader(loadZone))
		if !errors.Is(err, c.want) {
			t.Errorf("%q: %v, want %v", c.in, err, c.want)

			continue
		}

		line := 0
		if le := (*rrule.LineError)(nil); errors.As(err, &le) {
			line = le.Line
		}

		if line != c.line {
			t.Errorf("%q: %v, on line %d, want %d", c.in, err, line, c.line)
		}
	}

	_, err := rrule.ParseSet(start+"RRULE:FREQ=DAILY;BYDAY=1MO", rrule.WithLoader(loadZone))
	if want := "rrule: invalid, BYDAY=1MO: a number only with FREQ=MONTHLY or YEARLY, on line 2"; err == nil ||
		err.Error() != want {
		t.Errorf("got %v, want %s", err, want)
	}
}

// TestSet checks a Set built in Go: All, Between, After, and that it leaves its dates as they are.
func TestSet(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC)
	day := func(d int) time.Time { return start.AddDate(0, 0, d) }

	s := rrule.Set{
		Start:   start,
		RRules:  []rrule.Rule{{Freq: rrule.Daily, Count: 5}, {Freq: rrule.Weekly, Count: 3}},
		RDates:  []time.Time{day(30), day(1), day(30), day(20).In(zone(t, "America/New_York"))},
		ExDates: []time.Time{day(2), day(-1), day(14)},
		ExRules: []rrule.Rule{{Freq: rrule.Daily, Interval: 4, Count: 2}}, // start and day 4
	}
	rdates := slices.Clone(s.RDates)

	want := []time.Time{day(1), day(3), day(7), day(20), day(30)}

	got := take(s.All(), -1)
	if !slices.EqualFunc(got, want, time.Time.Equal) {
		t.Fatalf("got %s, want %s", got, want)
	}

	if got[3].Location() != time.UTC {
		t.Fatalf("%s in %s, want UTC", got[3], got[3].Location())
	}

	if !slices.Equal(s.RDates, rdates) {
		t.Fatalf("RDates %s, was %s", s.RDates, rdates)
	}

	if got := take(s.Between(day(3), day(20)), -1); !slices.EqualFunc(got, want[1:3], time.Time.Equal) {
		t.Fatalf("Between: got %s, want %s", got, want[1:3])
	}

	if got := take(s.Between(day(3), day(3)), -1); len(got) != 0 {
		t.Fatalf("Between: got %s, want none", got)
	}

	if got, ok := s.After(day(3)); !ok || !got.Equal(day(7)) {
		t.Fatalf("After: got %s, %t, want %s", got, ok, day(7))
	}

	if got, ok := s.After(day(30)); ok {
		t.Fatalf("After: got %s, want none", got)
	}

	for range s.All() {
		break // stops the merge early
	}

	// Every second is taken away: All would go on to 9999, but Between ends.
	none := rrule.Set{Start: start, RRules: []rrule.Rule{{Freq: rrule.Secondly}}, ExRules: []rrule.Rule{{Freq: rrule.Secondly}}}
	if got := take(none.Between(start.Add(-time.Hour), start.Add(time.Hour)), -1); len(got) != 0 {
		t.Fatalf("Between: got %s, want none", got)
	}
}

// TestSet_Validate checks what Validate refuses in a set built in Go, and that All yields nothing for it.
func TestSet_Validate(t *testing.T) {
	t.Parallel()

	midnight := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)

	for _, s := range []rrule.Set{
		{},
		{Start: midnight, Form: rrule.Date + 1},
		{Start: midnight, RRules: []rrule.Rule{{}}},
		{Start: midnight, ExRules: []rrule.Rule{{Freq: rrule.Daily, Count: -1}}},
		{Start: midnight.Add(time.Hour), Form: rrule.Date},
		{Start: midnight, Form: rrule.Date, RDates: []time.Time{midnight.Add(time.Nanosecond)}},
		{Start: midnight, Form: rrule.Date, ExDates: []time.Time{time.Date(2026, 1, 5, 0, 0, 0, 0, zone(t, "Europe/Paris"))}},
		{Start: midnight, Form: rrule.Date, RRules: []rrule.Rule{{Freq: rrule.Daily, ByMinute: []int{1}}}},
	} {
		if err := s.Validate(); !errors.Is(err, rrule.ErrInvalid) {
			t.Errorf("%+v: %v, want %v", s, err, rrule.ErrInvalid)
		}

		if got := take(s.All(), -1); len(got) != 0 {
			t.Errorf("%+v: got %s", s, got)
		}
	}

	if err := (rrule.Set{Start: midnight, Form: rrule.Date}).Validate(); err != nil {
		t.Errorf("got %v", err)
	}
}

// TestSet_Text checks that a set goes through JSON as its text.
func TestSet_Text(t *testing.T) {
	t.Parallel()

	in := rrule.Set{
		Start:  time.Date(2026, 1, 5, 9, 0, 0, 0, time.UTC),
		RRules: []rrule.Rule{{Freq: rrule.Weekly, Until: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)}},
	}

	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}

	if want := `"DTSTART:20260105T090000Z\nRRULE:FREQ=WEEKLY;UNTIL=20260201T000000Z"`; string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}

	var out rrule.Set
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}

	if got, want := take(out.All(), -1), take(in.All(), -1); !slices.EqualFunc(got, want, time.Time.Equal) {
		t.Fatalf("got %s, want %s", got, want)
	}

	if err := json.Unmarshal([]byte(`"RRULE:FREQ=DAILY"`), &out); !errors.Is(err, rrule.ErrInvalid) {
		t.Fatalf("got %v, want %v", err, rrule.ErrInvalid)
	}

	if _, err := json.Marshal(rrule.Set{}); !errors.Is(err, rrule.ErrInvalid) {
		t.Fatalf("got %v, want %v", err, rrule.ErrInvalid)
	}
}
