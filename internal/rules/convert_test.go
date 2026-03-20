package rules

import (
	"testing"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	pb "github.com/bilalbayram/opensnitch-web/proto"
)

func TestFormatStoredTime(t *testing.T) {
	ts := time.Date(2024, 6, 15, 14, 30, 45, 0, time.Local)
	got := FormatStoredTime(ts)
	want := "2024-06-15 14:30:45"
	if got != want {
		t.Errorf("FormatStoredTime = %q, want %q", got, want)
	}
}

func TestFormatStoredTimeZero(t *testing.T) {
	got := FormatStoredTime(time.Time{})
	if got != "" {
		t.Errorf("FormatStoredTime(zero) = %q, want empty", got)
	}
}

func TestParseStoredTime(t *testing.T) {
	tests := []struct {
		input string
		ok    bool
	}{
		{"2024-06-15 14:30:45", true},
		{"2024-06-15T14:30:45", true},
		{"2024-06-15T14:30:45Z", true},
		{"", false},
		{"  ", false},
		{"not-a-time", false},
	}
	for _, tt := range tests {
		_, err := ParseStoredTime(tt.input)
		if (err == nil) != tt.ok {
			t.Errorf("ParseStoredTime(%q) error=%v, wantOK=%v", tt.input, err, tt.ok)
		}
	}
}

func TestDBRuleToProto(t *testing.T) {
	rule := &db.DBRule{
		Name:            "test-rule",
		Enabled:         true,
		Action:          "allow",
		Duration:        "always",
		OperatorType:    "simple",
		OperatorOperand: "dest.host",
		OperatorData:    "example.com",
		Created:         "2024-06-15 14:30:45",
	}

	proto, err := DBRuleToProto(rule)
	if err != nil {
		t.Fatalf("DBRuleToProto: %v", err)
	}
	if proto.Name != "test-rule" {
		t.Errorf("Name = %q, want %q", proto.Name, "test-rule")
	}
	if proto.Action != "allow" {
		t.Errorf("Action = %q, want %q", proto.Action, "allow")
	}
	if proto.Operator == nil {
		t.Fatal("Operator is nil")
	}
	if proto.Operator.Operand != "dest.host" {
		t.Errorf("Operator.Operand = %q, want %q", proto.Operator.Operand, "dest.host")
	}
}

func TestDBRuleToProtoNil(t *testing.T) {
	_, err := DBRuleToProto(nil)
	if err == nil {
		t.Error("expected error for nil rule")
	}
}

func TestProtoToDBRule(t *testing.T) {
	now := time.Now()
	proto := &pb.Rule{
		Name:     "test-rule",
		Enabled:  true,
		Action:   "deny",
		Duration: "once",
		Operator: &pb.Operator{
			Type:    "simple",
			Operand: "process.path",
			Data:    "/usr/bin/curl",
		},
	}

	dbRule, err := ProtoToDBRule("192.168.1.1", now, proto)
	if err != nil {
		t.Fatalf("ProtoToDBRule: %v", err)
	}
	if dbRule.Name != "test-rule" {
		t.Errorf("Name = %q, want %q", dbRule.Name, "test-rule")
	}
	if dbRule.Node != "192.168.1.1" {
		t.Errorf("Node = %q, want %q", dbRule.Node, "192.168.1.1")
	}
	if dbRule.Action != "deny" {
		t.Errorf("Action = %q, want %q", dbRule.Action, "deny")
	}
	if dbRule.OperatorJSON == "" {
		t.Error("OperatorJSON should not be empty")
	}
}

func TestProtoToDBRuleNil(t *testing.T) {
	_, err := ProtoToDBRule("node", time.Now(), nil)
	if err == nil {
		t.Error("expected error for nil rule")
	}
}

func TestFingerprintForKey(t *testing.T) {
	key := LearningKey{
		Process:         "/usr/bin/curl",
		DestinationType: "dest.host",
		Destination:     "example.com",
		DstPort:         443,
		Protocol:        "tcp",
	}

	fp := FingerprintForKey(key)
	if len(fp) != generatedRuleFingerprintLen {
		t.Errorf("fingerprint length = %d, want %d", len(fp), generatedRuleFingerprintLen)
	}

	// Same key should produce same fingerprint
	fp2 := FingerprintForKey(key)
	if fp != fp2 {
		t.Error("same key should produce same fingerprint")
	}

	// Different key should produce different fingerprint
	key2 := key
	key2.DstPort = 80
	fp3 := FingerprintForKey(key2)
	if fp == fp3 {
		t.Error("different key should produce different fingerprint")
	}
}

func TestLearningKeyFromOperator(t *testing.T) {
	op := &pb.Operator{
		Type: "list",
		List: []*pb.Operator{
			{Type: "simple", Operand: "process.path", Data: "/usr/bin/curl"},
			{Type: "simple", Operand: "dest.host", Data: "example.com"},
			{Type: "simple", Operand: "dest.port", Data: "443"},
			{Type: "simple", Operand: "protocol", Data: "tcp"},
		},
	}

	key, ok := LearningKeyFromOperator(op)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if key.Process != "/usr/bin/curl" {
		t.Errorf("Process = %q", key.Process)
	}
	if key.Destination != "example.com" {
		t.Errorf("Destination = %q", key.Destination)
	}
	if key.DstPort != 443 {
		t.Errorf("DstPort = %d", key.DstPort)
	}
	if key.Protocol != "tcp" {
		t.Errorf("Protocol = %q", key.Protocol)
	}
}

func TestLearningKeyFromOperatorIncomplete(t *testing.T) {
	// Missing protocol
	op := &pb.Operator{
		Type: "list",
		List: []*pb.Operator{
			{Type: "simple", Operand: "process.path", Data: "/usr/bin/curl"},
			{Type: "simple", Operand: "dest.host", Data: "example.com"},
			{Type: "simple", Operand: "dest.port", Data: "443"},
		},
	}

	_, ok := LearningKeyFromOperator(op)
	if ok {
		t.Error("expected ok=false for incomplete operator")
	}
}

func TestLearningKeyFromOperatorSimple(t *testing.T) {
	op := &pb.Operator{
		Type:    "simple",
		Operand: "process.path",
		Data:    "/usr/bin/curl",
	}

	_, ok := LearningKeyFromOperator(op)
	if ok {
		t.Error("expected ok=false for simple operator")
	}
}

func TestBuildGeneratedRule(t *testing.T) {
	key := LearningKey{
		Process:         "/usr/bin/curl",
		DestinationType: "dest.host",
		Destination:     "example.com",
		DstPort:         443,
		Protocol:        "tcp",
	}

	rule := BuildGeneratedRule(key)
	if rule.Action != "allow" {
		t.Errorf("Action = %q, want allow", rule.Action)
	}
	if rule.Duration != "always" {
		t.Errorf("Duration = %q, want always", rule.Duration)
	}
	if !IsCompoundOperator(rule.Operator) {
		t.Error("expected compound operator")
	}
	if len(rule.Operator.List) != 4 {
		t.Errorf("operator list length = %d, want 4", len(rule.Operator.List))
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Hello World", "hello-world"},
		{"/usr/bin/curl", "usr-bin-curl"},
		{"example.com", "example-com"},
		{"", ""},
		{"---", ""},
		{"TCP", "tcp"},
	}
	for _, tt := range tests {
		got := slugify(tt.input)
		if got != tt.want {
			t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestShortenSlug(t *testing.T) {
	got := shortenSlug("very-long-process-name-that-exceeds-limit", 10)
	if len(got) > 10 {
		t.Errorf("shortenSlug length = %d, want <= 10", len(got))
	}
}

func TestDBRuleOperatorFromJSON(t *testing.T) {
	rule := &db.DBRule{
		OperatorJSON: `{"type":"simple","operand":"dest.host","data":"example.com"}`,
	}

	op, err := DBRuleOperator(rule)
	if err != nil {
		t.Fatalf("DBRuleOperator: %v", err)
	}
	if op.Operand != "dest.host" {
		t.Errorf("Operand = %q, want dest.host", op.Operand)
	}
	if op.Data != "example.com" {
		t.Errorf("Data = %q, want example.com", op.Data)
	}
}

func TestDBRuleOperatorFromFields(t *testing.T) {
	rule := &db.DBRule{
		OperatorType:    "simple",
		OperatorOperand: "process.path",
		OperatorData:    "/usr/bin/wget",
	}

	op, err := DBRuleOperator(rule)
	if err != nil {
		t.Fatalf("DBRuleOperator: %v", err)
	}
	if op.Operand != "process.path" {
		t.Errorf("Operand = %q, want process.path", op.Operand)
	}
}

func TestIsCompoundOperator(t *testing.T) {
	if IsCompoundOperator(nil) {
		t.Error("nil should not be compound")
	}

	simple := &pb.Operator{Type: "simple", Operand: "dest.host"}
	if IsCompoundOperator(simple) {
		t.Error("simple operator should not be compound")
	}

	compound := &pb.Operator{Type: "list", List: []*pb.Operator{
		{Type: "simple", Operand: "dest.host"},
	}}
	if !IsCompoundOperator(compound) {
		t.Error("list operator should be compound")
	}
}
