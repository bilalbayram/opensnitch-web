package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

const (
	nftTable        = "opensnitch-router"
	nftOutputChain  = "output"
	nftForwardChain = "forward"
)

func (d *daemon) ensureFirewall() error {
	if err := d.nftRun("add", "table", "inet", nftTable); err != nil && !nftAlreadyExists(err) {
		return err
	}
	if err := d.nftRun("add", "chain", "inet", nftTable, nftOutputChain, "{", "type", "filter", "hook", "output", "priority", "0;", "policy", "accept;", "}"); err != nil && !nftAlreadyExists(err) {
		return err
	}
	if err := d.nftRun("add", "chain", "inet", nftTable, nftForwardChain, "{", "type", "filter", "hook", "forward", "priority", "0;", "policy", "accept;", "}"); err != nil && !nftAlreadyExists(err) {
		return err
	}
	return nil
}

func (d *daemon) disableFirewall() error {
	err := d.nftRun("delete", "table", "inet", nftTable)
	if err != nil && strings.Contains(err.Error(), "No such file or directory") {
		return nil
	}
	return err
}

func (d *daemon) reloadFirewallState() error {
	if err := d.ensureFirewall(); err != nil {
		return err
	}
	if err := d.flushChain(nftOutputChain); err != nil {
		return err
	}
	return d.rebuildForwardRules()
}

func (d *daemon) rebuildForwardRules() error {
	if !d.isFirewallEnabled() {
		return nil
	}
	if err := d.ensureFirewall(); err != nil {
		return err
	}
	if err := d.flushChain(nftForwardChain); err != nil {
		return err
	}

	for _, rule := range d.snapshotRules() {
		spec := compileForwardRule(rule)
		if spec == nil {
			continue
		}
		if err := d.installForwardRule(spec); err != nil {
			return err
		}
	}
	return nil
}

func (d *daemon) installLocalDecision(flow *localFlow, action string) error {
	if !d.isFirewallEnabled() {
		return nil
	}
	if err := d.ensureFirewall(); err != nil {
		return err
	}

	args := []string{"add", "rule", "inet", nftTable, nftOutputChain}
	if flow.SrcIP != "" {
		args = append(args, sourceAddressMatch(flow.SrcIP)...)
	}
	if flow.DstIP != "" {
		args = append(args, nftAddressMatch(flow.DstIP)...)
	}
	if flow.Protocol != "" {
		args = append(args, protocolMatch(flow.Protocol)...)
	}
	if flow.SrcPort > 0 {
		args = append(args, portMatch(flow.Protocol, "sport", int(flow.SrcPort))...)
	}
	if flow.DstPort > 0 {
		args = append(args, portMatch(flow.Protocol, "dport", int(flow.DstPort))...)
	}
	args = append(args, nftVerdict(action))
	return d.nftRun(args...)
}

func (d *daemon) installForwardRule(spec *forwardRuleSpec) error {
	args := []string{"add", "rule", "inet", nftTable, nftForwardChain}
	args = append(args, sourceAddressMatch(spec.SourceIP)...)
	if spec.DestIP != "" {
		args = append(args, nftAddressMatch(spec.DestIP)...)
	}
	if spec.Protocol != "" {
		args = append(args, protocolMatch(spec.Protocol)...)
	}
	if spec.Port != "" {
		args = append(args, portMatch(spec.Protocol, "dport", mustAtoi(spec.Port))...)
	}
	args = append(args, nftVerdict(spec.Action))
	return d.nftRun(args...)
}

func protocolMatch(protocol string) []string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "tcp", "tcp6":
		return []string{"meta", "l4proto", "tcp"}
	case "udp", "udp6":
		return []string{"meta", "l4proto", "udp"}
	case "icmp", "icmp6":
		return []string{"meta", "l4proto", "icmp"}
	default:
		return nil
	}
}

func sourceAddressMatch(ip string) []string {
	if strings.Contains(ip, ":") {
		return []string{"ip6", "saddr", ip}
	}
	return []string{"ip", "saddr", ip}
}

func nftAddressMatch(ip string) []string {
	if strings.Contains(ip, ":") {
		return []string{"ip6", "daddr", ip}
	}
	return []string{"ip", "daddr", ip}
}

func portMatch(protocol, side string, port int) []string {
	if port <= 0 {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "udp", "udp6":
		return []string{"udp", side, fmt.Sprintf("%d", port)}
	default:
		return []string{"tcp", side, fmt.Sprintf("%d", port)}
	}
}

func nftVerdict(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "deny":
		return "drop"
	case "reject":
		return "reject"
	default:
		return "accept"
	}
}

func (d *daemon) flushChain(chain string) error {
	err := d.nftRun("flush", "chain", "inet", nftTable, chain)
	if err != nil && strings.Contains(err.Error(), "No such file or directory") {
		return nil
	}
	return err
}

func (d *daemon) nftRun(args ...string) error {
	cmd := exec.Command("nft", args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("nft %s: %w (%s)", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func nftAlreadyExists(err error) bool {
	return err != nil && strings.Contains(err.Error(), "File exists")
}

func mustAtoi(value string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(value))
	return n
}
