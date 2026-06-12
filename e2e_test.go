package main

import (
	"context"
	"os"
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/intel/goresctrl/pkg/blockio"
	"github.com/sirupsen/logrus"
)

const testDevice = "/dev/sda"

func newTestPlugin(cfg PluginConfig) *IOPSPlugin {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	return &IOPSPlugin{
		config: cfg,
		log:    log,
	}
}

func setupTestClasses(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(testDevice); err != nil {
		t.Skipf("skipping: %s not available", testDevice)
	}
	err := blockio.SetConfig(&blockio.Config{
		Classes: map[string][]blockio.DevicesParameters{
			"LowPrio": {
				{
					Devices:          []string{testDevice},
					ThrottleReadBps:  "50M",
					ThrottleWriteBps: "10M",
				},
			},
			"HighPrio": {
				{
					Devices:           []string{testDevice},
					ThrottleReadBps:   "500M",
					ThrottleWriteBps:  "200M",
					ThrottleReadIOPS:  "10000",
					ThrottleWriteIOPS: "5000",
				},
			},
		},
	}, true)
	if err != nil {
		t.Fatalf("setupTestClasses: %v", err)
	}
}

func makePod(id, name string, annotations map[string]string) *api.PodSandbox {
	return &api.PodSandbox{
		Id:          id,
		Name:        name,
		Namespace:   "default",
		Annotations: annotations,
	}
}

func makeContainer(id, podID, name string) *api.Container {
	return &api.Container{
		Id:           id,
		PodSandboxId: podID,
		Name:         name,
	}
}

// --- CreateContainer tests ---

func TestE2E_CreateContainer_ClassLookup(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: "HighPrio",
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, updates, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if updates != nil {
		t.Fatalf("expected no container updates, got %d", len(updates))
	}
	if adj == nil {
		t.Fatal("expected non-nil adjustment")
	}

	iomax := adj.Linux.Resources.Unified["io.max"]
	if iomax == "" {
		t.Fatal("expected non-empty io.max")
	}
	t.Logf("io.max = %q", iomax)
}

func TestE2E_CreateContainer_NoAnnotation(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", nil)
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, updates, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if adj != nil {
		t.Error("expected nil adjustment when no annotation present")
	}
	if updates != nil {
		t.Error("expected nil updates when no annotation present")
	}
}

func TestE2E_CreateContainer_ContainerScopedAnnotation(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey + "/container.writer": "LowPrio",
		AnnotationKey + "/container.reader": "HighPrio",
	})

	writerCtr := makeContainer("ctr-w", "pod-1", "writer")
	adjW, _, err := p.CreateContainer(context.Background(), pod, writerCtr)
	if err != nil {
		t.Fatalf("CreateContainer(writer): %v", err)
	}
	if adjW == nil {
		t.Fatal("expected adjustment for writer container")
	}

	readerCtr := makeContainer("ctr-r", "pod-1", "reader")
	adjR, _, err := p.CreateContainer(context.Background(), pod, readerCtr)
	if err != nil {
		t.Fatalf("CreateContainer(reader): %v", err)
	}
	if adjR == nil {
		t.Fatal("expected adjustment for reader container")
	}

	otherCtr := makeContainer("ctr-o", "pod-1", "sidecar")
	adjO, _, err := p.CreateContainer(context.Background(), pod, otherCtr)
	if err != nil {
		t.Fatalf("CreateContainer(sidecar): %v", err)
	}
	if adjO != nil {
		t.Error("expected nil adjustment for unmatched container")
	}
}

func TestE2E_CreateContainer_ContainerOverridesPod(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey:                    "LowPrio",
		AnnotationKey + "/container.app": "HighPrio",
	})

	appCtr := makeContainer("ctr-app", "pod-1", "app")
	adjApp, _, err := p.CreateContainer(context.Background(), pod, appCtr)
	if err != nil {
		t.Fatalf("CreateContainer(app): %v", err)
	}
	if adjApp == nil {
		t.Fatal("expected adjustment for app container")
	}

	sidecarCtr := makeContainer("ctr-sc", "pod-1", "sidecar")
	adjSc, _, err := p.CreateContainer(context.Background(), pod, sidecarCtr)
	if err != nil {
		t.Fatalf("CreateContainer(sidecar): %v", err)
	}
	if adjSc == nil {
		t.Fatal("expected adjustment for sidecar container")
	}
}

func TestE2E_CreateContainer_UnknownClass_WarnOnly(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{FailOnInvalidAnnotation: false})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: "NonExistentClass",
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("expected no error with FailOnInvalidAnnotation=false, got %v", err)
	}
	if adj != nil {
		t.Error("expected nil adjustment on unknown class")
	}
}

func TestE2E_CreateContainer_UnknownClass_FailMode(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{FailOnInvalidAnnotation: true})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: "NonExistentClass",
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	_, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err == nil {
		t.Fatal("expected error with FailOnInvalidAnnotation=true")
	}
}

// --- Synchronize tests ---

func TestE2E_Synchronize_UpdatesExistingContainers(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{
		makePod("pod-1", "test-pod", map[string]string{
			AnnotationKey: "LowPrio",
		}),
	}
	ctrs := []*api.Container{
		makeContainer("ctr-1", "pod-1", "app"),
		makeContainer("ctr-2", "pod-1", "sidecar"),
	}

	updates, err := p.Synchronize(context.Background(), pods, ctrs)
	if err != nil {
		t.Fatalf("Synchronize: %v", err)
	}

	if len(updates) != 2 {
		t.Fatalf("expected 2 updates, got %d", len(updates))
	}

	for i, u := range updates {
		iomax := u.Linux.Resources.Unified["io.max"]
		if iomax == "" {
			t.Errorf("update[%d] has empty io.max", i)
		}
	}
}

func TestE2E_Synchronize_SkipsContainersWithoutAnnotation(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{
		makePod("pod-1", "plain-pod", nil),
	}
	ctrs := []*api.Container{
		makeContainer("ctr-1", "pod-1", "app"),
	}

	updates, err := p.Synchronize(context.Background(), pods, ctrs)
	if err != nil {
		t.Fatalf("Synchronize: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("expected 0 updates, got %d", len(updates))
	}
}

func TestE2E_Synchronize_MixedPods(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{
		makePod("pod-1", "limited-pod", map[string]string{
			AnnotationKey: "LowPrio",
		}),
		makePod("pod-2", "unlimited-pod", nil),
	}
	ctrs := []*api.Container{
		makeContainer("ctr-1", "pod-1", "limited-app"),
		makeContainer("ctr-2", "pod-2", "unlimited-app"),
	}

	updates, err := p.Synchronize(context.Background(), pods, ctrs)
	if err != nil {
		t.Fatalf("Synchronize: %v", err)
	}

	if len(updates) != 1 {
		t.Fatalf("expected 1 update, got %d", len(updates))
	}
	if updates[0].GetContainerId() != "ctr-1" {
		t.Errorf("expected update for ctr-1, got %q", updates[0].GetContainerId())
	}
}

func TestE2E_Synchronize_UnknownClass(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{
		makePod("pod-1", "bad-pod", map[string]string{
			AnnotationKey: "UnknownClass",
		}),
	}
	ctrs := []*api.Container{
		makeContainer("ctr-1", "pod-1", "app"),
	}

	updates, err := p.Synchronize(context.Background(), pods, ctrs)
	if err != nil {
		t.Fatalf("Synchronize should not return error, got %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("expected 0 updates on unknown class, got %d", len(updates))
	}
}

func TestE2E_Synchronize_OrphanedContainer(t *testing.T) {
	setupTestClasses(t)

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{}
	ctrs := []*api.Container{
		makeContainer("ctr-1", "pod-99", "orphan"),
	}

	updates, err := p.Synchronize(context.Background(), pods, ctrs)
	if err != nil {
		t.Fatalf("Synchronize: %v", err)
	}
	if len(updates) != 0 {
		t.Errorf("expected 0 updates for orphaned container, got %d", len(updates))
	}
}

// --- Configure tests ---

func TestE2E_Configure(t *testing.T) {
	p := newTestPlugin(PluginConfig{})

	config := "failOnInvalidAnnotation: true\nlogLevel: debug\n"
	mask, err := p.Configure(context.Background(), config, "cri-o", "1.30.0")
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if mask != 0 {
		t.Errorf("expected event mask 0, got %d", mask)
	}

	if !p.config.FailOnInvalidAnnotation {
		t.Error("FailOnInvalidAnnotation should be true")
	}
	if p.config.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want debug", p.config.LogLevel)
	}
}

func TestE2E_Configure_EmptyConfig(t *testing.T) {
	p := newTestPlugin(PluginConfig{})

	mask, err := p.Configure(context.Background(), "", "cri-o", "1.30.0")
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if mask != 0 {
		t.Errorf("expected event mask 0, got %d", mask)
	}
}

func TestE2E_Configure_InvalidConfig(t *testing.T) {
	p := newTestPlugin(PluginConfig{})

	_, err := p.Configure(context.Background(), ":\n  :\n    invalid", "cri-o", "1.30.0")
	if err == nil {
		t.Fatal("expected error for invalid config YAML")
	}
}
