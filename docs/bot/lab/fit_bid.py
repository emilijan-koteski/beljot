import csv, sys, numpy as np
path=sys.argv[1]
rows=[]
with open(path) as f:
    for r in csv.DictReader(f):
        rows.append(r)
def F(r):
    g=lambda k: float(r[k])
    J,n9,A=g('J'),g('9'),g('A')
    pos=int(g('passesBefore')); rnd=int(g('round'))
    return [g('count'),J,n9,A,g('T'),g('K'),g('Q'),g('8'),g('7'),J*n9,J*A,n9*A,g('sideA'),g('sideT'),g('sideK'),
            1.0 if rnd==2 else 0.0, 1.0 if pos==1 else 0.0, 1.0 if pos==2 else 0.0, 1.0 if pos==3 else 0.0, 1.0]
names=['count','J','9','A','T','K','Q','8','7','J*9','J*A','9*A','sideA','sideT','sideK','round2','pos1','pos2','pos3','const']
X=np.array([F(r) for r in rows]); 
call=np.array([float(r['evCall']) for r in rows]); pas=np.array([float(r['evPass']) for r in rows]); nat=np.array([float(r['natural']) for r in rows])
gain=call-pas
for target,y in (('EV(call)',call),('gain=EV(call)-EV(pass)',gain)):
    coef=np.linalg.lstsq(X,y,rcond=None)[0]; pred=X@coef
    print(f"\n{path} fit of {target}: R^2={1-((y-pred)**2).sum()/((y-y.mean())**2).sum():.3f}")
    print("  "+"  ".join(f"{n}={c:+.1f}" for n,c in zip(names,coef)))
    print("  threshold policy (call iff pred>th):")
    for th in (-20,-10,0,10,20,30):
        sel=pred>th
        print(f"    th={th:+3d}: call rate {100*sel.mean():5.1f}%  per-row value {(gain*sel).mean():+6.2f}  mean gain|call {gain[sel].mean():+6.1f}   natural: rate {100*nat.mean():.1f}% value {(gain*nat).mean():+.2f}")
# by position: value of pred>0 vs natural
coef=np.linalg.lstsq(X,gain,rcond=None)[0]; pred=X@coef
print("\nper position (round,pos): natural value vs model(th=0) value, per decision row")
for rnd in (1,2):
    for pos in range(4):
        m=np.array([(int(r['round'])==rnd and int(r['passesBefore'])==pos) for r in rows])
        if m.sum()==0: continue
        sel=(pred>0)&m
        print(f"  r{rnd} p{pos}: n={m.sum():6d} natural {(gain*nat)[m].mean():+6.2f} (rate {100*nat[m].mean():4.1f}%)  model {(gain*(pred>0))[m].mean():+6.2f} (rate {100*(pred>0)[m].mean():4.1f}%)")
np.save(path+'.coef.npy',coef)
