import copy
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('audit_study', Path(__file__).with_name('audit_study.py'))
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)

class ArtifactAuditTests(unittest.TestCase):
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
        group={'Namespace':'n','Seed':42,'Count':1,'Indices':{k:[0] for k in ('attempted','accepted','stored')}}
        group['Indices']['ambiguous']=[]
        sets=audit.populations(group)
        r={k:0 for k in ('Missing','Unexpected','DuplicateRows','ContentMismatch','GeneratorMissed','Ambiguous','Rejected','AttemptedAfterWindow','AcceptedAfterWindow','AmbiguousUnobserved','LateSchedules','ObserverLatePolls','ObserverOutages')}
        r.update(correct=True,Planned=1,Attempted=1,Accepted=1,Persisted=1,ObservedEvents=1,
                 AttemptedInWindow=1,AcceptedInWindow=1,WindowNS=100,TotalNS=110,DrainNS=10,
                 AcceptedEPS=1e7,PersistedEPS=1e9/110,outcome='complete',
                 ack_ns=[10],visibility_ns=[20],backlog=[],id_set_sha256={k:audit.id_hash(v) for k,v in sets.items()},
                 request_samples=[dict(StartedNS=0,FinishedNS=10,ElapsedNS=10,Events=1,Accepted=True,Ambiguous=False,Status=202)],
                 ack_latency={k:10 for k in ('p50_ns','p95_ns','p99_ns')},
                 receipt_to_observed_storage_upper_bound={k:20 for k in ('p50_ns','p95_ns','p99_ns')})
        case={'Count':1,'BatchSize':1}
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

    def test_population_indices(self):
        group={'Namespace':'n','Seed':42,'Count':2,'Indices':{'accepted':[0,1]}}
        self.assertEqual(len(audit.populations(group)['accepted']),2)
        group['Indices']['accepted']=[2]
        with self.assertRaises(audit.AuditError): audit.populations(group)

if __name__=='__main__': unittest.main()
