"""Writes testdata/dateutil.txt: random valid rules, and their occurrences as python-dateutil expands them.

Run by `make vectors`, which puts a pinned python-dateutil on sys.path. The output only changes with the seed, the
generator, or dateutil, not with the machine: a case that takes over TIMEOUT stops the generator rather than being
left out.

Each case is a DTSTART, an RRULE, the END of the window looked at, and the occurrences up to it, at most LIMIT, on a
line. Starts are floating, which TestDateutil reads in UTC: dateutil's naive datetimes have no zone either. CASES
random rules come first, then WEEKNO_CASES yearly ones with BYWEEKNO and a WKST other than Monday, which random rules
seldom have.

Rules stay where dateutil follows RFC 5545, which TestDateutil checks the library against:
- a BYDAY has no number, or all its days have one: dateutil requires a day to match both kinds, rather than either;
- no BYSECOND=60, which dateutil cannot build a time of;
- no BYWEEKNO of 52, 53, -52 or -53, WRONG_WEEKS: dateutil leaves out the days at the end of a year that are in week
  -52 or -53 of the next, as its own TODO says, and miscounts the weeks of the year before for those at its start;
- cases dateutil refuses, an interval that never reaches a BYHOUR for one, are left out.
"""

import datetime
import random
import signal
import sys

import dateutil
from dateutil.rrule import rrulestr

SEED = 5545
CASES = 600
WEEKNO_CASES = 150
LIMIT = 50
TIMEOUT = 60  # seconds per case, which none comes near

FREQS = ["YEARLY"] * 4 + ["MONTHLY"] * 4 + ["WEEKLY"] * 3 + ["DAILY"] * 3 + ["HOURLY", "MINUTELY", "SECONDLY"]
DAYS = ["MO", "TU", "WE", "TH", "FR", "SA", "SU"]
WRONG_WEEKS = {52, 53, -52, -53}
# The weeks that may cross a year's ends, whose days WKST changes the most, but WRONG_WEEKS.
EDGE_WEEKS = [1, -1]
WINDOW = {
    "YEARLY": datetime.timedelta(days=366 * 12),
    "MONTHLY": datetime.timedelta(days=366 * 4),
    "WEEKLY": datetime.timedelta(days=366 * 2),
    "DAILY": datetime.timedelta(days=366),
    "HOURLY": datetime.timedelta(days=30),
    "MINUTELY": datetime.timedelta(days=3),
    "SECONDLY": datetime.timedelta(hours=6),
}
LAYOUT = "%Y%m%dT%H%M%S"


class Timeout(Exception):
    pass


def alarm(signum, frame):
    raise Timeout()


def signed(rng, limit):
    n = rng.randint(1, limit)
    return -n if rng.random() < 0.3 else n


def values(rng, gen, most):
    vs = []
    for _ in range(rng.randint(1, most)):
        v = gen()
        if v not in vs:
            vs.append(v)
    return ",".join(str(v) for v in vs)


def departs(text):
    """Reports whether a rule has one of WRONG_WEEKS, which random rules may: it is left out after being drawn, so that
    the next are as before."""
    for part in text.split(";"):
        name, _, value = part.partition("=")
        if name == "BYWEEKNO" and WRONG_WEEKS & {int(v) for v in value.split(",")}:
            return True
    return False


def rule(rng, start, weekno=False):
    """Returns a random rule, or with weekno, a yearly one with BYWEEKNO, often of EDGE_WEEKS, and WKST not Monday."""
    freq = "YEARLY" if weekno else rng.choice(FREQS)
    parts = ["FREQ=" + freq]
    sub_daily = freq in ("HOURLY", "MINUTELY", "SECONDLY")

    bound = rng.random()
    if bound < 0.3:
        parts.append("COUNT=%d" % rng.randint(1, 40))
    elif bound < 0.5:
        until = start + rng.random() * WINDOW[freq]
        parts.append("UNTIL=" + until.strftime(LAYOUT))

    if rng.random() < 0.4:
        if sub_daily:
            interval = rng.choice([2, 3, 5, 7, 15, 25, 90])
        else:
            interval = rng.randint(2, 5)
        parts.append("INTERVAL=%d" % interval)

    by = []
    if rng.random() < 0.35:
        by.append("BYMONTH=" + values(rng, lambda: rng.randint(1, 12), 3))
    if weekno:
        weeks = values(rng, lambda: rng.choice(EDGE_WEEKS) if rng.random() < 0.5 else signed(rng, 51), 2)
        by.append("BYWEEKNO=" + weeks)
    elif freq == "YEARLY" and rng.random() < 0.2:
        by.append("BYWEEKNO=" + values(rng, lambda: signed(rng, 53), 2))
    if freq in ("YEARLY",) + ("HOURLY", "MINUTELY", "SECONDLY") and rng.random() < 0.15:
        by.append("BYYEARDAY=" + values(rng, lambda: signed(rng, 366), 3))
    if freq != "WEEKLY" and rng.random() < 0.3:
        by.append("BYMONTHDAY=" + values(rng, lambda: signed(rng, 31), 3))
    if rng.random() < 0.45:
        numbered = freq in ("MONTHLY", "YEARLY") and not any(p.startswith("BYWEEKNO") for p in by)
        if numbered and rng.random() < 0.5:
            most = 5 if freq == "MONTHLY" or any(p.startswith("BYMONTH=") for p in by) else 53
            by.append("BYDAY=" + values(rng, lambda: "%d%s" % (signed(rng, most), rng.choice(DAYS)), 3))
        else:
            by.append("BYDAY=" + values(rng, lambda: rng.choice(DAYS), 4))
    if rng.random() < (0.3 if sub_daily else 0.2):
        by.append("BYHOUR=" + values(rng, lambda: rng.randint(0, 23), 3))
    if rng.random() < (0.3 if sub_daily else 0.15):
        by.append("BYMINUTE=" + values(rng, lambda: rng.randint(0, 59), 3))
    if rng.random() < (0.3 if sub_daily else 0.1):
        by.append("BYSECOND=" + values(rng, lambda: rng.randint(0, 59), 3))
    if by and rng.random() < 0.25:
        by.append("BYSETPOS=" + values(rng, lambda: signed(rng, 5), 2))
    if weekno:
        by.append("WKST=" + rng.choice(DAYS[1:]))
    elif rng.random() < 0.3:
        by.append("WKST=" + rng.choice(DAYS))

    rng.shuffle(by)
    return freq, ";".join(parts + by)


def main():
    rng = random.Random(SEED)
    signal.signal(signal.SIGALRM, alarm)
    out = sys.stdout

    out.write("# Written by tools/vectors/gen.py with python-dateutil %s: make vectors.\n" % dateutil.__version__)

    for weekno in [False] * CASES + [True] * WEEKNO_CASES:
        start = datetime.datetime(1990, 1, 1) + datetime.timedelta(seconds=rng.randint(0, 40 * 365 * 86400))
        if rng.random() < 0.5:
            start = start.replace(minute=0, second=0)
        freq, text = rule(rng, start, weekno)
        if departs(text):
            continue
        end = start + WINDOW[freq]
        dtstart = "DTSTART:" + start.strftime(LAYOUT)

        # dateutil goes on to datetime.MAXYEAR, which it reads at each period, for a rule nothing matches. The year
        # after END is as good, and far shorter.
        datetime.MAXYEAR = end.year + 1
        signal.alarm(TIMEOUT)
        try:
            occurrences = []
            for t in rrulestr(dtstart + "\nRRULE:" + text):
                if t > end or len(occurrences) == LIMIT:
                    break
                occurrences.append(t.strftime(LAYOUT))
        except Timeout:
            sys.exit("%s\nRRULE:%s: over %ds, which leaving it out would make the output depend on the machine" %
                     (dtstart, text, TIMEOUT))
        except ValueError:
            continue
        finally:
            signal.alarm(0)

        out.write("\n%s\nRRULE:%s\nEND:%s\n%s\n" % (dtstart, text, end.strftime(LAYOUT), " ".join(occurrences)))


if __name__ == "__main__":
    main()
