package nodemanager

import (
	"log"
	"sync"
	"sync/atomic"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

type Manager struct {
	nodes   map[string]*NodeState
	mu      sync.RWMutex
	counter atomic.Uint64

	// Callback when node connects/disconnects
	OnNodeConnected    func(addr string, node *NodeState)
	OnNodeDisconnected func(addr string)
}

func NewManager() *Manager {
	return &Manager{
		nodes: make(map[string]*NodeState),
	}
}

func (m *Manager) AddNode(addr string, config *pb.ClientConfig) *NodeState {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close existing node if reconnecting
	if existing, ok := m.nodes[addr]; ok {
		existing.Close()
	}

	node := NewNode(addr, config)
	m.nodes[addr] = node

	log.Printf("[nodemanager] Node connected: %s (hostname: %s, version: %s)", addr, node.Hostname, config.GetVersion())

	if m.OnNodeConnected != nil {
		go m.OnNodeConnected(addr, node)
	}

	return node
}

func (m *Manager) RemoveNode(addr string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if node, ok := m.nodes[addr]; ok {
		node.Close()
		delete(m.nodes, addr)
		log.Printf("[nodemanager] Node disconnected: %s", addr)

		if m.OnNodeDisconnected != nil {
			go m.OnNodeDisconnected(addr)
		}
	}
}

func (m *Manager) GetNode(addr string) *NodeState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.nodes[addr]
}

func (m *Manager) GetAllNodes() map[string]*NodeState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*NodeState, len(m.nodes))
	for k, v := range m.nodes {
		result[k] = v
	}
	return result
}

func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.nodes)
}

func (m *Manager) NextID() uint64 {
	return m.counter.Add(1)
}

// SendNotification sends a notification to a specific node
func (m *Manager) SendNotification(addr string, notif *pb.Notification) bool {
	m.mu.RLock()
	node, ok := m.nodes[addr]
	m.mu.RUnlock()

	if !ok {
		return false
	}
	return node.SendNotification(notif)
}

func (m *Manager) SendNotificationBatch(addr string, notifs []*pb.Notification) bool {
	m.mu.RLock()
	node, ok := m.nodes[addr]
	m.mu.RUnlock()

	if !ok {
		return false
	}
	return node.SendNotifications(notifs)
}

// BroadcastNotification sends a notification to all connected nodes
func (m *Manager) BroadcastNotification(notif *pb.Notification) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for addr, node := range m.nodes {
		if !node.SendNotification(notif) {
			log.Printf("[nodemanager] Failed to send notification to %s (queue full)", addr)
		}
	}
}
