package main

import (
	"testing"
)

func TestAnnotationKey(t *testing.T) {
	if AnnotationKey != "io-limits.noderesource.dev" {
		t.Errorf("AnnotationKey = %q, want %q", AnnotationKey, "io-limits.noderesource.dev")
	}
}
