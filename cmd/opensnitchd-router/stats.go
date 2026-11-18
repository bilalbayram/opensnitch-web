package main

import (
	"strconv"
	"strings"
	"sync"
	"time"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

type statsCollector struct {
	mu sync.Mutex

	daemonStarted time.Time
	connections   uint64
	accepted      uint64
	dropped       uint64
	ruleHits      uint64
	ruleMisses    uint64

	byProto      map[string]uint64
	byAddress    map[string]uint64
	byHost       map[string]uint64
	byPort       map[string]uint64
	byUID        map[string]uint64
	byExecutable map[string]uint64
	events       []*pb.Event
}

func newStatsCollector() *statsCollector {
	return &statsCollector{
		daemonStarted: time.Now(),
		byProto:       make(map[string]uint64),
		byAddress:     make(map[string]uint64),
		byHost:        make(map[string]uint64),
		byPort:        make(map[string]uint64),
		byUID:         make(map[string]uint64),
		byExecutable:  make(map[string]uint64),
	}
}

func (s *statsCollector) record(conn *pb.Connection, rule *pb.Rule, matched bool) {
	if conn == nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.connections++
	if matched {
		s.ruleHits++
	} else {
		s.ruleMisses++
	}

	action := strings.ToLower(strings.TrimSpace(rule.GetAction()))
	switch action {
	case "deny", "reject":
		s.dropped++
	default:
		s.accepted++
	}

	proto := strings.ToLower(strings.TrimSpace(conn.GetProtocol()))
	if proto != "" {
		s.byProto[proto]++
	}
	if dstIP := strings.TrimSpace(conn.GetDstIp()); dstIP != "" {
		s.byAddress[dstIP]++
	}
	if dstHost := strings.TrimSpace(conn.GetDstHost()); dstHost != "" {
		s.byHost[dstHost]++
	}
	if conn.GetDstPort() > 0 {
		s.byPort[strconv.Itoa(int(conn.GetDstPort()))]++
	}
	s.byUID[strconv.Itoa(int(conn.GetUserId()))]++
	if process := strings.TrimSpace(conn.GetProcessPath()); process != "" {
		s.byExecutable[process]++
	}

	s.events = append(s.events, &pb.Event{
		Time:       time.Now().Format("2006-01-02 15:04:05"),
		Connection: conn,
		Rule:       cloneRule(rule),
		Unixnano:   time.Now().UnixNano(),
	})
	if len(s.events) > 256 {
		s.events = s.events[len(s.events)-256:]
	}
}

func (s *statsCollector) snapshot(ruleCount int) *pb.Statistics {
	s.mu.Lock()
	defer s.mu.Unlock()

	events := make([]*pb.Event, len(s.events))
	copy(events, s.events)
	s.events = s.events[:0]

	return &pb.Statistics{
		DaemonVersion: daemonVersion(),
		Rules:         uint64(ruleCount),
		Uptime:        uint64(time.Since(s.daemonStarted).Seconds()),
		Connections:   s.connections,
		Accepted:      s.accepted,
		Dropped:       s.dropped,
		RuleHits:      s.ruleHits,
		RuleMisses:    s.ruleMisses,
		ByProto:       cloneCounterMap(s.byProto),
		ByAddress:     cloneCounterMap(s.byAddress),
		ByHost:        cloneCounterMap(s.byHost),
		ByPort:        cloneCounterMap(s.byPort),
		ByUid:         cloneCounterMap(s.byUID),
		ByExecutable:  cloneCounterMap(s.byExecutable),
		Events:        events,
	}
}

func cloneCounterMap(src map[string]uint64) map[string]uint64 {
	out := make(map[string]uint64, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
