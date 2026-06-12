package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/containerd/nri/pkg/stub"
	"github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"
)

var (
	pluginName = flag.String("plugin-name", "nri-iops", "NRI plugin name")
	pluginIdx  = flag.String("plugin-idx", "90", "NRI plugin index")
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	configFile = flag.String("config", "", "Path to config file (used when running as external plugin)")
)

func main() {
	flag.Parse()

	log := logrus.New()
	level, err := logrus.ParseLevel(*logLevel)
	if err != nil {
		log.WithError(err).Fatal("invalid log level")
	}
	log.SetLevel(level)

	p := &IOPSPlugin{log: log}

	if *configFile != "" {
		data, err := os.ReadFile(*configFile)
		if err != nil {
			log.WithError(err).Fatal("failed to read config file")
		}
		var cfg PluginConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			log.WithError(err).Fatal("failed to parse config file")
		}
		p.config = cfg
		if err := applyBlockIOConfig(cfg); err != nil {
			log.WithError(err).Fatal("failed to apply blockio config")
		}
	}

	opts := []stub.Option{
		stub.WithPluginName(*pluginName),
		stub.WithPluginIdx(*pluginIdx),
	}

	s, err := stub.New(p, opts...)
	if err != nil {
		log.WithError(err).Fatal("failed to create NRI stub")
	}
	p.stub = s

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigC
		log.WithField("signal", sig).Info("received signal, shutting down")
		cancel()
		p.stub.Stop()
	}()

	log.WithFields(logrus.Fields{
		"name": *pluginName,
		"idx":  *pluginIdx,
	}).Info("starting NRI IOPS plugin")

	if err := p.stub.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "plugin exited with error: %v\n", err)
		os.Exit(1)
	}
}
