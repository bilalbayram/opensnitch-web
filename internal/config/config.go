package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	UI       UIConfig       `yaml:"ui"`
}

type ServerConfig struct {
	HTTPAddr string `yaml:"http_addr"`
	GRPCAddr string `yaml:"grpc_addr"`
	GRPCUnix string `yaml:"grpc_unix"`
}

type DatabaseConfig struct {
	Path      string `yaml:"path"`
	PurgeDays int    `yaml:"purge_days"`
}

type AuthConfig struct {
	DefaultUser     string        `yaml:"default_user"`
	DefaultPassword string        `yaml:"default_password"`
	SessionTTL      time.Duration `yaml:"session_ttl"`
	JWTSecret       string        `yaml:"jwt_secret"`
}

type UIConfig struct {
	DefaultAction string `yaml:"default_action"`
	PromptTimeout int    `yaml:"prompt_timeout"`
}

func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			HTTPAddr: ":8080",
			GRPCAddr: "0.0.0.0:50051",
			GRPCUnix: "/tmp/osui.sock",
		},
		Database: DatabaseConfig{
			Path:      "./opensnitch-web.db",
			PurgeDays: 30,
		},
		Auth: AuthConfig{
			DefaultUser:     "admin",
			DefaultPassword: "opensnitch",
			SessionTTL:      24 * time.Hour,
			JWTSecret:       "change-me-in-production",
		},
		UI: UIConfig{
			DefaultAction: "deny",
			PromptTimeout: 120,
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
