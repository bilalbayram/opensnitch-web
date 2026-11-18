package rules

import (
	"errors"
	"fmt"
	"strings"

	pb "github.com/bilalbayram/opensnitch-web/proto"
)

type RouterManagedRuleError struct {
	Message string
}

func (e *RouterManagedRuleError) Error() string {
	return e.Message
}

var supportedRouterManagedOperands = map[string]struct{}{
	"process.path": {},
	"dest.ip":      {},
	"dest.port":    {},
	"protocol":     {},
	"user.id":      {},
}

func IsRouterManagedRuleError(err error) bool {
	var target *RouterManagedRuleError
	ok := errors.As(err, &target)
	return ok
}

func ValidateRouterManagedRule(rule *pb.Rule) error {
	if rule == nil {
		return &RouterManagedRuleError{Message: "router-daemon rules require an operator"}
	}
	return ValidateRouterManagedOperator(rule.GetOperator())
}

func ValidateRouterManagedOperator(operator *pb.Operator) error {
	if operator == nil {
		return &RouterManagedRuleError{Message: "router-daemon rules require an operator"}
	}

	operatorType := strings.ToLower(strings.TrimSpace(operator.GetType()))
	if len(operator.GetList()) > 0 && operatorType == "" {
		operatorType = compoundOperatorType
	}
	switch operatorType {
	case "", simpleOperatorType:
		return validateRouterManagedOperand(operator)
	case compoundOperatorType:
		if len(operator.GetList()) == 0 {
			return &RouterManagedRuleError{Message: "router-daemon compound rules require at least one operand"}
		}
		seen := make(map[string]struct{}, len(operator.GetList()))
		for _, item := range operator.GetList() {
			if item == nil {
				return &RouterManagedRuleError{Message: "router-daemon compound rules cannot contain empty operands"}
			}
			itemType := strings.ToLower(strings.TrimSpace(item.GetType()))
			if itemType != "" && itemType != simpleOperatorType {
				return &RouterManagedRuleError{Message: "router-daemon rules only support flat compound lists"}
			}
			operand := strings.TrimSpace(item.GetOperand())
			if _, exists := seen[operand]; exists {
				return &RouterManagedRuleError{Message: fmt.Sprintf("router-daemon rules only support one %s operand", operand)}
			}
			seen[operand] = struct{}{}
			if err := validateRouterManagedOperand(item); err != nil {
				return err
			}
		}
		return nil
	default:
		return &RouterManagedRuleError{Message: "router-daemon rules only support simple operators or flat compound lists"}
	}
}

func validateRouterManagedOperand(operator *pb.Operator) error {
	operand := strings.TrimSpace(operator.GetOperand())
	if _, ok := supportedRouterManagedOperands[operand]; ok {
		return nil
	}

	if operand == "" {
		return &RouterManagedRuleError{Message: "router-daemon rules require an operand"}
	}

	return &RouterManagedRuleError{
		Message: fmt.Sprintf("router-daemon rules do not support %s", operand),
	}
}
