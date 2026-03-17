package config

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Auth     AuthConfig     `yaml:"auth"`
	UI       UIConfig       `yaml:"ui"`
	Update   UpdateConfig   `yaml:"update"`
	GeoIP    GeoIPConfig    `yaml:"geoip"`
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

type UpdateConfig struct {
	Enabled       bool          `yaml:"enabled"`
	CheckInterval time.Duration `yaml:"check_interval"`
	GitHubRepo    string        `yaml:"github_repo"`
}

type GeoIPConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Provider string `yaml:"provider"`
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
		Update: UpdateConfig{
			Enabled:       true,
			CheckInterval: 6 * time.Hour,
			GitHubRepo:    "evilsocket/opensnitch-web",
		},
		GeoIP: GeoIPConfig{
			Enabled:  true,
			Provider: "ip-api",
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := bootstrapFromExample(path); err == nil {
				data, err = os.ReadFile(path)
				if err != nil {
					return cfg, nil
				}
			} else {
				return cfg, nil
			}
		} else {
			return nil, err
		}
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func bootstrapFromExample(path string) error {
	examplePath := path + ".example"
	data, err := os.ReadFile(examplePath)
	if err != nil {
		return err
	}

	jwtSecret, err := randomHex(32)
	if err != nil {
		return fmt.Errorf("generate jwt secret: %w", err)
	}
	password, err := randomHex(16)
	if err != nil {
		return fmt.Errorf("generate password: %w", err)
	}

	content := string(data)
	content = strings.Replace(content, `jwt_secret: "change-me-in-production"`, `jwt_secret: "`+jwtSecret+`"`, 1)
	content = strings.Replace(content, `default_password: "opensnitch"`, `default_password: "`+password+`"`, 1)

	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		return err
	}

	log.Printf("[config] Generated %s from %s (secrets auto-generated)", path, examplePath)
	log.Printf("[config] Admin password: %s", password)
	return nil
}

func randomHex(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
