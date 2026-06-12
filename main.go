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
)

var (
	pluginName = flag.String("plugin-name", "nri-iops", "NRI plugin name")
	pluginIdx  = flag.String("plugin-idx", "90", "NRI plugin index")
	logLevel   = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
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
