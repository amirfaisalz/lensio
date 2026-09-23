package integration_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// runDashboardConfig executes the dashboard container's entrypoint hook that
// turns API_URL into /config.js, writing into a temp file instead.
func runDashboardConfig(t *testing.T, apiURL string) (string, error) {
	t.Helper()
	out := filepath.Join(t.TempDir(), "config.js")
	cmd := exec.Command("sh", "../../apps/dashboard/40-lensio-config.sh")
	cmd.Env = append(os.Environ(), "API_URL="+apiURL, "LENSIO_CONFIG_PATH="+out)
	if logs, err := cmd.CombinedOutput(); err != nil {
		return string(logs), err
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("config.js not written: %v", err)
	}
	return string(body), nil
}

func TestDashboardConfig_WritesAPIOrigin(t *testing.T) {
	for _, apiURL := range []string{
		"https://api.lensio.tec.my.id",
		"https://ca-api-lensio-staging.wittymoss-ad389f76.southeastasia.azurecontainerapps.io",
		"http://localhost:8080",
	} {
		got, err := runDashboardConfig(t, apiURL)
		if err != nil {
			t.Fatalf("%s: unexpected failure: %v: %s", apiURL, err, got)
		}
		want := `window.__LENSIO_CONFIG__ = { apiUrl: "` + apiURL + `" };`
		if strings.TrimSpace(got) != want {
			t.Fatalf("%s: got %q, want %q", apiURL, got, want)
		}
	}
}

func TestDashboardConfig_EmptyMeansSameOrigin(t *testing.T) {
	got, err := runDashboardConfig(t, "")
	if err != nil {
		t.Fatalf("unexpected failure: %v: %s", err, got)
	}
	if !strings.Contains(got, `apiUrl: ""`) {
		t.Fatalf("expected empty apiUrl, got %q", got)
	}
}

// The value lands inside a JavaScript string literal the browser executes, so
// anything beyond a bare origin must stop the container from starting.
func TestDashboardConfig_RejectsNonOrigin(t *testing.T) {
	for _, apiURL := range []string{
		`https://x.com"};alert(1);//`,
		"https://api.lensio.tec.my.id/path",
		"javascript:alert(1)",
		"api.lensio.tec.my.id",
		"https://api.lensio.tec.my.id\nwindow.x=1",
	} {
		logs, err := runDashboardConfig(t, apiURL)
		if err == nil {
			t.Fatalf("%q: expected rejection, got config %q", apiURL, logs)
		}
		if !strings.Contains(logs, "API_URL must be an origin") {
			t.Fatalf("%q: unexpected output: %s", apiURL, logs)
		}
	}
}
