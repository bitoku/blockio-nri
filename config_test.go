package main

import (
	"testing"

	"github.com/intel/goresctrl/pkg/blockio"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantErr    bool
		wantLog    string
		wantFail   bool
		wantClasses int
	}{
		{
			name:  "empty string",
			input: "",
		},
		{
			name: "full config with classes",
			input: `logLevel: debug
failOnInvalidAnnotation: true
Classes:
  LowPrio:
    - Devices:
        - /dev/sda
      ThrottleReadBps: "50M"
`,
			wantLog:     "debug",
			wantFail:    true,
			wantClasses: 1,
		},
		{
			name:    "plugin settings only",
			input:   "logLevel: warn\nfailOnInvalidAnnotation: false\n",
			wantLog: "warn",
		},
		{
			name:    "invalid yaml",
			input:   ":\n  :\n    invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseConfig(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.LogLevel != tt.wantLog {
				t.Errorf("LogLevel = %q, want %q", got.LogLevel, tt.wantLog)
			}
			if got.FailOnInvalidAnnotation != tt.wantFail {
				t.Errorf("FailOnInvalidAnnotation = %v, want %v", got.FailOnInvalidAnnotation, tt.wantFail)
			}
			if len(got.Classes) != tt.wantClasses {
				t.Errorf("len(Classes) = %d, want %d", len(got.Classes), tt.wantClasses)
			}
		})
	}
}

func TestApplyBlockIOConfig(t *testing.T) {
	cfg := PluginConfig{
		Classes: map[string][]blockio.DevicesParameters{
			"TestClass": {
				{
					Devices:         []string{"/dev/null"},
					ThrottleReadBps: "10M",
				},
			},
		},
	}
	err := applyBlockIOConfig(cfg)
	if err != nil {
		t.Logf("applyBlockIOConfig() error = %v (expected in test environment without block devices)", err)
	}
}

func TestApplyBlockIOConfigEmpty(t *testing.T) {
	cfg := PluginConfig{}
	if err := applyBlockIOConfig(cfg); err != nil {
		t.Fatalf("applyBlockIOConfig() with empty classes should not error: %v", err)
	}
}
