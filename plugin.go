package main

import (
	"context"
	"strings"

	"github.com/containerd/nri/pkg/api"
	nriplugin "github.com/containerd/nri/pkg/plugin"
	"github.com/containerd/nri/pkg/stub"
	"github.com/intel/goresctrl/pkg/blockio"
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

	if config != "" {
		cfg, err := parseConfig(config)
		if err != nil {
			return 0, err
		}
		p.config = cfg
		if err := applyBlockIOConfig(cfg); err != nil {
			return 0, err
		}
	}

	if p.config.LogLevel != "" {
		level, err := logrus.ParseLevel(p.config.LogLevel)
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
	className, ok := nriplugin.GetEffectiveAnnotation(pod, AnnotationKey, containerName)
	if !ok {
		return "", nil
	}

	className = strings.TrimSpace(className)
	if className == "" {
		return "", nil
	}

	lbio, err := blockio.OciLinuxBlockIO(className)
	if err != nil {
		return "", err
	}

	return formatIOMaxFromOCI(lbio), nil
}
