package prompter

import (
	"testing"
	"time"

	pb "github.com/evilsocket/opensnitch-web/proto"
)

func testConnection() *pb.Connection {
	return &pb.Connection{
		ProcessPath: "/usr/bin/curl",
		DstHost:     "example.com",
		DstPort:     443,
		Protocol:    "tcp",
	}
}

func TestAskUserWithTimedReply(t *testing.T) {
	p := New(5) // 5s timeout

	var receivedPrompt *PendingPrompt
	p.OnNewPrompt = func(prompt *PendingPrompt) {
		receivedPrompt = prompt
	}

	conn := testConnection()

	// Start AskUser in a goroutine
	resultChan := make(chan *AskResult, 1)
	errChan := make(chan error, 1)
	go func() {
		result, err := p.AskUser("192.168.1.1", conn)
		resultChan <- result
		errChan <- err
	}()

	// Wait for prompt to be created
	time.Sleep(50 * time.Millisecond)

	if receivedPrompt == nil {
		t.Fatal("OnNewPrompt was not called")
	}

	// Reply with an allow rule
	rule := &pb.Rule{
		Name:     "test-allow",
		Action:   "allow",
		Duration: "always",
	}
	if err := p.Reply(receivedPrompt.ID, rule); err != nil {
		t.Fatalf("Reply: %v", err)
	}

	result := <-resultChan
	err := <-errChan
	if err != nil {
		t.Fatalf("AskUser: %v", err)
	}

	if result.Source != DecisionSourceUserReply {
		t.Errorf("Source = %q, want %q", result.Source, DecisionSourceUserReply)
	}
	if result.Rule.Action != "allow" {
		t.Errorf("Rule.Action = %q, want allow", result.Rule.Action)
	}
}

func TestAskUserWithTimeout(t *testing.T) {
	p := New(1) // 1s timeout

	var timedOutID string
	p.OnPromptTimeout = func(promptID string) {
		timedOutID = promptID
	}

	conn := testConnection()

	result, err := p.AskUser("192.168.1.1", conn)
	if err != nil {
		t.Fatalf("AskUser: %v", err)
	}

	if result.Source != DecisionSourceTimeout {
		t.Errorf("Source = %q, want %q", result.Source, DecisionSourceTimeout)
	}
	if result.Rule.Action != "deny" {
		t.Errorf("Rule.Action = %q, want deny", result.Rule.Action)
	}

	// Wait for timeout callback goroutine
	time.Sleep(50 * time.Millisecond)
	if timedOutID == "" {
		t.Error("OnPromptTimeout was not called")
	}
}

func TestReplyToNonExistentPrompt(t *testing.T) {
	p := New(5)

	rule := &pb.Rule{Action: "allow"}
	err := p.Reply("nonexistent", rule)
	if err == nil {
		t.Error("expected error for non-existent prompt")
	}
}

func TestGetPending(t *testing.T) {
	p := New(60) // long timeout to not expire

	conn := testConnection()

	// Start a prompt but don't reply
	go p.AskUser("192.168.1.1", conn)
	time.Sleep(50 * time.Millisecond)

	pending := p.GetPending()
	if len(pending) != 1 {
		t.Fatalf("GetPending() returned %d prompts, want 1", len(pending))
	}
	if pending[0].NodeAddr != "192.168.1.1" {
		t.Errorf("NodeAddr = %q, want 192.168.1.1", pending[0].NodeAddr)
	}
}

func TestPromptIDsAreUnique(t *testing.T) {
	p := New(60)
	conn := testConnection()

	// Start two prompts
	go p.AskUser("192.168.1.1", conn)
	go p.AskUser("192.168.1.2", conn)
	time.Sleep(50 * time.Millisecond)

	pending := p.GetPending()
	if len(pending) != 2 {
		t.Fatalf("GetPending() returned %d prompts, want 2", len(pending))
	}
	if pending[0].ID == pending[1].ID {
		t.Error("prompt IDs should be unique")
	}
}
