package main

import (
	"bufio"
	"context"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

type forwardFlow struct {
	Protocol string
	SrcIP    string
	SrcPort  uint32
	DstIP    string
	DstPort  uint32
}

func (f *forwardFlow) key() string {
	return strings.Join([]string{
		f.Protocol,
		f.SrcIP,
		strconv.Itoa(int(f.SrcPort)),
		f.DstIP,
		strconv.Itoa(int(f.DstPort)),
	}, "|")
}

func (f *forwardFlow) toProto() *pb.Connection {
	return &pb.Connection{
		Protocol:    f.Protocol,
		SrcIp:       f.SrcIP,
		SrcPort:     f.SrcPort,
		DstIp:       f.DstIP,
		DstPort:     f.DstPort,
		UserId:      0,
		ProcessId:   0,
		ProcessPath: "device:" + f.SrcIP,
	}
}

func (d *daemon) runForwardMonitor(ctx context.Context) {
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
			flows, err := discoverForwardFlows()
			if err != nil {
				d.logger.Printf("discover forward flows: %v", err)
				continue
			}

			localIPs := localIPSet()
			now := time.Now()
			live := make(map[string]struct{}, len(flows))
			for _, flow := range flows {
				if flow == nil {
					continue
				}
				if _, ok := localIPs[flow.SrcIP]; ok {
					continue
				}

				key := flow.key()
				live[key] = struct{}{}
				if _, ok := seen[key]; ok {
					continue
				}
				seen[key] = now
				d.handleForwardFlow(flow)
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

func (d *daemon) handleForwardFlow(flow *forwardFlow) {
	rule, matched := d.evaluateForwardFlow(flow)
	if !matched {
		rule = &pb.Rule{
			Action:   "allow",
			Duration: "once",
			Enabled:  true,
		}
	}
	d.stats.record(flow.toProto(), rule, matched)
}

func discoverForwardFlows() ([]*forwardFlow, error) {
	cmd := exec.Command("conntrack", "-L")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	flows := make([]*forwardFlow, 0)
	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		if flow := parseConntrackLine(scanner.Text()); flow != nil {
			flows = append(flows, flow)
		}
	}
	return flows, scanner.Err()
}

func parseConntrackLine(line string) *forwardFlow {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 5 {
		return nil
	}

	protocol := strings.ToLower(strings.TrimSpace(fields[0]))
	flow := &forwardFlow{Protocol: protocol}

	for _, field := range fields {
		switch {
		case strings.HasPrefix(field, "src=") && flow.SrcIP == "":
			flow.SrcIP = strings.TrimPrefix(field, "src=")
		case strings.HasPrefix(field, "dst=") && flow.DstIP == "":
			flow.DstIP = strings.TrimPrefix(field, "dst=")
		case strings.HasPrefix(field, "sport=") && flow.SrcPort == 0:
			if value, err := strconv.ParseUint(strings.TrimPrefix(field, "sport="), 10, 32); err == nil {
				flow.SrcPort = uint32(value)
			}
		case strings.HasPrefix(field, "dport=") && flow.DstPort == 0:
			if value, err := strconv.ParseUint(strings.TrimPrefix(field, "dport="), 10, 32); err == nil {
				flow.DstPort = uint32(value)
			}
		}
		if flow.SrcIP != "" && flow.DstIP != "" && flow.DstPort > 0 {
			break
		}
	}

	if flow.SrcIP == "" || flow.DstIP == "" || flow.DstPort == 0 {
		return nil
	}
	return flow
}

func localIPSet() map[string]struct{} {
	result := make(map[string]struct{})
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return result
	}
	for _, addr := range addrs {
		switch value := addr.(type) {
		case *net.IPNet:
			result[value.IP.String()] = struct{}{}
		case *net.IPAddr:
			result[value.IP.String()] = struct{}{}
		}
	}
	return result
}
