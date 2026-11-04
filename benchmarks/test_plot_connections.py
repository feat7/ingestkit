import importlib.util
from pathlib import Path
import unittest
spec=importlib.util.spec_from_file_location('plot_connections',Path(__file__).with_name('plot_connections.py'))
plot=importlib.util.module_from_spec(spec);spec.loader.exec_module(plot)
class PlotValidityTests(unittest.TestCase):
 def test_unavailable_probe_is_a_gap_and_verification_is_separate(self):
  r={'backlog':[dict(elapsed_ns=1_000_000_000,Visible=4,ObservationAvailable=True,FinalVerification=False),dict(elapsed_ns=2_000_000_000,Visible=4,ObservationAvailable=False,FinalVerification=False),dict(elapsed_ns=3_000_000_000,Visible=7,ObservationAvailable=True,FinalVerification=True)]}
  observed,verified=plot.observed_points(r)
  self.assertEqual(observed,[(1,4),(2,None)])
  self.assertEqual(verified,[(3,7)])
 def test_offset_snapshots_do_not_invent_zero_for_missing_or_racing_positions(self):
  self.assertEqual(plot.offset_lag({'Status':'available','End':{'0':10,'1':8,'2':4},'Committed':{'0':9,'1':3,'2':4}}),6)
  self.assertIsNone(plot.offset_lag({'Status':'unavailable'}))
  self.assertIsNone(plot.offset_lag({'Status':'available','End':{'0':10},'Committed':{'0':12}}))
if __name__=='__main__':unittest.main()
