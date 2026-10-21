# Deterministic generator for testdata/events.jsonl and expected_balances.json.
import json, random, datetime
random.seed(20261021)
rates={"USD":1.0,"EUR":1.0825,"GBP":1.2675}
accts=[f"acc-{i:02d}" for i in range(40)]
base=datetime.datetime(2026,1,1,tzinfo=datetime.timezone.utc)
def is_tie(amount,cur):
    x=amount*rates[cur]
    return abs(x-round(x))>0.4999 and abs(x-round(x))<0.5001
events=[]; seq=0
def ts(sec): return (base+datetime.timedelta(seconds=sec)).strftime("%Y-%m-%dT%H:%M:%SZ")
t=0
for i in range(5200):
    seq+=1; t+=random.randint(0,40)
    typ=random.choices(["deposit","withdrawal","transfer"],[5,3,3])[0]
    cur=random.choices(["USD","EUR","GBP"],[6,3,2])[0]
    while True:
        amt=random.randint(1,50000)
        if not is_tie(amt,cur): break
    e={"id":f"e{seq}","seq":seq,"ts":ts(t),"type":typ,"account":random.choice(accts),"amount":amt,"currency":cur}
    if typ=="transfer":
        e["to"]=random.choice([a for a in accts if a!=e["account"]])
    events.append(e)
# reversals
orig=[e for e in events]
for k in range(260):
    seq+=1
    target=random.choice(orig)
    tt=datetime.datetime.strptime(target["ts"],"%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=datetime.timezone.utc)
    off=random.choice([random.randint(1,3000)]*9+[-random.randint(1,500)])  # 10% earlier than original -> must be ignored
    r={"id":f"e{seq}","seq":seq,"ts":(tt+datetime.timedelta(seconds=off)).strftime("%Y-%m-%dT%H:%M:%SZ"),"type":"reversal","account":target["account"],"amount":0,"currency":"USD","reverses":target["id"]}
    events.append(r)
# a few reversals of unknown events
for k in range(10):
    seq+=1
    events.append({"id":f"e{seq}","seq":seq,"ts":ts(random.randint(0,t)),"type":"reversal","account":"acc-00","amount":0,"currency":"USD","reverses":f"ghost{k}"})
# equal timestamps with seq order mattering: deposit then reversal at same ts
# shuffle mildly: file order = ts order with local perturbations; reversals inserted randomly
events.sort(key=lambda e:(e["ts"],e["seq"]))
file=[]
for e in events:
    file.append(e)
# local shuffles
for i in range(0,len(file)-1):
    if random.random()<0.08:
        j=min(len(file)-1,i+random.randint(1,30))
        file[i],file[j]=file[j],file[i]
# move some reversals far earlier than their originals in the file
for i,e in enumerate(list(file)):
    if e["type"]=="reversal" and random.random()<0.3:
        file.remove(e); file.insert(max(0,i-random.randint(50,400)),e)
# duplicates (at-least-once)
dups=random.sample(file,180)
for d in dups:
    file.insert(random.randint(0,len(file)),dict(d))
# reference replay
seen=set(); uniq=[]
for e in file:
    if e["id"] in seen: continue
    seen.add(e["id"]); uniq.append(e)
uniq.sort(key=lambda e:(e["ts"],e["seq"]))
bal={a:0 for a in accts}; applied={}; reversed_=set()
def usd(e): return int(round(e["amount"]*rates[e["currency"]]))
def apply(e,sign):
    u=usd(e)
    if e["type"]=="deposit": bal[e["account"]]+=sign*u
    elif e["type"]=="withdrawal": bal[e["account"]]-=sign*u
    else:
        bal[e["account"]]-=sign*u; bal[e["to"]]+=sign*u
for e in uniq:
    if e["type"]=="reversal":
        o=applied.get(e["reverses"])
        if o is None or e["reverses"] in reversed_: continue
        apply(o,-1); reversed_.add(e["reverses"])
    else:
        apply(e,1); applied[e["id"]]=e
with open("repo/testdata/events.jsonl","w") as f:
    for e in file: f.write(json.dumps(e,separators=(",",":"))+"\n")
json.dump(bal,open("repo/testdata/expected_balances.json","w"),indent=1,sort_keys=True)
print(len(file),"events")
