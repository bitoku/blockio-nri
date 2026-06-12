package main

import (
	"fmt"
	"strings"
)

func formatIOMax(major, minor uint32, limit DeviceLimit) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%d:%d", major, minor))

	if limit.RBPS != nil {
		parts = append(parts, fmt.Sprintf("rbps=%d", *limit.RBPS))
	}
	if limit.WBPS != nil {
		parts = append(parts, fmt.Sprintf("wbps=%d", *limit.WBPS))
	}
	if limit.RIOPS != nil {
		parts = append(parts, fmt.Sprintf("riops=%d", *limit.RIOPS))
	}
	if limit.WIOPS != nil {
		parts = append(parts, fmt.Sprintf("wiops=%d", *limit.WIOPS))
	}

	return strings.Join(parts, " ")
}

func formatIOMaxMultiDevice(entries []string) string {
	return strings.Join(entries, "\n")
}
