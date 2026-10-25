import csv, sys, numpy as np, collections
def load(path):
    rows=[]
    with open(path) as f:
        for r in csv.DictReader(f):
            for k in list(r):
                if k not in ('variant','suit'): r[k]=float(r[k])
            rows.append(r)
    return rows
def cls(r):
    j,n9,a=int(r['J']),int(r['9']),int(r['A'])
    return ('J9' if j and n9 else 'J' if j else '9' if n9 else '-')+('A' if a else '')
def ruleA(r):
    c,J,n9,A,sA=r['count'],r['J'],r['9'],r['A'],r['sideA']
    if J and n9 and c>=2: return 1
    if J and c>=3: return 1
    if c>=4 and (J or n9 or A): return 1
    if c>=5: return 1
    if n9 and A and c>=3 and sA>=1: return 1
    if J and c==2 and sA>=2: return 1
    return 0
def ruleT(r, variant):
    # proposed table: per-variant
    c,J,n9,A,sA=r['count'],r['J'],r['9'],r['A'],r['sideA']
    pos=int(r['passesBefore'])
    if J and n9 and c>=2: return 1
    if J and c>=3: return 1
    if c>=4 and (J or n9 or A): return 1
    if c>=5: return 1
    if variant=='bitola' and n9 and A and c>=3 and sA>=1: return 1
    if J and c==2 and sA>=2: return 1
    if c>=4 and sA>=1: return 1
    if variant=='croatia' and pos==1 and J and c==2 and sA>=1: return 1
    return 0
for path in sys.argv[1:]:
    rows=load(path); variant=rows[0]['variant']
    free=[r for r in rows if not (variant=='croatia' and int(r['passesBefore'])==3)]
    g=np.array([r['evCall']-r['evPass'] for r in free])
    nat=np.array([r['natural'] for r in free]); rA=np.array([ruleA(r) for r in free]); rT=np.array([ruleT(r,variant) for r in free])
    print(f"\n== {variant}: {len(free)} non-forced rows ==")
    for name,sel in (('current wantsTrump',nat),('hand rule A',rA),('proposed table',rT)):
        print(f"  {name:20s} value/row {(g*sel).mean():+.2f}  call rate {100*sel.mean():.1f}%")
    by=collections.defaultdict(list)
    for r in free: by[(cls(r),int(r['count']),min(int(r['sideA']),2))].append(r['evCall']-r['evPass'])
    print("  class trumps sideA      n   gain    SE")
    for k in sorted(by,key=lambda k:(k[1],k[0],k[2])):
        v=np.array(by[k])
        if len(v)<150: continue
        print(f"  {k[0]:5s} {k[1]:6d} {k[2]:5d} {len(v):6d} {v.mean():+6.1f} {v.std(ddof=1)/np.sqrt(len(v)):5.1f}")
    # specific checks
    def cell(cond,label):
        v=np.array([r['evCall']-r['evPass'] for r in free if cond(r)])
        print(f"  {label:60s} n={len(v):5d} gain {v.mean():+6.1f} ± {v.std(ddof=1)/np.sqrt(len(v)):4.1f}")
    cell(lambda r: r['count']==4 and not r['J'] and not r['9'] and not r['A'] and r['sideA']==0, "four trumps, no J/9/A, no side Ace")
    cell(lambda r: r['count']==4 and not r['J'] and not r['9'] and not r['A'] and r['sideA']>=1, "four trumps, no J/9/A, side Ace")
    cell(lambda r: r['count']==3 and r['9'] and r['A'] and not r['J'] and r['sideA']>=1, "9+A+x, side Ace")
    cell(lambda r: r['count']==3 and r['J'] and not r['9'] and not r['A'] and r['8'] and r['7'] and r['sideA']==0, "J+8+7, no side Ace")
    cell(lambda r: r['count']==2 and r['J'] and r['9'] and r['sideA']==0, "J9 doubleton, no side Ace")
    cell(lambda r: r['count']==2 and r['J'] and not r['9'] and r['sideA']==0 and r['sideT']==0 and r['sideK']==0, "J+x doubleton, no side honours")
    cell(lambda r: r['count']==2 and r['J'] and not r['9'] and r['sideA']==1, "J+x doubleton, one side Ace")
    cell(lambda r: r['count']==3 and not r['J'] and not r['9'] and r['A'] and r['sideA']>=1, "A+x+x, side Ace")
