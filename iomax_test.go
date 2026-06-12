package main

import (
	"testing"

	oci "github.com/opencontainers/runtime-spec/specs-go"
)

func uint64Ptr(v uint64) *uint64 { return &v }
func uint16Ptr(v uint16) *uint16 { return &v }

func TestFormatIOMaxFromOCI(t *testing.T) {
	tests := []struct {
		name string
		lbio *oci.LinuxBlockIO
		want string
	}{
		{
			name: "nil",
			lbio: nil,
			want: "",
		},
		{
			name: "empty",
			lbio: &oci.LinuxBlockIO{},
			want: "",
		},
		{
			name: "read bps only",
			lbio: &oci.LinuxBlockIO{
				ThrottleReadBpsDevice: []oci.LinuxThrottleDevice{
					{LinuxBlockIODevice: oci.LinuxBlockIODevice{Major: 8, Minor: 0}, Rate: 52428800},
				},
			},
			want: "8:0 rbps=52428800",
		},
		{
			name: "all throttle types",
			lbio: &oci.LinuxBlockIO{
				ThrottleReadBpsDevice: []oci.LinuxThrottleDevice{
					{LinuxBlockIODevice: oci.LinuxBlockIODevice{Major: 8, Minor: 0}, Rate: 52428800},
				},
				ThrottleWriteBpsDevice: []oci.LinuxThrottleDevice{
					{LinuxBlockIODevice: oci.LinuxBlockIODevice{Major: 8, Minor: 0}, Rate: 10485760},
				},
				ThrottleReadIOPSDevice: []oci.LinuxThrottleDevice{
					{LinuxBlockIODevice: oci.LinuxBlockIODevice{Major: 8, Minor: 0}, Rate: 1000},
				},
				ThrottleWriteIOPSDevice: []oci.LinuxThrottleDevice{
					{LinuxBlockIODevice: oci.LinuxBlockIODevice{Major: 8, Minor: 0}, Rate: 500},
				},
			},
			want: "8:0 rbps=52428800 wbps=10485760 riops=1000 wiops=500",
		},
		{
			name: "multiple devices",
			lbio: &oci.LinuxBlockIO{
				ThrottleReadBpsDevice: []oci.LinuxThrottleDevice{
					{LinuxBlockIODevice: oci.LinuxBlockIODevice{Major: 8, Minor: 0}, Rate: 52428800},
					{LinuxBlockIODevice: oci.LinuxBlockIODevice{Major: 259, Minor: 0}, Rate: 104857600},
				},
				ThrottleWriteBpsDevice: []oci.LinuxThrottleDevice{
					{LinuxBlockIODevice: oci.LinuxBlockIODevice{Major: 8, Minor: 0}, Rate: 10485760},
				},
			},
			want: "8:0 rbps=52428800 wbps=10485760\n259:0 rbps=104857600",
		},
		{
			name: "weight only is ignored",
			lbio: &oci.LinuxBlockIO{
				Weight: uint16Ptr(400),
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatIOMaxFromOCI(tt.lbio)
			if got != tt.want {
				t.Errorf("formatIOMaxFromOCI() = %q, want %q", got, tt.want)
			}
		})
	}
}
