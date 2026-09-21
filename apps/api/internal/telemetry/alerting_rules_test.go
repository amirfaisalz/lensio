package telemetry_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate repository root containing go.mod")
		}
		dir = parent
	}
}

// TestAlertingRules_FileStructure verifies that alerting_rules.yml exists,
// defines the 4 mandatory Phase 11.7 alerts, and enforces SRE operational hygiene.
func TestAlertingRules_FileStructure(t *testing.T) {
	repoRoot := findRepoRoot(t)
	rulesPath := filepath.Join(repoRoot, "infra", "observability", "prometheus", "alerting_rules.yml")

	data, err := os.ReadFile(rulesPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", rulesPath, err)
	}
	content := string(data)

	// Verify group definition
	if !strings.Contains(content, "groups:") || !strings.Contains(content, "name: lensio_slo_alerts") {
		t.Errorf("alerting_rules.yml missing groups or 'lensio_slo_alerts' group name")
	}

	// The 4 mandatory alert rules defined in Phase 11.7:
	requiredAlerts := []struct {
		name        string
		keyExprPart string
		minFor      string
		severity    string
	}{
		{
			name:        "HighErrorRate",
			keyExprPart: "http_requests_total",
			minFor:      "2m",
			severity:    "critical",
		},
		{
			name:        "P95LatencyBreached",
			keyExprPart: "http_request_duration_seconds_bucket",
			minFor:      "5m",
			severity:    "warning",
		},
		{
			name:        "CircuitBreakerOpen",
			keyExprPart: "lensio_ocr_circuit_breaker_state",
			minFor:      "1m",
			severity:    "critical",
		},
		{
			name:        "RateLimitSurge",
			keyExprPart: "429",
			minFor:      "2m",
			severity:    "warning",
		},
	}

	for _, alert := range requiredAlerts {
		t.Run("Rule_"+alert.name, func(t *testing.T) {
			if !strings.Contains(content, "alert: "+alert.name) {
				t.Fatalf("alerting_rules.yml missing required alert: %s", alert.name)
			}
			if !strings.Contains(content, alert.keyExprPart) {
				t.Errorf("alert %s missing expected metric reference %q", alert.name, alert.keyExprPart)
			}
			if !strings.Contains(content, "for: "+alert.minFor) {
				t.Errorf("alert %s missing expected duration 'for: %s'", alert.name, alert.minFor)
			}
			if !strings.Contains(content, "severity: "+alert.severity) {
				t.Errorf("alert %s missing expected severity %q", alert.name, alert.severity)
			}
		})
	}

	// Verify mandatory annotations & valid runbook links
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "runbook_url:") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				url := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
				if strings.HasPrefix(url, "docs/") {
					runbookFile := filepath.Join(repoRoot, url)
					if _, err := os.Stat(runbookFile); err != nil {
						t.Errorf("referenced runbook file does not exist: %s (err: %v)", url, err)
					}
				}
			}
		}
	}
}

// TestPrometheusConfig_ReferencesRules verifies that prometheus.yml references alerting_rules.yml.
func TestPrometheusConfig_ReferencesRules(t *testing.T) {
	repoRoot := findRepoRoot(t)
	configPath := filepath.Join(repoRoot, "infra", "observability", "prometheus", "prometheus.yml")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", configPath, err)
	}
	content := string(data)

	if !strings.Contains(content, "rule_files:") {
		t.Fatalf("prometheus.yml does not contain 'rule_files:'")
	}
	if !strings.Contains(content, "alerting_rules.yml") {
		t.Fatalf("prometheus.yml does not reference 'alerting_rules.yml'")
	}
}

// TestSLODocumentation_Completeness verifies that slo-definition.md exists and covers
// the contract requirements specified in Phase 11.7.
func TestSLODocumentation_Completeness(t *testing.T) {
	repoRoot := findRepoRoot(t)
	docPath := filepath.Join(repoRoot, "docs", "observability", "slo-definition.md")

	data, err := os.ReadFile(docPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", docPath, err)
	}
	content := string(data)

	requiredSections := []string{
		"99.9%",                      // 99.9% Availability SLA/SLO contract
		"43.2 minutes",               // Permitted downtime calculation
		"P95",                        // Latency SLO contract
		"Error Budget",               // Error budget consumption
		"Burn Rate",                  // Burn rate calculations
		"Multi-Window",               // Multi-window multi-burn-rate alerting
		"HighErrorRate",              // Alerting matrix coverage
		"P95LatencyBreached",         // Latency alert
		"CircuitBreakerOpen",         // Circuit breaker alert
		"RateLimitSurge",             // Rate limit surge alert
		"Error Budget Freeze Policy", // Operational governance
	}

	for _, section := range requiredSections {
		if !strings.Contains(content, section) {
			t.Errorf("slo-definition.md missing expected term or section: %q", section)
		}
	}
}

// TestPromtool_E2EValidation executes promtool check rules and test rules in Docker if available.
func TestPromtool_E2EValidation(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("docker not available on host; skipping promtool container test")
	}

	repoRoot := findRepoRoot(t)
	promDir := filepath.Join(repoRoot, "infra", "observability", "prometheus")

	// 1. Check rules syntax
	cmdCheck := exec.Command("docker", "run", "--rm", "--entrypoint", "promtool",
		"-v", promDir+":/etc/prometheus",
		"prom/prometheus:latest",
		"check", "rules", "/etc/prometheus/alerting_rules.yml",
	)
	outCheck, err := cmdCheck.CombinedOutput()
	if err != nil {
		t.Fatalf("promtool check rules failed: %v\nOutput:\n%s", err, string(outCheck))
	}

	// 2. Check server config syntax
	cmdConfig := exec.Command("docker", "run", "--rm", "--entrypoint", "promtool",
		"-v", promDir+":/etc/prometheus",
		"prom/prometheus:latest",
		"check", "config", "/etc/prometheus/prometheus.yml",
	)
	outConfig, err := cmdConfig.CombinedOutput()
	if err != nil {
		t.Fatalf("promtool check config failed: %v\nOutput:\n%s", err, string(outConfig))
	}

	// 3. Run rule unit tests
	cmdTest := exec.Command("docker", "run", "--rm", "--entrypoint", "promtool",
		"-v", promDir+":/etc/prometheus",
		"-w", "/etc/prometheus",
		"prom/prometheus:latest",
		"test", "rules", "alerting_rules_test.yml",
	)
	outTest, err := cmdTest.CombinedOutput()
	if err != nil {
		t.Fatalf("promtool test rules failed: %v\nOutput:\n%s", err, string(outTest))
	}
}
