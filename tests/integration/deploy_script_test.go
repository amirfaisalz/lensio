package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fakeAz answers the az calls deploy.sh makes and records every invocation.
// AZ_MODE=ready reports the new revision ready from the second poll onward;
// AZ_MODE=stuck never does, like a revision that crashes on boot.
const fakeAz = `#!/bin/sh
echo "$*" >> "$AZ_LOG"
case "$*" in
  # Idle revisions include the latest (rev2), as after a rollback.
  "containerapp revision list"*) printf 'old1\nrev2\n' ;;
  *latestReadyRevisionName*)
    n=$(cat "$AZ_STATE" 2>/dev/null || echo 0); n=$((n+1)); echo "$n" > "$AZ_STATE"
    if [ "$AZ_MODE" = ready ] && [ "$n" -ge 2 ]; then printf 'rev2\nrev2\n'; else printf 'rev2\nrev1\n'; fi ;;
  "containerapp show"*) echo rev2 ;;
esac
exit 0
`

func runDeploy(t *testing.T, mode string, args ...string) (output, azLog string, err error) {
	t.Helper()
	dir := t.TempDir()
	if werr := os.WriteFile(filepath.Join(dir, "az"), []byte(fakeAz), 0o755); werr != nil {
		t.Fatalf("writing fake az: %v", werr)
	}
	logPath := filepath.Join(dir, "az.log")
	cmd := exec.Command("../../scripts/deploy.sh", args...)
	cmd.Env = append(os.Environ(),
		"PATH="+dir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"AZ_LOG="+logPath,
		"AZ_STATE="+filepath.Join(dir, "state"),
		"AZ_MODE="+mode,
		"READY_TIMEOUT_SEC=3",
	)
	out, err := cmd.CombinedOutput()
	logBytes, _ := os.ReadFile(logPath)
	return string(out), string(logBytes), err
}

func TestDeployScript_ShiftsTrafficOnceRevisionIsReady(t *testing.T) {
	out, azLog, err := runDeploy(t, "ready", "staging", "sha-abc1234")
	if err != nil {
		t.Fatalf("deploy.sh failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, "[SUCCESS] staging now serving sha-abc1234") {
		t.Fatalf("missing success line:\n%s", out)
	}
	// After a rollback the latest revision is the idle one; switching it off
	// would leave "latest=100" pointing at an inactive revision.
	if strings.Contains(azLog, "--revision rev2") {
		t.Errorf("deploy.sh must never deactivate the latest revision:\n%s", azLog)
	}
	for _, want := range []string{
		"containerapp revision deactivate --name ca-api-lensio-staging --resource-group rg-lensio-staging --revision old1",
		"containerapp update --name ca-api-lensio-staging --resource-group rg-lensio-staging --image ghcr.io/amirfaisalz/lensio/lensio-api:sha-abc1234",
		"containerapp ingress traffic set --name ca-api-lensio-staging --resource-group rg-lensio-staging --revision-weight latest=100",
		"containerapp update --name ca-dash-lensio-staging --resource-group rg-lensio-staging --image ghcr.io/amirfaisalz/lensio/lensio-dashboard:sha-abc1234",
		"containerapp ingress traffic set --name ca-dash-lensio-staging --resource-group rg-lensio-staging --revision-weight latest=100",
	} {
		if !strings.Contains(azLog, want) {
			t.Errorf("expected az call %q, got:\n%s", want, azLog)
		}
	}
}

// A revision that never becomes ready must fail the deploy loudly and leave
// traffic and the dashboard alone. On 2026-09-21 staging reported success
// while the new API revision crashed on boot.
func TestDeployScript_FailsWhenRevisionNeverReady(t *testing.T) {
	out, azLog, err := runDeploy(t, "stuck", "staging", "sha-abc1234")
	if err == nil {
		t.Fatalf("expected deploy.sh to fail, got success:\n%s", out)
	}
	if !strings.Contains(out, "ca-api-lensio-staging: revision rev2 never became ready (last ready: rev1)") {
		t.Fatalf("expected a readiness error naming both revisions:\n%s", out)
	}
	if strings.Contains(azLog, "ingress traffic set") {
		t.Errorf("traffic must not move when the revision is not ready:\n%s", azLog)
	}
	if strings.Contains(azLog, "ca-dash-lensio-staging") {
		t.Errorf("the dashboard must not be deployed after the API failed:\n%s", azLog)
	}
}

func TestDeployScript_RejectsUnknownEnvironment(t *testing.T) {
	out, azLog, err := runDeploy(t, "ready", "prod", "sha-abc1234")
	if err == nil || !strings.Contains(out, "env must be staging or production") {
		t.Fatalf("expected rejection of env 'prod': err=%v\n%s", err, out)
	}
	if azLog != "" {
		t.Errorf("no az call may run for an invalid env, got:\n%s", azLog)
	}
}
