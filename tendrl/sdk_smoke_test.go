package tendrl

import (
	"os"
	"path/filepath"
	"testing"
)

// This repository had no tests at all. The Python SDK was in the same state and
// shipped a client that raised on construction for every user — nothing caught
// it because nothing ran. These cover the parts that break that way: the
// defaults a first-time user gets, config round-tripping, and the batch sizing
// that decides how much a managed client sends.

func TestExampleConfigHasUsableDefaults(t *testing.T) {
	c := GenerateExampleConfig()
	if c == nil {
		t.Fatal("GenerateExampleConfig returned nil; it is what the docs tell a new user to start from")
	}
	if c.MinBatchSize <= 0 {
		t.Errorf("MinBatchSize = %d, must be positive", c.MinBatchSize)
	}
	if c.MaxBatchSize < c.MinBatchSize {
		t.Errorf("MaxBatchSize (%d) is below MinBatchSize (%d); batch sizing clamps between them "+
			"and would invert", c.MaxBatchSize, c.MinBatchSize)
	}
	for _, p := range []struct {
		name string
		v    float64
	}{{"TargetCPUPercent", c.TargetCPUPercent}, {"TargetMemPercent", c.TargetMemPercent}} {
		// Both are divisors in calculateDynamicBatchSize. Zero yields +Inf and
		// then a NaN resource factor, which silently collapses the batch size.
		if p.v <= 0 || p.v > 100 {
			t.Errorf("%s = %v, must be within (0,100]", p.name, p.v)
		}
	}
}

func TestConfigSurvivesASaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tendrl.json")
	want := GenerateExampleConfig()

	if err := SaveConfigFile(want, path); err != nil {
		t.Fatalf("SaveConfigFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file was not written: %v", err)
	}

	got, err := loadConfigFromPath(path)
	if err != nil {
		t.Fatalf("loadConfigFromPath: %v", err)
	}
	if got.MinBatchSize != want.MinBatchSize || got.MaxBatchSize != want.MaxBatchSize {
		t.Errorf("batch sizes did not survive the round trip: got (%d,%d) want (%d,%d)",
			got.MinBatchSize, got.MaxBatchSize, want.MinBatchSize, want.MaxBatchSize)
	}
	if got.TargetCPUPercent != want.TargetCPUPercent {
		t.Errorf("TargetCPUPercent = %v, want %v", got.TargetCPUPercent, want.TargetCPUPercent)
	}
}

func TestLoadingAMissingConfigIsAnError(t *testing.T) {
	if _, err := loadConfigFromPath(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Error("loading a nonexistent config succeeded; a typo'd path would look like a working default")
	}
}

func TestDefaultConfigPathIsResolvable(t *testing.T) {
	if GetDefaultConfigPath() == "" {
		t.Error("GetDefaultConfigPath is empty; a client with no explicit path has nowhere to look")
	}
}

// batchClient builds only the fields calculateDynamicBatchSize reads, so the
// sizing can be exercised without starting a managed client's goroutines.
func batchClient(minB, maxB int, cpu, mem, queue float64) *Client {
	return &Client{
		config: &Config{
			MinBatchSize:     minB,
			MaxBatchSize:     maxB,
			TargetCPUPercent: 70,
			TargetMemPercent: 70,
		},
		metrics: &SystemMetrics{CPUUsage: cpu, MemoryUsage: mem, QueueLoad: queue},
	}
}

func TestBatchSizeStaysWithinConfiguredBounds(t *testing.T) {
	// The floor is the case that matters. The resource factor is a weighted sum
	// capped at 1.0, so the raw size can never exceed MaxBatchSize — the upper
	// clamp is structurally unreachable and testing it proves nothing. The lower
	// clamp is real: a saturated client computes well under MinBatchSize when the
	// floor is set high, and without the clamp it would send tiny batches.
	cases := []struct {
		name            string
		minB, maxB      int
		cpu, mem, queue float64
	}{
		{"idle", 10, 500, 0, 0, 0},
		{"typical", 10, 500, 35, 40, 10},
		{"saturated", 10, 500, 100, 100, 100},
		{"beyond target", 10, 500, 300, 300, 500},
		// Raw size lands at ~100 here, below the 200 floor, so the clamp has to lift it.
		{"floor above computed size", 200, 500, 100, 100, 100},
		{"floor equal to ceiling", 300, 300, 50, 50, 25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := batchClient(tc.minB, tc.maxB, tc.cpu, tc.mem, tc.queue)
			got := c.calculateDynamicBatchSize()
			if got < c.config.MinBatchSize || got > c.config.MaxBatchSize {
				t.Errorf("batch size %d escaped the configured bounds [%d,%d]",
					got, c.config.MinBatchSize, c.config.MaxBatchSize)
			}
		})
	}
}

// A loaded machine must not batch more aggressively than an idle one; that is
// the entire purpose of sizing on metrics.
func TestBatchSizeShrinksUnderLoad(t *testing.T) {
	idle := batchClient(10, 500, 0, 0, 0).calculateDynamicBatchSize()
	loaded := batchClient(10, 500, 100, 100, 0).calculateDynamicBatchSize()
	if loaded > idle {
		t.Errorf("a saturated client batched %d, more than an idle one at %d", loaded, idle)
	}
}
