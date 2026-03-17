package grpcserver

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/db"
	"github.com/evilsocket/opensnitch-web/internal/nodemanager"
	"github.com/evilsocket/opensnitch-web/internal/prompter"
	ruleutil "github.com/evilsocket/opensnitch-web/internal/rules"
	"github.com/evilsocket/opensnitch-web/internal/ws"
	pb "github.com/evilsocket/opensnitch-web/proto"
	"google.golang.org/grpc/peer"
)

// UIService implements the proto UI gRPC service interface.
// The OpenSnitch daemon connects to this server.
type UIService struct {
	pb.UnimplementedUIServer

	nodes    *nodemanager.Manager
	db       *db.Database
	hub      *ws.Hub
	prompter *prompter.Prompter
}

func NewUIService(nodes *nodemanager.Manager, database *db.Database, hub *ws.Hub, p *prompter.Prompter) *UIService {
	return &UIService{
		nodes:    nodes,
		db:       database,
		hub:      hub,
		prompter: p,
	}
}

func peerAddrFromCtx(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}
	return p.Addr.String()
}

func normalizeEventTime(value string, unixnano int64) string {
	if unixnano > 0 {
		return ruleutil.FormatStoredTime(time.Unix(0, unixnano))
	}
	if ts, err := ruleutil.ParseStoredTime(value); err == nil {
		return ruleutil.FormatStoredTime(ts)
	}
	return value
}

// Subscribe is called when a daemon first connects.
func (s *UIService) Subscribe(ctx context.Context, config *pb.ClientConfig) (*pb.ClientConfig, error) {
	peerAddr := peerAddrFromCtx(ctx)
	log.Printf("[grpc] Subscribe from %s (name: %s, version: %s, rules: %d)",
		peerAddr, config.GetName(), config.GetVersion(), len(config.GetRules()))

	s.nodes.AddNode(peerAddr, config)

	s.db.UpsertNode(&db.Node{
		Addr:          peerAddr,
		Hostname:      config.GetName(),
		DaemonVersion: config.GetVersion(),
		Status:        "online",
		LastConn:      time.Now().Format("2006-01-02 15:04:05"),
		DaemonRules:   int64(len(config.GetRules())),
	})

	// Store rules from daemon
	for _, r := range config.GetRules() {
		dbRule, err := ruleutil.ProtoToDBRule(peerAddr, time.Now(), r)
		if err != nil {
			log.Printf("[grpc] Failed to convert rule %q from %s: %v", r.GetName(), peerAddr, err)
			continue
		}
		s.db.UpsertRule(dbRule)
	}

	s.hub.BroadcastEvent(ws.EventNodeConnected, map[string]interface{}{
		"addr":     peerAddr,
		"hostname": config.GetName(),
		"version":  config.GetVersion(),
	})

	return config, nil
}

// Ping is the heartbeat — daemon sends stats every ~1s
func (s *UIService) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingReply, error) {
	peerAddr := peerAddrFromCtx(ctx)

	node := s.nodes.GetNode(peerAddr)
	if node == nil {
		return &pb.PingReply{Id: req.Id}, nil
	}

	node.UpdateStats(req.Stats)

	stats := req.GetStats()
	if stats != nil {
		s.db.UpsertNode(&db.Node{
			Addr:          peerAddr,
			Hostname:      node.Hostname,
			DaemonVersion: stats.DaemonVersion,
			DaemonUptime:  int64(stats.Uptime),
			DaemonRules:   int64(stats.Rules),
			Cons:          int64(stats.Connections),
			ConsDropped:   int64(stats.Dropped),
			Status:        "online",
			LastConn:      time.Now().Format("2006-01-02 15:04:05"),
		})

		// Store connection events
		for _, evt := range stats.Events {
			if evt.Connection == nil {
				continue
			}
			conn := evt.Connection
			action := ""
			ruleName := ""
			if evt.Rule != nil {
				action = evt.Rule.Action
				ruleName = evt.Rule.Name
			}

			s.db.InsertConnection(&db.Connection{
				Time:        normalizeEventTime(evt.Time, evt.Unixnano),
				Node:        peerAddr,
				Action:      action,
				Protocol:    conn.Protocol,
				SrcIP:       conn.SrcIp,
				SrcPort:     int(conn.SrcPort),
				DstIP:       conn.DstIp,
				DstHost:     conn.DstHost,
				DstPort:     int(conn.DstPort),
				UID:         int(conn.UserId),
				PID:         int(conn.ProcessId),
				Process:     conn.ProcessPath,
				ProcessArgs: strings.Join(conn.ProcessArgs, " "),
				ProcessCwd:  conn.ProcessCwd,
				Rule:        ruleName,
			})

			// Record domain-to-IP mapping for non-DNS connections
			if conn.DstHost != "" && conn.DstIp != "" && conn.DstPort != 53 {
				s.db.UpsertDNSDomain(peerAddr, conn.DstHost, conn.DstIp, normalizeEventTime(evt.Time, evt.Unixnano))
			}

			s.hub.BroadcastEvent(ws.EventConnectionEvent, map[string]interface{}{
				"time":         evt.Time,
				"node":         peerAddr,
				"action":       action,
				"rule":         ruleName,
				"protocol":     conn.Protocol,
				"src_ip":       conn.SrcIp,
				"src_port":     conn.SrcPort,
				"dst_ip":       conn.DstIp,
				"dst_host":     conn.DstHost,
				"dst_port":     conn.DstPort,
				"uid":          conn.UserId,
				"pid":          conn.ProcessId,
				"process":      conn.ProcessPath,
				"process_args": conn.ProcessArgs,
			})
		}

		// Update stats tables
		for k, v := range stats.ByHost {
			s.db.UpsertStat("hosts", k, peerAddr, int64(v))
		}
		for k, v := range stats.ByExecutable {
			s.db.UpsertStat("procs", k, peerAddr, int64(v))
		}
		for k, v := range stats.ByAddress {
			s.db.UpsertStat("addrs", k, peerAddr, int64(v))
		}
		for k, v := range stats.ByPort {
			s.db.UpsertStat("ports", k, peerAddr, int64(v))
		}
		for k, v := range stats.ByUid {
			s.db.UpsertStat("users", k, peerAddr, int64(v))
		}

		// Broadcast stats to browsers
		s.hub.BroadcastEvent(ws.EventStatsUpdate, map[string]interface{}{
			"node":           peerAddr,
			"daemon_version": stats.DaemonVersion,
			"uptime":         stats.Uptime,
			"rules":          stats.Rules,
			"connections":    stats.Connections,
			"dropped":        stats.Dropped,
			"accepted":       stats.Accepted,
			"ignored":        stats.Ignored,
			"dns_responses":  stats.DnsResponses,
			"rule_hits":      stats.RuleHits,
			"rule_misses":    stats.RuleMisses,
			"by_proto":       stats.ByProto,
			"by_address":     stats.ByAddress,
			"by_host":        stats.ByHost,
			"by_port":        stats.ByPort,
			"by_uid":         stats.ByUid,
			"by_executable":  stats.ByExecutable,
		})
	}

	return &pb.PingReply{Id: req.Id}, nil
}

// AskRule blocks until the browser user allows/denies or timeout (120s).
// Pipeline: blocklist check → trust list check → node mode check → prompt user.
func (s *UIService) AskRule(ctx context.Context, conn *pb.Connection) (*pb.Rule, error) {
	peerAddr := peerAddrFromCtx(ctx)
	log.Printf("[grpc] AskRule from %s: %s -> %s:%d (%s)",
		peerAddr, conn.ProcessPath, conn.DstHost, conn.DstPort, conn.Protocol)
	seenFlowKey, learningKey, trackSeenFlow := buildSeenFlowKey(peerAddr, conn)

	// 1. Check blocklist — auto-deny blocked domains (even in silent_allow mode)
	if conn.DstHost != "" && s.db.IsDomainBlocked(conn.DstHost) {
		log.Printf("[grpc] AskRule: domain %s blocked by blocklist", conn.DstHost)
		return &pb.Rule{
			Name:     "blocklist-deny",
			Action:   "deny",
			Duration: "once",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "dest.host",
				Data:    conn.DstHost,
			},
		}, nil
	}

	// 2. Check process trust list
	trustLevel := s.db.GetProcessTrustLevel(peerAddr, conn.ProcessPath)
	switch trustLevel {
	case "trusted":
		log.Printf("[grpc] AskRule: process %s trusted, auto-allow", conn.ProcessPath)
		return &pb.Rule{
			Name:     "trust-allow",
			Action:   "allow",
			Duration: "once",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "process.path",
				Data:    conn.ProcessPath,
			},
		}, nil
	case "untrusted":
		log.Printf("[grpc] AskRule: process %s untrusted, forcing prompt", conn.ProcessPath)
		result, err := s.prompter.AskUser(peerAddr, conn)
		if err != nil {
			return nil, err
		}
		s.persistPromptDecision(seenFlowKey, result, trackSeenFlow)
		return result.Rule, nil
	}

	// 3. Check node mode — auto-allow or auto-deny without prompting
	mode, _ := s.db.GetNodeMode(peerAddr)
	switch mode {
	case "silent_allow":
		log.Printf("[grpc] AskRule: silent_allow for node %s", peerAddr)
		return &pb.Rule{
			Name:     "silent-allow",
			Action:   "allow",
			Duration: "once",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "dest.host",
				Data:    conn.DstHost,
			},
		}, nil
	case "silent_deny":
		log.Printf("[grpc] AskRule: silent_deny for node %s", peerAddr)
		return &pb.Rule{
			Name:     "silent-deny",
			Action:   "deny",
			Duration: "once",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "dest.host",
				Data:    conn.DstHost,
			},
		}, nil
	}

	// 4. "ask" mode — fall through to user prompt
	if trackSeenFlow {
		now := time.Now()
		flow, err := s.db.GetSeenFlow(seenFlowKey)
		if err != nil {
			log.Printf("[grpc] AskRule: seen flow lookup failed for %s: %v", peerAddr, err)
		} else if flow != nil {
			if flow.IsExpired(now) {
				if err := s.db.DeleteSeenFlow(seenFlowKey); err != nil {
					log.Printf("[grpc] AskRule: failed to delete expired seen flow for %s: %v", peerAddr, err)
				}
			} else {
				expiresAt, _ := flow.ExpiryTime()
				log.Printf("[grpc] AskRule: reusing remembered %s decision for %s -> %s:%d (%s)",
					flow.Action, conn.ProcessPath, flow.Destination, flow.DstPort, flow.Protocol)
				if err := s.db.UpsertSeenFlow(seenFlowKey, flow.Action, flow.SourceRuleName, now, expiresAt); err != nil {
					log.Printf("[grpc] AskRule: failed to refresh seen flow for %s: %v", peerAddr, err)
				}
				return ruleutil.BuildSeenFlowRule(learningKey, flow.Action), nil
			}
		}
	}

	result, err := s.prompter.AskUser(peerAddr, conn)
	if err != nil {
		return nil, err
	}
	s.persistPromptDecision(seenFlowKey, result, trackSeenFlow)

	return result.Rule, nil
}

func buildSeenFlowKey(node string, conn *pb.Connection) (db.SeenFlowKey, ruleutil.LearningKey, bool) {
	learningKey, ok := ruleutil.LearningKeyFromConnection(conn)
	if !ok {
		return db.SeenFlowKey{}, ruleutil.LearningKey{}, false
	}

	return db.SeenFlowKey{
		Node:               node,
		Process:            learningKey.Process,
		Protocol:           learningKey.Protocol,
		DstPort:            learningKey.DstPort,
		DestinationOperand: learningKey.DestinationType,
		Destination:        learningKey.Destination,
	}, learningKey, true
}

func (s *UIService) persistPromptDecision(key db.SeenFlowKey, result *prompter.AskResult, trackSeenFlow bool) {
	if !trackSeenFlow || result == nil || result.Rule == nil || result.Source != prompter.DecisionSourceUserReply {
		return
	}

	now := time.Now()
	expiresAt, persist := seenFlowRetention(result.Rule, now)
	if !persist {
		return
	}

	if err := s.db.UpsertSeenFlow(key, result.Rule.GetAction(), strings.TrimSpace(result.Rule.GetName()), now, expiresAt); err != nil {
		log.Printf("[grpc] AskRule: failed to persist seen flow for %s: %v", key.Node, err)
	}
}

func seenFlowRetention(rule *pb.Rule, now time.Time) (time.Time, bool) {
	if rule == nil {
		return time.Time{}, false
	}

	switch strings.ToLower(strings.TrimSpace(rule.GetDuration())) {
	case "always":
		return time.Time{}, true
	case "5m":
		return now.Add(5 * time.Minute), true
	case "15m":
		return now.Add(15 * time.Minute), true
	case "30m":
		return now.Add(30 * time.Minute), true
	case "1h":
		return now.Add(time.Hour), true
	default:
		return time.Time{}, false
	}
}

// Notifications is the bidirectional streaming RPC.
func (s *UIService) Notifications(stream pb.UI_NotificationsServer) error {
	peerAddr := ""
	if p, ok := peer.FromContext(stream.Context()); ok {
		peerAddr = p.Addr.String()
	}

	node := s.nodes.GetNode(peerAddr)
	if node == nil {
		return fmt.Errorf("node %s not registered", peerAddr)
	}

	log.Printf("[grpc] Notifications stream opened for %s", peerAddr)

	// Read replies from daemon in a goroutine
	errChan := make(chan error, 1)
	go func() {
		for {
			reply, err := stream.Recv()
			if err == io.EOF {
				errChan <- nil
				return
			}
			if err != nil {
				errChan <- err
				return
			}
			log.Printf("[grpc] NotificationReply from %s: id=%d code=%v", peerAddr, reply.Id, reply.Code)
		}
	}()

	// Send notifications to daemon from the node's channel
	for {
		select {
		case notif := <-node.NotifyChan:
			if notif == nil || notif.Type == -1 {
				log.Printf("[grpc] Notifications stream closing for %s (sentinel)", peerAddr)
				return nil
			}
			if err := stream.Send(notif); err != nil {
				log.Printf("[grpc] Error sending notification to %s: %v", peerAddr, err)
				return err
			}
			log.Printf("[grpc] Sent notification to %s: type=%v", peerAddr, notif.Type)

		case err := <-errChan:
			log.Printf("[grpc] Notifications stream ended for %s: %v", peerAddr, err)
			s.nodes.RemoveNode(peerAddr)
			s.db.SetNodeStatus(peerAddr, "offline")
			s.hub.BroadcastEvent(ws.EventNodeDisconnected, map[string]interface{}{
				"addr": peerAddr,
			})
			return err

		case <-stream.Context().Done():
			log.Printf("[grpc] Notifications context done for %s", peerAddr)
			s.nodes.RemoveNode(peerAddr)
			s.db.SetNodeStatus(peerAddr, "offline")
			s.hub.BroadcastEvent(ws.EventNodeDisconnected, map[string]interface{}{
				"addr": peerAddr,
			})
			return stream.Context().Err()
		}
	}
}

// PostAlert is called when the daemon sends an alert
func (s *UIService) PostAlert(ctx context.Context, alert *pb.Alert) (*pb.MsgResponse, error) {
	peerAddr := peerAddrFromCtx(ctx)
	log.Printf("[grpc] PostAlert from %s: type=%v priority=%v what=%v", peerAddr, alert.Type, alert.Priority, alert.What)

	body := ""
	switch d := alert.Data.(type) {
	case *pb.Alert_Text:
		body = d.Text
	}

	s.db.InsertAlert(&db.DBAlert{
		Time:     time.Now().Format("2006-01-02 15:04:05"),
		Node:     peerAddr,
		Type:     int(alert.Type),
		Action:   int(alert.Action),
		Priority: int(alert.Priority),
		What:     int(alert.What),
		Body:     body,
		Status:   "new",
	})

	s.hub.BroadcastEvent(ws.EventNewAlert, map[string]interface{}{
		"node":     peerAddr,
		"type":     alert.Type,
		"priority": alert.Priority,
		"what":     alert.What,
		"body":     body,
	})

	return &pb.MsgResponse{Id: alert.Id}, nil
}
