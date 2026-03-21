package templatesync

import (
	"database/sql"
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	"github.com/bilalbayram/opensnitch-web/internal/nodemanager"
	ruleutil "github.com/bilalbayram/opensnitch-web/internal/rules"
	pb "github.com/bilalbayram/opensnitch-web/proto"
)

type Service struct {
	db    *db.Database
	nodes *nodemanager.Manager
}

type resolvedCandidate struct {
	attachment        db.TemplateAttachment
	templateRule      db.TemplateRule
	dbRule            *db.DBRule
	protoRule         *pb.Rule
	canonicalOperator string
	scopeRank         int
}

func New(database *db.Database, nodes *nodemanager.Manager) *Service {
	return &Service{
		db:    database,
		nodes: nodes,
	}
}

func (s *Service) DecorateStoredRule(rule *db.DBRule) error {
	if rule == nil {
		return nil
	}

	templateID, templateRuleID, ok := ruleutil.ParseManagedRuleName(rule.Name)
	if !ok {
		rule.SourceKind = db.RuleSourceManual
		if strings.TrimSpace(rule.DisplayName) == "" {
			rule.DisplayName = rule.Name
		}
		return nil
	}

	rule.SourceKind = db.RuleSourceManaged
	rule.TemplateID = templateID
	rule.TemplateRuleID = templateRuleID
	if strings.TrimSpace(rule.DisplayName) == "" {
		rule.DisplayName = rule.Name
	}

	templateRule, err := s.db.GetTemplateRule(templateID, templateRuleID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}

	rule.DisplayName = templateRule.Name
	return nil
}

func (s *Service) ReconcileTemplate(templateID int64) error {
	nodes, err := s.AffectedNodesForTemplates([]int64{templateID})
	if err != nil {
		return err
	}
	return s.ReconcileNodes(nodes)
}

func (s *Service) ReconcileNodes(nodes []string) error {
	seen := map[string]struct{}{}
	var firstErr error

	for _, node := range nodes {
		node = strings.TrimSpace(node)
		if node == "" {
			continue
		}
		if _, ok := seen[node]; ok {
			continue
		}
		seen[node] = struct{}{}
		if err := s.ReconcileNode(node); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

func (s *Service) ReconcileNode(node string) error {
	desiredDBRules, desiredProtoRules, err := s.ResolveManagedRules(node)
	if err != nil {
		return err
	}

	currentRules, err := s.db.GetManagedRules(node)
	if err != nil {
		return err
	}

	currentByName := make(map[string]db.DBRule, len(currentRules))
	for _, rule := range currentRules {
		currentByName[rule.Name] = rule
	}

	desiredByName := make(map[string]*db.DBRule, len(desiredDBRules))
	for _, rule := range desiredDBRules {
		desiredByName[rule.Name] = rule
	}

	deleteNames := make([]string, 0)
	for name := range currentByName {
		if _, ok := desiredByName[name]; !ok {
			deleteNames = append(deleteNames, name)
		}
	}
	sort.Strings(deleteNames)

	changeRules := make([]*pb.Rule, 0, len(desiredProtoRules))
	for idx, desired := range desiredDBRules {
		current, ok := currentByName[desired.Name]
		if ok && managedRuleEquals(&current, desired) {
			continue
		}
		changeRules = append(changeRules, desiredProtoRules[idx])
	}

	if len(deleteNames) == 0 && len(changeRules) == 0 {
		return s.db.SetNodeTemplateSync(node, false, "")
	}

	if s.nodes.GetNode(node) == nil {
		return s.db.SetNodeTemplateSync(node, true, "node offline")
	}

	notifications := make([]*pb.Notification, 0, 2)
	if len(deleteNames) > 0 {
		deleteRules := make([]*pb.Rule, 0, len(deleteNames))
		for _, name := range deleteNames {
			deleteRules = append(deleteRules, &pb.Rule{Name: name})
		}
		notifications = append(notifications, &pb.Notification{
			Id:    s.nodes.NextID(),
			Type:  pb.Action_DELETE_RULE,
			Rules: deleteRules,
		})
	}
	if len(changeRules) > 0 {
		notifications = append(notifications, &pb.Notification{
			Id:    s.nodes.NextID(),
			Type:  pb.Action_CHANGE_RULE,
			Rules: changeRules,
		})
	}

	if !s.nodes.SendNotificationBatch(node, notifications) {
		return s.db.SetNodeTemplateSync(node, true, "notification queue full")
	}

	if err := s.db.ReplaceManagedRules(node, desiredDBRules); err != nil {
		_ = s.db.SetNodeTemplateSync(node, true, err.Error())
		return err
	}

	return s.db.SetNodeTemplateSync(node, false, "")
}

func (s *Service) ResolveManagedRules(node string) ([]*db.DBRule, []*pb.Rule, error) {
	routerManaged := false
	routerRecord, err := s.db.GetRouterByLinkedNodeAddr(node)
	switch {
	case err == nil:
		routerManaged = routerRecord.DaemonMode == db.RouterDaemonModeRouterDaemon
	case err == sql.ErrNoRows:
	default:
		return nil, nil, err
	}

	nodeTags, err := s.db.GetNodeTags(node)
	if err != nil {
		return nil, nil, err
	}

	attachments, err := s.db.GetAllTemplateAttachments()
	if err != nil {
		return nil, nil, err
	}

	attached := make([]db.TemplateAttachment, 0)
	for _, attachment := range attachments {
		switch attachment.TargetType {
		case "node":
			if attachment.TargetRef == node {
				attached = append(attached, attachment)
			}
		case "tag":
			if slices.Contains(nodeTags, attachment.TargetRef) {
				attached = append(attached, attachment)
			}
		}
	}

	ruleCache := map[int64][]db.TemplateRule{}
	candidates := make([]resolvedCandidate, 0)
	now := time.Now()

	for _, attachment := range attached {
		templateRules, ok := ruleCache[attachment.TemplateID]
		if !ok {
			templateRules, err = s.db.GetTemplateRules(attachment.TemplateID)
			if err != nil {
				return nil, nil, err
			}
			ruleCache[attachment.TemplateID] = templateRules
		}

		for _, templateRule := range templateRules {
			dbRule, protoRule, err := ruleutil.MaterializeTemplateRule(node, attachment.TemplateID, &templateRule, now)
			if err != nil {
				return nil, nil, err
			}
			if routerManaged {
				if err := ruleutil.ValidateRouterManagedRule(protoRule); err != nil {
					return nil, nil, err
				}
			}

			canonicalOperator, err := ruleutil.CanonicalOperatorJSONFromRule(dbRule)
			if err != nil {
				return nil, nil, err
			}

			scopeRank := 1
			if attachment.TargetType == "node" {
				scopeRank = 0
			}

			candidates = append(candidates, resolvedCandidate{
				attachment:        attachment,
				templateRule:      templateRule,
				dbRule:            dbRule,
				protoRule:         protoRule,
				canonicalOperator: canonicalOperator,
				scopeRank:         scopeRank,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		left := candidates[i]
		right := candidates[j]
		switch {
		case left.attachment.Priority != right.attachment.Priority:
			return left.attachment.Priority < right.attachment.Priority
		case left.scopeRank != right.scopeRank:
			return left.scopeRank < right.scopeRank
		case left.templateRule.Position != right.templateRule.Position:
			return left.templateRule.Position < right.templateRule.Position
		case left.attachment.ID != right.attachment.ID:
			return left.attachment.ID < right.attachment.ID
		default:
			return left.templateRule.ID < right.templateRule.ID
		}
	})

	selectedByName := map[string]struct{}{}
	selectedByOperator := map[string]struct{}{}
	selected := make([]resolvedCandidate, 0, len(candidates))

	for _, candidate := range candidates {
		if _, ok := selectedByName[candidate.dbRule.Name]; ok {
			continue
		}
		if candidate.canonicalOperator != "" {
			if _, ok := selectedByOperator[candidate.canonicalOperator]; ok {
				continue
			}
			selectedByOperator[candidate.canonicalOperator] = struct{}{}
		}

		selectedByName[candidate.dbRule.Name] = struct{}{}
		selected = append(selected, candidate)
	}

	sort.Slice(selected, func(i, j int) bool {
		return selected[i].dbRule.Name < selected[j].dbRule.Name
	})

	desiredDBRules := make([]*db.DBRule, 0, len(selected))
	desiredProtoRules := make([]*pb.Rule, 0, len(selected))
	for _, candidate := range selected {
		desiredDBRules = append(desiredDBRules, candidate.dbRule)
		desiredProtoRules = append(desiredProtoRules, candidate.protoRule)
	}

	return desiredDBRules, desiredProtoRules, nil
}

func (s *Service) AffectedNodesForTemplates(templateIDs []int64) ([]string, error) {
	if len(templateIDs) == 0 {
		return nil, nil
	}

	templateSet := make(map[int64]struct{}, len(templateIDs))
	for _, id := range templateIDs {
		if id > 0 {
			templateSet[id] = struct{}{}
		}
	}

	attachments, err := s.db.GetAllTemplateAttachments()
	if err != nil {
		return nil, err
	}
	nodeTags, err := s.db.GetAllNodeTags()
	if err != nil {
		return nil, err
	}

	resultSet := map[string]struct{}{}
	for _, attachment := range attachments {
		if _, ok := templateSet[attachment.TemplateID]; !ok {
			continue
		}

		switch attachment.TargetType {
		case "node":
			if attachment.TargetRef != "" {
				resultSet[attachment.TargetRef] = struct{}{}
			}
		case "tag":
			for node, tags := range nodeTags {
				if slices.Contains(tags, attachment.TargetRef) {
					resultSet[node] = struct{}{}
				}
			}
		}
	}

	nodes := make([]string, 0, len(resultSet))
	for node := range resultSet {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	return nodes, nil
}

func managedRuleEquals(left, right *db.DBRule) bool {
	if left == nil || right == nil {
		return left == right
	}

	if left.Name != right.Name ||
		left.DisplayName != right.DisplayName ||
		left.SourceKind != right.SourceKind ||
		left.TemplateID != right.TemplateID ||
		left.TemplateRuleID != right.TemplateRuleID ||
		left.Enabled != right.Enabled ||
		left.Precedence != right.Precedence ||
		left.Action != right.Action ||
		left.Duration != right.Duration ||
		left.OperatorType != right.OperatorType ||
		left.OperatorSensitive != right.OperatorSensitive ||
		left.OperatorOperand != right.OperatorOperand ||
		left.OperatorData != right.OperatorData ||
		left.OperatorJSON != right.OperatorJSON ||
		left.Description != right.Description ||
		left.Nolog != right.Nolog {
		return false
	}

	return true
}

func (s *Service) EnsureNodeExists(node string) error {
	if strings.TrimSpace(node) == "" {
		return fmt.Errorf("node is required")
	}
	if _, err := s.db.GetNode(node); err != nil {
		return err
	}
	return nil
}
