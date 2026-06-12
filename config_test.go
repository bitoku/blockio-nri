package main

import (
	"testing"
)

func TestParseConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PluginConfig
		wantErr bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  PluginConfig{},
		},
		{
			name:  "full config",
			input: "defaultDevice: /dev/sda\nfailOnInvalidAnnotation: true\nlogLevel: debug\n",
			want: PluginConfig{
				DefaultDevice:           "/dev/sda",
				FailOnInvalidAnnotation: true,
				LogLevel:                "debug",
			},
		},
		{
			name:  "partial config",
			input: "logLevel: warn\n",
			want: PluginConfig{
				LogLevel: "warn",
			},
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
			if got != tt.want {
				t.Errorf("parseConfig() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
