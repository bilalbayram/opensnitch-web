package ws

// Event types sent from server to browser
const (
	EventStatsUpdate     = "stats_update"
	EventConnectionEvent = "connection_event"
	EventPromptRequest   = "prompt_request"
	EventPromptTimeout   = "prompt_timeout"
	EventNodeConnected   = "node_connected"
	EventNodeDisconnected = "node_disconnected"
	EventNewAlert        = "new_alert"
	EventRuleChanged     = "rule_changed"
)

// Event types sent from browser to server
const (
	EventPromptReply = "prompt_reply"
	EventSubscribe   = "subscribe"
)

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type PromptPayload struct {
	ID         string      `json:"id"`
	NodeAddr   string      `json:"node_addr"`
	Connection interface{} `json:"connection"`
	CreatedAt  string      `json:"created_at"`
}

type PromptReplyPayload struct {
	PromptID  string `json:"prompt_id"`
	Action    string `json:"action"`
	Duration  string `json:"duration"`
	Name      string `json:"name"`
	Operand   string `json:"operand"`
	Data      string `json:"data"`
	Operator  string `json:"operator"`
}
