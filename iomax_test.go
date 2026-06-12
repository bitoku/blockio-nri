package main

import (
	"testing"
)

func TestFormatIOMax(t *testing.T) {
	tests := []struct {
		name  string
		major uint32
		minor uint32
		limit DeviceLimit
		want  string
	}{
		{
			name:  "all fields",
			major: 8, minor: 0,
			limit: DeviceLimit{
				RBPS:  uint64Ptr(52428800),
				WBPS:  uint64Ptr(10485760),
				RIOPS: uint64Ptr(1000),
				WIOPS: uint64Ptr(500),
			},
			want: "8:0 rbps=52428800 wbps=10485760 riops=1000 wiops=500",
		},
		{
			name:  "riops only",
			major: 8, minor: 0,
			limit: DeviceLimit{RIOPS: uint64Ptr(1000)},
			want:  "8:0 riops=1000",
		},
		{
			name:  "wbps and wiops",
			major: 259, minor: 0,
			limit: DeviceLimit{WBPS: uint64Ptr(104857600), WIOPS: uint64Ptr(5000)},
			want:  "259:0 wbps=104857600 wiops=5000",
		},
		{
			name:  "zero value for rbps",
			major: 8, minor: 16,
			limit: DeviceLimit{RBPS: uint64Ptr(0), RIOPS: uint64Ptr(100)},
			want:  "8:16 rbps=0 riops=100",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIOMax(tt.major, tt.minor, tt.limit)
			if got != tt.want {
				t.Errorf("formatIOMax() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatIOMaxMultiDevice(t *testing.T) {
	entries := []string{
		"8:0 rbps=52428800 riops=1000",
		"259:0 wbps=104857600",
	}
	want := "8:0 rbps=52428800 riops=1000\n259:0 wbps=104857600"
	got := formatIOMaxMultiDevice(entries)
	if got != want {
		t.Errorf("formatIOMaxMultiDevice() = %q, want %q", got, want)
	}
}

func TestFormatIOMaxSingleEntry(t *testing.T) {
	entries := []string{"8:0 riops=500"}
	want := "8:0 riops=500"
	got := formatIOMaxMultiDevice(entries)
	if got != want {
		t.Errorf("formatIOMaxMultiDevice() = %q, want %q", got, want)
	}
}
