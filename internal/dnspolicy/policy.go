package dnspolicy

import (
	"strings"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

// DNSPolicy holds the configuration for a node's DNS enforcement policy.
type DNSPolicy struct {
	Enabled           bool     `json:"enabled"`
	AllowedResolvers  []string `json:"allowed_resolvers"`
	BlockDoT          bool     `json:"block_dot"`
	BlockDoHIPs       bool     `json:"block_doh_ips"`
	BlockDoHHostnames bool     `json:"block_doh_hostnames"`
	EnabledAt         string   `json:"enabled_at,omitempty"`
}

// ConfigKey returns the web_config key used to persist a node's DNS policy.
func ConfigKey(node string) string {
	return "dns_policy:" + node
}

// ConfigPrefix is the prefix used to find all DNS policy entries.
const ConfigPrefix = "dns_policy:"

// ruleSlug converts an address or hostname into a safe string for use in rule names.
func ruleSlug(value string) string {
	s := strings.ReplaceAll(value, ".", "-")
	s = strings.ReplaceAll(s, ":", "-")
	return s
}

// BuildRules generates the OpenSnitch protobuf rules that enforce the given
// DNS policy. The caller is responsible for persisting and pushing these rules.
func BuildRules(policy DNSPolicy) []*pb.Rule {
	var rules []*pb.Rule

	// 1. Allow rules for each permitted resolver (port 53)
	for _, ip := range policy.AllowedResolvers {
		ip = strings.TrimSpace(ip)
		if ip == "" {
			continue
		}
		rules = append(rules, &pb.Rule{
			Name:        "dnspol-allow-dns-" + ruleSlug(ip),
			Description: "DNS policy: allow resolver " + ip,
			Enabled:     true,
			Precedence:  true,
			Action:      "allow",
			Duration:    "always",
			Operator: &pb.Operator{
				Type: "list",
				List: []*pb.Operator{
					{Type: "simple", Operand: "dest.port", Data: "53"},
					{Type: "simple", Operand: "dest.ip", Data: ip},
				},
			},
		})
	}

	// 2. Deny catch-all for port 53 (low precedence — matched last)
	rules = append(rules, &pb.Rule{
		Name:        "dnspol-deny-dns-catchall",
		Description: "DNS policy: deny all non-allowed DNS",
		Enabled:     true,
		Precedence:  false,
		Action:      "deny",
		Duration:    "always",
		Operator: &pb.Operator{
			Type:    "simple",
			Operand: "dest.port",
			Data:    "53",
		},
	})

	// 3. Block DNS-over-TLS (port 853)
	if policy.BlockDoT {
		rules = append(rules, &pb.Rule{
			Name:        "dnspol-deny-dot",
			Description: "DNS policy: block DNS-over-TLS (port 853)",
			Enabled:     true,
			Precedence:  true,
			Action:      "deny",
			Duration:    "always",
			Operator: &pb.Operator{
				Type:    "simple",
				Operand: "dest.port",
				Data:    "853",
			},
		})
	}

	// 4. Block known DoH provider IPs on port 443
	if policy.BlockDoHIPs {
		for _, ip := range DoHProviderIPs {
			rules = append(rules, &pb.Rule{
				Name:        "dnspol-deny-doh-" + ruleSlug(ip),
				Description: "DNS policy: block DoH provider " + ip,
				Enabled:     true,
				Precedence:  true,
				Action:      "deny",
				Duration:    "always",
				Operator: &pb.Operator{
					Type: "list",
					List: []*pb.Operator{
						{Type: "simple", Operand: "dest.port", Data: "443"},
						{Type: "simple", Operand: "dest.ip", Data: ip},
					},
				},
			})
		}
	}

	// 5. Block known DoH hostnames on port 443.
	if policy.BlockDoHHostnames {
		for _, host := range DoHHostnames {
			host = strings.TrimSpace(host)
			if host == "" {
				continue
			}
			rules = append(rules, &pb.Rule{
				Name:        "dnspol-deny-doh-host-" + ruleSlug(host),
				Description: "DNS policy: block DoH hostname " + host,
				Enabled:     true,
				Precedence:  true,
				Action:      "deny",
				Duration:    "always",
				Operator: &pb.Operator{
					Type: "list",
					List: []*pb.Operator{
						{Type: "simple", Operand: "dest.port", Data: "443"},
						{Type: "simple", Operand: "dest.host", Data: host},
					},
				},
			})
		}
	}

	return rules
}
