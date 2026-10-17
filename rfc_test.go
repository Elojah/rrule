package rrule_test

import (
	"strings"
	"testing"
)

// rfcExample is an example of RFC 5545, section 3.8.5.3: its DTSTART, its RRULE, and its EXDATE.
type rfcExample struct {
	name  string
	start string
	rules []string // each gives the occurrences on its own
	extra []string // other lines
	days  []string // at 09:00, as days expands them
	times []string // or else, as "2006-01-02 15:04"
}

// rfcExamples hold all of them, with all their occurrences if each rule has a COUNT or an UNTIL, those the RFC lists
// otherwise.
var rfcExamples = []rfcExample{
	{
		name:  "Daily for 10 occurrences",
		start: "19970902T090000",
		rules: []string{"FREQ=DAILY;COUNT=10"},
		days:  []string{"1997-09 2-11"},
	},
	{
		name:  "Daily until December 24, 1997",
		start: "19970902T090000",
		rules: []string{"FREQ=DAILY;UNTIL=19971224T000000Z"},
		days:  []string{"1997-09 2-30", "1997-10 1-31", "1997-11 1-30", "1997-12 1-23"},
	},
	{
		name:  "Every other day - forever",
		start: "19970902T090000",
		rules: []string{"FREQ=DAILY;INTERVAL=2"},
		days:  []string{"1997-09 2-30/2", "1997-10 2-30/2", "1997-11 1-29/2", "1997-12 1,3"},
	},
	{
		name:  "Every 10 days, 5 occurrences",
		start: "19970902T090000",
		rules: []string{"FREQ=DAILY;INTERVAL=10;COUNT=5"},
		days:  []string{"1997-09 2,12,22", "1997-10 2,12"},
	},
	{
		name:  "Every day in January, for 3 years",
		start: "19980101T090000",
		rules: []string{
			"FREQ=YEARLY;UNTIL=20000131T140000Z;BYMONTH=1;BYDAY=SU,MO,TU,WE,TH,FR,SA",
			"FREQ=DAILY;UNTIL=20000131T140000Z;BYMONTH=1",
		},
		days: []string{"1998-01 1-31", "1999-01 1-31", "2000-01 1-31"},
	},
	{
		name:  "Weekly for 10 occurrences",
		start: "19970902T090000",
		rules: []string{"FREQ=WEEKLY;COUNT=10"},
		days:  []string{"1997-09 2,9,16,23,30", "1997-10 7,14,21,28", "1997-11 4"},
	},
	{
		name:  "Weekly until December 24, 1997",
		start: "19970902T090000",
		rules: []string{"FREQ=WEEKLY;UNTIL=19971224T000000Z"},
		days: []string{"1997-09 2,9,16,23,30", "1997-10 7,14,21,28", "1997-11 4,11,18,25",
			"1997-12 2,9,16,23"},
	},
	{
		name:  "Every other week - forever",
		start: "19970902T090000",
		rules: []string{"FREQ=WEEKLY;INTERVAL=2;WKST=SU"},
		days: []string{"1997-09 2,16,30", "1997-10 14,28", "1997-11 11,25", "1997-12 9,23",
			"1998-01 6,20", "1998-02 3,17"},
	},
	{
		name:  "Weekly on Tuesday and Thursday for five weeks",
		start: "19970902T090000",
		rules: []string{
			"FREQ=WEEKLY;UNTIL=19971007T000000Z;WKST=SU;BYDAY=TU,TH",
			"FREQ=WEEKLY;COUNT=10;WKST=SU;BYDAY=TU,TH",
		},
		days: []string{"1997-09 2,4,9,11,16,18,23,25,30", "1997-10 2"},
	},
	{
		name:  "Every other week on Monday, Wednesday, and Friday until December 24, 1997",
		start: "19970901T090000",
		rules: []string{"FREQ=WEEKLY;INTERVAL=2;UNTIL=19971224T000000Z;WKST=SU;BYDAY=MO,WE,FR"},
		days: []string{"1997-09 1,3,5,15,17,19,29", "1997-10 1,3,13,15,17,27,29,31",
			"1997-11 10,12,14,24,26,28", "1997-12 8,10,12,22"},
	},
	{
		name:  "Every other week on Tuesday and Thursday, for 8 occurrences",
		start: "19970902T090000",
		rules: []string{"FREQ=WEEKLY;INTERVAL=2;COUNT=8;WKST=SU;BYDAY=TU,TH"},
		days:  []string{"1997-09 2,4,16,18,30", "1997-10 2,14,16"},
	},
	{
		name:  "Monthly on the first Friday for 10 occurrences",
		start: "19970905T090000",
		rules: []string{"FREQ=MONTHLY;COUNT=10;BYDAY=1FR"},
		days: []string{"1997-09 5", "1997-10 3", "1997-11 7", "1997-12 5", "1998-01 2", "1998-02 6",
			"1998-03 6", "1998-04 3", "1998-05 1", "1998-06 5"},
	},
	{
		name:  "Monthly on the first Friday until December 24, 1997",
		start: "19970905T090000",
		rules: []string{"FREQ=MONTHLY;UNTIL=19971224T000000Z;BYDAY=1FR"},
		days:  []string{"1997-09 5", "1997-10 3", "1997-11 7", "1997-12 5"},
	},
	{
		name:  "Every other month on the first and last Sunday of the month for 10 occurrences",
		start: "19970907T090000",
		rules: []string{"FREQ=MONTHLY;INTERVAL=2;COUNT=10;BYDAY=1SU,-1SU"},
		days: []string{"1997-09 7,28", "1997-11 2,30", "1998-01 4,25", "1998-03 1,29",
			"1998-05 3,31"},
	},
	{
		name:  "Monthly on the second-to-last Monday of the month for 6 months",
		start: "19970922T090000",
		rules: []string{"FREQ=MONTHLY;COUNT=6;BYDAY=-2MO"},
		days: []string{"1997-09 22", "1997-10 20", "1997-11 17", "1997-12 22", "1998-01 19",
			"1998-02 16"},
	},
	{
		name:  "Monthly on the third-to-the-last day of the month, forever",
		start: "19970928T090000",
		rules: []string{"FREQ=MONTHLY;BYMONTHDAY=-3"},
		days: []string{"1997-09 28", "1997-10 29", "1997-11 28", "1997-12 29", "1998-01 29",
			"1998-02 26"},
	},
	{
		name:  "Monthly on the 2nd and 15th of the month for 10 occurrences",
		start: "19970902T090000",
		rules: []string{"FREQ=MONTHLY;COUNT=10;BYMONTHDAY=2,15"},
		days:  []string{"1997-09 2,15", "1997-10 2,15", "1997-11 2,15", "1997-12 2,15", "1998-01 2,15"},
	},
	{
		name:  "Monthly on the first and last day of the month for 10 occurrences",
		start: "19970930T090000",
		rules: []string{"FREQ=MONTHLY;COUNT=10;BYMONTHDAY=1,-1"},
		days: []string{"1997-09 30", "1997-10 1,31", "1997-11 1,30", "1997-12 1,31", "1998-01 1,31",
			"1998-02 1"},
	},
	{
		name:  "Every 18 months on the 10th thru 15th of the month for 10 occurrences",
		start: "19970910T090000",
		rules: []string{"FREQ=MONTHLY;INTERVAL=18;COUNT=10;BYMONTHDAY=10,11,12,13,14,15"},
		days:  []string{"1997-09 10-15", "1999-03 10-13"},
	},
	{
		name:  "Every Tuesday, every other month",
		start: "19970902T090000",
		rules: []string{"FREQ=MONTHLY;INTERVAL=2;BYDAY=TU"},
		days: []string{"1997-09 2,9,16,23,30", "1997-11 4,11,18,25", "1998-01 6,13,20,27",
			"1998-03 3,10,17,24,31"},
	},
	{
		name:  "Yearly in June and July for 10 occurrences",
		start: "19970610T090000",
		rules: []string{"FREQ=YEARLY;COUNT=10;BYMONTH=6,7"},
		days: []string{"1997-06 10", "1997-07 10", "1998-06 10", "1998-07 10", "1999-06 10",
			"1999-07 10", "2000-06 10", "2000-07 10", "2001-06 10", "2001-07 10"},
	},
	{
		name:  "Every other year on January, February, and March for 10 occurrences",
		start: "19970310T090000",
		rules: []string{"FREQ=YEARLY;INTERVAL=2;COUNT=10;BYMONTH=1,2,3"},
		days: []string{"1997-03 10", "1999-01 10", "1999-02 10", "1999-03 10", "2001-01 10",
			"2001-02 10", "2001-03 10", "2003-01 10", "2003-02 10", "2003-03 10"},
	},
	{
		name:  "Every third year on the 1st, 100th, and 200th day for 10 occurrences",
		start: "19970101T090000",
		rules: []string{"FREQ=YEARLY;INTERVAL=3;COUNT=10;BYYEARDAY=1,100,200"},
		days: []string{"1997-01 1", "1997-04 10", "1997-07 19", "2000-01 1", "2000-04 9", "2000-07 18",
			"2003-01 1", "2003-04 10", "2003-07 19", "2006-01 1"},
	},
	{
		name:  "Every 20th Monday of the year, forever",
		start: "19970519T090000",
		rules: []string{"FREQ=YEARLY;BYDAY=20MO"},
		days:  []string{"1997-05 19", "1998-05 18", "1999-05 17"},
	},
	{
		name:  "Monday of week number 20, forever",
		start: "19970512T090000",
		rules: []string{"FREQ=YEARLY;BYWEEKNO=20;BYDAY=MO"},
		days:  []string{"1997-05 12", "1998-05 11", "1999-05 17"},
	},
	{
		name:  "Every Thursday in March, forever",
		start: "19970313T090000",
		rules: []string{"FREQ=YEARLY;BYMONTH=3;BYDAY=TH"},
		days:  []string{"1997-03 13,20,27", "1998-03 5,12,19,26", "1999-03 4,11,18,25"},
	},
	{
		name:  "Every Thursday, but only during June, July, and August, forever",
		start: "19970605T090000",
		rules: []string{"FREQ=YEARLY;BYDAY=TH;BYMONTH=6,7,8"},
		days: []string{"1997-06 5,12,19,26", "1997-07 3,10,17,24,31", "1997-08 7,14,21,28",
			"1998-06 4,11,18,25", "1998-07 2,9,16,23,30", "1998-08 6,13,20,27",
			"1999-06 3,10,17,24", "1999-07 1,8,15,22,29", "1999-08 5,12,19,26"},
	},
	{
		name:  "Every Friday the 13th, forever",
		start: "19970902T090000",
		extra: []string{"EXDATE;TZID=America/New_York:19970902T090000"},
		rules: []string{"FREQ=MONTHLY;BYDAY=FR;BYMONTHDAY=13"},
		days:  []string{"1998-02 13", "1998-03 13", "1998-11 13", "1999-08 13", "2000-10 13"},
	},
	{
		name:  "The first Saturday that follows the first Sunday of the month, forever",
		start: "19970913T090000",
		rules: []string{"FREQ=MONTHLY;BYDAY=SA;BYMONTHDAY=7,8,9,10,11,12,13"},
		days: []string{"1997-09 13", "1997-10 11", "1997-11 8", "1997-12 13", "1998-01 10",
			"1998-02 7", "1998-03 7", "1998-04 11", "1998-05 9", "1998-06 13"},
	},
	{
		name:  "Every 4 years, the first Tuesday after a Monday in November, forever",
		start: "19961105T090000",
		rules: []string{"FREQ=YEARLY;INTERVAL=4;BYMONTH=11;BYDAY=TU;BYMONTHDAY=2,3,4,5,6,7,8"},
		days:  []string{"1996-11 5", "2000-11 7", "2004-11 2"},
	},
	{
		name:  "The third instance into the month of one of Tuesday, Wednesday, or Thursday, for the next 3 months",
		start: "19970904T090000",
		rules: []string{"FREQ=MONTHLY;COUNT=3;BYDAY=TU,WE,TH;BYSETPOS=3"},
		days:  []string{"1997-09 4", "1997-10 7", "1997-11 6"},
	},
	{
		name:  "The second-to-last weekday of the month",
		start: "19970929T090000",
		rules: []string{"FREQ=MONTHLY;BYDAY=MO,TU,WE,TH,FR;BYSETPOS=-2"},
		days: []string{"1997-09 29", "1997-10 30", "1997-11 27", "1997-12 30", "1998-01 29",
			"1998-02 26", "1998-03 30"},
	},
	{
		// The RFC lists 15:00 too, but 15:00 EDT is 19:00Z, after UNTIL: its errata 1913 and 3883.
		name:  "Every 3 hours from 9:00 AM to 5:00 PM on a specific day",
		start: "19970902T090000",
		rules: []string{"FREQ=HOURLY;INTERVAL=3;UNTIL=19970902T170000Z"},
		times: []string{"1997-09-02 09:00", "1997-09-02 12:00"},
	},
	{
		name:  "Every 15 minutes for 6 occurrences",
		start: "19970902T090000",
		rules: []string{"FREQ=MINUTELY;INTERVAL=15;COUNT=6"},
		times: []string{
			"1997-09-02 09:00", "1997-09-02 09:15", "1997-09-02 09:30", "1997-09-02 09:45",
			"1997-09-02 10:00", "1997-09-02 10:15",
		},
	},
	{
		name:  "Every hour and a half for 4 occurrences",
		start: "19970902T090000",
		rules: []string{"FREQ=MINUTELY;INTERVAL=90;COUNT=4"},
		times: []string{"1997-09-02 09:00", "1997-09-02 10:30", "1997-09-02 12:00", "1997-09-02 13:30"},
	},
	{
		name:  "Every 20 minutes from 9:00 AM to 4:40 PM every day",
		start: "19970902T090000",
		rules: []string{
			"FREQ=DAILY;BYHOUR=9,10,11,12,13,14,15,16;BYMINUTE=0,20,40",
			"FREQ=MINUTELY;INTERVAL=20;BYHOUR=9,10,11,12,13,14,15,16",
		},
		times: twentyMinutes("1997-09-02", "1997-09-03"),
	},
	{
		name:  "An example where the days generated makes a difference because of WKST",
		start: "19970805T090000",
		rules: []string{"FREQ=WEEKLY;INTERVAL=2;COUNT=4;BYDAY=TU,SU;WKST=MO"},
		days:  []string{"1997-08 5,10,19,24"},
	},
	{
		name:  "Changing only WKST from MO to SU, yields different results",
		start: "19970805T090000",
		rules: []string{"FREQ=WEEKLY;INTERVAL=2;COUNT=4;BYDAY=TU,SU;WKST=SU"},
		days:  []string{"1997-08 5,17,19,31"},
	},
	{
		name:  "An example where an invalid date (i.e., February 30) is ignored",
		start: "20070115T090000",
		rules: []string{"FREQ=MONTHLY;BYMONTHDAY=15,30;COUNT=5"},
		days:  []string{"2007-01 15,30", "2007-02 15", "2007-03 15,30"},
	},
}

// TestRFC runs each example of RFC 5545 through ParseSet.
func TestRFC(t *testing.T) {
	t.Parallel()

	ny := zone(t, "America/New_York")

	for _, e := range rfcExamples {
		for _, rule := range e.rules {
			t.Run(e.name+"/"+rule, func(t *testing.T) {
				t.Parallel()

				lines := append([]string{"DTSTART;TZID=America/New_York:" + e.start, "RRULE:" + rule}, e.extra...)
				s := parseSet(t, lines...)

				want := e.times
				if len(e.days) > 0 {
					want = days(t, "09:00", e.days...)
				}

				got := take(s.All(), len(want)+1)
				if len(got) > len(want) && (strings.Contains(rule, "COUNT") || strings.Contains(rule, "UNTIL")) {
					t.Fatalf("got more than %d: %s", len(want), got[len(want)])
				}

				got = got[:min(len(got), len(want))]
				for _, o := range got {
					if o.Location() != ny {
						t.Fatalf("%s in %s, want America/New_York", o, o.Location())
					}
				}

				equal(t, format(got, "2006-01-02 15:04"), want)
			})
		}
	}
}

// twentyMinutes returns every 20 minutes from 9:00 to 16:40 on each day.
func twentyMinutes(days ...string) []string {
	var out []string

	for _, d := range days {
		for h := 9; h <= 16; h++ {
			for _, m := range []string{"00", "20", "40"} {
				out = append(out, d+" "+pad(h)+":"+m)
			}
		}
	}

	return out
}
