package api

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/evilsocket/opensnitch-web/internal/db"
	ruleutil "github.com/evilsocket/opensnitch-web/internal/rules"
	"github.com/evilsocket/opensnitch-web/internal/ws"
	pb "github.com/evilsocket/opensnitch-web/proto"
)

type ruleRequest struct {
	Name              string `json:"name"`
	Node              string `json:"node"`
	Enabled           bool   `json:"enabled"`
	Precedence        bool   `json:"precedence"`
	Action            string `json:"action"`
	Duration          string `json:"duration"`
	OperatorType      string `json:"operator_type"`
	OperatorSensitive bool   `json:"operator_sensitive"`
	OperatorOperand   string `json:"operator_operand"`
	OperatorData      string `json:"operator_data"`
	Description       string `json:"description"`
	Nolog             bool   `json:"nolog"`
}

type ruleResponse struct {
	ID                int64        `json:"id"`
	Time              string       `json:"time"`
	Node              string       `json:"node"`
	Name              string       `json:"name"`
	Enabled           bool         `json:"enabled"`
	Precedence        bool         `json:"precedence"`
	Action            string       `json:"action"`
	Duration          string       `json:"duration"`
	OperatorType      string       `json:"operator_type"`
	OperatorSensitive bool         `json:"operator_sensitive"`
	OperatorOperand   string       `json:"operator_operand"`
	OperatorData      string       `json:"operator_data"`
	Operator          *pb.Operator `json:"operator,omitempty"`
	IsCompound        bool         `json:"is_compound"`
	Description       string       `json:"description"`
	Nolog             bool         `json:"nolog"`
	Created           string       `json:"created"`
}

type generateRulesRequest struct {
	Node             string   `json:"node"`
	Since            string   `json:"since"`
	Until            string   `json:"until"`
	ExcludeProcesses []string `json:"exclude_processes"`
}

type applyGeneratedRulesRequest struct {
	Node             string   `json:"node"`
	Since            string   `json:"since"`
	Until            string   `json:"until"`
	ExcludeProcesses []string `json:"exclude_processes"`
	Fingerprints     []string `json:"fingerprints"`
}

type generatedRuleProposal struct {
	Fingerprint        string        `json:"fingerprint"`
	Process            string        `json:"process"`
	Destination        string        `json:"destination"`
	DestinationOperand string        `json:"destination_operand"`
	DstPort            int           `json:"dst_port"`
	Protocol           string        `json:"protocol"`
	Hits               int           `json:"hits"`
	FirstSeen          string        `json:"first_seen"`
	LastSeen           string        `json:"last_seen"`
	Rule               *ruleResponse `json:"rule"`
}

type generateRulesPreviewResponse struct {
	Data            []generatedRuleProposal `json:"data"`
	SkippedExisting int                     `json:"skipped_existing"`
	SkippedExcluded int                     `json:"skipped_excluded"`
}

type generateRulesFilters struct {
	Node             string
	Since            time.Time
	Until            time.Time
	ExcludeProcesses []string
}

func (a *API) handleGetRules(w http.ResponseWriter, r *http.Request) {
	node := r.URL.Query().Get("node")
	rules, err := a.db.GetRules(node)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	response := make([]ruleResponse, 0, len(rules))
	for i := range rules {
		item, err := buildRuleResponse(&rules[i])
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		response = append(response, *item)
	}

	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var req ruleRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	protoRule := buildSimpleRule(req)
	dbRule, err := ruleutil.ProtoToDBRule(req.Node, time.Now(), protoRule)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := a.db.UpsertRule(dbRule); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.pushRulesToDaemon(req.Node, []*pb.Rule{protoRule}, pb.Action_CHANGE_RULE)

	response, err := buildRuleResponse(dbRule)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]interface{}{
		"action": "created",
		"rule":   response,
	})

	writeJSON(w, http.StatusCreated, response)
}

func (a *API) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")

	var req ruleRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	req.Name = name

	existing, err := a.db.GetRule(req.Node, name)
	switch {
	case err == nil:
		isCompound, err := dbRuleIsCompound(existing)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if isCompound {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "compound rules cannot be edited yet"})
			return
		}
	case errors.Is(err, sql.ErrNoRows):
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	protoRule := buildSimpleRule(req)
	dbRule, err := ruleutil.ProtoToDBRule(req.Node, time.Now(), protoRule)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := a.db.UpdateRuleAndResetSeenFlows(dbRule); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.pushRulesToDaemon(req.Node, []*pb.Rule{protoRule}, pb.Action_CHANGE_RULE)

	response, err := buildRuleResponse(dbRule)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]interface{}{
		"action": "updated",
		"rule":   response,
	})

	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleGenerateRulesPreview(w http.ResponseWriter, r *http.Request) {
	var req generateRulesRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	filters, err := parseGenerateRulesFilters(req.Node, req.Since, req.Until, req.ExcludeProcesses)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	response, err := a.buildGeneratedRulesPreview(filters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleGenerateRulesApply(w http.ResponseWriter, r *http.Request) {
	var req applyGeneratedRulesRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	filters, err := parseGenerateRulesFilters(req.Node, req.Since, req.Until, req.ExcludeProcesses)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	selectedFingerprints := normalizeStringList(req.Fingerprints)
	if len(selectedFingerprints) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one rule must be selected"})
		return
	}

	preview, err := a.buildGeneratedRulesPreview(filters)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	selectedSet := make(map[string]struct{}, len(selectedFingerprints))
	for _, fingerprint := range selectedFingerprints {
		selectedSet[fingerprint] = struct{}{}
	}

	selectedRules := []*pb.Rule{}
	selectedResponses := []*ruleResponse{}
	dbRules := []*db.DBRule{}
	selectedRuleNames := []string{}
	for _, proposal := range preview.Data {
		if _, ok := selectedSet[proposal.Fingerprint]; !ok {
			continue
		}

		protoRule, err := buildProtoRuleFromResponse(proposal.Rule)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		dbRule, err := ruleutil.ProtoToDBRule(filters.Node, time.Now(), protoRule)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		response, err := buildRuleResponse(dbRule)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		selectedRules = append(selectedRules, protoRule)
		selectedResponses = append(selectedResponses, response)
		dbRules = append(dbRules, dbRule)
		selectedRuleNames = append(selectedRuleNames, dbRule.Name)
	}

	if len(selectedRules) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selected rules are no longer available"})
		return
	}

	if a.nodes.GetNode(filters.Node) == nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node not connected"})
		return
	}

	previousMode, err := a.db.ApplyGeneratedRules(filters.Node, dbRules, "ask")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if !a.pushRulesToDaemon(filters.Node, selectedRules, pb.Action_CHANGE_RULE) {
		if err := a.db.RevertGeneratedRules(filters.Node, selectedRuleNames, previousMode); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to rollback generated rules after notification failure: " + err.Error()})
			return
		}
		writeJSON(w, http.StatusConflict, map[string]string{"error": "node notification queue is full"})
		return
	}

	for _, response := range selectedResponses {
		a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]interface{}{
			"action": "created",
			"rule":   response,
		})
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"status": "ok",
		"mode":   "ask",
		"data":   selectedResponses,
		"count":  len(selectedResponses),
	})
}

func (a *API) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	node := r.URL.Query().Get("node")

	if err := a.db.DeleteRule(node, name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	notif := &pb.Notification{
		Id:   a.nodes.NextID(),
		Type: pb.Action_DELETE_RULE,
		Rules: []*pb.Rule{
			{Name: name},
		},
	}

	if node != "" {
		a.nodes.SendNotification(node, notif)
	} else {
		a.nodes.BroadcastNotification(notif)
	}

	a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]interface{}{
		"action": "deleted",
		"name":   name,
		"node":   node,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleToggleRule(enable bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		node := r.URL.Query().Get("node")

		var actionType pb.Action
		if enable {
			actionType = pb.Action_ENABLE_RULE
		} else {
			actionType = pb.Action_DISABLE_RULE
		}

		notif := &pb.Notification{
			Id:   a.nodes.NextID(),
			Type: actionType,
			Rules: []*pb.Rule{
				{Name: name, Enabled: enable},
			},
		}

		if node != "" {
			a.nodes.SendNotification(node, notif)
		} else {
			a.nodes.BroadcastNotification(notif)
		}

		if err := a.db.SetRuleEnabled(node, name, enable); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func buildRuleResponse(rule *db.DBRule) (*ruleResponse, error) {
	operator, err := ruleutil.DBRuleOperator(rule)
	if err != nil {
		return nil, err
	}

	return &ruleResponse{
		ID:                rule.ID,
		Time:              rule.Time,
		Node:              rule.Node,
		Name:              rule.Name,
		Enabled:           rule.Enabled,
		Precedence:        rule.Precedence,
		Action:            rule.Action,
		Duration:          rule.Duration,
		OperatorType:      rule.OperatorType,
		OperatorSensitive: rule.OperatorSensitive,
		OperatorOperand:   rule.OperatorOperand,
		OperatorData:      rule.OperatorData,
		Operator:          operator,
		IsCompound:        ruleutil.IsCompoundOperator(operator),
		Description:       rule.Description,
		Nolog:             rule.Nolog,
		Created:           rule.Created,
	}, nil
}

func buildProtoRuleFromResponse(rule *ruleResponse) (*pb.Rule, error) {
	if rule == nil {
		return nil, errors.New("rule is nil")
	}

	var created int64
	if ts, err := ruleutil.ParseStoredTime(rule.Created); err == nil {
		created = ts.Unix()
	}

	operator := rule.Operator
	if operator == nil && (rule.OperatorType != "" || rule.OperatorOperand != "" || rule.OperatorData != "") {
		operatorType := rule.OperatorType
		if operatorType == "" {
			operatorType = "simple"
		}
		operator = &pb.Operator{
			Type:      operatorType,
			Operand:   rule.OperatorOperand,
			Data:      rule.OperatorData,
			Sensitive: rule.OperatorSensitive,
		}
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

func buildSimpleRule(req ruleRequest) *pb.Rule {
	operatorType := req.OperatorType
	if operatorType == "" {
		operatorType = "simple"
	}

	return &pb.Rule{
		Name:        req.Name,
		Enabled:     req.Enabled,
		Precedence:  req.Precedence,
		Action:      req.Action,
		Duration:    req.Duration,
		Description: req.Description,
		Nolog:       req.Nolog,
		Operator: &pb.Operator{
			Type:      operatorType,
			Operand:   req.OperatorOperand,
			Data:      req.OperatorData,
			Sensitive: req.OperatorSensitive,
		},
	}
}

func dbRuleIsCompound(rule *db.DBRule) (bool, error) {
	operator, err := ruleutil.DBRuleOperator(rule)
	if err != nil {
		return false, err
	}
	return ruleutil.IsCompoundOperator(operator), nil
}

func parseGenerateRulesFilters(node, sinceValue, untilValue string, excludeProcesses []string) (*generateRulesFilters, error) {
	node = strings.TrimSpace(node)
	if node == "" {
		return nil, errors.New("node is required")
	}

	since, err := parseAPITime(sinceValue)
	if err != nil {
		return nil, errors.New("invalid since timestamp")
	}

	until, err := parseAPITime(untilValue)
	if err != nil {
		return nil, errors.New("invalid until timestamp")
	}

	if until.Before(since) {
		return nil, errors.New("until must be after since")
	}

	return &generateRulesFilters{
		Node:             node,
		Since:            since,
		Until:            until,
		ExcludeProcesses: normalizeStringList(excludeProcesses),
	}, nil
}

func parseAPITime(value string) (time.Time, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return time.Time{}, errors.New("empty timestamp")
	}
	if ts, err := time.Parse(time.RFC3339Nano, trimmed); err == nil {
		return ts, nil
	}
	return ruleutil.ParseStoredTime(trimmed)
}

func normalizeStringList(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || slices.Contains(result, value) {
			continue
		}
		result = append(result, value)
	}
	return result
}

func (a *API) buildGeneratedRulesPreview(filters *generateRulesFilters) (*generateRulesPreviewResponse, error) {
	flows, err := a.db.GetUniqueFlows(filters.Node, filters.Since, filters.Until, nil)
	if err != nil {
		return nil, err
	}

	rules, err := a.db.GetRules("")
	if err != nil {
		return nil, err
	}

	existingFingerprints := make(map[string]struct{})
	for i := range rules {
		if rules[i].Node != "" && rules[i].Node != filters.Node {
			continue
		}
		protoRule, err := ruleutil.DBRuleToProto(&rules[i])
		if err != nil {
			return nil, err
		}
		key, ok := ruleutil.LearningKeyFromRule(protoRule)
		if !ok {
			continue
		}
		existingFingerprints[ruleutil.FingerprintForKey(key)] = struct{}{}
	}

	excludedSet := make(map[string]struct{}, len(filters.ExcludeProcesses))
	for _, process := range filters.ExcludeProcesses {
		excludedSet[process] = struct{}{}
	}

	response := &generateRulesPreviewResponse{
		Data: make([]generatedRuleProposal, 0, len(flows)),
	}

	observedAt := time.Now()
	for _, flow := range flows {
		if _, excluded := excludedSet[flow.Process]; excluded {
			response.SkippedExcluded++
			continue
		}

		key := ruleutil.LearningKey{
			Process:         flow.Process,
			DestinationType: flow.DestinationOperand,
			Destination:     flow.Destination,
			DstPort:         flow.DstPort,
			Protocol:        flow.Protocol,
		}
		fingerprint := ruleutil.FingerprintForKey(key)
		if _, exists := existingFingerprints[fingerprint]; exists {
			response.SkippedExisting++
			continue
		}

		protoRule := ruleutil.BuildGeneratedRule(key)
		dbRule, err := ruleutil.ProtoToDBRule(filters.Node, observedAt, protoRule)
		if err != nil {
			return nil, err
		}
		rule, err := buildRuleResponse(dbRule)
		if err != nil {
			return nil, err
		}

		response.Data = append(response.Data, generatedRuleProposal{
			Fingerprint:        fingerprint,
			Process:            flow.Process,
			Destination:        flow.Destination,
			DestinationOperand: flow.DestinationOperand,
			DstPort:            flow.DstPort,
			Protocol:           flow.Protocol,
			Hits:               flow.Hits,
			FirstSeen:          flow.FirstSeen,
			LastSeen:           flow.LastSeen,
			Rule:               rule,
		})
	}

	return response, nil
}

func (a *API) pushRulesToDaemon(nodeAddr string, rules []*pb.Rule, action pb.Action) bool {
	notif := &pb.Notification{
		Id:    a.nodes.NextID(),
		Type:  action,
		Rules: rules,
	}

	if nodeAddr != "" {
		return a.nodes.SendNotification(nodeAddr, notif)
	}

	a.nodes.BroadcastNotification(notif)
	return true
}
