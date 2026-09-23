package integration_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

// -----------------------------------------------------------------------------
// Scenario C: Broken Deployment Smoke Test (PRD Section 26)
// -----------------------------------------------------------------------------
// Verifies that a broken container/deployment that fails health probes causes
// smoke-test.sh to fail with exit code 1, halting CI/CD promotion to production.
func TestIntegration_Drill_ScenarioC_BrokenDeploymentSmokeTest(t *testing.T) {
	// Create a broken mock server where /health returns 500 Internal Server Error
	brokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"status":"fatal_startup_crash"}`, http.StatusInternalServerError)
	}))
	defer brokenServer.Close()

	// Run smoke test script with reduced retries against broken server
	cmd := exec.Command("../../scripts/smoke-test.sh", brokenServer.URL)
	cmd.Env = append(cmd.Environ(),
		"MAX_RETRIES=2",
		"RETRY_DELAY_SEC=1",
	)

	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	err := cmd.Run()
	if err == nil {
		t.Fatalf("expected smoke-test.sh to fail against broken server, but it succeeded: %s", outputBuf.String())
	}

	outStr := outputBuf.String()
	if !strings.Contains(outStr, "[FAIL]") || !strings.Contains(outStr, "Liveness probe failed") {
		t.Errorf("expected smoke-test output to report liveness probe failure, got: %s", outStr)
	}
}

// -----------------------------------------------------------------------------
// Scenario D: Production Regression & Rapid Rollback Drill (PRD Section 26)
// -----------------------------------------------------------------------------
// Verifies that rollback.sh can be invoked to shift 100% traffic back to the
// previous stable revision, validating parameter handling and execution safety.
func TestIntegration_Drill_ScenarioD_ProductionRollbackExecution(t *testing.T) {
	cmd := exec.Command("../../scripts/rollback.sh",
		"--env", "staging",
		"--app", "api",
		"--target-revision", "ca-api-lensio-staging--stable",
		"--traffic", "100",
		"--dry-run",
		"--no-verify",
	)

	var outputBuf bytes.Buffer
	cmd.Stdout = &outputBuf
	cmd.Stderr = &outputBuf

	if err := cmd.Run(); err != nil {
		t.Fatalf("rollback.sh execution failed: %v. Output: %s", err, outputBuf.String())
	}

	outStr := outputBuf.String()
	if !strings.Contains(outStr, "Initiating traffic shift") || !strings.Contains(outStr, "DRY-RUN") {
		t.Errorf("unexpected rollback script output: %s", outStr)
	}
	// "az containerapp revision set-traffic" does not exist; the shift is an ingress operation.
	if !strings.Contains(outStr, "az containerapp ingress traffic set") || !strings.Contains(outStr, "--revision-weight ca-api-lensio-staging--stable=100") {
		t.Errorf("rollback must shift traffic with az containerapp ingress traffic set: %s", outStr)
	}
}

// Revision names are per app, so one explicit name can never be right for both
// the API and the dashboard; only "previous" (resolved per app) is accepted.
func TestIntegration_Drill_ScenarioD_RollbackAllRequiresPrevious(t *testing.T) {
	cmd := exec.Command("../../scripts/rollback.sh",
		"--env", "staging",
		"--app", "all",
		"--target-revision", "ca-api-lensio-staging--stable",
		"--dry-run",
		"--no-verify",
	)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected rollback.sh to reject --app all with an explicit revision, got success: %s", out)
	}
	if !strings.Contains(string(out), "--app all needs --target-revision previous") {
		t.Errorf("unexpected rejection output: %s", out)
	}

	cmd = exec.Command("../../scripts/rollback.sh",
		"--env", "staging",
		"--app", "all",
		"--target-revision", "previous",
		"--dry-run",
		"--no-verify",
	)
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("expected --app all --target-revision previous to succeed: %v: %s", err, out)
	}
	for _, app := range []string{"ca-api-lensio-staging", "ca-dash-lensio-staging"} {
		if !strings.Contains(string(out), "--name "+app) {
			t.Errorf("expected a traffic shift for %s: %s", app, out)
		}
	}
}
