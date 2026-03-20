package prompter

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

type PendingPrompt struct {
	ID         string         `json:"id"`
	Connection *pb.Connection `json:"connection"`
	NodeAddr   string         `json:"node_addr"`
	CreatedAt  time.Time      `json:"created_at"`
	ReplyChan  chan *pb.Rule  `json:"-"`
}

type DecisionSource string

const (
	DecisionSourceUserReply DecisionSource = "user_reply"
	DecisionSourceTimeout   DecisionSource = "timeout"
)

type AskResult struct {
	Rule   *pb.Rule
	Source DecisionSource
}

type Prompter struct {
	pending map[string]*PendingPrompt
	mu      sync.RWMutex
	counter atomic.Uint64
	timeout time.Duration

	// Called when a new prompt arrives (to broadcast via WebSocket)
	OnNewPrompt func(prompt *PendingPrompt)
	// Called when a prompt times out
	OnPromptTimeout func(promptID string)
}

func New(timeoutSeconds int) *Prompter {
	return &Prompter{
		pending: make(map[string]*PendingPrompt),
		timeout: time.Duration(timeoutSeconds) * time.Second,
	}
}

// AskUser creates a pending prompt and blocks until the browser user responds or timeout
func (p *Prompter) AskUser(nodeAddr string, conn *pb.Connection) (*AskResult, error) {
	id := fmt.Sprintf("prompt-%d", p.counter.Add(1))

	prompt := &PendingPrompt{
		ID:         id,
		Connection: conn,
		NodeAddr:   nodeAddr,
		CreatedAt:  time.Now(),
		ReplyChan:  make(chan *pb.Rule, 1),
	}

	p.mu.Lock()
	p.pending[id] = prompt
	p.mu.Unlock()

	log.Printf("[prompter] New prompt %s for node %s: %s -> %s:%d (%s)",
		id, nodeAddr, conn.ProcessPath, conn.DstHost, conn.DstPort, conn.Protocol)

	if p.OnNewPrompt != nil {
		go p.OnNewPrompt(prompt)
	}

	// Block until reply or timeout
	select {
	case rule := <-prompt.ReplyChan:
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()
		log.Printf("[prompter] Prompt %s answered: action=%s duration=%s", id, rule.Action, rule.Duration)
		return &AskResult{
			Rule:   rule,
			Source: DecisionSourceUserReply,
		}, nil

	case <-time.After(p.timeout):
		p.mu.Lock()
		delete(p.pending, id)
		p.mu.Unlock()

		log.Printf("[prompter] Prompt %s timed out", id)
		if p.OnPromptTimeout != nil {
			go p.OnPromptTimeout(id)
		}

		// Return default deny rule on timeout
		return &AskResult{
			Rule: &pb.Rule{
				Name:     fmt.Sprintf("ui-timeout-%d", time.Now().UnixNano()),
				Action:   "deny",
				Duration: "once",
				Enabled:  true,
				Operator: &pb.Operator{
					Type:    "simple",
					Operand: "process.path",
					Data:    conn.ProcessPath,
				},
			},
			Source: DecisionSourceTimeout,
		}, nil
	}
}

// Reply sends a rule back for a pending prompt
func (p *Prompter) Reply(promptID string, rule *pb.Rule) error {
	p.mu.RLock()
	prompt, ok := p.pending[promptID]
	p.mu.RUnlock()

	if !ok {
		return fmt.Errorf("prompt %s not found or expired", promptID)
	}

	select {
	case prompt.ReplyChan <- rule:
		return nil
	default:
		return fmt.Errorf("prompt %s already answered", promptID)
	}
}

// GetPending returns all pending prompts (for browser reconnection)
func (p *Prompter) GetPending() []*PendingPrompt {
	p.mu.RLock()
	defer p.mu.RUnlock()

	prompts := make([]*PendingPrompt, 0, len(p.pending))
	for _, prompt := range p.pending {
		prompts = append(prompts, prompt)
	}
	return prompts
}
