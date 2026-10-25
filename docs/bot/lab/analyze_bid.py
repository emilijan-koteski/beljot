import csv, sys, collections, math
path=sys.argv[1]
rows=[]
with open(path) as f:
    for r in csv.DictReader(f):
        for k in list(r):
            if k not in ('variant','suit'):
                r[k]=float(r[k])
        rows.append(r)
print(path, len(rows), "rows")
def mean(xs): return sum(xs)/len(xs) if xs else float('nan')
# 1. by round/position: decision points (group rows by same decision point = consecutive rows with same deal? not stored) -> use per-row stats
byRP=collections.defaultdict(list)
for r in rows: byRP[(r['round'],r['passesBefore'])].append(r)
print("\nround pos   n     EV(call) EV(pass)  nat%   P(gain>0)  missed(E[gain+ | pass])  bad(E[gain- | call])")
for k in sorted(byRP):
    rs=byRP[k]
    gains=[r['evCall']-r['evPass'] for r in rs]
    nat=[r['natural'] for r in rs]
    missed=[g for g,r in zip(gains,rs) if r['natural']==0 and g>0]
    bad=[-g for g,r in zip(gains,rs) if r['natural']==1 and g<0]
    print(f"{int(k[0])}     {int(k[1])}   {len(rs):6d}  {mean([r['evCall'] for r in rs]):+7.1f} {mean([r['evPass'] for r in rs]):+7.1f}  {100*mean(nat):4.1f}%  {100*mean([g>0 for g in gains]):5.1f}%   {sum(missed)/len(rs):6.2f}                  {sum(bad)/len(rs):6.2f}")
# 2. class table: trump holding class x count x sideA
def cls(r):
    j,n9,a=int(r['J']),int(r['9']),int(r['A'])
    return ('J9' if j and n9 else 'J' if j else '9' if n9 else '-')+('A' if a else '')
byC=collections.defaultdict(list)
for r in rows: byC[(cls(r),int(r['count']),min(int(r['sideA']),2))].append(r)
print("\nclass count sideA     n   EV(call)  EV(pass)   gain   nat%   (all positions pooled)")
for k in sorted(byC, key=lambda k:(k[1],k[0],k[2])):
    rs=byC[k]
    if len(rs)<200: continue
    g=mean([r['evCall']-r['evPass'] for r in rs])
    print(f"{k[0]:5s} {k[1]:5d} {k[2]:5d} {len(rs):6d}  {mean([r['evCall'] for r in rs]):+8.1f} {mean([r['evPass'] for r in rs]):+8.1f} {g:+7.1f}  {100*mean([r['natural'] for r in rs]):4.0f}%")
# 3. linear fit of evCall on features
try:
    import numpy as np
    feats=['count','J','9','A','T','K','Q','8','7','sideA','sideT','sideK']
    X=np.array([[r[f] for f in feats]+[1.0] for r in rows]); y=np.array([r['evCall'] for r in rows])
    coef,res,rk,sv=np.linalg.lstsq(X,y,rcond=None)
    pred=X@coef
    print("\nlinear fit of EV(call): R^2=%.3f"%(1-((y-pred)**2).sum()/((y-y.mean())**2).sum()))
    for f,c in zip(feats+['const'],coef): print(f"  {f:7s} {c:+7.1f}")
    # trump-points based: J=20,9=14,A=11,T=10,K=4,Q=3 + len bonus
    tp=np.array([20*r['J']+14*r['9']+11*r['A']+10*r['T']+4*r['K']+3*r['Q'] for r in rows])
    X2=np.column_stack([tp, np.array([r['count'] for r in rows]), np.array([r['sideA'] for r in rows]), np.ones(len(rows))])
    c2=np.linalg.lstsq(X2,y,rcond=None)[0]; p2=X2@c2
    print("fit EV = a*trumpPts + b*count + c*sideA + d: a=%.2f b=%.1f c=%.1f d=%.1f R^2=%.3f"%(c2[0],c2[1],c2[2],c2[3],1-((y-p2)**2).sum()/((y-y.mean())**2).sum()))
    # threshold analysis: call if pred>th: realized mean gain vs natural
    for th in (-10,0,10,20,30):
        sel=pred>th
        g=np.array([r['evCall']-r['evPass'] for r in rows])
        print(f"  policy call if linearEV>{th:+d}: call rate {100*sel.mean():.1f}%, mean realized gain when calling {g[sel].mean():+.1f}, per-row value {(g*sel).mean():+.2f}  (natural: rate {100*np.mean([r['natural'] for r in rows]):.1f}%, per-row value {np.mean([ (r['evCall']-r['evPass'])*r['natural'] for r in rows]):+.2f})")
except ImportError:
    print("numpy missing")
