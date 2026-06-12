package main

import (
	"github.com/intel/goresctrl/pkg/blockio"
	"sigs.k8s.io/yaml"
)

type PluginConfig struct {
	Classes                 map[string][]blockio.DevicesParameters `json:"Classes,omitempty"`
	FailOnInvalidAnnotation bool                                  `json:"failOnInvalidAnnotation"`
	LogLevel                string                                `json:"logLevel"`
}

func parseConfig(data string) (PluginConfig, error) {
	var cfg PluginConfig
	if data == "" {
		return cfg, nil
	}
	if err := yaml.Unmarshal([]byte(data), &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func applyBlockIOConfig(cfg PluginConfig) error {
	if len(cfg.Classes) == 0 {
		return nil
	}
	return blockio.SetConfig(&blockio.Config{Classes: cfg.Classes}, true)
}
