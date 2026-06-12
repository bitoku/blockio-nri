package main

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/containerd/nri/pkg/api"
	"github.com/sirupsen/logrus"
)

func newTestPlugin(cfg PluginConfig) *IOPSPlugin {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	return &IOPSPlugin{
		config: cfg,
		log:    log,
	}
}

func seedDevice(path string, major, minor uint32) {
	deviceCache.Store(path, deviceInfo{major: major, minor: minor})
}

func clearDeviceCache() {
	deviceCache.Range(func(key, _ any) bool {
		deviceCache.Delete(key)
		return true
	})
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

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	return string(data)
}

// --- CreateContainer tests ---

func TestE2E_CreateContainer_SingleDevice(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/sda", RIOPS: uint64Ptr(1000), WIOPS: uint64Ptr(500)},
			},
		}),
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
	expected := "8:0 riops=1000 wiops=500"
	if iomax != expected {
		t.Errorf("io.max = %q, want %q", iomax, expected)
	}
}

func TestE2E_CreateContainer_AllFourLimits(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{
					Path:  "/dev/sda",
					RBPS:  uint64Ptr(52428800),
					WBPS:  uint64Ptr(10485760),
					RIOPS: uint64Ptr(1000),
					WIOPS: uint64Ptr(500),
				},
			},
		}),
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}

	iomax := adj.Linux.Resources.Unified["io.max"]
	expected := "8:0 rbps=52428800 wbps=10485760 riops=1000 wiops=500"
	if iomax != expected {
		t.Errorf("io.max = %q, want %q", iomax, expected)
	}
}

func TestE2E_CreateContainer_MultipleDevices(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)
	seedDevice("/dev/nvme0n1", 259, 0)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/sda", RIOPS: uint64Ptr(1000)},
				{Path: "/dev/nvme0n1", WBPS: uint64Ptr(104857600)},
			},
		}),
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}

	iomax := adj.Linux.Resources.Unified["io.max"]
	expected := "8:0 riops=1000\n259:0 wbps=104857600"
	if iomax != expected {
		t.Errorf("io.max = %q, want %q", iomax, expected)
	}
}

func TestE2E_CreateContainer_NoAnnotation(t *testing.T) {
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
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey + "/container.writer": mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/sda", WBPS: uint64Ptr(10485760), WIOPS: uint64Ptr(200)},
			},
		}),
		AnnotationKey + "/container.reader": mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/sda", RBPS: uint64Ptr(52428800), RIOPS: uint64Ptr(1000)},
			},
		}),
	})

	writerCtr := makeContainer("ctr-w", "pod-1", "writer")
	adjW, _, err := p.CreateContainer(context.Background(), pod, writerCtr)
	if err != nil {
		t.Fatalf("CreateContainer(writer): %v", err)
	}
	if adjW == nil {
		t.Fatal("expected adjustment for writer container")
	}
	wantW := "8:0 wbps=10485760 wiops=200"
	if got := adjW.Linux.Resources.Unified["io.max"]; got != wantW {
		t.Errorf("writer io.max = %q, want %q", got, wantW)
	}

	readerCtr := makeContainer("ctr-r", "pod-1", "reader")
	adjR, _, err := p.CreateContainer(context.Background(), pod, readerCtr)
	if err != nil {
		t.Fatalf("CreateContainer(reader): %v", err)
	}
	if adjR == nil {
		t.Fatal("expected adjustment for reader container")
	}
	wantR := "8:0 rbps=52428800 riops=1000"
	if got := adjR.Linux.Resources.Unified["io.max"]; got != wantR {
		t.Errorf("reader io.max = %q, want %q", got, wantR)
	}

	// A container with no matching annotation should get no adjustment
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
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		// Pod-scoped: all containers get 100 riops
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/sda", RIOPS: uint64Ptr(100)},
			},
		}),
		// Container-scoped override: "app" gets 5000 riops
		AnnotationKey + "/container.app": mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/sda", RIOPS: uint64Ptr(5000)},
			},
		}),
	})

	// "app" should get the container-scoped override
	appCtr := makeContainer("ctr-app", "pod-1", "app")
	adjApp, _, err := p.CreateContainer(context.Background(), pod, appCtr)
	if err != nil {
		t.Fatalf("CreateContainer(app): %v", err)
	}
	wantApp := "8:0 riops=5000"
	if got := adjApp.Linux.Resources.Unified["io.max"]; got != wantApp {
		t.Errorf("app io.max = %q, want %q", got, wantApp)
	}

	// "sidecar" should get the pod-scoped default
	sidecarCtr := makeContainer("ctr-sc", "pod-1", "sidecar")
	adjSc, _, err := p.CreateContainer(context.Background(), pod, sidecarCtr)
	if err != nil {
		t.Fatalf("CreateContainer(sidecar): %v", err)
	}
	wantSc := "8:0 riops=100"
	if got := adjSc.Linux.Resources.Unified["io.max"]; got != wantSc {
		t.Errorf("sidecar io.max = %q, want %q", got, wantSc)
	}
}

func TestE2E_CreateContainer_DefaultDevice(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/vda", 252, 0)

	p := newTestPlugin(PluginConfig{DefaultDevice: "/dev/vda"})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{RIOPS: uint64Ptr(500)}, // no path — should use default
			},
		}),
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}

	iomax := adj.Linux.Resources.Unified["io.max"]
	expected := "252:0 riops=500"
	if iomax != expected {
		t.Errorf("io.max = %q, want %q", iomax, expected)
	}
}

func TestE2E_CreateContainer_NoPathNoDefault(t *testing.T) {
	clearDeviceCache()

	p := newTestPlugin(PluginConfig{}) // no default device
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{RIOPS: uint64Ptr(500)}, // no path, no default
			},
		}),
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer should not error (warn only): %v", err)
	}
	if adj != nil {
		t.Error("expected nil adjustment when device path cannot be resolved")
	}
}

func TestE2E_CreateContainer_UnresolvableDevice(t *testing.T) {
	clearDeviceCache()
	// deliberately do NOT seed /dev/missing

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/missing-device", RIOPS: uint64Ptr(500)},
			},
		}),
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer should not error (warn only): %v", err)
	}
	if adj != nil {
		t.Error("expected nil adjustment when device cannot be resolved")
	}
}

func TestE2E_CreateContainer_InvalidAnnotation_WarnOnly(t *testing.T) {
	p := newTestPlugin(PluginConfig{FailOnInvalidAnnotation: false})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: "not valid json{{{",
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("expected no error with FailOnInvalidAnnotation=false, got %v", err)
	}
	if adj != nil {
		t.Error("expected nil adjustment on invalid annotation")
	}
}

func TestE2E_CreateContainer_InvalidAnnotation_FailMode(t *testing.T) {
	p := newTestPlugin(PluginConfig{FailOnInvalidAnnotation: true})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: "not valid json{{{",
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	_, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err == nil {
		t.Fatal("expected error with FailOnInvalidAnnotation=true")
	}
}

func TestE2E_CreateContainer_EmptyDevices_FailMode(t *testing.T) {
	p := newTestPlugin(PluginConfig{FailOnInvalidAnnotation: true})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: `{"devices":[]}`,
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	_, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err == nil {
		t.Fatal("expected error for empty devices list")
	}
}

func TestE2E_CreateContainer_PartialDeviceFailure(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)
	// /dev/missing is NOT seeded

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/sda", RIOPS: uint64Ptr(1000)},
				{Path: "/dev/missing", WIOPS: uint64Ptr(200)},
			},
		}),
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if adj == nil {
		t.Fatal("expected adjustment for the resolvable device")
	}

	iomax := adj.Linux.Resources.Unified["io.max"]
	expected := "8:0 riops=1000"
	if iomax != expected {
		t.Errorf("io.max = %q, want %q (should skip unresolvable device)", iomax, expected)
	}
}

// --- Synchronize tests ---

func TestE2E_Synchronize_UpdatesExistingContainers(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{
		makePod("pod-1", "test-pod", map[string]string{
			AnnotationKey: mustJSON(t, IOLimits{
				Devices: []DeviceLimit{
					{Path: "/dev/sda", RIOPS: uint64Ptr(1000), WIOPS: uint64Ptr(500)},
				},
			}),
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

	expected := "8:0 riops=1000 wiops=500"
	for i, u := range updates {
		iomax := u.Linux.Resources.Unified["io.max"]
		if iomax != expected {
			t.Errorf("update[%d] io.max = %q, want %q", i, iomax, expected)
		}
	}

	if updates[0].GetContainerId() != "ctr-1" {
		t.Errorf("update[0] container id = %q, want %q", updates[0].GetContainerId(), "ctr-1")
	}
	if updates[1].GetContainerId() != "ctr-2" {
		t.Errorf("update[1] container id = %q, want %q", updates[1].GetContainerId(), "ctr-2")
	}
}

func TestE2E_Synchronize_SkipsContainersWithoutAnnotation(t *testing.T) {
	clearDeviceCache()

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
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{
		makePod("pod-1", "limited-pod", map[string]string{
			AnnotationKey: mustJSON(t, IOLimits{
				Devices: []DeviceLimit{
					{Path: "/dev/sda", RBPS: uint64Ptr(50000000)},
				},
			}),
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
	want := "8:0 rbps=50000000"
	if got := updates[0].Linux.Resources.Unified["io.max"]; got != want {
		t.Errorf("io.max = %q, want %q", got, want)
	}
}

func TestE2E_Synchronize_ContainerScopedAnnotation(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{
		makePod("pod-1", "test-pod", map[string]string{
			AnnotationKey + "/container.app": mustJSON(t, IOLimits{
				Devices: []DeviceLimit{
					{Path: "/dev/sda", WIOPS: uint64Ptr(300)},
				},
			}),
		}),
	}
	ctrs := []*api.Container{
		makeContainer("ctr-1", "pod-1", "app"),
		makeContainer("ctr-2", "pod-1", "monitor"),
	}

	updates, err := p.Synchronize(context.Background(), pods, ctrs)
	if err != nil {
		t.Fatalf("Synchronize: %v", err)
	}

	if len(updates) != 1 {
		t.Fatalf("expected 1 update (only 'app'), got %d", len(updates))
	}
	if updates[0].GetContainerId() != "ctr-1" {
		t.Errorf("expected update for ctr-1, got %q", updates[0].GetContainerId())
	}
}

func TestE2E_Synchronize_OrphanedContainer(t *testing.T) {
	clearDeviceCache()

	p := newTestPlugin(PluginConfig{})

	// Container references pod-99 which is not in the pods list
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

func TestE2E_Synchronize_InvalidAnnotation(t *testing.T) {
	clearDeviceCache()

	p := newTestPlugin(PluginConfig{})

	pods := []*api.PodSandbox{
		makePod("pod-1", "bad-pod", map[string]string{
			AnnotationKey: "invalid json!!!",
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
		t.Errorf("expected 0 updates on invalid annotation, got %d", len(updates))
	}
}

// --- Configure tests ---

func TestE2E_Configure(t *testing.T) {
	p := newTestPlugin(PluginConfig{})

	config := "defaultDevice: /dev/nvme0n1\nfailOnInvalidAnnotation: true\nlogLevel: debug\n"
	mask, err := p.Configure(context.Background(), config, "cri-o", "1.30.0")
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if mask != 0 {
		t.Errorf("expected event mask 0, got %d", mask)
	}

	if p.config.DefaultDevice != "/dev/nvme0n1" {
		t.Errorf("DefaultDevice = %q, want /dev/nvme0n1", p.config.DefaultDevice)
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
	if p.config.DefaultDevice != "" {
		t.Errorf("expected empty DefaultDevice, got %q", p.config.DefaultDevice)
	}
}

func TestE2E_Configure_InvalidConfig(t *testing.T) {
	p := newTestPlugin(PluginConfig{})

	_, err := p.Configure(context.Background(), ":\n  :\n    invalid", "cri-o", "1.30.0")
	if err == nil {
		t.Fatal("expected error for invalid config YAML")
	}
}

// --- Full lifecycle test ---

func TestE2E_ConfigureThenCreate(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/nvme0n1", 259, 0)

	p := newTestPlugin(PluginConfig{})

	// Configure with a default device
	_, err := p.Configure(context.Background(), "defaultDevice: /dev/nvme0n1\n", "cri-o", "1.30.0")
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	// Create a container with annotation that omits device path
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey: mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{WBPS: uint64Ptr(20971520)},
			},
		}),
	})
	ctr := makeContainer("ctr-1", "pod-1", "app")

	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if adj == nil {
		t.Fatal("expected adjustment")
	}

	want := "259:0 wbps=20971520"
	if got := adj.Linux.Resources.Unified["io.max"]; got != want {
		t.Errorf("io.max = %q, want %q", got, want)
	}
}

func TestE2E_PodScopedAnnotation(t *testing.T) {
	clearDeviceCache()
	seedDevice("/dev/sda", 8, 0)

	p := newTestPlugin(PluginConfig{})
	pod := makePod("pod-1", "test-pod", map[string]string{
		AnnotationKey + "/pod": mustJSON(t, IOLimits{
			Devices: []DeviceLimit{
				{Path: "/dev/sda", RIOPS: uint64Ptr(750)},
			},
		}),
	})

	ctr := makeContainer("ctr-1", "pod-1", "app")
	adj, _, err := p.CreateContainer(context.Background(), pod, ctr)
	if err != nil {
		t.Fatalf("CreateContainer: %v", err)
	}
	if adj == nil {
		t.Fatal("expected adjustment for pod-scoped annotation")
	}
	want := "8:0 riops=750"
	if got := adj.Linux.Resources.Unified["io.max"]; got != want {
		t.Errorf("io.max = %q, want %q", got, want)
	}
}
