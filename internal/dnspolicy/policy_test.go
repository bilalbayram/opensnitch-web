package dnspolicy

import (
	"strings"
	"testing"
)

func TestConfigKey(t *testing.T) {
	got := ConfigKey("192.168.1.100:50051")
	want := "dns_policy:192.168.1.100:50051"
	if got != want {
		t.Errorf("ConfigKey() = %q, want %q", got, want)
	}
}

func TestBuildRules_MinimalPolicy(t *testing.T) {
	policy := DNSPolicy{
		Enabled:          true,
		AllowedResolvers: []string{"1.1.1.1"},
	}

	rules := BuildRules(policy)

	// Expect: 1 allow + 1 deny catch-all = 2
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules, got %d", len(rules))
	}

	if rules[0].Name != "dnspol-allow-dns-1-1-1-1" {
		t.Errorf("rule[0].Name = %q, want dnspol-allow-dns-1-1-1-1", rules[0].Name)
	}
	if rules[0].Action != "allow" {
		t.Errorf("rule[0].Action = %q, want allow", rules[0].Action)
	}
	if !rules[0].Precedence {
		t.Error("allow rule should have precedence=true")
	}

	if rules[1].Name != "dnspol-deny-dns-catchall" {
		t.Errorf("rule[1].Name = %q, want dnspol-deny-dns-catchall", rules[1].Name)
	}
	if rules[1].Action != "deny" {
		t.Errorf("rule[1].Action = %q, want deny", rules[1].Action)
	}
	if rules[1].Precedence {
		t.Error("deny catch-all should have precedence=false")
	}
}

func TestBuildRules_FullPolicy(t *testing.T) {
	policy := DNSPolicy{
		Enabled:           true,
		AllowedResolvers:  []string{"1.1.1.1", "8.8.8.8"},
		BlockDoT:          true,
		BlockDoHIPs:       true,
		BlockDoHHostnames: true,
	}

	rules := BuildRules(policy)

	// 2 allow + 1 deny catch-all + 1 DoT + len(DoHProviderIPs) DoH IPs + len(DoHHostnames) DoH hostnames
	expectedCount := 2 + 1 + 1 + len(DoHProviderIPs) + len(DoHHostnames)
	if len(rules) != expectedCount {
		t.Fatalf("expected %d rules, got %d", expectedCount, len(rules))
	}

	// Verify all rules have dnspol- prefix
	for _, r := range rules {
		if !strings.HasPrefix(r.Name, "dnspol-") {
			t.Errorf("rule %q missing dnspol- prefix", r.Name)
		}
	}

	// Find DoT rule
	var foundDoT bool
	for _, r := range rules {
		if r.Name == "dnspol-deny-dot" {
			foundDoT = true
			if r.Operator.Data != "853" {
				t.Errorf("DoT rule operator data = %q, want 853", r.Operator.Data)
			}
		}
	}
	if !foundDoT {
		t.Error("missing dnspol-deny-dot rule")
	}

	// Verify DoH rules block port 443
	var dohCount int
	for _, r := range rules {
		if strings.HasPrefix(r.Name, "dnspol-deny-doh-host-") {
			continue
		}
		if strings.HasPrefix(r.Name, "dnspol-deny-doh-") {
			dohCount++
			if r.Operator.List[0].Data != "443" {
				t.Errorf("DoH rule %q: port = %q, want 443", r.Name, r.Operator.List[0].Data)
			}
		}
	}
	if dohCount != len(DoHProviderIPs) {
		t.Errorf("expected %d DoH rules, got %d", len(DoHProviderIPs), dohCount)
	}

	var dohHostCount int
	for _, r := range rules {
		if strings.HasPrefix(r.Name, "dnspol-deny-doh-host-") {
			dohHostCount++
			if r.Operator.List[0].Data != "443" {
				t.Errorf("DoH hostname rule %q: port = %q, want 443", r.Name, r.Operator.List[0].Data)
			}
			if r.Operator.List[1].Operand != "dest.host" {
				t.Errorf("DoH hostname rule %q: operand = %q, want dest.host", r.Name, r.Operator.List[1].Operand)
			}
		}
	}
	if dohHostCount != len(DoHHostnames) {
		t.Errorf("expected %d DoH hostname rules, got %d", len(DoHHostnames), dohHostCount)
	}
}

func TestBuildRules_EmptyResolvers(t *testing.T) {
	policy := DNSPolicy{
		Enabled:          true,
		AllowedResolvers: []string{"", "  ", "1.1.1.1"},
	}

	rules := BuildRules(policy)

	// Only the non-empty resolver should produce an allow rule, plus the catch-all
	if len(rules) != 2 {
		t.Fatalf("expected 2 rules (1 allow + 1 deny), got %d", len(rules))
	}
}

func TestBuildRules_NoDoTWithoutFlag(t *testing.T) {
	policy := DNSPolicy{
		Enabled:          true,
		AllowedResolvers: []string{"1.1.1.1"},
		BlockDoT:         false,
		BlockDoHIPs:      false,
	}

	rules := BuildRules(policy)

	for _, r := range rules {
		if r.Name == "dnspol-deny-dot" {
			t.Error("DoT rule should not exist when BlockDoT=false")
		}
		if strings.HasPrefix(r.Name, "dnspol-deny-doh-") {
			t.Error("DoH rules should not exist when BlockDoHIPs=false")
		}
		if strings.HasPrefix(r.Name, "dnspol-deny-doh-host-") {
			t.Error("DoH hostname rules should not exist when BlockDoHHostnames=false")
		}
	}
}
