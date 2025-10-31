package config

import (
	"errors"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config captures runtime configuration for the SCTE-224 scheduler service.
type Config struct {
	HTTP struct {
		Address string `yaml:"address"`
	} `yaml:"http"`
	Temporal struct {
		HostPort  string `yaml:"hostPort"`
		Namespace string `yaml:"namespace"`
		TaskQueue string `yaml:"taskQueue"`
	} `yaml:"temporal"`
	Audiences []string `yaml:"audiences"`
}

// Load reads configuration from a YAML file and environment variables.
func Load(path string) (Config, error) {
	cfg := Config{}
	cfg.HTTP.Address = getEnv("HTTP_ADDRESS", ":8080")
	cfg.Temporal.HostPort = getEnv("TEMPORAL_HOST_PORT", "127.0.0.1:7233")
	cfg.Temporal.Namespace = getEnv("TEMPORAL_NAMESPACE", "default")
	cfg.Temporal.TaskQueue = getEnv("TEMPORAL_TASK_QUEUE", "scte224-scheduler")
	defaultAudience := getEnv("DEFAULT_AUDIENCE", "urn:scte:224:audience:us")
	cfg.Audiences = splitAndTrim(defaultAudience)

	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return cfg, nil
			}
			return cfg, err
		}
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return cfg, err
		}
		if len(cfg.Audiences) == 0 {
			cfg.Audiences = splitAndTrim(defaultAudience)
		}
	}
	return cfg, nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func splitAndTrim(value string) []string {
	parts := strings.Split(value, ",")
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}
