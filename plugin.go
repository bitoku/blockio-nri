package main

import (
	"context"

	"github.com/containerd/nri/pkg/api"
	"github.com/containerd/nri/pkg/stub"
	nriplugin "github.com/containerd/nri/pkg/plugin"
	"github.com/sirupsen/logrus"
)

type IOPSPlugin struct {
	stub   stub.Stub
	config PluginConfig
	log    *logrus.Logger
}

func (p *IOPSPlugin) Configure(_ context.Context, config, runtime, version string) (stub.EventMask, error) {
	p.log.WithFields(logrus.Fields{
		"runtime": runtime,
		"version": version,
	}).Info("configuring plugin")

	cfg, err := parseConfig(config)
	if err != nil {
		return 0, err
	}
	p.config = cfg

	if cfg.LogLevel != "" {
		level, err := logrus.ParseLevel(cfg.LogLevel)
		if err == nil {
			p.log.SetLevel(level)
		}
	}

	return 0, nil
}

func (p *IOPSPlugin) Synchronize(_ context.Context, pods []*api.PodSandbox, ctrs []*api.Container) ([]*api.ContainerUpdate, error) {
	p.log.WithFields(logrus.Fields{
		"pods":       len(pods),
		"containers": len(ctrs),
	}).Info("synchronizing")

	podsByID := make(map[string]*api.PodSandbox, len(pods))
	for _, pod := range pods {
		podsByID[pod.GetId()] = pod
	}

	var updates []*api.ContainerUpdate
	for _, ctr := range ctrs {
		pod := podsByID[ctr.GetPodSandboxId()]
		if pod == nil {
			continue
		}

		ioMaxValue, err := p.buildIOMax(pod, ctr.GetName())
		if err != nil {
			p.log.WithError(err).WithField("container", ctr.GetName()).Warn("skipping container")
			continue
		}
		if ioMaxValue == "" {
			continue
		}

		update := &api.ContainerUpdate{}
		update.SetContainerId(ctr.GetId())
		update.AddLinuxUnified("io.max", ioMaxValue)
		updates = append(updates, update)

		p.log.WithFields(logrus.Fields{
			"container": ctr.GetName(),
			"io.max":    ioMaxValue,
		}).Info("updating existing container")
	}

	return updates, nil
}

func (p *IOPSPlugin) CreateContainer(_ context.Context, pod *api.PodSandbox, ctr *api.Container) (*api.ContainerAdjustment, []*api.ContainerUpdate, error) {
	ioMaxValue, err := p.buildIOMax(pod, ctr.GetName())
	if err != nil {
		p.log.WithError(err).WithField("container", ctr.GetName()).Warn("failed to build io.max")
		if p.config.FailOnInvalidAnnotation {
			return nil, nil, err
		}
		return nil, nil, nil
	}
	if ioMaxValue == "" {
		return nil, nil, nil
	}

	adjustment := &api.ContainerAdjustment{}
	adjustment.AddLinuxUnified("io.max", ioMaxValue)

	p.log.WithFields(logrus.Fields{
		"container": ctr.GetName(),
		"io.max":    ioMaxValue,
	}).Info("adjusting container")

	return adjustment, nil, nil
}

func (p *IOPSPlugin) buildIOMax(pod *api.PodSandbox, containerName string) (string, error) {
	annotation, ok := nriplugin.GetEffectiveAnnotation(pod, AnnotationKey, containerName)
	if !ok {
		return "", nil
	}

	limits, err := parseIOLimits(annotation)
	if err != nil {
		return "", err
	}

	var entries []string
	for _, dev := range limits.Devices {
		path := dev.Path
		if path == "" {
			path = p.config.DefaultDevice
		}
		if path == "" {
			p.log.Warn("device has no path and no default device configured, skipping")
			continue
		}

		major, minor, err := resolveDevice(path)
		if err != nil {
			p.log.WithError(err).WithField("device", path).Warn("failed to resolve device, skipping")
			continue
		}

		entries = append(entries, formatIOMax(major, minor, dev))
	}

	if len(entries) == 0 {
		return "", nil
	}

	return formatIOMaxMultiDevice(entries), nil
}
