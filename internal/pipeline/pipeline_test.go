// internal/pipeline/pipeline_test.go
package pipeline

import (
	"testing"
)

func TestPipelineConfigValidation(t *testing.T) {
	cfg := Config{}
	if err := cfg.Validate(); err == nil {
		t.Error("empty config should fail validation")
	}

	cfg = Config{
		StateFile:      "state.json",
		DigestsDir:     "digests",
		APIDir:         "api",
		AccountsFile:   "config/accounts.json",
		TopicsFile:     "config/topics.json",
		SubredditsFile: "config/subreddits.json",
	}
	if err := cfg.Validate(); err != nil {
		t.Errorf("valid config failed: %v", err)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config should be valid: %v", err)
	}
	if cfg.MinScore != 6.0 {
		t.Errorf("default MinScore: got %f, want 6.0", cfg.MinScore)
	}
	if cfg.MaxTweets != 5 {
		t.Errorf("default MaxTweets: got %d, want 5", cfg.MaxTweets)
	}
}
