package main

import (
	"fmt"
	"os"

	"github.com/pkg/errors"
	"gopkg.in/yaml.v3"
)

// Config represents the main configuration structure
type Config struct {
	// Global shared configuration
	Shared SharedConfig `yaml:"shared"`
	// gRPC servers to run
	Servers []ServerConfig `yaml:"servers"`
}

// SharedConfig contains shared configuration used by multiple services
type SharedConfig struct {
	// Database configuration
	Database DatabaseConfig `yaml:"database"`
	// TimescaleDB configuration
	TimeScale TimescaleConfig `yaml:"timescale"`
	// ClickHouse configuration
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	// Redis configuration
	Redis RedisConfig `yaml:"redis"`
	// Storage configuration
	Storage StorageConfig `yaml:"storage"`
}

// DatabaseConfig contains database connection settings
type DatabaseConfig struct {
	URL string `yaml:"url"`
}

// ClickHouseConfig contains ClickHouse-specific settings
type ClickHouseConfig struct {
	// Path to ClickHouse configuration file (clickhouseConfigPath)
	Path string `yaml:"path"`
}

type TimescaleConfig struct {
	Path string `yaml:"path"`
}

// RedisConfig contains Redis connection settings
type RedisConfig struct {
	URL      string `yaml:"url"`
	Password string `yaml:"password"`
	DB       int    `yaml:"db"`
}

// StorageConfig contains storage settings
type StorageConfig struct {
	DefaultStorage string `yaml:"default_storage"`
	IpfsApiUrl     string `yaml:"ipfs_api_url"`
	IpfsGatewayUrl string `yaml:"ipfs_gateway_url"`
}

// ServerConfig represents configuration for a single gRPC server
type ServerConfig struct {
	// Server name
	Name string `yaml:"name"`
	// Enabled flag
	Enabled bool `yaml:"enabled"`
	// Port configuration for the server
	Port int `yaml:"port"`
	// Services running on this server
	Services []ServiceConfig `yaml:"services"`
}

// ServiceConfig represents configuration for a single service
type ServiceConfig struct {
	// Service name
	Name string `yaml:"name"`
	// Service type (processor, web, etc.)
	Type string `yaml:"type"`
	// Enabled flag
	Enabled bool `yaml:"enabled"`
	// Service-specific configuration
	Config map[string]interface{} `yaml:"config"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read config file %s", configPath)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, errors.Wrapf(err, "failed to parse YAML config")
	}

	// Validate configuration
	if err := validateConfig(&config); err != nil {
		return nil, errors.Wrapf(err, "config validation failed")
	}

	return &config, nil
}

// validateConfig validates the loaded configuration
func validateConfig(config *Config) error {
	if len(config.Servers) == 0 {
		return fmt.Errorf("no servers configured")
	}
	serverNames := make(map[string]bool)

	for i, server := range config.Servers {
		if server.Name == "" {
			return fmt.Errorf("server at index %d has no name", i)
		}

		// Check for duplicate server names
		if serverNames[server.Name] {
			return fmt.Errorf("duplicate server name: %s", server.Name)
		}
		serverNames[server.Name] = true

		// Validate services within each server
		if len(server.Services) == 0 {
			return fmt.Errorf("server %s has no services configured", server.Name)
		}

		serviceNames := make(map[string]bool)
		for j, service := range server.Services {
			if service.Name == "" {
				return fmt.Errorf("service at index %d in server %s has no name", j, server.Name)
			}

			if service.Type == "" {
				return fmt.Errorf("service %s in server %s has no type", service.Name, server.Name)
			}

			// Check for duplicate service names within the same server
			if serviceNames[service.Name] {
				return fmt.Errorf("duplicate service name %s in server %s", service.Name, server.Name)
			}
			serviceNames[service.Name] = true
		}
	}

	return nil
}
