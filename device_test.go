package main

import (
	"testing"
)

func TestResolveDeviceNotFound(t *testing.T) {
	_, _, err := resolveDevice("/dev/nonexistent-device-xyz")
	if err == nil {
		t.Error("expected error for non-existent device")
	}
}

func TestResolveDeviceNotBlock(t *testing.T) {
	// /dev/null is a character device, not a block device
	_, _, err := resolveDevice("/dev/null")
	if err == nil {
		t.Error("expected error for non-block device /dev/null")
	}
}
