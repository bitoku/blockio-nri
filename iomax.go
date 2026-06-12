package main

import (
	"fmt"
	"strings"

	oci "github.com/opencontainers/runtime-spec/specs-go"
)

func formatIOMaxFromOCI(lbio *oci.LinuxBlockIO) string {
	if lbio == nil {
		return ""
	}

	type devKey struct {
		major, minor int64
	}
	type devLimits struct {
		rbps, wbps, riops, wiops *uint64
	}

	devices := map[devKey]*devLimits{}
	var order []devKey

	ensure := func(major, minor int64) *devLimits {
		k := devKey{major, minor}
		if dl, ok := devices[k]; ok {
			return dl
		}
		dl := &devLimits{}
		devices[k] = dl
		order = append(order, k)
		return dl
	}

	for _, d := range lbio.ThrottleReadBpsDevice {
		ensure(d.Major, d.Minor).rbps = &d.Rate
	}
	for _, d := range lbio.ThrottleWriteBpsDevice {
		ensure(d.Major, d.Minor).wbps = &d.Rate
	}
	for _, d := range lbio.ThrottleReadIOPSDevice {
		ensure(d.Major, d.Minor).riops = &d.Rate
	}
	for _, d := range lbio.ThrottleWriteIOPSDevice {
		ensure(d.Major, d.Minor).wiops = &d.Rate
	}

	var entries []string
	for _, k := range order {
		dl := devices[k]
		var parts []string
		parts = append(parts, fmt.Sprintf("%d:%d", k.major, k.minor))
		if dl.rbps != nil {
			parts = append(parts, fmt.Sprintf("rbps=%d", *dl.rbps))
		}
		if dl.wbps != nil {
			parts = append(parts, fmt.Sprintf("wbps=%d", *dl.wbps))
		}
		if dl.riops != nil {
			parts = append(parts, fmt.Sprintf("riops=%d", *dl.riops))
		}
		if dl.wiops != nil {
			parts = append(parts, fmt.Sprintf("wiops=%d", *dl.wiops))
		}
		entries = append(entries, strings.Join(parts, " "))
	}

	return strings.Join(entries, "\n")
}
