package grpcserver

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/nodemanager"
	"github.com/bilalbayram/opensnitch-web/internal/prompter"
	ruleutil "github.com/bilalbayram/opensnitch-web/internal/rules"
	"github.com/bilalbayram/opensnitch-web/internal/templatesync"
	"github.com/bilalbayram/opensnitch-web/internal/ws"
	pb "github.com/bilalbayram/opensnitch-web/proto"
	"google.golang.org/grpc/peer"
)

// UIService implements the proto UI gRPC service interface.
// The OpenSnitch daemon connects to this server.
type UIService struct {
	pb.UnimplementedUIServer

	nodes        *nodemanager.Manager
	db           *db.Database
	hub          *ws.Hub
	prompter     *prompter.Prompter
	templateSync *templatesync.Service
}

func NewUIService(nodes *nodemanager.Manager, database *db.Database, hub *ws.Hub, p *prompter.Prompter, templateSync *templatesync.Service) *UIService {
	return &UIService{
		nodes:        nodes,
		db:           database,
		hub:          hub,
		prompter:     p,
		templateSync: templateSync,
	}
}

func peerAddrFromCtx(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		return "unknown"
	}
	return p.Addr.String()
}

func nodeAddrFromCtx(ctx context.Context) string {
	if resolved := resolvedNodeAddrFromContext(ctx); resolved != "" {
		return resolved
	}
	return peerAddrFromCtx(ctx)
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

func normalizePeerAddr(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return host
	}
	return value
}

func (s *UIService) persistConnection(node string, conn *pb.Connection, rule *pb.Rule, eventTime string) {
	if conn == nil {
		return
	}

	action := ""
	ruleName := ""
	if rule != nil {
		action = strings.TrimSpace(rule.GetAction())
		ruleName = strings.TrimSpace(rule.GetName())
	}
	if eventTime == "" {
		eventTime = ruleutil.FormatStoredTime(time.Now())
	}

	record := &db.Connection{
		Time:        eventTime,
		Node:        node,
		Action:      action,
		Protocol:    conn.GetProtocol(),
		SrcIP:       normalizePeerAddr(conn.GetSrcIp()),
		SrcPort:     int(conn.GetSrcPort()),
		DstIP:       normalizePeerAddr(conn.GetDstIp()),
		DstHost:     conn.GetDstHost(),
		DstPort:     int(conn.GetDstPort()),
		UID:         int(conn.GetUserId()),
		PID:         int(conn.GetProcessId()),
		Process:     conn.GetProcessPath(),
		ProcessArgs: strings.Join(conn.GetProcessArgs(), " "),
		ProcessCwd:  conn.GetProcessCwd(),
		Rule:        ruleName,
	}

	if err := s.db.InsertConnection(record); err != nil {
		log.Printf("[grpc] Failed to store connection for %s: %v", node, err)
	}

	if record.DstHost != "" && record.DstIP != "" && record.DstPort != 53 {
		if err := s.db.UpsertDNSDomain(node, record.DstHost, record.DstIP, eventTime); err != nil {
			log.Printf("[grpc] Failed to upsert DNS mapping for %s: %v", node, err)
		}
	}

	s.hub.BroadcastEvent(ws.EventConnectionEvent, map[string]interface{}{
		"time":         record.Time,
		"node":         record.Node,
		"action":       record.Action,
		"rule":         record.Rule,
		"protocol":     record.Protocol,
		"src_ip":       record.SrcIP,
		"src_port":     record.SrcPort,
		"dst_ip":       record.DstIP,
		"dst_host":     record.DstHost,
		"dst_port":     record.DstPort,
		"uid":          record.UID,
		"pid":          record.PID,
		"process":      record.Process,
		"process_args": conn.GetProcessArgs(),
	})
}

func formatAlertBody(alert *pb.Alert) string {
	if alert == nil {
		return ""
	}

	switch data := alert.GetData().(type) {
	case *pb.Alert_Text:
		return strings.TrimSpace(data.Text)
	case *pb.Alert_Proc:
		proc := data.Proc
		if proc == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("Process %s (pid %d)", proc.GetPath(), proc.GetPid()))
	case *pb.Alert_Conn:
		conn := data.Conn
		if conn == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf(
			"%s -> %s:%d (%s)",
			conn.GetProcessPath(),
			conn.GetDstHost(),
			conn.GetDstPort(),
			conn.GetProtocol(),
		))
	case *pb.Alert_Rule:
		rule := data.Rule
		if rule == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("Rule %s (%s)", rule.GetName(), rule.GetAction()))
	case *pb.Alert_Fwrule:
		fwRule := data.Fwrule
		if fwRule == nil {
			return ""
		}
		return strings.TrimSpace(fmt.Sprintf("Firewall rule %s", fwRule.GetDescription()))
	default:
		return ""
	}
}

// Subscribe is called when a daemon first connects.
func (s *UIService) Subscribe(ctx context.Context, config *pb.ClientConfig) (*pb.ClientConfig, error) {
	peerAddr := peerAddrFromCtx(ctx)
	nodeAddr := nodeAddrFromCtx(ctx)
	log.Printf("[grpc] Subscribe from %s (name: %s, version: %s, rules: %d)",
		peerAddr, config.GetName(), config.GetVersion(), len(config.GetRules()))

	s.nodes.AddNode(nodeAddr, config)

	s.db.UpsertNode(&db.Node{
		Addr:          nodeAddr,
		Hostname:      config.GetName(),
		DaemonVersion: config.GetVersion(),
		Status:        db.NodeStatusOnline,
		LastConn:      time.Now().Format("2006-01-02 15:04:05"),
		DaemonRules:   int64(len(config.GetRules())),
	})

	// Replace the node rule snapshot with the daemon's current view.
	snapshotRules := make([]*db.DBRule, 0, len(config.GetRules()))
	observedAt := time.Now()
	for _, r := range config.GetRules() {
		dbRule, err := ruleutil.ProtoToDBRule(nodeAddr, observedAt, r)
		if err != nil {
			log.Printf("[grpc] Failed to convert rule %q from %s: %v", r.GetName(), nodeAddr, err)
			continue
		}
		if s.templateSync != nil {
			if err := s.templateSync.DecorateStoredRule(dbRule); err != nil {
				log.Printf("[grpc] Failed to decorate stored rule %q from %s: %v", r.GetName(), nodeAddr, err)
			}
		}
		snapshotRules = append(snapshotRules, dbRule)
	}
	if err := s.db.ReplaceNodeRulesSnapshot(nodeAddr, snapshotRules); err != nil {
		return nil, err
	}
	if s.templateSync != nil {
		if err := s.templateSync.ReconcileNode(nodeAddr); err != nil {
			log.Printf("[grpc] Failed to reconcile templates for %s: %v", nodeAddr, err)
		}
	}

	s.hub.BroadcastEvent(ws.EventNodeConnected, map[string]interface{}{
		"addr":     nodeAddr,
		"hostname": config.GetName(),
		"version":  config.GetVersion(),
	})

	return config, nil
}

// Ping is the heartbeat — daemon sends stats every ~1s
func (s *UIService) Ping(ctx context.Context, req *pb.PingRequest) (*pb.PingReply, error) {
	nodeAddr := nodeAddrFromCtx(ctx)

	node := s.nodes.GetNode(nodeAddr)
	if node == nil {
		return &pb.PingReply{Id: req.Id}, nil
	}

	node.UpdateStats(req.Stats)

	stats := req.GetStats()
	if stats != nil {
		s.db.UpsertNode(&db.Node{
			Addr:          nodeAddr,
			Hostname:      node.Hostname,
			DaemonVersion: stats.DaemonVersion,
			DaemonUptime:  int64(stats.Uptime),
			DaemonRules:   int64(stats.Rules),
			Cons:          int64(stats.Connections),
			ConsDropped:   int64(stats.Dropped),
			Status:        db.NodeStatusOnline,
			LastConn:      time.Now().Format("2006-01-02 15:04:05"),
		})

		// Store connection events
		for _, evt := range stats.Events {
			if evt.Connection == nil {
				continue
			}
			s.persistConnection(nodeAddr, evt.Connection, evt.Rule, normalizeEventTime(evt.Time, evt.Unixnano))
		}

		// Update stats tables
		for k, v := range stats.ByHost {
			s.db.UpsertStat("hosts", k, nodeAddr, int64(v))
		}
		for k, v := range stats.ByExecutable {
			s.db.UpsertStat("procs", k, nodeAddr, int64(v))
		}
		for k, v := range stats.ByAddress {
			s.db.UpsertStat("addrs", k, nodeAddr, int64(v))
		}
		for k, v := range stats.ByPort {
			s.db.UpsertStat("ports", k, nodeAddr, int64(v))
		}
		for k, v := range stats.ByUid {
			s.db.UpsertStat("users", k, nodeAddr, int64(v))
		}

		// Broadcast stats to browsers
		s.hub.BroadcastEvent(ws.EventStatsUpdate, map[string]interface{}{
			"node":           nodeAddr,
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
	nodeAddr := nodeAddrFromCtx(ctx)
	log.Printf("[grpc] AskRule from %s: %s -> %s:%d (%s)",
		peerAddr, conn.ProcessPath, conn.DstHost, conn.DstPort, conn.Protocol)
	seenFlowKey, learningKey, trackSeenFlow := buildSeenFlowKey(nodeAddr, conn)

	// 1. Check blocklist — auto-deny blocked domains (even in silent_allow mode)
	if conn.DstHost != "" && s.db.IsDomainBlocked(conn.DstHost) {
		log.Printf("[grpc] AskRule: domain %s blocked by blocklist", conn.DstHost)
		rule := &pb.Rule{
			Name:     "blocklist-deny",
			Action:   "deny",
			Duration: "once",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "dest.host",
				Data:    conn.DstHost,
			},
		}
		s.persistConnection(nodeAddr, conn, rule, "")
		return rule, nil
	}

	// 2. Check process trust list
	trustLevel := s.db.GetProcessTrustLevel(nodeAddr, conn.ProcessPath)
	switch trustLevel {
	case db.TrustLevelTrusted:
		log.Printf("[grpc] AskRule: process %s trusted, auto-allow", conn.ProcessPath)
		rule := &pb.Rule{
			Name:     "trust-allow",
			Action:   "allow",
			Duration: "once",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "process.path",
				Data:    conn.ProcessPath,
			},
		}
		s.persistConnection(nodeAddr, conn, rule, "")
		return rule, nil
	case db.TrustLevelUntrusted:
		log.Printf("[grpc] AskRule: process %s untrusted, forcing prompt", conn.ProcessPath)
		result, err := s.prompter.AskUser(nodeAddr, conn)
		if err != nil {
			return nil, err
		}
		s.persistPromptDecision(seenFlowKey, result, trackSeenFlow)
		s.persistConnection(nodeAddr, conn, result.Rule, "")
		return result.Rule, nil
	}

	// 3. Check node mode — auto-allow or auto-deny without prompting
	mode, _ := s.db.GetNodeMode(nodeAddr)
	switch mode {
	case db.ModeSilentAllow:
		log.Printf("[grpc] AskRule: silent_allow for node %s", nodeAddr)
		rule := &pb.Rule{
			Name:     "silent-allow",
			Action:   "allow",
			Duration: "once",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "dest.host",
				Data:    conn.DstHost,
			},
		}
		s.persistConnection(nodeAddr, conn, rule, "")
		return rule, nil
	case db.ModeSilentDeny:
		log.Printf("[grpc] AskRule: silent_deny for node %s", nodeAddr)
		rule := &pb.Rule{
			Name:     "silent-deny",
			Action:   "deny",
			Duration: "once",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "dest.host",
				Data:    conn.DstHost,
			},
		}
		s.persistConnection(nodeAddr, conn, rule, "")
		return rule, nil
	}

	// 4. "ask" mode — fall through to user prompt
	if trackSeenFlow {
		now := time.Now()
		flow, err := s.db.GetSeenFlow(seenFlowKey)
		if err != nil {
			log.Printf("[grpc] AskRule: seen flow lookup failed for %s: %v", nodeAddr, err)
		} else if flow != nil {
			if flow.IsExpired(now) {
				if err := s.db.DeleteSeenFlow(seenFlowKey); err != nil {
					log.Printf("[grpc] AskRule: failed to delete expired seen flow for %s: %v", nodeAddr, err)
				}
			} else {
				expiresAt, _ := flow.ExpiryTime()
				log.Printf("[grpc] AskRule: reusing remembered %s decision for %s -> %s:%d (%s)",
					flow.Action, conn.ProcessPath, flow.Destination, flow.DstPort, flow.Protocol)
				if err := s.db.UpsertSeenFlow(seenFlowKey, flow.Action, flow.SourceRuleName, now, expiresAt); err != nil {
					log.Printf("[grpc] AskRule: failed to refresh seen flow for %s: %v", nodeAddr, err)
				}
				return ruleutil.BuildSeenFlowRule(learningKey, flow.Action), nil
			}
		}
	}

	result, err := s.prompter.AskUser(nodeAddr, conn)
	if err != nil {
		return nil, err
	}
	s.persistPromptDecision(seenFlowKey, result, trackSeenFlow)
	s.persistConnection(nodeAddr, conn, result.Rule, "")

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
	nodeAddr := nodeAddrFromCtx(stream.Context())

	node := s.nodes.GetNode(nodeAddr)
	if node == nil {
		return fmt.Errorf("node %s not registered", nodeAddr)
	}

	log.Printf("[grpc] Notifications stream opened for %s", nodeAddr)

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
			log.Printf("[grpc] NotificationReply from %s: id=%d code=%v", nodeAddr, reply.Id, reply.Code)
		}
	}()

	// Send notifications to daemon from the node's channel
	for {
		select {
		case notif := <-node.NotifyChan:
			if notif == nil || notif.Type == -1 {
				log.Printf("[grpc] Notifications stream closing for %s (sentinel)", nodeAddr)
				return nil
			}
			if err := stream.Send(notif); err != nil {
				log.Printf("[grpc] Error sending notification to %s: %v", nodeAddr, err)
				return err
			}
			log.Printf("[grpc] Sent notification to %s: type=%v", nodeAddr, notif.Type)

		case err := <-errChan:
			log.Printf("[grpc] Notifications stream ended for %s: %v", nodeAddr, err)
			s.handleNodeDisconnect(nodeAddr)
			return err

		case <-stream.Context().Done():
			log.Printf("[grpc] Notifications context done for %s", nodeAddr)
			s.handleNodeDisconnect(nodeAddr)
			return stream.Context().Err()
		}
	}
}

func (s *UIService) handleNodeDisconnect(addr string) {
	s.nodes.RemoveNode(addr)
	s.db.SetNodeStatus(addr, db.NodeStatusOffline)
	s.hub.BroadcastEvent(ws.EventNodeDisconnected, map[string]interface{}{
		"addr": addr,
	})
}

// PostAlert is called when the daemon sends an alert
func (s *UIService) PostAlert(ctx context.Context, alert *pb.Alert) (*pb.MsgResponse, error) {
	peerAddr := peerAddrFromCtx(ctx)
	nodeAddr := nodeAddrFromCtx(ctx)
	log.Printf("[grpc] PostAlert from %s: type=%v priority=%v what=%v", peerAddr, alert.Type, alert.Priority, alert.What)

	body := formatAlertBody(alert)

	s.db.InsertAlert(&db.DBAlert{
		Time:     time.Now().Format("2006-01-02 15:04:05"),
		Node:     nodeAddr,
		Type:     int(alert.Type),
		Action:   int(alert.Action),
		Priority: int(alert.Priority),
		What:     int(alert.What),
		Body:     body,
		Status:   "new",
	})

	s.hub.BroadcastEvent(ws.EventNewAlert, map[string]interface{}{
		"node":     nodeAddr,
		"type":     alert.Type,
		"priority": alert.Priority,
		"what":     alert.What,
		"body":     body,
	})

	return &pb.MsgResponse{Id: alert.Id}, nil
}
