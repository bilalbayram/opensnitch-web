package api

import (
	"net/http"
	"strings"

	"github.com/bilalbayram/opensnitch-web/internal/db"
	ruleutil "github.com/bilalbayram/opensnitch-web/internal/rules"
)

func (a *API) validateTemplateRuleForManagedTargets(templateID int64, candidate *db.TemplateRule) error {
	managedNodes, err := a.managedNodesForTemplate(templateID)
	if err != nil || len(managedNodes) == 0 {
		return err
	}

	protoRule, err := ruleutil.TemplateRuleToProto(candidate)
	if err != nil {
		return err
	}
	return ruleutil.ValidateRouterManagedRule(protoRule)
}

func (a *API) validateTemplateAttachmentForManagedTargets(templateID int64, targetType, targetRef string) error {
	targetNodes, err := a.resolveAttachmentTargetNodes(targetType, targetRef)
	if err != nil {
		return err
	}

	managedNodes, err := a.filterManagedNodes(targetNodes)
	if err != nil || len(managedNodes) == 0 {
		return err
	}

	rules, err := a.db.GetTemplateRules(templateID)
	if err != nil {
		return err
	}
	for i := range rules {
		protoRule, err := ruleutil.TemplateRuleToProto(&rules[i])
		if err != nil {
			return err
		}
		if err := ruleutil.ValidateRouterManagedRule(protoRule); err != nil {
			return err
		}
	}
	return nil
}

func (a *API) managedNodesForTemplate(templateID int64) ([]string, error) {
	attachments, err := a.db.GetTemplateAttachments(templateID)
	if err != nil {
		return nil, err
	}

	targetNodes := make([]string, 0, len(attachments))
	for _, attachment := range attachments {
		nodes, err := a.resolveAttachmentTargetNodes(attachment.TargetType, attachment.TargetRef)
		if err != nil {
			return nil, err
		}
		targetNodes = append(targetNodes, nodes...)
	}

	return a.filterManagedNodes(targetNodes)
}

func (a *API) resolveAttachmentTargetNodes(targetType, targetRef string) ([]string, error) {
	targetType = strings.TrimSpace(targetType)
	targetRef = strings.TrimSpace(targetRef)

	switch targetType {
	case "node":
		if targetRef == "" {
			return nil, nil
		}
		return []string{targetRef}, nil
	case "tag":
		allTags, err := a.db.GetAllNodeTags()
		if err != nil {
			return nil, err
		}

		result := make([]string, 0)
		for node, tags := range allTags {
			for _, tag := range tags {
				if tag == targetRef {
					result = append(result, node)
					break
				}
			}
		}
		return result, nil
	default:
		return nil, nil
	}
}

func (a *API) filterManagedNodes(nodes []string) ([]string, error) {
	seen := make(map[string]struct{}, len(nodes))
	result := make([]string, 0, len(nodes))
	for _, node := range nodes {
		node = strings.TrimSpace(node)
		if node == "" {
			continue
		}
		if _, ok := seen[node]; ok {
			continue
		}
		seen[node] = struct{}{}

		managed, err := a.isRouterManagedNode(node)
		if err != nil {
			return nil, err
		}
		if managed {
			result = append(result, node)
		}
	}
	return result, nil
}

func writeRouterManagedSyncError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if ruleutil.IsRouterManagedRuleError(err) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return true
	}
	return false
}
