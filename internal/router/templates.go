package router

import (
	_ "embed"
	"fmt"
)

//go:embed agent.sh
var agentScript string

//go:embed initd.sh
var initdScript string

type AgentConfig struct {
	ServerURL  string
	APIKey     string
	RouterName string
	LANPrefix  string // e.g. "192.168.1."
}

func RenderAgentConfig(cfg AgentConfig) string {
	return fmt.Sprintf(`SERVER_URL="%s"
API_KEY="%s"
ROUTER_NAME="%s"
LAN_PREFIX="%s"
BATCH_INTERVAL=5
BATCH_SIZE=20
`, cfg.ServerURL, cfg.APIKey, cfg.RouterName, cfg.LANPrefix)
}

func AgentScript() string {
	return agentScript
}

func InitdScript() string {
	return initdScript
}
