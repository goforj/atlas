package eval

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// repairFinalizationArtifacts replaces terminal reports after manifest finalization fails so retained outcomes cannot contradict the API result.
func repairFinalizationArtifacts(artifacts *AttemptArtifacts, request AttemptRequest, result *AttemptResult) {
	if artifacts == nil || result == nil {
		return
	}
	root, err := openArtifactRepairRoot(artifacts)
	if err != nil {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_repair", Message: err.Error()})
		return
	}
	defer root.Close()
	if err := root.Remove(artifactManifestName); err != nil && !errors.Is(err, os.ErrNotExist) {
		result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: "artifact_manifest_cleanup", Message: err.Error()})
	}
	repairTextArtifact(root, artifacts, result, "summary.txt", "artifact_summary_repair", attemptSummary(request, *result))
	repairJSONArtifact(root, artifacts, result, "scorecard.json", "artifact_scorecard_repair", attemptScorecard(*result))
	if attemptNeedsTriage(*result) {
		repairJSONArtifact(root, artifacts, result, "triage.json", "artifact_triage_repair", TriageRecord{State: TriageUnreviewed})
	}
	repairJSONArtifact(root, artifacts, result, "run.json", "artifact_run_repair", *result)
}

// openArtifactRepairRoot binds terminal repairs to the directory created for this attempt even if its pathname is replaced later.
func openArtifactRepairRoot(artifacts *AttemptArtifacts) (*os.Root, error) {
	root, err := os.OpenRoot(artifacts.directory)
	if err != nil {
		return nil, fmt.Errorf("open attempt artifacts for repair: %w", err)
	}
	identity, err := root.Stat(".")
	if err != nil || !identity.IsDir() || !os.SameFile(artifacts.directoryIdentity, identity) {
		_ = root.Close()
		if err != nil {
			return nil, fmt.Errorf("inspect attempt artifacts for repair: %w", err)
		}
		return nil, fmt.Errorf("attempt artifact directory changed before repair")
	}
	return root, nil
}

// repairTextArtifact atomically replaces one fixed terminal report or removes the stale version when repair cannot complete.
func repairTextArtifact(root *os.Root, artifacts *AttemptArtifacts, result *AttemptResult, name, phase, content string) {
	body := []byte(artifacts.redactor.Text(content))
	repairArtifactBody(root, artifacts, result, name, phase, body)
}

// repairJSONArtifact preserves the normal recursive redaction boundary while replacing a terminal report.
func repairJSONArtifact(root *os.Root, artifacts *AttemptArtifacts, result *AttemptResult, name, phase string, value any) {
	body, err := json.MarshalIndent(value, "", "  ")
	if err == nil {
		var decoded any
		err = json.Unmarshal(body, &decoded)
		if err == nil {
			body, err = json.MarshalIndent(artifacts.redactor.JSONValue(decoded), "", "  ")
		}
	}
	if err != nil {
		recordArtifactRepairFailure(root, result, name, phase, fmt.Errorf("encode %s: %w", name, err))
		return
	}
	body = append(body, '\n')
	repairArtifactBody(root, artifacts, result, name, phase, body)
}

// repairArtifactBody applies the normal bounds and secret canary before publishing one replacement.
func repairArtifactBody(root *os.Root, artifacts *AttemptArtifacts, result *AttemptResult, name, phase string, body []byte) {
	if !artifacts.closed {
		recordArtifactRepairFailure(root, result, name, phase, fmt.Errorf("attempt artifacts are not finalized"))
		return
	}
	if !allowedArtifactFiles[name] || name == "events.jsonl" {
		recordArtifactRepairFailure(root, result, name, phase, fmt.Errorf("artifact name %q is not a terminal report", name))
		return
	}
	if len(body) > maxArtifactFileSize {
		recordArtifactRepairFailure(root, result, name, phase, fmt.Errorf("artifact %s exceeds %d bytes", name, maxArtifactFileSize))
		return
	}
	if err := artifacts.rejectRegisteredSecrets(body); err != nil {
		recordArtifactRepairFailure(root, result, name, phase, err)
		return
	}
	if err := replaceArtifactAtomically(root, name, body); err != nil {
		recordArtifactRepairFailure(root, result, name, phase, err)
	}
}

// recordArtifactRepairFailure removes contradictory stale output before retaining the repair failure in the returned result.
func recordArtifactRepairFailure(root *os.Root, result *AttemptResult, name, phase string, err error) {
	removeErr := root.Remove(name)
	message := err.Error()
	if removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		message = errors.Join(err, fmt.Errorf("remove stale %s: %w", name, removeErr)).Error()
	}
	result.SecondaryFailures = append(result.SecondaryFailures, SecondaryFailure{Phase: phase, Message: message})
}

// replaceArtifactAtomically keeps a failed terminal repair from truncating the last complete report in place.
func replaceArtifactAtomically(root *os.Root, name string, body []byte) error {
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		return fmt.Errorf("name replacement %s: %w", name, err)
	}
	temporaryName := "." + name + "-" + hex.EncodeToString(random)
	temporary, err := root.OpenFile(temporaryName, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create replacement %s: %w", name, err)
	}
	defer root.Remove(temporaryName)
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
	if err := root.Rename(temporaryName, name); err != nil {
		return fmt.Errorf("publish replacement %s: %w", name, err)
	}
	return nil
}
