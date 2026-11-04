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
    if 'HTTPMaxConnsPerHost' in case:
        conn=run['connections']
        check(conn['ActiveRequests']==0 and conn['WaitingForConnection']==0,'requests still active')
        check(conn['MaxConnsPerHost']==case['HTTPMaxConnsPerHost'],'active cap differs')
        idle=case['HTTPMaxIdleConnsPerHost'] or 2
        check(conn['MaxIdleConnsPerHost']==idle,'idle retention differs')
        check(conn['TCPTrackingStatus']=='transport_owned_connected_tcp','TCP tracking unavailable')
        check(conn['PeakOpenTCP']>=conn['OpenTCP']>=0 and conn['PeakActiveRequests']>=0,'socket counts differ')
        if conn['MaxConnsPerHost']:
            check(conn['PeakOpenTCP']<=conn['MaxConnsPerHost'],'active TCP cap exceeded')
        check(conn['ConnectionAcquireNS']>=0,'negative connection duration')
        check(set(conn['Errors'])<=set(('fd_limit','address_unavailable','connection_refused','cancelled','timeout','dns','other')),'unbounded error labels')
        check(set(conn['Protocols'])<=set(('HTTP/1.1','HTTP/2.0','HTTP/1.0','other')),'unbounded protocol labels')
        check(sum(conn['Protocols'].values())<=len(requests),'protocol request counts exceed attempts')
    window=run["WindowNS"]
    inside=sum(q["Events"] for q in requests if q["Accepted"] and q["FinishedNS"]<=window)
    starts=sum(q["Events"] for q in requests if q["StartedNS"]<=window)
    check(inside==run["AcceptedInWindow"] and starts==run["AttemptedInWindow"], "window classification differs")
    check(run["AcceptedAfterWindow"]==len(accepted)-inside and run["AttemptedAfterWindow"]==len(attempted)-starts, "tail classification differs")
    check(run["ack_ns"]==[q["ElapsedNS"] for q in requests if q["Accepted"]], "ACK samples differ from requests")
    check("PersistedEPS" not in run,"legacy ambiguous rate field is not schema2")
    check(run["VisibilitySampleCount"]==len(run["visibility_ns"])<=run["ObservedEvents"],"visibility sample count differs")
    observation=run["observation_status"]
    check(observation in ("complete","partial","failed","not_run"),"invalid observation status")
    if observation=="complete":
        check(len(run["visibility_ns"])==run["ObservedEvents"],"complete visibility population differs")
    if observation!="complete" or not run["visibility_ns"]:
        check(run["receipt_to_observed_storage_upper_bound"] is None,"unavailable visibility must be null")
    check(run["delivery_status"]=="passed","delivery was not independently verified")
    for samples,key in ((run["ack_ns"],"ack_latency"),(run["visibility_ns"],"receipt_to_observed_storage_upper_bound")):
        check(all(isinstance(v,int) and v>=0 for v in samples), "invalid latency sample")
        if key=="receipt_to_observed_storage_upper_bound" and (observation!="complete" or not samples):continue
        for p,name in ((.5,"p50_ns"),(.95,"p95_ns"),(.99,"p99_ns")):
            check(percentile(samples,p)==run[key][name], "latency percentile differs")
    check(run["TotalNS"]>=window and run["DrainNS"]==run["TotalNS"]-window, "complete duration differs")
    check(math.isclose(run["AcceptedEPS"],inside*1e9/window,rel_tol=1e-12), "ACK rate denominator differs")
    check(math.isclose(run["VerifiedCompletionEPS"],len(stored)*1e9/run["TotalNS"],rel_tol=1e-12), "storage rate denominator differs")
    unknown_count=len(set(ambiguous)-set(stored));check(unknown_count==run["AmbiguousUnobserved"], "unknown-by-verification count differs")
    previous_visible=0;previous_time=0
    for point in run["backlog"]:
        check(previous_time<=point["elapsed_ns"]<=run["TotalNS"],"observation timeline differs")
        previous_time=point["elapsed_ns"]
        check(previous_visible<=point["Visible"]<=run["Persisted"],"observed row count exceeds verified population")
        previous_visible=point["Visible"]
        if not point["FinalVerification"]:check(point["Visible"]<=run["ObservedEvents"],"observer population differs")
        else:check(point["Visible"]==run["Persisted"],"final row count differs")
        expected=sum(q["Events"] for q in requests if q["Accepted"] and q["FinishedNS"]<=point["elapsed_ns"])
        check(point["Accepted"]==expected, "backlog ACK timeline differs")
    load_limited=any(run[k] for k in ("GeneratorMissed","LateSchedules","Rejected","Ambiguous","AttemptedAfterWindow","AcceptedAfterWindow"))
    check(run["load_status"]==("limited" if load_limited else "met"),"load status differs")
    limited=observation in ("failed","partial") or run["ObserverSlowQueries"]>0 or any(run[k] for k in ("GeneratorMissed","LateSchedules","Rejected","Ambiguous","AttemptedAfterWindow","AcceptedAfterWindow","ObserverLatePolls","ObserverOutages"))
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
              HTTPMaxConnsPerHost=0,HTTPMaxIdleConnsPerHost=0,DuplicateFraction=0,Window=0,Warmup=0,Drain=0,Poll=0,FaultAt=0,RestoreAt=0)
    def case(**changes): return dict(zero,**changes)
    if mode=='smoke':
        return [case(ID='smoke-database',Kind='database',Count=100,BatchSize=10,PayloadBytes=256),
                case(ID='smoke-pipeline',Kind='pipeline',Pattern='steady',Count=10,BatchSize=2,
                     PayloadBytes=256,Rate=20,Concurrency=2,Window=500_000_000,Warmup=100_000_000,
                     Drain=5_000_000_000,Poll=25_000_000)]
    if mode=='connections':
        base=case(Kind='pipeline',Pattern='steady',Count=150000,BatchSize=100,PayloadBytes=1024,
                  Rate=15000,Concurrency=128,HTTPMaxConnsPerHost=128,HTTPMaxIdleConnsPerHost=128,
                  Window=10_000_000_000,Warmup=2_000_000_000,Drain=30_000_000_000,Poll=1_000_000_000)
        result=[dict(base,ID=f'connections-cap-{cap}',HTTPMaxConnsPerHost=cap) for cap in (4,8,16,32,64,128,0)]
        result += [dict(base,ID=f'connections-rate-{rate}',Rate=rate,Count=rate*10) for rate in (30000,50000)]
        result += [dict(base,ID=f'connections-payload-{size}',PayloadBytes=size,BatchSize=10,Rate=1000,Count=10000) for size in (256,1024,16384)]
        return result
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

def validate_manifest(value, identity):
    check(value['source_sha']==identity['SourceSHA'] and value['broker_fsync_bypass'] is False,
          'manifest revision/durability differs')
    settings=value['settings']
    check(settings['api_tls'] is True and settings['api_route']=='https_gateway'
          and settings['tls_version'] in ('TLS 1.2','TLS 1.3')
          and settings['redpanda_memory']=='1G' and settings['redpanda_smp']==1
          and settings['api_rate_limit_rps']==10000 and settings['api_log_level']=='info',
          'runtime controls differ')
    check(all(value['database'][key]=='on' for key in ('fsync','synchronous_commit','full_page_writes'))
          and value['database']['server_version']=='18.6','database durability/version differs')
    topics=value['TopicConfiguration']
    caching=re.findall(r'^write\.caching\s+(\S+)',topics,re.M)
    retention=re.findall(r'^retention\.ms\s+(\S+)',topics,re.M)
    check(len(caching)>=2 and set(caching)=={'false'},'topic write caching differs')
    check(len(retention)>=2 and set(retention)=={'604800000'},'topic retention differs')
    for name,partitions in (('ingestkit.events',3),('ingestkit.dlq',1)):
        check(re.search(r'NAME\s+'+re.escape(name)+r'\s+PARTITIONS\s+'+str(partitions)+r'\s+REPLICAS\s+1',topics),
              'topic partition/replica controls differ')

def canonical_bytes(value):
    return json.dumps(value,ensure_ascii=False,separators=(',',':'),allow_nan=False).replace('&','\\u0026').replace('<','\\u003c').replace('>','\\u003e').encode()

def object_hash(value):
    return hashlib.sha256(canonical_bytes(value)).hexdigest()

def sorted_maps(value):
    if isinstance(value,dict):return {k:sorted_maps(v) for k,v in sorted(value.items())}
    if isinstance(value,list):return [sorted_maps(v) for v in value]
    return value

def verify_inventory(root):
    manifest=json.loads((root/'evidence-manifest.json').read_text())
    check(manifest['Schema']==2,'unsupported evidence inventory')
    paths=[]
    for ref in manifest['Artifacts']:
        path=(root/ref['Path']).resolve()
        check(path.is_relative_to(root.resolve()) and path.is_file(),'evidence escapes output')
        check(path.stat().st_size==ref['Size']<=64*1024*1024,'evidence size differs')
        check(hashlib.sha256(path.read_bytes()).hexdigest()==ref['SHA256'],'evidence checksum differs')
        paths.append(ref['Path'])
    actual={str(p.relative_to(root)) for p in root.rglob('*') if p.is_file() and p.name!='evidence-manifest.json'}
    check(len(paths)==len(set(paths)) and set(paths)==actual,'evidence inventory differs')

def verify_calibration_run(run,rate,window):
    samples=run['RequestSamples'];conn=run['Connections']
    attempted=sum(q['Events'] for q in samples)
    accepted=sum(q['Events'] for q in samples if q['Accepted'])
    inside=sum(q['Events'] for q in samples if q['Accepted'] and q['FinishedNS']<=window)
    check(run['Correct'] and run['Planned']==int(rate*window/1e9),'calibration plan differs')
    check(run['Attempted']==attempted and run['Accepted']==accepted and run['AcceptedInWindow']==inside and run['GeneratorMissed']+attempted==run['Planned'],'calibration accounting differs')
    check(run['WindowNS']==window and run['TotalNS']>=window and math.isclose(run['AcceptedEPS'],inside*1e9/window,rel_tol=1e-12),'calibration rate denominator differs')
    check(all(q['FinishedNS']>=q['StartedNS']>=0 and q['ElapsedNS']==q['FinishedNS']-q['StartedNS'] and q['Status']==202 for q in samples),'calibration timing/ACK differs')
    check(conn['ActiveRequests']==0 and conn['WaitingForConnection']==0 and not conn['Errors'] and conn['Protocols']=={'HTTP/1.1':len(samples)} and 0<conn['PeakOpenTCP']<=128,'calibration transport differs')
    hashes=run['IDSetSHA256']
    check(set(hashes)=={'attempted','accepted','sink_received'} and len(set(hashes.values()))==1 and all(re.fullmatch('[0-9a-f]{64}',v) for v in hashes.values()),'calibration ID hashes differ')

# Recorded source did not check terminal SQL row-stream errors. Preserve its
# immutable bytes, but expose no successful session population from that source.
UNRELIABLE_SESSION_SOURCES={'043f584cf8638269a53afd5c9ec35a2721bc4148'}

def resource_samples(root,index):
    samples=[]
    for ref in index['Resources']:samples.extend(read_artifact(root,ref))
    if index['Identity']['SourceSHA'] in UNRELIABLE_SESSION_SOURCES:
        for sample in samples:
            runtime=sample.get('runtime',{})
            if 'database_sessions' in runtime:
                runtime['database_sessions']={'status':'unavailable','reason':'recorded_source_did_not_check_row_stream_errors'}
    return samples

def verify_connection_evidence(root,index,repo=None):
    verify_inventory(root)
    identity=index['Identity']
    check(object_hash([row['Case'] for row in index['Cases']])==identity['ControlsFingerprint'],'controls fingerprint differs')
    for filename in index['ManifestFiles']:
        refs={ref['Path']:ref for ref in index['MetadataArtifacts']}
        check(filename in refs,'manifest is not an indexed artifact')
        m=read_artifact(root,refs[filename])
        host={k:m['host'].get(k,'') for k in ('go','os','architecture','generator_logical_cpus','kernel','docker_vm','memory_bytes','os_version','docker','compose')}
        if m['host']['os']=='darwin':host['cpu']=m['host']['cpu']
        else:
            values=[]
            for line in m['host']['cpu'].splitlines():
                if ':' in line:
                    k,v=line.split(':',1)
                    if k.strip() in ('model name','Hardware','CPU architecture','CPU part'):values.append(k.strip()+':'+v.strip())
            host['cpu']='\n'.join(sorted(set(values)))
        fingerprint=object_hash(dict(Settings=sorted_maps(m['settings']),Database=sorted_maps(m['database']),Topics=m['TopicConfiguration'],Host=sorted_maps(host)))
        check(fingerprint==identity['ConfigFingerprint'],'runtime fingerprint differs')
    images=json.loads((root/'image-identity.json').read_text());signatures=[]
    for image in images:
        image['Config']=sorted_maps(image['Config'])
        signatures.append(object_hash(image))
    check(object_hash(sorted(signatures))==identity['ImagesFingerprint'],'image fingerprint differs')
    calibration=json.loads((root/'calibration-index.json').read_text())
    check(calibration['SourceSHA']==identity['SourceSHA'] and len(calibration['Artifacts'])==3,'calibration source/cardinality differs')
    rates=[]
    for ref in calibration['Artifacts']:
        report=read_artifact(root,dict(ref,RawBytes=ref['Size']))
        c=report['Config'];rate=c['target_events_per_second'];rates.append(rate)
        check(report['Schema']==2 and report['Kind']=='tls_driver_calibration' and report['SourceSHA']==identity['SourceSHA'] and c['https'] is True,'calibration identity/transport differs')
        check(c['http_batch_size']==100 and c['payload_bytes']==1024 and c['concurrency']==128 and c['http_max_conns_per_host']==128 and c['http_max_idle_conns_per_host']==128 and c['window_ns']==10_000_000_000,'calibration controls differ')
        check(report.get('status','complete')=='complete','calibration is incomplete')
        check(len(report['Runs'])==3,'calibration repetitions differ')
        for run in report['Runs']:verify_calibration_run(run,rate,c['window_ns'])
    check(sorted(rates)==[15000,30000,50000],'calibration rate cohort differs')
    if repo is not None:
        import subprocess
        def git(*args):return subprocess.check_output(['git','-C',str(repo),*args])
        files=[]
        for entry in git('ls-tree','-rz',identity['SourceSHA']).split(b'\0'):
            if not entry:continue
            header,name=entry.split(b'\t');mode,kind,blob=header.split()
            modes={b'100644':'-rw-r--r--',b'100755':'-rwxr-xr-x',b'120000':'Lrwxrwxrwx'}
            check(mode in modes and kind==b'blob','unsupported source entry')
            files.append(dict(Name=name.decode(),Mode=modes[mode],Digest=hashlib.sha256(git('cat-file','blob',blob.decode())).hexdigest()))
        check(object_hash(sorted(files,key=lambda x:x['Name']))==identity['SourceFingerprint'],'source fingerprint differs')

def audit(directory, allow_smoke=False, secrets=(), repo=None):
    root=Path(directory);index=json.loads((root/"index.json").read_text())
    check(index["Schema"]==2 and index["Status"]=="complete", "study is incomplete")
    identity=index["Identity"];mode=identity["Mode"]
    check(mode in ("full","connections") or allow_smoke and mode=="smoke", "smoke is not publication evidence")
    check(mode not in ("full","connections") or not identity["SourceDirty"], "full study used dirty sources")
    for key in ("SourceFingerprint","ControlsFingerprint","ImagesFingerprint","ConfigFingerprint"):
        check(re.fullmatch(r"[0-9a-f]{64}",identity[key]) is not None, "invalid provenance fingerprint")
    check(re.fullmatch(r"[0-9a-f]{40}",identity["SourceSHA"]) is not None, "invalid source revision")
    check(bool(index["MetadataArtifacts"]) and bool(index["Resources"]), "provenance/resources missing")
    for ref in index["MetadataArtifacts"]+index["Resources"]:
        value=read_artifact(root,ref,secrets)
        if ref.get("Codec")=="json":
            validate_manifest(value,identity)
    cases=index["Cases"];validate_registry(cases,mode);check(len(cases)==(25 if mode=="full" else 12 if mode=="connections" else 2), "registry cardinality differs")
    check(len({row["Case"]["ID"] for row in cases})==len(cases), "duplicate case ID")
    if mode=="connections":verify_connection_evidence(root,index,repo)
    totals={"database_runs":0,"pipeline_runs":0,"recovery_runs":0,"limited_cases":0}
    for row in cases:
        c=row["Case"];check(row["Status"] in ("complete","limited"), "case is not complete")
        current=read_artifact(root,row["Artifact"],secrets)
        check(current["Schema"]==2,"unsupported capture schema")
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
    if identity["SourceSHA"] in UNRELIABLE_SESSION_SOURCES:
        resource_samples(root,index)
        totals["unavailable_metrics"]=["database_sessions"]
    return totals

if __name__=="__main__":
    parser=argparse.ArgumentParser();parser.add_argument("directory");parser.add_argument("--allow-smoke",action="store_true");parser.add_argument("--repo",type=Path,help="verify source bytes from the recorded Git revision");args=parser.parse_args()
    try:
        print(json.dumps(audit(args.directory,args.allow_smoke,repo=args.repo),sort_keys=True))
    except (AuditError,KeyError,ValueError,OSError) as error:
        parser.exit(1,"audit failed: "+str(error)+"\n")
