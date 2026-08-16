package eval

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// repairFinalizationArtifacts replaces terminal reports after manifest finalization fails so retained outcomes cannot contradict the API result.
func repairFinalizationArtifacts(artifacts *AttemptArtifacts, request AttemptRequest, result *AttemptResult) {
	if artifacts == nil || result == nil {
		return
	}
	manifestPath := filepath.Join(artifacts.directory, "manifest.json")
	if err := os.Remove(manifestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_manifest_cleanup", Message: err.Error()})
	}
	repairTextArtifact(artifacts, result, "summary.txt", "artifact_summary_repair", attemptSummary(request, *result))
	repairJSONArtifact(artifacts, result, "scorecard.json", "artifact_scorecard_repair", attemptScorecard(*result))
	if attemptNeedsTriage(*result) {
		repairJSONArtifact(artifacts, result, "triage.json", "artifact_triage_repair", TriageRecord{State: TriageUnreviewed})
	}
	repairJSONArtifact(artifacts, result, "run.json", "artifact_run_repair", *result)
}

// repairTextArtifact atomically replaces one fixed terminal report or removes the stale version when repair cannot complete.
func repairTextArtifact(artifacts *AttemptArtifacts, result *AttemptResult, name, phase, content string) {
	body := []byte(artifacts.redactor.Text(content))
	repairArtifactBody(artifacts, result, name, phase, body)
}

// repairJSONArtifact preserves the normal recursive redaction boundary while replacing a terminal report.
func repairJSONArtifact(artifacts *AttemptArtifacts, result *AttemptResult, name, phase string, value any) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		var decoded any
		err = json.Unmarshal(body, &decoded)
		if err == nil {
			body, err = json.MarshalIndent(artifacts.redactor.JSONValue(decoded), "", "  ")
		}
	}
	if err != nil {
		recordArtifactRepairFailure(artifacts, result, name, phase, fmt.Errorf("encode %s: %w", name, err))
		return
	}
	body = append(body, '\n')
	repairArtifactBody(artifacts, result, name, phase, body)
}

// repairArtifactBody applies the normal bounds and secret canary before publishing one replacement.
func repairArtifactBody(artifacts *AttemptArtifacts, result *AttemptResult, name, phase string, body []byte) {
	if !artifacts.closed {
		recordArtifactRepairFailure(artifacts, result, name, phase, fmt.Errorf("attempt artifacts are not finalized"))
		return
	}
	if !allowedArtifactFiles[name] || name == "events.jsonl" {
		recordArtifactRepairFailure(artifacts, result, name, phase, fmt.Errorf("artifact name %q is not a terminal report", name))
		return
	}
	if len(body) > maxArtifactFileSize {
		recordArtifactRepairFailure(artifacts, result, name, phase, fmt.Errorf("artifact %s exceeds %d bytes", name, maxArtifactFileSize))
		return
	}
	if err := artifacts.rejectRegisteredSecrets(body); err != nil {
		recordArtifactRepairFailure(artifacts, result, name, phase, err)
		return
	}
	if err := replaceArtifactAtomically(artifacts.directory, name, body); err != nil {
		recordArtifactRepairFailure(artifacts, result, name, phase, err)
	}
}

// recordArtifactRepairFailure removes contradictory stale output before retaining the repair failure in the returned result.
func recordArtifactRepairFailure(artifacts *AttemptArtifacts, result *AttemptResult, name, phase string, err error) {
	path := filepath.Join(artifacts.directory, name)
	removeErr := os.Remove(path)
	message := err.Error()
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		message = errors.Join(err, fmt.Errorf("remove stale %s: %w", name, removeErr)).Error()
	}
	result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: phase, Message: message})
}

// replaceArtifactAtomically keeps a failed terminal repair from truncating the last complete report in place.
func replaceArtifactAtomically(directory, name string, body []byte) error {
	temporary, err := os.CreateTemp(directory, "."+name+"-*")
	if err != nil {
		return fmt.Errorf("create replacement %s: %w", name, err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure replacement %s: %w", name, err)
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write replacement %s: %w", name, err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync replacement %s: %w", name, err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close replacement %s: %w", name, err)
	}
	if err := os.Rename(temporaryPath, filepath.Join(directory, name)); err != nil {
		return fmt.Errorf("publish replacement %s: %w", name, err)
	}
	return nil
}
