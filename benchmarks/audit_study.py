#!/usr/bin/env python3
"""Independent read-only checks of recorded study bytes, sets and statistics."""
import argparse
import copy
import gzip
import hashlib
import json
import math
from pathlib import Path
import re
import uuid

class AuditError(ValueError):
    pass

def check(condition, message):
    if not condition:
        raise AuditError(message)

def percentile(values, p):
    return sorted(values)[max(0, math.ceil(len(values)*p)-1)] if values else 0

def id_hash(values):
    return hashlib.sha256("\n".join(sorted(set(values))).encode()).hexdigest()

def read_artifact(root, ref, secrets=()):
    path = (root/ref["Path"]).resolve()
    check(path.is_relative_to(root.resolve()), "artifact escapes output")
    data = path.read_bytes()
    check(len(data) == ref["Size"] <= 64*1024*1024, "artifact size differs")
    check(hashlib.sha256(data).hexdigest() == ref["SHA256"], "artifact checksum differs")
    if ref.get("Codec", "gzip") == "json":
        raw = data
    else:
        check(data[4:8] == b"\0\0\0\0", "gzip timestamp is not zero")
        import io
        with gzip.GzipFile(fileobj=io.BytesIO(data)) as stream:
            raw = stream.read(128*1024*1024+1)
    check(len(raw) == ref["RawBytes"] <= 128*1024*1024, "raw artifact bound differs")
    text = raw.decode()
    for secret in secrets:
        check(secret not in text, "private credential found in public artifact")
    for marker in ("postgres://", "POSTGRES_ADMIN_PASSWORD", "POSTGRES_WRITER_PASSWORD", "-----BEGIN PRIVATE KEY"):
        check(marker not in text, "private configuration found in public artifact")
    return json.loads(raw)

def populations(group):
    if "Explicit" in group:
        return group["Explicit"]
    lookup = [str(uuid.uuid5(uuid.NAMESPACE_OID, f"{group['Namespace']}/{group['Seed']}/{i}")) for i in range(group["Count"])]
    sets = {}
    for name, indices in group["Indices"].items():
        check(all(isinstance(i,int) and 0<=i<len(lookup) for i in indices), "population index out of range")
        sets[name] = [lookup[i] for i in indices] + group.get("Extra",{}).get(name,[])
    return sets

def pipeline(run, group, case):
    sets = populations(group)
    attempted,accepted,ambiguous,stored = [sets.get(name,[]) for name in ("attempted","accepted","ambiguous","stored")]
    check(run["correct"] and not any(run[k] for k in ("Missing","Unexpected","DuplicateRows","ContentMismatch")), "pipeline correctness failed")
    for name,values in sets.items():
        check(id_hash(values)==run["id_set_sha256"][name], "ID set hash differs")
    check(set(accepted)<=set(attempted) and set(ambiguous)<=set(attempted), "unattempted ACK/ambiguity")
    check(not set(accepted)&set(ambiguous), "ACK and ambiguity overlap")
    check(set(accepted)<=set(stored)<=set(attempted), "stored/accepted set mismatch")
    check(run["Planned"]==case["Count"]==run["Attempted"]+run["GeneratorMissed"], "offered count mismatch")
    check(run["Attempted"]==len(attempted)==run["Accepted"]+run["Ambiguous"]+run["Rejected"], "attempt classification mismatch")
    check(run["Accepted"]==len(accepted) and run["Persisted"]==len(stored), "stored/ACK count mismatch")
    requests=run["request_samples"]
    cursor=0;acked=[];unknown=[]
    for request in requests:
        n=request["Events"];check(0<n<=case["BatchSize"], "invalid request event population")
        check(request["FinishedNS"]>=request["StartedNS"]>=0, "invalid request interval")
        check(request["ElapsedNS"]==request["FinishedNS"]-request["StartedNS"], "request elapsed differs")
        ids=attempted[cursor:cursor+n];cursor+=n
        if request["Accepted"]: acked.extend(ids)
        if request["Ambiguous"]: unknown.extend(ids)
    check(cursor==len(attempted) and acked==accepted and unknown==ambiguous, "request/ID population differs")
    window=run["WindowNS"]
    inside=sum(q["Events"] for q in requests if q["Accepted"] and q["FinishedNS"]<=window)
    starts=sum(q["Events"] for q in requests if q["StartedNS"]<=window)
    check(inside==run["AcceptedInWindow"] and starts==run["AttemptedInWindow"], "window classification differs")
    check(run["AcceptedAfterWindow"]==len(accepted)-inside and run["AttemptedAfterWindow"]==len(attempted)-starts, "tail classification differs")
    check(run["ack_ns"]==[q["ElapsedNS"] for q in requests if q["Accepted"]], "ACK samples differ from requests")
    check(len(run["visibility_ns"])==run["ObservedEvents"], "visibility population differs")
    for samples,key in ((run["ack_ns"],"ack_latency"),(run["visibility_ns"],"receipt_to_observed_storage_upper_bound")):
        check(all(isinstance(v,int) and v>=0 for v in samples), "invalid latency sample")
        for p,name in ((.5,"p50_ns"),(.95,"p95_ns"),(.99,"p99_ns")):
            check(percentile(samples,p)==run[key][name], "latency percentile differs")
    check(run["TotalNS"]>=window and run["DrainNS"]==run["TotalNS"]-window, "complete duration differs")
    check(math.isclose(run["AcceptedEPS"],inside*1e9/window,rel_tol=1e-12), "ACK rate denominator differs")
    check(math.isclose(run["PersistedEPS"],len(stored)*1e9/run["TotalNS"],rel_tol=1e-12), "storage rate denominator differs")
    unknown_count=len(set(ambiguous)-set(stored));check(unknown_count==run["AmbiguousUnobserved"], "unknown-by-verification count differs")
    for point in run["backlog"]:
        expected=sum(q["Events"] for q in requests if q["Accepted"] and q["FinishedNS"]<=point["elapsed_ns"])
        check(point["Accepted"]==expected, "backlog ACK timeline differs")
    limited=any(run[k] for k in ("GeneratorMissed","LateSchedules","Rejected","Ambiguous","AttemptedAfterWindow","AcceptedAfterWindow","ObserverLatePolls","ObserverOutages"))
    check(run["outcome"]==("limited" if limited else "complete"), "limit incorrectly labelled")

def stripped(report, kind):
    summary=copy.deepcopy(report)
    def strip(run):
        for field in ("ack_ns","visibility_ns","backlog","request_samples"):
            run[field]=None
    if kind=="database":
        for run in summary["runs"]: run["batch_ns"]=None
    elif kind=="pipeline":
        for run in summary["runs"]: strip(run)
    else:
        for run in summary["Runs"]:
            if run.get("Pipeline"): strip(run["Pipeline"])
    return summary

def registered_cases(mode):
    """An independent transcription of the approved fixed experimental design."""
    zero=dict(ID='',Kind='',Recovery='',Pattern='',PayloadProfile='varied',Count=0,
              BatchSize=0,PayloadBytes=0,Rate=0,Concurrency=0,Repetitions=3,
              DuplicateFraction=0,Window=0,Warmup=0,Drain=0,Poll=0,FaultAt=0,RestoreAt=0)
    def case(**changes): return dict(zero,**changes)
    if mode=='smoke':
        return [case(ID='smoke-database',Kind='database',Count=100,BatchSize=10,PayloadBytes=256),
                case(ID='smoke-pipeline',Kind='pipeline',Pattern='steady',Count=10,BatchSize=2,
                     PayloadBytes=256,Rate=20,Concurrency=2,Window=500_000_000,Warmup=100_000_000,
                     Drain=5_000_000_000,Poll=25_000_000)]
    check(mode=='full','unknown study mode')
    db=case(Kind='database',Count=5000,BatchSize=100,PayloadBytes=256)
    result=[dict(db,ID=f'db-batch-{b}',BatchSize=b) for b in (1,10,50,100,500,1000)]
    result += [dict(db,ID=f'db-payload-{p}',PayloadBytes=p) for p in (1024,16384)]
    result += [dict(db,ID=f'db-duplicates-{d}',DuplicateFraction=d/100) for d in (10,50,90)]
    base=case(Kind='pipeline',Pattern='steady',Count=10000,BatchSize=100,PayloadBytes=256,
              Rate=1000,Concurrency=32,Window=10_000_000_000,Warmup=2_000_000_000,
              Drain=30_000_000_000,Poll=50_000_000)
    result += [dict(base,ID=f'pipeline-batch-{b}',BatchSize=b) for b in (1,10,100)]
    result += [dict(base,ID=f'pipeline-rate-{r}',Count=r*10,Rate=r) for r in (100,5000,10000)]
    result += [dict(base,ID=f'pipeline-payload-{p}',PayloadBytes=p,BatchSize=10) for p in (1024,16384)]
    result += [dict(base,ID='pipeline-burst',Pattern='burst')]
    result += [dict(base,ID=f'pipeline-concurrency-{c}',Concurrency=c,BatchSize=1) for c in (8,64)]
    for name,restore in (('database',8),('consumer',5)):
        result.append(dict(base,ID='recovery-'+name,Kind='recovery',Recovery=name,Rate=500,
                           Count=7500,Window=15_000_000_000,FaultAt=3_000_000_000,RestoreAt=restore*1_000_000_000))
    result.append(case(ID='recovery-outbox',Kind='recovery',Recovery='outbox',Count=100,
                       BatchSize=100,PayloadBytes=256,RestoreAt=8_000_000_000,Drain=30_000_000_000,Poll=50_000_000))
    return result

def validate_registry(rows, mode):
    check([r['Case'] for r in rows]==registered_cases(mode),'registered cases or controls differ')

def audit(directory, allow_smoke=False, secrets=()):
    root=Path(directory);index=json.loads((root/"index.json").read_text())
    check(index["Schema"]==1 and index["Status"]=="complete", "study is incomplete")
    identity=index["Identity"];mode=identity["Mode"]
    check(mode=="full" or allow_smoke and mode=="smoke", "smoke is not publication evidence")
    check(mode!="full" or not identity["SourceDirty"], "full study used dirty sources")
    for key in ("SourceFingerprint","ControlsFingerprint","ImagesFingerprint","ConfigFingerprint"):
        check(re.fullmatch(r"[0-9a-f]{64}",identity[key]) is not None, "invalid provenance fingerprint")
    check(re.fullmatch(r"[0-9a-f]{40}",identity["SourceSHA"]) is not None, "invalid source revision")
    check(bool(index["MetadataArtifacts"]) and bool(index["Resources"]), "provenance/resources missing")
    for ref in index["MetadataArtifacts"]+index["Resources"]:
        value=read_artifact(root,ref,secrets)
        if ref.get("Codec")=="json":
            check(value["source_sha"]==identity["SourceSHA"] and value["broker_fsync_bypass"] is False, "manifest revision/durability differs")
            check(value["settings"]["api_tls"] is True and value["settings"]["redpanda_memory"]=="1G" and value["settings"]["redpanda_smp"]==1, "runtime controls differ")
            check(all(value["database"][key]=="on" for key in ("fsync","synchronous_commit")), "database durability differs")
    cases=index["Cases"];validate_registry(cases,mode);check(len(cases)==(25 if mode=="full" else 2), "registry cardinality differs")
    check(len({row["Case"]["ID"] for row in cases})==len(cases), "duplicate case ID")
    totals={"database_runs":0,"pipeline_runs":0,"recovery_runs":0,"limited_cases":0}
    for row in cases:
        c=row["Case"];check(row["Status"] in ("complete","limited"), "case is not complete")
        current=read_artifact(root,row["Artifact"],secrets)
        check(current["Identity"]==identity and current["Case"]==c and current["Status"]==row["Status"], "capture identity/control differs")
        for history in row["History"]:
            read_artifact(root,history["Artifact"],secrets)
        report=current["Report"];check(stripped(report,c["Kind"])==row["Summary"], "summary differs from raw capture")
        runs=report.get("runs",report.get("Runs",[]));repetitions={r.get("repetition",r.get("Repetition")) for r in runs}
        check(repetitions=={1,2,3}, "repetition set differs")
        if row["Status"]=="limited": totals["limited_cases"]+=1
        if c["Kind"]=="database":
            check(len(runs)==9 and {(r["method"],r["repetition"]) for r in runs}=={(m,r) for m in ("insert","batch-insert","copy-merge") for r in (1,2,3)}, "database method/repetition missing")
            unique=c["Count"]-int(c["Count"]*c["DuplicateFraction"])
            for run in runs:
                check(run["correct"] and run["submitted"]==c["Count"] and run["inserted"]==unique and run["duplicates"]==c["Count"]-unique, "database counts differ")
                size=1 if run["method"]=="insert" else c["BatchSize"]
                check(len(run["batch_ns"])==math.ceil(c["Count"]/size) and 0<sum(run["batch_ns"])<=run["elapsed_ns"], "database sample duration/population differs")
            totals["database_runs"]+=len(runs)
        else:
            groups={g["Repetition"]:g for g in current["Populations"]};check(set(groups)=={1,2,3} and len(runs)==3, "population/repetition missing")
            for run in runs:
                rep=run.get("repetition",run.get("Repetition"));group=groups[rep]
                if c["Kind"]=="pipeline": pipeline(run,group,c);totals["pipeline_runs"]+=1
                elif run.get("Pipeline"):
                    pipeline(run["Pipeline"],group,c);check(run["Correct"] and len(run["Faults"])>=4, "recovery control failed");totals["recovery_runs"]+=1
                else:
                    sets=populations(group);check(run["Correct"] and set(sets["accepted"])==set(sets["stored"]) and len(sets["stored"])==100, "outbox sets differ")
                    check(run["FirstAttemptNS"]>=5e9 and run["FirstAttemptStatus"] in ("http_503","delivery_uncertain"), "outbox timeout was not established")
                    check(len(sets["ambiguous"])==100 and len(run["Faults"])>=4, "outbox uncertainty/control missing");totals["recovery_runs"]+=1
    return totals

if __name__=="__main__":
    parser=argparse.ArgumentParser();parser.add_argument("directory");parser.add_argument("--allow-smoke",action="store_true");args=parser.parse_args()
    try:
        print(json.dumps(audit(args.directory,args.allow_smoke),sort_keys=True))
    except (AuditError,KeyError,ValueError,OSError) as error:
        parser.exit(1,"audit failed: "+str(error)+"\n")
