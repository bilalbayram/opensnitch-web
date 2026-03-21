package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

type localFlow struct {
	Protocol    string
	SrcIP       string
	SrcPort     uint32
	DstIP       string
	DstPort     uint32
	UID         uint32
	PID         int
	ProcessPath string
	ProcessCwd  string
	ProcessArgs []string
	Inode       string
}

func (f *localFlow) key() string {
	return strings.Join([]string{
		f.Protocol,
		f.SrcIP,
		strconv.Itoa(int(f.SrcPort)),
		f.DstIP,
		strconv.Itoa(int(f.DstPort)),
		strconv.Itoa(f.PID),
		f.Inode,
	}, "|")
}

func (f *localFlow) toProto() *pb.Connection {
	return &pb.Connection{
		Protocol:    f.Protocol,
		SrcIp:       f.SrcIP,
		SrcPort:     f.SrcPort,
		DstIp:       f.DstIP,
		DstPort:     f.DstPort,
		UserId:      f.UID,
		ProcessId:   uint32(f.PID),
		ProcessPath: f.ProcessPath,
		ProcessCwd:  f.ProcessCwd,
		ProcessArgs: f.ProcessArgs,
	}
}

type procSocket struct {
	Protocol string
	SrcIP    string
	SrcPort  uint32
	DstIP    string
	DstPort  uint32
	UID      uint32
	Inode    string
}

type procMeta struct {
	PID         int
	ProcessPath string
	ProcessCwd  string
	ProcessArgs []string
}

func (d *daemon) runLocalMonitor(ctx context.Context) {
	ticker := time.NewTicker(d.pollInterval)
	defer ticker.Stop()

	seen := make(map[string]time.Time)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !d.isInterceptionEnabled() {
				continue
			}
			flows, err := discoverLocalFlows()
			if err != nil {
				d.logger.Printf("discover local flows: %v", err)
				continue
			}

			now := time.Now()
			live := make(map[string]struct{}, len(flows))
			for _, flow := range flows {
				if flow == nil || flow.PID == os.Getpid() || strings.Contains(flow.ProcessPath, "opensnitchd-router") {
					continue
				}
				key := flow.key()
				live[key] = struct{}{}
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = now

				flowCopy := *flow
				go d.handleLocalFlow(ctx, &flowCopy)
			}

			for key, lastSeen := range seen {
				if _, ok := live[key]; ok {
					continue
				}
				if now.Sub(lastSeen) > 2*d.pollInterval {
					delete(seen, key)
				}
			}
		}
	}
}

func (d *daemon) handleLocalFlow(ctx context.Context, flow *localFlow) {
	rule, matched, err := d.evaluateLocalFlow(flow)
	if err != nil {
		d.logger.Printf("evaluate local flow %s: %v", flow.key(), err)
		return
	}

	if !matched {
		rule, err = d.askRule(ctx, flow)
		if err != nil {
			d.logger.Printf("AskRule failed for %s: %v", flow.key(), err)
			rule = &pb.Rule{
				Name:     fmt.Sprintf("fallback-%d", time.Now().UnixNano()),
				Action:   d.defaultAction,
				Duration: "once",
				Enabled:  true,
				Operator: &pb.Operator{
					Type:    "simple",
					Operand: "process.path",
					Data:    flow.ProcessPath,
				},
			}
		} else if !strings.EqualFold(rule.GetDuration(), "once") {
			if err := d.upsertRule(rule, ruleTimestamp(rule)); err != nil {
				d.logger.Printf("cache AskRule decision %s: %v", rule.GetName(), err)
			}
		}
	}

	if err := d.installLocalDecision(flow, rule.GetAction()); err != nil {
		d.logger.Printf("install local decision %s: %v", flow.key(), err)
	}
	d.stats.record(flow.toProto(), rule, matched)
}

func discoverLocalFlows() ([]*localFlow, error) {
	sockets := make(map[string]*procSocket)
	for _, spec := range []struct {
		path     string
		protocol string
	}{
		{path: "/proc/net/tcp", protocol: "tcp"},
		{path: "/proc/net/tcp6", protocol: "tcp6"},
		{path: "/proc/net/udp", protocol: "udp"},
		{path: "/proc/net/udp6", protocol: "udp6"},
	} {
		entries, err := readProcNet(spec.path, spec.protocol)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			sockets[entry.Inode] = entry
		}
	}

	if len(sockets) == 0 {
		return nil, nil
	}

	metaByInode := resolveProcMetadata(sockets)
	flows := make([]*localFlow, 0, len(metaByInode))
	for inode, socket := range sockets {
		meta, ok := metaByInode[inode]
		if !ok || meta.ProcessPath == "" {
			continue
		}
		flows = append(flows, &localFlow{
			Protocol:    socket.Protocol,
			SrcIP:       socket.SrcIP,
			SrcPort:     socket.SrcPort,
			DstIP:       socket.DstIP,
			DstPort:     socket.DstPort,
			UID:         socket.UID,
			PID:         meta.PID,
			ProcessPath: meta.ProcessPath,
			ProcessCwd:  meta.ProcessCwd,
			ProcessArgs: meta.ProcessArgs,
			Inode:       inode,
		})
	}

	sort.Slice(flows, func(i, j int) bool {
		return flows[i].key() < flows[j].key()
	})
	return flows, nil
}

func readProcNet(path, protocol string) ([]*procSocket, error) {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(data), "\n")
	result := make([]*procSocket, 0, len(lines))
	for _, line := range lines[1:] {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) < 10 {
			continue
		}

		srcIP, srcPort, err := decodeProcAddr(fields[1])
		if err != nil {
			continue
		}
		dstIP, dstPort, err := decodeProcAddr(fields[2])
		if err != nil || dstPort == 0 || dstIP == "" {
			continue
		}
		state := fields[3]
		if strings.EqualFold(state, "0A") {
			continue
		}

		uid, err := strconv.ParseUint(fields[7], 10, 32)
		if err != nil {
			continue
		}
		inode := fields[9]
		if inode == "" {
			continue
		}

		result = append(result, &procSocket{
			Protocol: protocol,
			SrcIP:    srcIP,
			SrcPort:  srcPort,
			DstIP:    dstIP,
			DstPort:  dstPort,
			UID:      uint32(uid),
			Inode:    inode,
		})
	}
	return result, nil
}

func resolveProcMetadata(targets map[string]*procSocket) map[string]procMeta {
	result := make(map[string]procMeta, len(targets))
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return result
	}

	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		fdEntries, err := os.ReadDir(filepath.Join("/proc", entry.Name(), "fd"))
		if err != nil {
			continue
		}

		for _, fdEntry := range fdEntries {
			link, err := os.Readlink(filepath.Join("/proc", entry.Name(), "fd", fdEntry.Name()))
			if err != nil || !strings.HasPrefix(link, "socket:[") {
				continue
			}
			inode := strings.TrimSuffix(strings.TrimPrefix(link, "socket:["), "]")
			if _, ok := targets[inode]; !ok {
				continue
			}
			if _, exists := result[inode]; exists {
				continue
			}
			result[inode] = procMeta{
				PID:         pid,
				ProcessPath: readProcLink(filepath.Join("/proc", entry.Name(), "exe")),
				ProcessCwd:  readProcLink(filepath.Join("/proc", entry.Name(), "cwd")),
				ProcessArgs: readCmdline(filepath.Join("/proc", entry.Name(), "cmdline")),
			}
		}
	}

	return result
}

func readProcLink(path string) string {
	value, err := os.Readlink(filepath.Clean(path))
	if err != nil {
		return ""
	}
	return value
}

func readCmdline(path string) []string {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil || len(data) == 0 {
		return nil
	}
	parts := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func decodeProcAddr(value string) (string, uint32, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid proc address %q", value)
	}

	portValue, err := strconv.ParseUint(parts[1], 16, 32)
	if err != nil {
		return "", 0, err
	}

	raw, err := hex.DecodeString(parts[0])
	if err != nil {
		return "", 0, err
	}
	switch len(raw) {
	case 4:
		reverse(raw)
	case 16:
		for idx := 0; idx < len(raw); idx += 4 {
			reverse(raw[idx : idx+4])
		}
	default:
		return "", 0, fmt.Errorf("unsupported proc address width %d", len(raw))
	}

	ip := net.IP(raw).String()
	if ip == "<nil>" || ip == "::" || ip == "0.0.0.0" {
		return "", 0, nil
	}
	return ip, uint32(portValue), nil
}

func reverse(data []byte) {
	for left, right := 0, len(data)-1; left < right; left, right = left+1, right-1 {
		data[left], data[right] = data[right], data[left]
	}
}
