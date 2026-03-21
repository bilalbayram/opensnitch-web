package router

import (
	_ "embed"
	"encoding/json"
)

//go:embed daemon_initd.sh
var daemonInitdScript string

type DaemonConfig struct {
	GRPCAddr        string `json:"grpc_addr"`
	APIKey          string `json:"api_key"`
	NodeName        string `json:"node_name"`
	DefaultAction   string `json:"default_action"`
	PollIntervalMS  int    `json:"poll_interval_ms"`
	FirewallBackend string `json:"firewall_backend"`
}

func RenderDaemonConfig(cfg DaemonConfig) (string, error) {
	payload, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(payload) + "\n", nil
}

func DaemonInitdScript() string {
	return daemonInitdScript
}
