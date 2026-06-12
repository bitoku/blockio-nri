package main

import (
	"encoding/json"
	"fmt"
)

const AnnotationKey = "io-limits.noderesource.dev"

type IOLimits struct {
	Devices []DeviceLimit `json:"devices"`
}

type DeviceLimit struct {
	Path  string  `json:"path"`
	RBPS  *uint64 `json:"rbps,omitempty"`
	WBPS  *uint64 `json:"wbps,omitempty"`
	RIOPS *uint64 `json:"riops,omitempty"`
	WIOPS *uint64 `json:"wiops,omitempty"`
}

func (d DeviceLimit) hasLimits() bool {
	return d.RBPS != nil || d.WBPS != nil || d.RIOPS != nil || d.WIOPS != nil
}

func parseIOLimits(data string) (*IOLimits, error) {
	var limits IOLimits
	if err := json.Unmarshal([]byte(data), &limits); err != nil {
		return nil, fmt.Errorf("parsing io-limits annotation: %w", err)
	}
	if len(limits.Devices) == 0 {
		return nil, fmt.Errorf("io-limits annotation has no devices")
	}
	for i, d := range limits.Devices {
		if !d.hasLimits() {
			return nil, fmt.Errorf("device %d (%s) has no rate limits specified", i, d.Path)
		}
	}
	return &limits, nil
}
