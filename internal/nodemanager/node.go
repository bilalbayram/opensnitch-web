package nodemanager

import (
	"sync"
	"time"

	pb "github.com/evilsocket/opensnitch-web/proto"
)

type NodeState struct {
	Addr              string
	Hostname          string
	DaemonVersion     string
	Config            *pb.ClientConfig
	LastPing          time.Time
	Connected         bool
	IsFirewallRunning bool

	// Stats from last ping
	Stats *pb.Statistics

	// Channel to send notifications to daemon via Notifications() stream
	NotifyChan chan *pb.Notification

	// Rules loaded from daemon on Subscribe
	Rules []*pb.Rule

	// System firewall state
	SystemFirewall *pb.SysFirewall

	mu sync.RWMutex
}

func NewNode(addr string, config *pb.ClientConfig) *NodeState {
	hostname := ""
	if config != nil {
		hostname = config.Name
	}

	return &NodeState{
		Addr:              addr,
		Hostname:          hostname,
		Config:            config,
		Connected:         true,
		LastPing:          time.Now(),
		NotifyChan:        make(chan *pb.Notification, 256),
		IsFirewallRunning: config.GetIsFirewallRunning(),
		Rules:             config.GetRules(),
		SystemFirewall:    config.GetSystemFirewall(),
	}
}

func (n *NodeState) UpdateStats(stats *pb.Statistics) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.Stats = stats
	n.LastPing = time.Now()
	if stats != nil {
		n.DaemonVersion = stats.DaemonVersion
	}
}

func (n *NodeState) GetStats() *pb.Statistics {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.Stats
}

func (n *NodeState) SendNotification(notif *pb.Notification) bool {
	return n.SendNotifications([]*pb.Notification{notif})
}

func (n *NodeState) SendNotifications(notifs []*pb.Notification) bool {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.Connected {
		return false
	}
	if len(notifs) == 0 {
		return true
	}
	if len(n.NotifyChan)+len(notifs) > cap(n.NotifyChan) {
		return false
	}

	for _, notif := range notifs {
		if notif == nil {
			continue
		}
		n.NotifyChan <- notif
	}

	return true
}

func (n *NodeState) Close() {
	n.mu.Lock()
	defer n.mu.Unlock()

	if !n.Connected {
		return
	}
	n.Connected = false
	// Send sentinel to break Notifications loop
	select {
	case n.NotifyChan <- &pb.Notification{Type: -1}:
	default:
	}
}
