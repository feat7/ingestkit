package benchmark

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/feat7/ingestkit/internal/runenv"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type StudyConfig struct {
	OutputDirectory string
	Smoke, Resume   bool
	Seed            int64
	Writer          io.Writer
	OnCaseStart     func(Case, *runenv.Stack)
}
type StudyCaseResult struct {
	Case     Case
	Status   string
	Attempt  int
	Artifact ArtifactRef
	History  []CaseAttempt
	Summary  json.RawMessage
}
type StudyIndex struct {
	MetadataArtifacts []ArtifactRef
	Schema            int
	Status            string
	Identity          StudyIdentity
	RunNamespace      string
	ManifestFiles     []string
	Resources         []ArtifactRef
	Cases             []StudyCaseResult
	Notes             []string
}
type CaseAttempt struct {
	Attempt            int
	Status, ErrorClass string
	Artifact           ArtifactRef
}
type StudyCapture struct {
	Schema                        int
	Identity                      StudyIdentity
	Case                          Case
	Namespace, Status, ErrorClass string
	Report                        any
	Populations                   []PopulationEvidence
}
type PopulationEvidence struct {
	Repetition, Count int
	Namespace         string
	Seed              int64
	Indices           map[string][]int       `json:",omitempty"`
	Explicit          map[string][]uuid.UUID `json:",omitempty"`
	Extra             map[string][]uuid.UUID `json:",omitempty"`
}

func pipelineCaseConfig(ctx context.Context, s *runenv.Stack, p *pgxpool.Pool, client *http.Client, c Case, seed int64, namespace string) PipelineConfig {
	return PipelineConfig{Pool: p, HTTPClient: client, APIURL: s.APIURL, APIKey: s.APIKey(), Tenant: "default", Namespace: namespace, PayloadProfile: c.PayloadProfile, Seed: seed, Rate: c.Rate, Concurrency: c.Concurrency, BatchSize: c.BatchSize, PayloadBytes: c.PayloadBytes, Repetitions: c.Repetitions, Duration: c.Window, Warmup: c.Warmup, DrainDeadline: c.Drain, PollInterval: c.Poll, Pattern: c.Pattern, ReconnectObserver: c.Recovery == "database", SampleFinished: func(int) error { return waitConsumerCaughtUp(ctx, s) }}
}
func doneCase(status string) bool { return status == "complete" || status == "limited" }
func validateStudyRepetitions(c Case, value any) error {
	seen := map[int]bool{}
	switch r := value.(type) {
	case DatabaseReport:
		if err := r.Validate(); err != nil {
			return err
		}
		if r.Repetitions != c.Repetitions {
			return errors.New("database repetition controls differ")
		}
		return nil
	case PipelineReport:
		for _, run := range r.Runs {
			if !run.Correct || run.Repetition < 1 || run.Repetition > c.Repetitions || seen[run.Repetition] {
				return errors.New("missing, duplicate or incorrect pipeline repetition")
			}
			seen[run.Repetition] = true
		}
	case RecoveryReport:
		for _, run := range r.Runs {
			if !run.Correct || run.Repetition < 1 || run.Repetition > c.Repetitions || seen[run.Repetition] {
				return errors.New("missing, duplicate or incorrect recovery repetition")
			}
			seen[run.Repetition] = true
		}
	default:
		return errors.New("unknown study report type")
	}
	if len(seen) != c.Repetitions {
		return errors.New("not every measured repetition was retained")
	}
	return nil
}
func completedRepetitions(report any) []int {
	var completed []int
	switch r := report.(type) {
	case DatabaseReport:
		counts := map[int]int{}
		for _, run := range r.Runs {
			if run.Correct {
				counts[run.Repetition]++
			}
		}
		for rep, count := range counts {
			if count == 3 {
				completed = append(completed, rep)
			}
		}
	case PipelineReport:
		for _, run := range r.Runs {
			if run.Correct {
				completed = append(completed, run.Repetition)
			}
		}
	case RecoveryReport:
		for _, run := range r.Runs {
			if run.Correct {
				completed = append(completed, run.Repetition)
			}
		}
	}
	slices.Sort(completed)
	return completed
}
func stripPipelineSamples(run *PipelineRun) {
	run.AckNS = nil
	run.VisibilityNS = nil
	run.Backlog = nil
	run.RequestSamples = nil
}
func reportSummary(value any) any {
	switch r := value.(type) {
	case DatabaseReport:
		r.Runs = slices.Clone(r.Runs)
		for i := range r.Runs {
			r.Runs[i].BatchNS = nil
		}
		return r
	case PipelineReport:
		r.Runs = slices.Clone(r.Runs)
		for i := range r.Runs {
			stripPipelineSamples(&r.Runs[i])
		}
		return r
	case RecoveryReport:
		r.Runs = slices.Clone(r.Runs)
		for i := range r.Runs {
			if r.Runs[i].Pipeline != nil {
				sample := *r.Runs[i].Pipeline
				stripPipelineSamples(&sample)
				r.Runs[i].Pipeline = &sample
			}
		}
		return r
	default:
		return nil
	}
}
func studyOutcome(value any) string {
	switch r := value.(type) {
	case PipelineReport:
		for _, run := range r.Runs {
			if run.Outcome == "limited" {
				return "limited"
			}
		}
	case RecoveryReport:
		for _, run := range r.Runs {
			if run.Pipeline != nil && run.Pipeline.Outcome == "limited" {
				return "limited"
			}
			for _, fault := range run.Faults {
				if strings.HasSuffix(fault.Action, "_requested") && fault.ElapsedNS-fault.PlannedNS > int64(250*time.Millisecond) {
					return "limited"
				}
			}
		}
	}
	return "complete"
}
func populations(c Case, report any, namespace string, seed int64) []PopulationEvidence {
	var result []PopulationEvidence
	add := func(rep int, sets map[string][]uuid.UUID, explicit bool) {
		population := PopulationEvidence{Repetition: rep, Count: c.Count, Namespace: namespace, Seed: seed + int64(rep)}
		if explicit {
			population.Explicit = sets
			result = append(result, population)
			return
		}
		lookup := make(map[uuid.UUID]int, c.Count)
		for i := 0; i < c.Count; i++ {
			id := uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%s/%d/%d", namespace, population.Seed, i)))
			lookup[id] = i
		}
		population.Indices = map[string][]int{}
		population.Extra = map[string][]uuid.UUID{}
		for name, ids := range sets {
			population.Indices[name] = []int{}
			for _, id := range ids {
				if index, ok := lookup[id]; ok {
					population.Indices[name] = append(population.Indices[name], index)
				} else {
					population.Extra[name] = append(population.Extra[name], id)
				}
			}
		}
		result = append(result, population)
	}
	switch r := report.(type) {
	case PipelineReport:
		for _, run := range r.Runs {
			add(run.Repetition, map[string][]uuid.UUID{"attempted": run.AttemptedIDs, "accepted": run.AcceptedIDs, "ambiguous": run.AmbiguousIDs, "stored": run.StoredIDs}, false)
		}
	case RecoveryReport:
		for _, run := range r.Runs {
			sets := map[string][]uuid.UUID{"accepted": run.AcceptedIDs, "stored": run.StoredIDs, "ambiguous": run.AmbiguousIDs}
			if run.Pipeline != nil {
				sets["attempted"] = run.Pipeline.AttemptedIDs
				sets["ambiguous"] = run.Pipeline.AmbiguousIDs
			}
			add(run.Repetition, sets, c.Recovery == "outbox")
		}
	}
	return result
}
func writeStudyIndex(out string, j StudyJournal, cases []Case, status string) error {
	index := StudyIndex{Schema: 2, Status: status, Identity: j.Identity, RunNamespace: j.RunNamespace, ManifestFiles: j.ManifestFiles, Resources: j.Resources, MetadataArtifacts: j.MetadataArtifacts, Notes: []string{"Smoke runs verify correctness only. Full runs retain every configured repetition; limited outcomes are not capacity claims.", "Raw captures contain empirical samples and UUID population indices (or explicit outbox UUIDs); gzip mtime=0 with SHA256. Submitted IDs regenerate from namespace/seed/index.", "Failed and interrupted attempts remain in case History. A resumed incomplete case restarts all three repetitions on a fresh owned stack; completed captures are immutable."}}
	for _, c := range cases {
		progress := j.Cases[c.ID]
		index.Cases = append(index.Cases, StudyCaseResult{Case: c, Status: progress.Status, Attempt: progress.Attempt, Artifact: progress.Artifact, History: progress.History, Summary: progress.Summary})
	}
	if err := atomicJSON(filepath.Join(out, "index.json"), index); err != nil {
		return err
	}
	file, err := os.CreateTemp(out, ".summary-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	writer := csv.NewWriter(file)
	err = writer.Write([]string{"case", "kind", "status", "attempt", "completed_repetitions", "artifact", "sha256"})
	for _, c := range cases {
		p := j.Cases[c.ID]
		if e := writer.Write([]string{c.ID, c.Kind, p.Status, strconv.Itoa(p.Attempt), strconv.Itoa(len(p.CompletedRepetitions)), p.Artifact.Path, p.Artifact.SHA256}); err == nil {
			err = e
		}
	}
	writer.Flush()
	err = errors.Join(err, writer.Error(), file.Sync(), file.Close())
	if err != nil {
		return err
	}
	return os.Rename(file.Name(), filepath.Join(out, "summary.csv"))
}
func RunStudy(ctx context.Context, cfg StudyConfig) (result error) {
	root, err := runenv.Root()
	if err != nil {
		return err
	}
	out := cfg.OutputDirectory
	if out == "" {
		out = filepath.Join(root, "tmp", "studies", uuid.NewString())
	}
	out, err = filepath.Abs(out)
	if err != nil {
		return err
	}
	if out == root {
		return errors.New("study output must be a dedicated directory")
	}
	existing, err := os.ReadDir(out)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if !cfg.Resume && len(existing) > 0 {
		return errors.New("refusing existing study output; use resume or a new directory")
	}
	if cfg.OnCaseStart != nil && !cfg.Smoke {
		return errors.New("custom study hooks are restricted to correctness smoke runs")
	}
	cases := StudyCases()
	mode := "full"
	if cfg.Smoke {
		cases = smokeCases()
		mode = "smoke"
	}
	sha, sourceHash, dirty, err := sourceIdentity(ctx, root, out)
	if err != nil {
		return err
	}
	if dirty && !cfg.Smoke {
		return errors.New("commit all source inputs before running a full study")
	}
	identity := StudyIdentity{SourceSHA: sha, SourceFingerprint: sourceHash, ControlsFingerprint: fingerprint(cases), Mode: mode, Seed: cfg.Seed, SourceDirty: dirty}
	journalPath := filepath.Join(out, "journal.json")
	journal := StudyJournal{Schema: 2, Identity: identity, RunNamespace: uuid.NewString(), Cases: map[string]CaseProgress{}}
	if cfg.Resume {
		journal, err = readJournal(journalPath)
		if err != nil {
			return err
		}
		preflight := identity
		preflight.ImagesFingerprint = journal.Identity.ImagesFingerprint
		preflight.ConfigFingerprint = journal.Identity.ConfigFingerprint
		if err = journal.Compatible(preflight); err != nil {
			return err
		}
		known := map[string]bool{}
		allDone := true
		for _, c := range cases {
			known[c.ID] = true
			p := journal.Cases[c.ID]
			if !doneCase(p.Status) {
				allDone = false
				continue
			}
			if err = VerifyArtifact(out, p.Artifact); err != nil {
				return err
			}
		}
		for id := range journal.Cases {
			if !known[id] {
				return errors.New("journal contains an unknown case")
			}
		}
		for _, ref := range append(slices.Clone(journal.Resources), journal.MetadataArtifacts...) {
			if err = VerifyArtifact(out, ref); err != nil {
				return err
			}
		}
		if allDone {
			if journal.Status != "complete" {
				return errors.New("case captures exist but prior finalization was not verified; inspect private evidence")
			}
			return nil
		}
	} else {
		for _, c := range cases {
			journal.Cases[c.ID] = CaseProgress{Status: "pending"}
		}
		if err = atomicJSON(journalPath, journal); err != nil {
			return err
		}
	}
	rawDir := filepath.Join(out, "raw")
	if info, e := os.Lstat(rawDir); e == nil && !info.IsDir() {
		return errors.New("raw capture path must be a real directory")
	}
	if err = os.MkdirAll(rawDir, 0755); err != nil {
		return err
	}
	writer := cfg.Writer
	if writer == nil {
		writer = io.Discard
	}
	fmt.Fprintf(writer, "study output: %s\nstudy %s: %d %s cases\n", out, journal.RunNamespace, len(cases), mode)
	s, err := runenv.Start(ctx, runenv.Options{Purpose: "study", Profile: "selfhost", ExposeDependencies: true})
	if err != nil {
		return err
	}
	bound := false
	var resourceCancel context.CancelFunc
	var resourceDone <-chan []ResourceSample
	session := journal.ExecutionAttempts + 1
	defer func() {
		if resourceCancel != nil {
			resourceCancel()
			samples := <-resourceDone
			if bound {
				ref, e := WriteCompressed(filepath.Join(out, fmt.Sprintf("resources-%d.json.gz", session)), samples)
				if e == nil {
					journal.Resources = append(journal.Resources, ref)
				}
				result = errors.Join(result, e)
			}
		}
		restoreCtx, restoreEnd := context.WithTimeout(context.Background(), 20*time.Second)
		for _, service := range []string{"postgres", "redpanda", "api", "consumer"} {
			result = errors.Join(result, s.SetServiceRunning(restoreCtx, service, true))
		}
		restoreEnd()
		evidence, evidenceEnd := context.WithTimeout(context.Background(), 10*time.Second)
		evidenceErr := s.CollectEvidence(evidence, s.Directory)
		evidenceEnd()
		cleanup, cleanupEnd := context.WithTimeout(context.Background(), 30*time.Second)
		cleanupErr := s.Destroy(cleanup)
		cleanupEnd()
		result = errors.Join(result, evidenceErr, cleanupErr)
		if bound {
			check, checkEnd := context.WithTimeout(context.Background(), 10*time.Second)
			lastSHA, lastHash, _, e := sourceIdentity(check, root, out)
			checkEnd()
			if e != nil || lastSHA != sha || lastHash != sourceHash {
				result = errors.Join(result, errors.New("source changed during study"))
			}
			status := "complete"
			if result != nil {
				status = "failed"
				if ctx.Err() != nil {
					status = "incomplete"
				}
			}
			journal.Status = status
			result = errors.Join(result, atomicJSON(journalPath, journal), writeStudyIndex(out, journal, cases, status))
		}
		fmt.Fprintf(writer, "owned stack evidence: %s\n", s.Directory)
	}()
	poolConfig, err := pgxpool.ParseConfig(s.DatabaseURL)
	if err != nil {
		return err
	}
	poolConfig.ConnConfig.RuntimeParams["application_name"] = "ingestkit-benchmark"
	p, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return err
	}
	defer p.Close()
	manifest, err := CaptureManifest(ctx, s, p)
	if err != nil {
		return err
	}
	manifest.SourceSHA = sha
	manifest.SourceDirty = dirty
	manifest.Settings["source_cleanliness"] = "Git status excludes only the owned output directory; actual source-file fingerprint is checked again after execution"
	if diagnostic, e := json.MarshalIndent(manifest, "", "  "); e == nil {
		_ = os.WriteFile(filepath.Join(s.Directory, "runtime-manifest.json"), diagnostic, 0600)
	}
	identity.ImagesFingerprint, err = imageFingerprint(ctx, manifest)
	if err != nil {
		return err
	}
	identity.ConfigFingerprint = runtimeFingerprint(manifest)
	if cfg.Resume && journal.Identity.ImagesFingerprint != "" {
		if err = journal.Compatible(identity); err != nil {
			return err
		}
	}
	journal.Identity = identity
	bound = true
	journal.ExecutionAttempts = session
	journal.Status = "running"
	if err = atomicJSON(journalPath, journal); err != nil {
		return err
	}
	manifestFile := fmt.Sprintf("manifest-%d.json", session)
	metadata, err := WriteJSONArtifact(filepath.Join(out, manifestFile), manifest)
	if err != nil {
		return err
	}
	journal.MetadataArtifacts = append(journal.MetadataArtifacts, metadata)
	journal.ManifestFiles = append(journal.ManifestFiles, manifestFile)
	if err = atomicJSON(journalPath, journal); err != nil {
		return err
	}
	sampleCtx, cancel := context.WithCancel(ctx)
	resourceCancel = cancel
	resourceDone = SampleResources(sampleCtx, s, p)
	client, err := s.HTTPClient()
	if err != nil {
		return err
	}
	defer client.CloseIdleConnections()
	for _, c := range cases {
		progress := journal.Cases[c.ID]
		if doneCase(progress.Status) {
			continue
		}
		if err = ctx.Err(); err != nil {
			return err
		}
		progress.Attempt++
		progress.Status = "running"
		progress.ErrorClass = ""
		journal.Cases[c.ID] = progress
		if err = atomicJSON(journalPath, journal); err != nil {
			return err
		}
		fmt.Fprintf(writer, "case %s attempt %d\n", c.ID, progress.Attempt)
		if cfg.OnCaseStart != nil {
			cfg.OnCaseStart(c, s)
		}
		namespace := journal.RunNamespace + "/" + c.ID
		var report any
		var measurementErr error
		caseCtx, caseEnd := context.WithTimeout(ctx, 6*time.Minute)
		switch c.Kind {
		case "database":
			report, measurementErr = RunDatabase(caseCtx, DatabaseConfig{Pool: p, Workload: WorkloadConfig{Namespace: namespace, PayloadProfile: c.PayloadProfile, Seed: cfg.Seed, Count: c.Count, PayloadBytes: c.PayloadBytes, DuplicateFraction: c.DuplicateFraction}, BatchSize: c.BatchSize, Repetitions: c.Repetitions})
		case "pipeline":
			report, measurementErr = RunPipeline(caseCtx, pipelineCaseConfig(caseCtx, s, p, client, c, cfg.Seed, namespace))
		case "recovery":
			report, measurementErr = RunRecoveryCase(caseCtx, s, p, c, cfg.Seed, namespace)
		default:
			measurementErr = errors.New("unknown study case")
		}
		caseEnd()
		if measurementErr == nil || verifiedObservationOnly(c, report, measurementErr) {
			measurementErr = errors.Join(measurementErr, validateStudyRepetitions(c, report))
		}
		progress.CompletedRepetitions = completedRepetitions(report)
		progress.Status = studyOutcome(report)
		observationLimited := verifiedObservationOnly(c, report, measurementErr)
		if measurementErr != nil {
			_ = os.WriteFile(filepath.Join(s.Directory, fmt.Sprintf("%s-attempt-%d-error.txt", c.ID, progress.Attempt)), []byte(measurementErr.Error()), 0600)
			progress.Status = "failed"
			progress.ErrorClass = "measurement_failed"
			if observationLimited {
				progress.Status = "limited"
				progress.ErrorClass = "observation_failed"
			}
			if ctx.Err() != nil {
				progress.Status = "incomplete"
				progress.ErrorClass = "interrupted"
			}
		}
		summary, e := json.Marshal(reportSummary(report))
		if e != nil {
			return e
		}
		progress.Summary = summary
		capture := StudyCapture{Schema: 2, Identity: identity, Case: c, Namespace: namespace, Status: progress.Status, ErrorClass: progress.ErrorClass, Report: report, Populations: populations(c, report, namespace, cfg.Seed)}
		path := filepath.Join(rawDir, fmt.Sprintf("%s-attempt-%d.json.gz", c.ID, progress.Attempt))
		ref, e := WriteCompressed(path, capture)
		if e != nil {
			return e
		}
		ref.Path = filepath.ToSlash(filepath.Join("raw", ref.Path))
		progress.Artifact = ref
		progress.History = append(progress.History, CaseAttempt{Attempt: progress.Attempt, Status: progress.Status, ErrorClass: progress.ErrorClass, Artifact: ref})
		journal.Cases[c.ID] = progress
		if e = atomicJSON(journalPath, journal); e != nil {
			return e
		}
		if e = writeStudyIndex(out, journal, cases, "incomplete"); e != nil {
			return e
		}
		fmt.Fprintf(writer, "case %s: %s (%d repetitions retained)\n", c.ID, progress.Status, len(progress.CompletedRepetitions))
		if measurementErr != nil && !observationLimited {
			return fmt.Errorf("case %s failed; retained capture and private evidence", c.ID)
		}
	}
	return nil
}
