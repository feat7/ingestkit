import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest
import uuid

spec = importlib.util.spec_from_file_location('audit_study', Path(__file__).with_name('audit_study.py'))
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)

class ArtifactAuditTests(unittest.TestCase):
    def valid_pipeline(self):
        group={'Namespace':'n','Seed':42,'Count':1,'Indices':{k:[0] for k in ('attempted','accepted','stored')}}
        group['Indices']['ambiguous']=[]
        expected=str(uuid.uuid5(uuid.NAMESPACE_OID,'n/42/0'))
        digest=hashlib.sha256(expected.encode()).hexdigest()
        r={k:0 for k in ('Missing','Unexpected','DuplicateRows','ContentMismatch','GeneratorMissed','Ambiguous','Rejected','AttemptedAfterWindow','AcceptedAfterWindow','AmbiguousUnobserved','LateSchedules','ObserverLatePolls','ObserverOutages','ObserverSlowQueries')}
        r.update(correct=True,Planned=1,Attempted=1,Accepted=1,Persisted=1,ObservedEvents=1,VisibilitySampleCount=1,
                 AttemptedInWindow=1,AcceptedInWindow=1,WindowNS=100,TotalNS=110,DrainNS=10,
                 AcceptedEPS=1e7,VerifiedCompletionEPS=1e9/110,outcome='complete',
                 load_status='met',delivery_status='passed',observation_status='complete',
                 ack_ns=[10],visibility_ns=[20],backlog=[],id_set_sha256={k:digest for k in ('attempted','accepted','stored')},
                 request_samples=[dict(StartedNS=0,FinishedNS=10,ElapsedNS=10,Events=1,Accepted=True,Ambiguous=False,Status=202)],
                 ack_latency={k:10 for k in ('p50_ns','p95_ns','p99_ns')},
                 receipt_to_observed_storage_upper_bound={k:20 for k in ('p50_ns','p95_ns','p99_ns')})
        r['id_set_sha256']['ambiguous']=hashlib.sha256(b'').hexdigest()
        return r,group,{'Count':1,'BatchSize':1}

    def test_failed_visibility_cannot_serialize_a_numeric_percentile(self):
        r,g,c=self.valid_pipeline();r['observation_status']='failed'
        with self.assertRaises(audit.AuditError):audit.pipeline(r,g,c)

    def test_load_status_cannot_claim_met_when_events_were_unsent(self):
        r,g,c=self.valid_pipeline();r.update(Planned=2,GeneratorMissed=1,outcome='limited')
        g['Count']=2;c['Count']=2
        with self.assertRaises(audit.AuditError):audit.pipeline(r,g,c)

    def test_verified_completion_denominator_is_independently_checked(self):
        r,g,c=self.valid_pipeline();r['VerifiedCompletionEPS']=1000000000
        with self.assertRaises(audit.AuditError):audit.pipeline(r,g,c)

    def test_sample_count_forgery_is_rejected(self):
        r,g,c=self.valid_pipeline();r['VisibilitySampleCount']=999
        with self.assertRaises(audit.AuditError):audit.pipeline(r,g,c)

    def test_valid_observer_failure_keeps_verified_delivery(self):
        r,g,c=self.valid_pipeline()
        r.update(observation_status='failed',outcome='limited',observation_error_class='timeout',receipt_to_observed_storage_upper_bound=None)
        audit.pipeline(r,g,c)

    def test_altered_stored_id_hash_is_rejected(self):
        r,g,c=self.valid_pipeline();r['id_set_sha256']['stored']='0'*64
        with self.assertRaises(audit.AuditError):audit.pipeline(r,g,c)

    def test_nearest_rank_and_set_hash(self):
        self.assertEqual(audit.percentile(list(range(1, 101)), .95), 95)
        self.assertEqual(audit.percentile([], .99), 0)
        self.assertEqual(audit.id_hash(['b', 'a', 'a']), hashlib.sha256(b'a\nb').hexdigest())

    def test_checksum_privacy_and_confinement(self):
        with tempfile.TemporaryDirectory() as d:
            root = Path(d)
            raw = b'{"value":"private-canary"}'
            data = gzip.compress(raw, mtime=0)
            (root/'data.gz').write_bytes(data)
            ref = {'Path':'data.gz','Size':len(data),'RawBytes':len(raw),'SHA256':hashlib.sha256(data).hexdigest()}
            self.assertEqual(audit.read_artifact(root, ref)['value'], 'private-canary')
            with self.assertRaises(audit.AuditError): audit.read_artifact(root, ref, ['private-canary'])
            bad = dict(ref, SHA256='0'*64)
            with self.assertRaises(audit.AuditError): audit.read_artifact(root,bad)
            (root/'link').symlink_to('/etc/hosts')
            with self.assertRaises(audit.AuditError): audit.read_artifact(root,dict(ref,Path='link'))

    def test_forged_statistics_and_late_window_are_rejected(self):
        r,group,case=self.valid_pipeline()
        audit.pipeline(r,group,case)
        bad=copy.deepcopy(r);bad['ack_latency']['p99_ns']=9
        with self.assertRaises(audit.AuditError): audit.pipeline(bad,group,case)
        bad=copy.deepcopy(r);bad['request_samples'][0]['FinishedNS']=101;bad['request_samples'][0]['ElapsedNS']=101
        with self.assertRaises(audit.AuditError): audit.pipeline(bad,group,case)

    def test_registry_rejects_missing_or_changed_controls(self):
        cases=audit.registered_cases('full')
        self.assertEqual(len(cases),25)
        rows=[{'Case':c} for c in cases]
        audit.validate_registry(rows,'full')
        with self.assertRaises(audit.AuditError): audit.validate_registry(rows[:-1],'full')
        changed=copy.deepcopy(rows);changed[0]['Case']['PayloadBytes']=512
        with self.assertRaises(audit.AuditError): audit.validate_registry(changed,'full')
        changed=copy.deepcopy(rows);changed[-1]['Case']['RestoreAt']=1
        with self.assertRaises(audit.AuditError): audit.validate_registry(changed,'full')

    def test_runtime_controls_reject_relaxed_durability_or_route(self):
        m=dict(source_sha='a'*40,broker_fsync_bypass=False,
               database=dict(fsync='on',synchronous_commit='on',full_page_writes='on',server_version='18.6'),
               settings=dict(api_tls=True,api_route='https_gateway',tls_version='TLS 1.3',
                             api_rate_limit_rps=10000,api_log_level='info',redpanda_memory='1G',redpanda_smp=1),
               TopicConfiguration='NAME ingestkit.events PARTITIONS 3 REPLICAS 1\nwrite.caching false\nretention.ms 604800000\nNAME ingestkit.dlq PARTITIONS 1 REPLICAS 1\nwrite.caching false\nretention.ms 604800000\n')
        audit.validate_manifest(m,dict(SourceSHA='a'*40))
        for field,value in [('broker_fsync_bypass',True),('TopicConfiguration',m['TopicConfiguration'].replace('write.caching false','write.caching true',1))]:
            bad=copy.deepcopy(m);bad[field]=value
            with self.assertRaises(audit.AuditError):audit.validate_manifest(bad,dict(SourceSHA='a'*40))
        for group,field,value in [('settings','api_route','direct_http'),('settings','api_tls',False),('database','synchronous_commit','off')]:
            bad=copy.deepcopy(m);bad[group][field]=value
            with self.assertRaises(audit.AuditError):audit.validate_manifest(bad,dict(SourceSHA='a'*40))

    def test_connection_registry_and_transport_counts(self):
        cases=audit.registered_cases('connections')
        self.assertEqual(len(cases),12)
        self.assertEqual([c['HTTPMaxConnsPerHost'] for c in cases[:7]],[4,8,16,32,64,128,0])
        r,g,c=self.valid_pipeline()
        r['connections']=dict(ActiveRequests=0,PeakActiveRequests=1,OpenTCP=1,PeakOpenTCP=1,Dials=1,Reused=0,ConnectionAcquireNS=1,WaitingForConnection=0,PeakWaitingForConnection=1,MaxConnsPerHost=4,MaxIdleConnsPerHost=128,Protocols={'HTTP/1.1':1},Errors={},TLSVersions={'TLS 1.3':1},TCPTrackingStatus='transport_owned_connected_tcp')
        c.update(HTTPMaxConnsPerHost=4,HTTPMaxIdleConnsPerHost=128)
        audit.pipeline(r,g,c)
        r['connections']['PeakOpenTCP']=5
        with self.assertRaises(audit.AuditError): audit.pipeline(r,g,c)

    def test_evidence_inventory_rejects_tampering_and_extra_files(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);p=root/'sample.json';p.write_text('hello')
            manifest={'Schema':2,'Artifacts':[dict(Path='sample.json',Size=5,SHA256='2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824')]}
            (root/'evidence-manifest.json').write_text(json.dumps(manifest))
            audit.verify_inventory(root)
            p.write_text('other')
            with self.assertRaises(audit.AuditError):audit.verify_inventory(root)
            p.write_text('hello');(root/'extra').write_text('unexpected')
            with self.assertRaises(audit.AuditError):audit.verify_inventory(root)

    def test_calibration_rejects_forged_inside_rate(self):
        run=dict(Planned=100,Attempted=100,Accepted=100,AcceptedInWindow=100,GeneratorMissed=0,
                 WindowNS=1_000_000_000,TotalNS=1_000_000_001,AcceptedEPS=100,Correct=True,
                 Connections=dict(ActiveRequests=0,WaitingForConnection=0,PeakOpenTCP=1,Errors={},Protocols={'HTTP/1.1':1}),
                 IDSetSHA256=dict(attempted='a'*64,accepted='a'*64,sink_received='a'*64),
                 RequestSamples=[dict(StartedNS=0,FinishedNS=10,ElapsedNS=10,Status=202,Events=100,Accepted=True)])
        audit.verify_calibration_run(run,100,1_000_000_000)
        run['AcceptedEPS']=150
        with self.assertRaises(audit.AuditError):audit.verify_calibration_run(run,100,1_000_000_000)

    def test_observed_curve_cannot_exceed_verified_population(self):
        r,g,c=self.valid_pipeline()
        r['backlog']=[dict(elapsed_ns=30,Accepted=1,Visible=2,ObservationAvailable=True,FinalVerification=False)]
        with self.assertRaises(audit.AuditError):audit.pipeline(r,g,c)

    def test_historical_session_snapshots_are_unavailable_without_editing_raw(self):
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory)
            value=[{'runtime':{'database_sessions':{'status':'available','groups':[{'count':1}]},'consumer':{'status':'available','metrics':{'ingestkit_db_pool_empty_acquires_total':2}}}}]
            raw=json.dumps(value).encode();data=gzip.compress(raw,mtime=0);(root/'resources.gz').write_bytes(data)
            ref=dict(Path='resources.gz',Size=len(data),RawBytes=len(raw),SHA256=hashlib.sha256(data).hexdigest(),Codec='gzip')
            index={'Identity':{'SourceSHA':'043f584cf8638269a53afd5c9ec35a2721bc4148'},'Resources':[ref]}
            samples=audit.resource_samples(root,index)
            self.assertEqual(samples[0]['runtime']['database_sessions']['status'],'unavailable')
            self.assertNotIn('groups',samples[0]['runtime']['database_sessions'])
            self.assertEqual(samples[0]['runtime']['consumer'],value[0]['runtime']['consumer'])
            self.assertEqual((root/'resources.gz').read_bytes(),data)

    def test_population_indices(self):
        group={'Namespace':'n','Seed':42,'Count':2,'Indices':{'accepted':[0,1]}}
        self.assertEqual(len(audit.populations(group)['accepted']),2)
        group['Indices']['accepted']=[2]
        with self.assertRaises(audit.AuditError): audit.populations(group)

if __name__=='__main__': unittest.main()
