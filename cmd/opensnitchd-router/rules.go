package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

type ruleEntry struct {
	Rule    *pb.Rule  `json:"rule"`
	AddedAt time.Time `json:"added_at"`
}

func (d *daemon) loadRules() error {
	data, err := os.ReadFile(filepath.Clean(d.rulesPath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var stored []*ruleEntry
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}

	now := time.Now()
	d.rulesMu.Lock()
	defer d.rulesMu.Unlock()
	for _, entry := range stored {
		if entry == nil || entry.Rule == nil || strings.TrimSpace(entry.Rule.GetName()) == "" {
			continue
		}
		if ruleExpired(entry, now) {
			continue
		}
		d.rules[entry.Rule.GetName()] = entry
	}
	return nil
}

func (d *daemon) snapshotRules() []*pb.Rule {
	d.rulesMu.Lock()
	defer d.rulesMu.Unlock()
	d.pruneExpiredRulesLocked(time.Now())

	names := make([]string, 0, len(d.rules))
	for name := range d.rules {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left := d.rules[names[i]]
		right := d.rules[names[j]]
		switch {
		case left.Rule.GetPrecedence() != right.Rule.GetPrecedence():
			return left.Rule.GetPrecedence()
		case !left.AddedAt.Equal(right.AddedAt):
			return left.AddedAt.Before(right.AddedAt)
		default:
			return names[i] < names[j]
		}
	})

	result := make([]*pb.Rule, 0, len(names))
	for _, name := range names {
		result = append(result, cloneRule(d.rules[name].Rule))
	}
	return result
}

func (d *daemon) upsertRule(rule *pb.Rule, addedAt time.Time) error {
	if rule == nil || strings.TrimSpace(rule.GetName()) == "" {
		return nil
	}
	if addedAt.IsZero() {
		addedAt = time.Now()
	}

	d.rulesMu.Lock()
	defer d.rulesMu.Unlock()
	d.rules[rule.GetName()] = &ruleEntry{
		Rule:    cloneRule(rule),
		AddedAt: addedAt,
	}
	d.pruneExpiredRulesLocked(time.Now())
	return d.saveRulesLocked()
}

func (d *daemon) deleteRule(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	d.rulesMu.Lock()
	defer d.rulesMu.Unlock()
	delete(d.rules, name)
	return d.saveRulesLocked()
}

func (d *daemon) saveRulesLocked() error {
	stored := make([]*ruleEntry, 0, len(d.rules))
	for _, entry := range d.rules {
		if !ruleShouldPersist(entry.Rule) {
			continue
		}
		stored = append(stored, &ruleEntry{
			Rule:    cloneRule(entry.Rule),
			AddedAt: entry.AddedAt,
		})
	}
	sort.Slice(stored, func(i, j int) bool {
		return stored[i].Rule.GetName() < stored[j].Rule.GetName()
	})

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(d.rulesPath, append(data, '\n'), 0600)
}

func (d *daemon) pruneExpiredRulesLocked(now time.Time) {
	dirty := false
	for name, entry := range d.rules {
		if ruleExpired(entry, now) {
			delete(d.rules, name)
			dirty = true
		}
	}
	if dirty {
		_ = d.saveRulesLocked()
	}
}

func (d *daemon) evaluateLocalFlow(flow *localFlow) (*pb.Rule, bool, error) {
	d.rulesMu.Lock()
	defer d.rulesMu.Unlock()
	d.pruneExpiredRulesLocked(time.Now())

	names := d.sortedRuleNamesLocked()
	for _, name := range names {
		entry := d.rules[name]
		if !ruleEnabled(entry.Rule) || !matchesLocalRule(entry.Rule, flow) {
			continue
		}
		rule := cloneRule(entry.Rule)
		if strings.EqualFold(rule.GetDuration(), "once") {
			delete(d.rules, name)
			_ = d.saveRulesLocked()
		}
		return rule, true, nil
	}
	return nil, false, nil
}

func (d *daemon) evaluateForwardFlow(flow *forwardFlow) (*pb.Rule, bool) {
	d.rulesMu.Lock()
	defer d.rulesMu.Unlock()
	d.pruneExpiredRulesLocked(time.Now())

	names := d.sortedRuleNamesLocked()
	for _, name := range names {
		entry := d.rules[name]
		if !ruleEnabled(entry.Rule) || !matchesForwardRule(entry.Rule, flow) {
			continue
		}
		rule := cloneRule(entry.Rule)
		if strings.EqualFold(rule.GetDuration(), "once") {
			delete(d.rules, name)
			_ = d.saveRulesLocked()
		}
		return rule, true
	}
	return nil, false
}

func (d *daemon) sortedRuleNamesLocked() []string {
	names := make([]string, 0, len(d.rules))
	for name := range d.rules {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		left := d.rules[names[i]]
		right := d.rules[names[j]]
		switch {
		case left.Rule.GetPrecedence() != right.Rule.GetPrecedence():
			return left.Rule.GetPrecedence()
		case !left.AddedAt.Equal(right.AddedAt):
			return left.AddedAt.Before(right.AddedAt)
		default:
			return names[i] < names[j]
		}
	})
	return names
}

func (d *daemon) handleNotification(notif *pb.Notification) error {
	switch notif.GetType() {
	case pb.Action_CHANGE_RULE:
		for _, rule := range notif.GetRules() {
			if err := d.upsertRule(rule, ruleTimestamp(rule)); err != nil {
				return err
			}
		}
		return d.rebuildForwardRules()
	case pb.Action_DELETE_RULE:
		for _, rule := range notif.GetRules() {
			if err := d.deleteRule(rule.GetName()); err != nil {
				return err
			}
		}
		return d.rebuildForwardRules()
	case pb.Action_ENABLE_RULE, pb.Action_DISABLE_RULE:
		enabled := notif.GetType() == pb.Action_ENABLE_RULE
		for _, rule := range notif.GetRules() {
			if err := d.setRuleEnabled(rule.GetName(), enabled); err != nil {
				return err
			}
		}
		return d.rebuildForwardRules()
	case pb.Action_ENABLE_INTERCEPTION:
		d.setInterceptionEnabled(true)
		return nil
	case pb.Action_DISABLE_INTERCEPTION:
		d.setInterceptionEnabled(false)
		return nil
	case pb.Action_ENABLE_FIREWALL:
		d.setFirewallEnabled(true)
		return d.rebuildForwardRules()
	case pb.Action_DISABLE_FIREWALL:
		d.setFirewallEnabled(false)
		return d.disableFirewall()
	case pb.Action_RELOAD_FW_RULES:
		return d.reloadFirewallState()
	case pb.Action_CHANGE_CONFIG:
		return d.applyConfigChange(notif.GetData())
	case pb.Action_STOP:
		return fmt.Errorf("received stop notification")
	default:
		return nil
	}
}

func (d *daemon) applyConfigChange(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return err
	}

	if value, ok := payload["default_action"].(string); ok && strings.TrimSpace(value) != "" {
		d.defaultAction = strings.ToLower(strings.TrimSpace(value))
	}
	return nil
}

func (d *daemon) setRuleEnabled(name string, enabled bool) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	d.rulesMu.Lock()
	defer d.rulesMu.Unlock()

	entry := d.rules[name]
	if entry == nil || entry.Rule == nil {
		return nil
	}
	entry.Rule.Enabled = enabled
	return d.saveRulesLocked()
}

func (d *daemon) setInterceptionEnabled(enabled bool) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	d.interceptionEnabled = enabled
}

func (d *daemon) setFirewallEnabled(enabled bool) {
	d.stateMu.Lock()
	defer d.stateMu.Unlock()
	d.firewallEnabled = enabled
}

func (d *daemon) isInterceptionEnabled() bool {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()
	return d.interceptionEnabled
}

func (d *daemon) isFirewallEnabled() bool {
	d.stateMu.RLock()
	defer d.stateMu.RUnlock()
	return d.firewallEnabled
}

func ruleEnabled(rule *pb.Rule) bool {
	return rule != nil && rule.GetEnabled()
}

func ruleShouldPersist(rule *pb.Rule) bool {
	switch strings.ToLower(strings.TrimSpace(rule.GetDuration())) {
	case "once", "until restart":
		return false
	default:
		return true
	}
}

func ruleTimestamp(rule *pb.Rule) time.Time {
	if rule != nil && rule.GetCreated() > 0 {
		return time.Unix(rule.GetCreated(), 0)
	}
	return time.Now()
}

func ruleExpired(entry *ruleEntry, now time.Time) bool {
	if entry == nil || entry.Rule == nil {
		return true
	}

	addedAt := entry.AddedAt
	if addedAt.IsZero() {
		addedAt = time.Now()
	}

	switch strings.ToLower(strings.TrimSpace(entry.Rule.GetDuration())) {
	case "", "always", "until restart", "once":
		return false
	case "5m":
		return now.After(addedAt.Add(5 * time.Minute))
	case "15m":
		return now.After(addedAt.Add(15 * time.Minute))
	case "30m":
		return now.After(addedAt.Add(30 * time.Minute))
	case "1h":
		return now.After(addedAt.Add(time.Hour))
	default:
		return false
	}
}

func cloneRule(rule *pb.Rule) *pb.Rule {
	if rule == nil {
		return nil
	}
	data, err := json.Marshal(rule)
	if err != nil {
		return rule
	}
	var cloned pb.Rule
	if err := json.Unmarshal(data, &cloned); err != nil {
		return rule
	}
	return &cloned
}

func matchesLocalRule(rule *pb.Rule, flow *localFlow) bool {
	return matchRule(rule, func(operand, value string) bool {
		switch operand {
		case "process.path":
			return strings.TrimSpace(flow.ProcessPath) == value
		case "dest.ip":
			return strings.TrimSpace(flow.DstIP) == value
		case "dest.port":
			return strconv.Itoa(int(flow.DstPort)) == value
		case "protocol":
			return strings.EqualFold(flow.Protocol, value)
		case "user.id":
			return strconv.Itoa(int(flow.UID)) == value
		default:
			return false
		}
	})
}

func matchesForwardRule(rule *pb.Rule, flow *forwardFlow) bool {
	return matchRule(rule, func(operand, value string) bool {
		switch operand {
		case "process.path":
			return value == "device:"+flow.SrcIP
		case "dest.ip":
			return strings.TrimSpace(flow.DstIP) == value
		case "dest.port":
			return strconv.Itoa(int(flow.DstPort)) == value
		case "protocol":
			return strings.EqualFold(flow.Protocol, value)
		case "user.id":
			return value == "0"
		default:
			return false
		}
	})
}

func matchRule(rule *pb.Rule, match func(operand, value string) bool) bool {
	if rule == nil || rule.GetOperator() == nil {
		return false
	}
	return matchOperator(rule.GetOperator(), match)
}

func matchOperator(operator *pb.Operator, match func(operand, value string) bool) bool {
	if operator == nil {
		return false
	}

	operatorType := strings.ToLower(strings.TrimSpace(operator.GetType()))
	if len(operator.GetList()) > 0 && operatorType == "" {
		operatorType = "list"
	}

	switch operatorType {
	case "", "simple":
		return match(strings.TrimSpace(operator.GetOperand()), strings.TrimSpace(operator.GetData()))
	case "list":
		for _, item := range operator.GetList() {
			if !matchOperator(item, match) {
				return false
			}
		}
		return len(operator.GetList()) > 0
	default:
		return false
	}
}

type forwardRuleSpec struct {
	Name     string
	SourceIP string
	DestIP   string
	Port     string
	Protocol string
	Action   string
}

func compileForwardRule(rule *pb.Rule) *forwardRuleSpec {
	if rule == nil || !ruleEnabled(rule) {
		return nil
	}

	spec := &forwardRuleSpec{
		Name:   strings.TrimSpace(rule.GetName()),
		Action: strings.ToLower(strings.TrimSpace(rule.GetAction())),
	}

	var operands []struct {
		operand string
		data    string
	}
	collectOperators(rule.GetOperator(), &operands)
	for _, item := range operands {
		switch item.operand {
		case "process.path":
			if !strings.HasPrefix(item.data, "device:") {
				return nil
			}
			spec.SourceIP = strings.TrimPrefix(item.data, "device:")
		case "dest.ip":
			spec.DestIP = item.data
		case "dest.port":
			spec.Port = item.data
		case "protocol":
			spec.Protocol = strings.ToLower(item.data)
		case "user.id":
		default:
			return nil
		}
	}

	if spec.SourceIP == "" {
		return nil
	}
	if !slices.Contains([]string{"allow", "deny", "reject"}, spec.Action) {
		return nil
	}
	return spec
}

func collectOperators(operator *pb.Operator, out *[]struct {
	operand string
	data    string
}) {
	if operator == nil {
		return
	}

	operatorType := strings.ToLower(strings.TrimSpace(operator.GetType()))
	if len(operator.GetList()) > 0 && operatorType == "" {
		operatorType = "list"
	}

	if operatorType == "list" {
		for _, item := range operator.GetList() {
			collectOperators(item, out)
		}
		return
	}

	*out = append(*out, struct {
		operand string
		data    string
	}{
		operand: strings.TrimSpace(operator.GetOperand()),
		data:    strings.TrimSpace(operator.GetData()),
	})
}
