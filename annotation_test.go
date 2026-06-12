package main

import (
	"testing"
)

func uint64Ptr(v uint64) *uint64 { return &v }

func TestParseIOLimits(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *IOLimits
		wantErr bool
	}{
		{
			name:  "all fields",
			input: `{"devices":[{"path":"/dev/sda","rbps":1000,"wbps":2000,"riops":100,"wiops":200}]}`,
			want: &IOLimits{
				Devices: []DeviceLimit{
					{
						Path:  "/dev/sda",
						RBPS:  uint64Ptr(1000),
						WBPS:  uint64Ptr(2000),
						RIOPS: uint64Ptr(100),
						WIOPS: uint64Ptr(200),
					},
				},
			},
		},
		{
			name:  "partial fields",
			input: `{"devices":[{"path":"/dev/sda","riops":500}]}`,
			want: &IOLimits{
				Devices: []DeviceLimit{
					{
						Path:  "/dev/sda",
						RIOPS: uint64Ptr(500),
					},
				},
			},
		},
		{
			name:  "multiple devices",
			input: `{"devices":[{"path":"/dev/sda","riops":100},{"path":"/dev/nvme0n1","wbps":50000}]}`,
			want: &IOLimits{
				Devices: []DeviceLimit{
					{Path: "/dev/sda", RIOPS: uint64Ptr(100)},
					{Path: "/dev/nvme0n1", WBPS: uint64Ptr(50000)},
				},
			},
		},
		{
			name:  "zero value means max",
			input: `{"devices":[{"path":"/dev/sda","rbps":0,"riops":100}]}`,
			want: &IOLimits{
				Devices: []DeviceLimit{
					{Path: "/dev/sda", RBPS: uint64Ptr(0), RIOPS: uint64Ptr(100)},
				},
			},
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
		{
			name:    "empty devices",
			input:   `{"devices":[]}`,
			wantErr: true,
		},
		{
			name:    "no rate limits",
			input:   `{"devices":[{"path":"/dev/sda"}]}`,
			wantErr: true,
		},
		{
			name:    "empty string",
			input:   ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseIOLimits(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseIOLimits() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if len(got.Devices) != len(tt.want.Devices) {
				t.Fatalf("got %d devices, want %d", len(got.Devices), len(tt.want.Devices))
			}
			for i, d := range got.Devices {
				w := tt.want.Devices[i]
				if d.Path != w.Path {
					t.Errorf("device[%d].Path = %s, want %s", i, d.Path, w.Path)
				}
				assertUint64Ptr(t, i, "RBPS", d.RBPS, w.RBPS)
				assertUint64Ptr(t, i, "WBPS", d.WBPS, w.WBPS)
				assertUint64Ptr(t, i, "RIOPS", d.RIOPS, w.RIOPS)
				assertUint64Ptr(t, i, "WIOPS", d.WIOPS, w.WIOPS)
			}
		})
	}
}

func assertUint64Ptr(t *testing.T, idx int, name string, got, want *uint64) {
	t.Helper()
	if got == nil && want == nil {
		return
	}
	if got == nil || want == nil {
		t.Errorf("device[%d].%s: got %v, want %v", idx, name, got, want)
		return
	}
	if *got != *want {
		t.Errorf("device[%d].%s = %d, want %d", idx, name, *got, *want)
	}
}

func TestDeviceLimitHasLimits(t *testing.T) {
	tests := []struct {
		name string
		d    DeviceLimit
		want bool
	}{
		{"no limits", DeviceLimit{Path: "/dev/sda"}, false},
		{"rbps only", DeviceLimit{RBPS: uint64Ptr(100)}, true},
		{"wiops only", DeviceLimit{WIOPS: uint64Ptr(100)}, true},
		{"all limits", DeviceLimit{RBPS: uint64Ptr(1), WBPS: uint64Ptr(2), RIOPS: uint64Ptr(3), WIOPS: uint64Ptr(4)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.d.hasLimits(); got != tt.want {
				t.Errorf("hasLimits() = %v, want %v", got, tt.want)
			}
		})
	}
}
