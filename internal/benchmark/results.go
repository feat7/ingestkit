package benchmark

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
)

func (r DatabaseReport) Validate() error {
	if r.Repetitions < 3 || len(r.Runs) != 3*r.Repetitions {
		return errors.New("report requires every method/repetition")
	}
	seen := map[string]bool{}
	for _, v := range r.Runs {
		key := fmt.Sprintf("%s/%d", v.Method, v.Repetition)
		if !slices.Contains(methods, v.Method) || v.Repetition < 1 || v.Repetition > r.Repetitions || seen[key] || !v.Correct || v.ElapsedNS <= 0 || v.Submitted <= 0 || v.Inserted < 0 || v.Duplicates < 0 || v.Inserted+v.Duplicates != int64(v.Submitted) {
			return errors.New("incorrect or incomplete database run")
		}
		seen[key] = true
	}
	return nil
}

type DatabaseSummary struct {
	Method             string  `json:"method"`
	MedianElapsedNS    int64   `json:"median_elapsed_ns"`
	MedianSubmittedEPS float64 `json:"median_submitted_eps"`
}

func (r DatabaseReport) Summaries() []DatabaseSummary {
	out := []DatabaseSummary{}
	for _, m := range methods {
		var ns []int64
		var eps []float64
		for _, v := range r.Runs {
			if v.Method == m {
				ns = append(ns, v.ElapsedNS)
				eps = append(eps, float64(v.Submitted)*1e9/float64(v.ElapsedNS))
			}
		}
		slices.Sort(ns)
		slices.Sort(eps)
		if len(ns) > 0 {
			out = append(out, DatabaseSummary{m, ns[len(ns)/2], eps[len(eps)/2]})
		}
	}
	return out
}
func WriteReport(dir string, report any) error {
	if e := os.MkdirAll(dir, 0755); e != nil {
		return e
	}
	name := "report"
	var records [][]string
	switch r := report.(type) {
	case PipelineReport:
		if len(r.Runs) < 3 {
			return errors.New("pipeline report requires three repetitions")
		}
		name = "pipeline"
		records = append(records, []string{"repetition", "planned", "attempted", "accepted", "persisted", "generator_missed", "ambiguous", "rejected", "window_ns", "total_ns", "drain_ns", "ack_p95_ns", "visibility_p95_ns", "correct", "outcome", "accepted_in_window", "accepted_after_window", "attempted_after_window", "ambiguous_unobserved_at_final_verification", "observer_polls", "observer_sql_statements", "observer_elapsed_ns", "observed_events", "verification_ns"})
		for _, v := range r.Runs {
			if !v.Correct {
				return errors.New("incorrect pipeline run")
			}
			records = append(records, []string{strconv.Itoa(v.Repetition), strconv.Itoa(v.Planned), strconv.Itoa(v.Attempted), strconv.Itoa(v.Accepted), strconv.Itoa(v.Persisted), strconv.Itoa(v.GeneratorMissed), strconv.Itoa(v.Ambiguous), strconv.Itoa(v.Rejected), strconv.FormatInt(v.WindowNS, 10), strconv.FormatInt(v.TotalNS, 10), strconv.FormatInt(v.DrainNS, 10), strconv.FormatInt(v.Ack.P95, 10), strconv.FormatInt(v.Visibility.P95, 10), strconv.FormatBool(v.Correct), v.Outcome, strconv.Itoa(v.AcceptedInWindow), strconv.Itoa(v.AcceptedAfterWindow), strconv.Itoa(v.AttemptedAfterWindow), strconv.Itoa(v.AmbiguousUnobserved), strconv.FormatInt(v.ObserverPolls, 10), strconv.FormatInt(v.ObserverQueries, 10), strconv.FormatInt(v.ObserverQueryNS, 10), strconv.Itoa(v.ObservedEvents), strconv.FormatInt(v.VerificationNS, 10)})
		}
	case DatabaseReport:
		if e := r.Validate(); e != nil {
			return e
		}
		name = "database"
		records = append(records, []string{"method", "repetition", "submitted", "inserted", "duplicates", "elapsed_ns", "correct"})
		for _, v := range r.Runs {
			records = append(records, []string{v.Method, strconv.Itoa(v.Repetition), strconv.Itoa(v.Submitted), strconv.FormatInt(v.Inserted, 10), strconv.FormatInt(v.Duplicates, 10), strconv.FormatInt(v.ElapsedNS, 10), strconv.FormatBool(v.Correct)})
		}
	}
	raw, e := json.MarshalIndent(report, "", "  ")
	if e != nil {
		return e
	}
	if e = os.WriteFile(filepath.Join(dir, name+".json"), append(raw, '\n'), 0644); e != nil {
		return e
	}
	if records != nil {
		f, e := os.Create(filepath.Join(dir, name+".csv"))
		if e != nil {
			return e
		}
		w := csv.NewWriter(f)
		e = w.WriteAll(records)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		return closeErr
	}
	return nil
}
