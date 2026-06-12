package main

import (
	"sigs.k8s.io/yaml"
)

type PluginConfig struct {
	DefaultDevice           string `json:"defaultDevice"`
	FailOnInvalidAnnotation bool   `json:"failOnInvalidAnnotation"`
	LogLevel                string `json:"logLevel"`
}

func parseConfig(data string) (PluginConfig, error) {
	var cfg PluginConfig
	if data == "" {
		return cfg, nil
	}
	err := yaml.Unmarshal([]byte(data), &cfg)
	return cfg, err
}
