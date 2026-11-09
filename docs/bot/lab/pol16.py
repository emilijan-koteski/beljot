import csv
import sys

import numpy as np


def load(path):
    rows = []
    with open(path) as f:
        for r in csv.DictReader(f):
            for k in r:
                if k not in ('variant', 'suit'):
                    r[k] = float(r[k])
            rows.append(r)
    return rows


def shape(r):
    return int(r['count']), r['J'], r['9'], r['A'], int(r['sideA']), int(r['passesBefore'])


def table_49f242f(r, hc):
    c, J, n9, A, sA, pos = shape(r)
    return bool(J and n9 or J and c >= 3 or c >= 4 and n9 or c >= 4 and A or c >= 4 and sA >= 1 or c >= 5 or
                hc and n9 and A and c == 3 and sA >= 1 or hc and J and A and c == 2 and sA >= 1 or
                hc and J and c == 2 and sA >= 2 or hc and n9 and not J and not A and c == 3 and sA >= 1 or
                hc and pos == 2 and J and c == 2 and sA >= 1 or (not hc) and pos == 1 and J and c == 2 and sA >= 1)


def row4_out(r, hc):
    c, J, n9, A, sA, pos = shape(r)
    return bool(J and n9 or J and c >= 3 or c >= 4 and n9 or c >= 4 and sA >= 1 or c >= 5 or
                hc and n9 and A and c == 3 and sA >= 1 or hc and J and A and c == 2 and sA >= 1 or
                hc and J and c == 2 and sA >= 2 or hc and n9 and not J and not A and c == 3 and sA >= 1 or
                hc and pos == 2 and J and c == 2 and sA >= 1 or (not hc) and pos == 1 and J and c == 2 and sA >= 1)


def rows478_out(r, hc):
    c, J, n9, A, sA, pos = shape(r)
    return bool(J and n9 or J and c >= 3 or c >= 4 and n9 or c >= 4 and sA >= 1 or c >= 5 or
                hc and J and c == 2 and sA >= 2 or hc and n9 and not J and not A and c == 3 and sA >= 1 or
                hc and pos == 2 and J and c == 2 and sA >= 1 or (not hc) and pos == 1 and J and c == 2 and sA >= 1)


def score(path):
    rows = load(path)
    var = rows[0]['variant']
    hc = var == 'bitola'
    free = [r for r in rows if not (var == 'croatia' and r['passesBefore'] == 3)]
    gain = np.array([r['evCall'] - r['evPass'] for r in free])
    old = np.array([table_49f242f(r, hc) for r in free])
    no4 = np.array([row4_out(r, hc) for r in free])
    no478 = np.array([rows478_out(r, hc) for r in free])
    natural = np.array([r['natural'] for r in free])
    policies = (('natural', natural), ('table at 49f242f', old), ('after the flips (row 4 out)', no4),
                ('rows 4,7,8 out (first commit)', no478))
    for name, calls in policies:
        v = gain * calls
        print(var, name, f"{v.mean():+.2f} ± {v.std(ddof=1) / np.sqrt(len(v)):.2f} call {100 * calls.mean():.1f}%")
    for name, calls in (('row4 out vs 49f242f', no4), ('rows 4,7,8 out vs 49f242f', no478)):
        d = gain * (calls.astype(float) - old)
        print(var, name, f"{d.mean():+.3f} ± {d.std(ddof=1) / np.sqrt(len(d)):.3f}",
              'rows changed', int((calls != old).sum()))


for p in sys.argv[1:]:
    score(p)
