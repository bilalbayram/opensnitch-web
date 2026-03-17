package rules

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/evilsocket/opensnitch-web/internal/db"
	pb "github.com/evilsocket/opensnitch-web/proto"
)

const managedRulePrefix = "tpl-"

func ManagedRuleName(templateID, templateRuleID int64) string {
	return fmt.Sprintf("%s%d-%d", managedRulePrefix, templateID, templateRuleID)
}

func ParseManagedRuleName(name string) (int64, int64, bool) {
	if !strings.HasPrefix(name, managedRulePrefix) {
		return 0, 0, false
	}

	parts := strings.Split(strings.TrimPrefix(name, managedRulePrefix), "-")
	if len(parts) != 2 {
		return 0, 0, false
	}

	templateID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || templateID <= 0 {
		return 0, 0, false
	}

	templateRuleID, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || templateRuleID <= 0 {
		return 0, 0, false
	}

	return templateID, templateRuleID, true
}

func CanonicalOperatorJSON(operator *pb.Operator) (string, error) {
	if operator == nil {
		return "", nil
	}

	payload, err := json.Marshal(operator)
	if err != nil {
		return "", err
	}

	return string(payload), nil
}

func CanonicalOperatorJSONFromRule(rule *db.DBRule) (string, error) {
	operator, err := DBRuleOperator(rule)
	if err != nil {
		return "", err
	}
	return CanonicalOperatorJSON(operator)
}

func TemplateRuleToProto(rule *db.TemplateRule) (*pb.Rule, error) {
	if rule == nil {
		return nil, fmt.Errorf("template rule is nil")
	}

	view := &db.DBRule{
		Name:              rule.Name,
		DisplayName:       rule.Name,
		Enabled:           rule.Enabled,
		Precedence:        rule.Precedence,
		Action:            rule.Action,
		Duration:          rule.Duration,
		OperatorType:      rule.OperatorType,
		OperatorSensitive: rule.OperatorSensitive,
		OperatorOperand:   rule.OperatorOperand,
		OperatorData:      rule.OperatorData,
		OperatorJSON:      rule.OperatorJSON,
		Description:       rule.Description,
		Nolog:             rule.Nolog,
		Created:           rule.CreatedAt,
	}

	return DBRuleToProto(view)
}

func MaterializeTemplateRule(node string, templateID int64, rule *db.TemplateRule, observedAt time.Time) (*db.DBRule, *pb.Rule, error) {
	protoRule, err := TemplateRuleToProto(rule)
	if err != nil {
		return nil, nil, err
	}

	protoRule.Name = ManagedRuleName(templateID, rule.ID)
	dbRule, err := ProtoToDBRule(node, observedAt, protoRule)
	if err != nil {
		return nil, nil, err
	}

	dbRule.DisplayName = rule.Name
	dbRule.SourceKind = db.RuleSourceManaged
	dbRule.TemplateID = templateID
	dbRule.TemplateRuleID = rule.ID
	return dbRule, protoRule, nil
}
