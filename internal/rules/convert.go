package rules

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/db"
	pb "github.com/evilsocket/opensnitch-web/proto"
)

const (
	storedTimeLayout            = "2006-01-02 15:04:05"
	generatedRuleDescription    = "Generated from observed traffic history."
	simpleOperatorType          = "simple"
	compoundOperatorType        = "list"
	generatedRuleFingerprintLen = 12
)

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	storedTimeLayout,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
}

type LearningKey struct {
	Process         string
	DestinationType string
	Destination     string
	DstPort         int
	Protocol        string
}

func FormatStoredTime(ts time.Time) string {
	if ts.IsZero() {
		return ""
	}
	return ts.In(time.Local).Format(storedTimeLayout)
}

func ParseStoredTime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errors.New("empty timestamp")
	}

	for _, layout := range timeLayouts {
		if ts, err := time.ParseInLocation(layout, trimmed, time.Local); err == nil {
			return ts, nil
		}
		if ts, err := time.Parse(layout, trimmed); err == nil {
			return ts, nil
		}
	}

	return time.Time{}, fmt.Errorf("unsupported timestamp format: %q", value)
}

func DBRuleOperator(rule *db.DBRule) (*pb.Operator, error) {
	if rule == nil {
		return nil, errors.New("rule is nil")
	}

	if strings.TrimSpace(rule.OperatorJSON) != "" {
		var operator pb.Operator
		if err := json.Unmarshal([]byte(rule.OperatorJSON), &operator); err != nil {
			return nil, fmt.Errorf("decode operator json: %w", err)
		}
		return &operator, nil
	}

	if rule.OperatorType == "" && rule.OperatorOperand == "" && rule.OperatorData == "" {
		return nil, nil
	}

	operatorType := rule.OperatorType
	if operatorType == "" {
		operatorType = simpleOperatorType
	}

	return &pb.Operator{
		Type:      operatorType,
		Operand:   rule.OperatorOperand,
		Data:      rule.OperatorData,
		Sensitive: rule.OperatorSensitive,
	}, nil
}

func DBRuleToProto(rule *db.DBRule) (*pb.Rule, error) {
	if rule == nil {
		return nil, errors.New("rule is nil")
	}

	operator, err := DBRuleOperator(rule)
	if err != nil {
		return nil, err
	}

	var created int64
	if ts, err := ParseStoredTime(rule.Created); err == nil {
		created = ts.Unix()
	}

	return &pb.Rule{
		Created:     created,
		Name:        rule.Name,
		Description: rule.Description,
		Enabled:     rule.Enabled,
		Precedence:  rule.Precedence,
		Nolog:       rule.Nolog,
		Action:      rule.Action,
		Duration:    rule.Duration,
		Operator:    operator,
	}, nil
}

func ProtoToDBRule(node string, observedAt time.Time, rule *pb.Rule) (*db.DBRule, error) {
	if rule == nil {
		return nil, errors.New("rule is nil")
	}
	if observedAt.IsZero() {
		observedAt = time.Now()
	}

	var operatorJSON string
	operator := rule.GetOperator()
	if rule.GetOperator() != nil {
		payload, err := json.Marshal(operator)
		if err != nil {
			return nil, fmt.Errorf("encode operator json: %w", err)
		}
		operatorJSON = string(payload)
	}

	createdAt := observedAt
	if rule.GetCreated() > 0 {
		createdAt = time.Unix(rule.GetCreated(), 0)
	}

	return &db.DBRule{
		Time:              FormatStoredTime(observedAt),
		Node:              node,
		Name:              rule.GetName(),
		Enabled:           rule.GetEnabled(),
		Precedence:        rule.GetPrecedence(),
		Action:            rule.GetAction(),
		Duration:          rule.GetDuration(),
		OperatorType:      operator.GetType(),
		OperatorSensitive: operator.GetSensitive(),
		OperatorOperand:   operator.GetOperand(),
		OperatorData:      operator.GetData(),
		OperatorJSON:      operatorJSON,
		Description:       rule.GetDescription(),
		Nolog:             rule.GetNolog(),
		Created:           FormatStoredTime(createdAt),
	}, nil
}

func IsCompoundOperator(operator *pb.Operator) bool {
	if operator == nil {
		return false
	}
	return strings.EqualFold(operator.GetType(), compoundOperatorType) || len(operator.GetList()) > 0
}

func LearningKeyFromRule(rule *pb.Rule) (LearningKey, bool) {
	if rule == nil {
		return LearningKey{}, false
	}
	return LearningKeyFromOperator(rule.GetOperator())
}

func LearningKeyFromOperator(operator *pb.Operator) (LearningKey, bool) {
	if !IsCompoundOperator(operator) {
		return LearningKey{}, false
	}

	var key LearningKey
	seen := map[string]bool{}

	for _, item := range operator.GetList() {
		operand := strings.TrimSpace(item.GetOperand())
		if operand == "" || seen[operand] {
			return LearningKey{}, false
		}
		seen[operand] = true

		switch operand {
		case "process.path":
			key.Process = strings.TrimSpace(item.GetData())
		case "dest.host", "dest.ip":
			if key.DestinationType != "" {
				return LearningKey{}, false
			}
			key.DestinationType = operand
			key.Destination = strings.TrimSpace(item.GetData())
		case "dest.port":
			port, err := strconv.Atoi(strings.TrimSpace(item.GetData()))
			if err != nil {
				return LearningKey{}, false
			}
			key.DstPort = port
		case "protocol":
			key.Protocol = strings.ToLower(strings.TrimSpace(item.GetData()))
		default:
			return LearningKey{}, false
		}
	}

	if key.Process == "" || key.DestinationType == "" || key.Destination == "" || key.DstPort <= 0 || key.Protocol == "" {
		return LearningKey{}, false
	}

	return key, true
}

func FingerprintForKey(key LearningKey) string {
	raw := strings.Join([]string{
		key.Process,
		key.DestinationType,
		key.Destination,
		strconv.Itoa(key.DstPort),
		strings.ToLower(key.Protocol),
	}, "\x00")

	sum := sha1.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])[:generatedRuleFingerprintLen]
}

func BuildGeneratedRule(key LearningKey) *pb.Rule {
	protocol := strings.ToLower(strings.TrimSpace(key.Protocol))
	return &pb.Rule{
		Name:        GeneratedRuleName(key),
		Description: generatedRuleDescription,
		Enabled:     true,
		Precedence:  false,
		Nolog:       false,
		Action:      "allow",
		Duration:    "always",
		Operator: &pb.Operator{
			Type: compoundOperatorType,
			List: []*pb.Operator{
				{Type: simpleOperatorType, Operand: "process.path", Data: key.Process},
				{Type: simpleOperatorType, Operand: key.DestinationType, Data: key.Destination},
				{Type: simpleOperatorType, Operand: "dest.port", Data: strconv.Itoa(key.DstPort)},
				{Type: simpleOperatorType, Operand: "protocol", Data: protocol},
			},
		},
	}
}

func GeneratedRuleName(key LearningKey) string {
	proc := shortenSlug(filepath.Base(key.Process), 24)
	if proc == "" {
		proc = "process"
	}

	dest := shortenSlug(key.Destination, 24)
	if dest == "" {
		dest = "destination"
	}

	proto := shortenSlug(key.Protocol, 8)
	if proto == "" {
		proto = "proto"
	}

	fingerprint := FingerprintForKey(key)
	return fmt.Sprintf("learn-%s-%s-%d-%s-%s", proc, dest, key.DstPort, proto, fingerprint[:8])
}

func shortenSlug(value string, max int) string {
	slug := slugify(value)
	if slug == "" {
		return ""
	}
	if len(slug) <= max {
		return slug
	}
	return strings.Trim(slug[:max], "-")
}

func slugify(value string) string {
	var builder strings.Builder
	lastDash := false

	for _, ch := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case ch >= 'a' && ch <= 'z':
			builder.WriteRune(ch)
			lastDash = false
		case ch >= '0' && ch <= '9':
			builder.WriteRune(ch)
			lastDash = false
		default:
			if builder.Len() == 0 || lastDash {
				continue
			}
			builder.WriteByte('-')
			lastDash = true
		}
	}

	return strings.Trim(builder.String(), "-")
}
