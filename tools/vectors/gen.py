"""Writes testdata/dateutil.txt: random valid rules, and their occurrences as python-dateutil expands them.

Run by `make vectors`, which puts a pinned python-dateutil on sys.path. The output only changes with the seed, the
generator, or dateutil.

Each case is a DTSTART, an RRULE, the END of the window looked at, and the occurrences up to it, at most LIMIT, on a
line. Starts are floating, which TestDateutil reads in UTC: dateutil's naive datetimes have no zone either.

Rules stay where dateutil follows RFC 5545, which TestDateutil checks the library against:
- a BYDAY has no number, or all its days have one: dateutil requires a day to match both kinds, rather than either;
- no BYSECOND=60, which dateutil cannot build a time of;
- cases dateutil refuses, an interval that never reaches a BYHOUR for one, are left out, and so are those that take
  too long, which dateutil scans until 9999 when nothing matches.
"""

import datetime
import random
import signal
import sys

import dateutil
from dateutil.rrule import rrulestr

SEED = 5545
CASES = 600
LIMIT = 50
TIMEOUT = 2  # seconds per case

FREQS = ["YEARLY"] * 4 + ["MONTHLY"] * 4 + ["WEEKLY"] * 3 + ["DAILY"] * 3 + ["HOURLY", "MINUTELY", "SECONDLY"]
DAYS = ["MO", "TU", "WE", "TH", "FR", "SA", "SU"]
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


def rule(rng, start):
    freq = rng.choice(FREQS)
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
    if freq == "YEARLY" and rng.random() < 0.2:
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
    if rng.random() < 0.3:
        by.append("WKST=" + rng.choice(DAYS))

    rng.shuffle(by)
    return freq, ";".join(parts + by)


def main():
    rng = random.Random(SEED)
    signal.signal(signal.SIGALRM, alarm)
    out = sys.stdout

    out.write("# Written by tools/vectors/gen.py with python-dateutil %s: make vectors.\n" % dateutil.__version__)

    for _ in range(CASES):
        start = datetime.datetime(1990, 1, 1) + datetime.timedelta(seconds=rng.randint(0, 40 * 365 * 86400))
        if rng.random() < 0.5:
            start = start.replace(minute=0, second=0)
        freq, text = rule(rng, start)
        end = start + WINDOW[freq]
        dtstart = "DTSTART:" + start.strftime(LAYOUT)

        signal.alarm(TIMEOUT)
        try:
            occurrences = []
            for t in rrulestr(dtstart + "\nRRULE:" + text):
                if t > end or len(occurrences) == LIMIT:
                    break
                occurrences.append(t.strftime(LAYOUT))
        except (Timeout, ValueError):
            continue
        finally:
            signal.alarm(0)

        out.write("\n%s\nRRULE:%s\nEND:%s\n%s\n" % (dtstart, text, end.strftime(LAYOUT), " ".join(occurrences)))


if __name__ == "__main__":
    main()
