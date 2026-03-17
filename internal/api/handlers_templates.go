package api

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/evilsocket/opensnitch-web/internal/db"
	ruleutil "github.com/evilsocket/opensnitch-web/internal/rules"
	pb "github.com/evilsocket/opensnitch-web/proto"
)

type templateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

type templateRuleRequest struct {
	Name              string `json:"name"`
	Position          int    `json:"position"`
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

type templateAttachmentRequest struct {
	TargetType string `json:"target_type"`
	TargetRef  string `json:"target_ref"`
	Priority   int    `json:"priority"`
}

type templateRuleResponse struct {
	ID                int64        `json:"id"`
	TemplateID        int64        `json:"template_id"`
	Position          int          `json:"position"`
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
	CreatedAt         string       `json:"created_at"`
	UpdatedAt         string       `json:"updated_at"`
}

type templateAttachmentResponse struct {
	ID         int64  `json:"id"`
	TemplateID int64  `json:"template_id"`
	TargetType string `json:"target_type"`
	TargetRef  string `json:"target_ref"`
	Priority   int    `json:"priority"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type templateResponse struct {
	ID          int64                        `json:"id"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	CreatedAt   string                       `json:"created_at"`
	UpdatedAt   string                       `json:"updated_at"`
	Rules       []templateRuleResponse       `json:"rules"`
	Attachments []templateAttachmentResponse `json:"attachments"`
}

func (a *API) handleGetTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := a.db.GetRuleTemplates()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	response := make([]templateResponse, 0, len(templates))
	for _, tpl := range templates {
		item, err := a.buildTemplateResponse(&tpl)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		response = append(response, *item)
	}

	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleGetTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}

	template, err := a.db.GetRuleTemplate(templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	response, err := a.buildTemplateResponse(template)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleCreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req templateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	template, err := a.db.CreateRuleTemplate(&db.RuleTemplate{
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	response, err := a.buildTemplateResponse(template)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func (a *API) handleUpdateTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}

	template, err := a.db.GetRuleTemplate(templateID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var req templateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	template.Name = req.Name
	template.Description = req.Description
	if err := a.db.UpdateRuleTemplate(template); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	updated, err := a.db.GetRuleTemplate(templateID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	response, err := a.buildTemplateResponse(updated)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleDeleteTemplate(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}

	if _, err := a.db.GetRuleTemplate(templateID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	affectedNodes := []string{}
	if a.templateSync != nil {
		affectedNodes, _ = a.templateSync.AffectedNodesForTemplates([]int64{templateID})
	}

	if err := a.db.DeleteRuleTemplate(templateID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if a.templateSync != nil {
		if err := a.templateSync.ReconcileNodes(affectedNodes); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleCreateTemplateRule(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}
	if _, err := a.db.GetRuleTemplate(templateID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var req templateRuleRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	templateRule, err := a.db.CreateTemplateRule(buildTemplateRuleRecord(templateID, 0, req))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if a.templateSync != nil {
		if err := a.templateSync.ReconcileTemplate(templateID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	response, err := buildTemplateRuleResponse(templateRule)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func (a *API) handleUpdateTemplateRule(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}
	ruleID, ok := parseInt64Param(chi.URLParam(r, "ruleId"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template rule id"})
		return
	}

	if _, err := a.db.GetRuleTemplate(templateID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := a.db.GetTemplateRule(templateID, ruleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template rule not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var req templateRuleRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	templateRule := buildTemplateRuleRecord(templateID, ruleID, req)
	if err := a.db.UpdateTemplateRule(templateRule); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if a.templateSync != nil {
		if err := a.templateSync.ReconcileTemplate(templateID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	updated, err := a.db.GetTemplateRule(templateID, ruleID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	response, err := buildTemplateRuleResponse(updated)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (a *API) handleDeleteTemplateRule(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}
	ruleID, ok := parseInt64Param(chi.URLParam(r, "ruleId"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template rule id"})
		return
	}

	if _, err := a.db.GetTemplateRule(templateID, ruleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template rule not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if err := a.db.DeleteTemplateRule(templateID, ruleID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if a.templateSync != nil {
		if err := a.templateSync.ReconcileTemplate(templateID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) handleCreateTemplateAttachment(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}
	if _, err := a.db.GetRuleTemplate(templateID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var req templateAttachmentRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := a.validateAttachmentRequest(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	attachment, err := a.db.CreateTemplateAttachment(&db.TemplateAttachment{
		TemplateID: templateID,
		TargetType: req.TargetType,
		TargetRef:  req.TargetRef,
		Priority:   req.Priority,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if a.templateSync != nil {
		if err := a.templateSync.ReconcileTemplate(templateID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusCreated, attachment)
}

func (a *API) handleUpdateTemplateAttachment(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}
	attachmentID, ok := parseInt64Param(chi.URLParam(r, "attachmentId"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid attachment id"})
		return
	}

	if _, err := a.db.GetRuleTemplate(templateID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if _, err := a.db.GetTemplateAttachment(templateID, attachmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "attachment not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	var req templateAttachmentRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if err := a.validateAttachmentRequest(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	beforeNodes := []string{}
	if a.templateSync != nil {
		beforeNodes, _ = a.templateSync.AffectedNodesForTemplates([]int64{templateID})
	}

	if err := a.db.UpdateTemplateAttachment(&db.TemplateAttachment{
		ID:         attachmentID,
		TemplateID: templateID,
		TargetType: req.TargetType,
		TargetRef:  req.TargetRef,
		Priority:   req.Priority,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if a.templateSync != nil {
		afterNodes, _ := a.templateSync.AffectedNodesForTemplates([]int64{templateID})
		if err := a.templateSync.ReconcileNodes(append(beforeNodes, afterNodes...)); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	updated, err := a.db.GetTemplateAttachment(templateID, attachmentID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

func (a *API) handleDeleteTemplateAttachment(w http.ResponseWriter, r *http.Request) {
	templateID, ok := parseInt64Param(chi.URLParam(r, "id"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid template id"})
		return
	}
	attachmentID, ok := parseInt64Param(chi.URLParam(r, "attachmentId"))
	if !ok {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid attachment id"})
		return
	}

	if _, err := a.db.GetTemplateAttachment(templateID, attachmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "attachment not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	affectedNodes := []string{}
	if a.templateSync != nil {
		affectedNodes, _ = a.templateSync.AffectedNodesForTemplates([]int64{templateID})
	}

	if err := a.db.DeleteTemplateAttachment(templateID, attachmentID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	if a.templateSync != nil {
		if err := a.templateSync.ReconcileNodes(affectedNodes); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *API) buildTemplateResponse(template *db.RuleTemplate) (*templateResponse, error) {
	rules, err := a.db.GetTemplateRules(template.ID)
	if err != nil {
		return nil, err
	}
	attachments, err := a.db.GetTemplateAttachments(template.ID)
	if err != nil {
		return nil, err
	}

	ruleResponses := make([]templateRuleResponse, 0, len(rules))
	for _, rule := range rules {
		response, err := buildTemplateRuleResponse(&rule)
		if err != nil {
			return nil, err
		}
		ruleResponses = append(ruleResponses, *response)
	}

	attachmentResponses := make([]templateAttachmentResponse, 0, len(attachments))
	for _, attachment := range attachments {
		attachmentResponses = append(attachmentResponses, templateAttachmentResponse{
			ID:         attachment.ID,
			TemplateID: attachment.TemplateID,
			TargetType: attachment.TargetType,
			TargetRef:  attachment.TargetRef,
			Priority:   attachment.Priority,
			CreatedAt:  attachment.CreatedAt,
			UpdatedAt:  attachment.UpdatedAt,
		})
	}

	return &templateResponse{
		ID:          template.ID,
		Name:        template.Name,
		Description: template.Description,
		CreatedAt:   template.CreatedAt,
		UpdatedAt:   template.UpdatedAt,
		Rules:       ruleResponses,
		Attachments: attachmentResponses,
	}, nil
}

func buildTemplateRuleRecord(templateID, ruleID int64, req templateRuleRequest) *db.TemplateRule {
	operatorType := strings.TrimSpace(req.OperatorType)
	if operatorType == "" {
		operatorType = "simple"
	}

	operator := &pb.Operator{
		Type:      operatorType,
		Operand:   req.OperatorOperand,
		Data:      req.OperatorData,
		Sensitive: req.OperatorSensitive,
	}
	operatorJSON, _ := ruleutil.CanonicalOperatorJSON(operator)

	return &db.TemplateRule{
		ID:                ruleID,
		TemplateID:        templateID,
		Position:          req.Position,
		Name:              strings.TrimSpace(req.Name),
		Enabled:           req.Enabled,
		Precedence:        req.Precedence,
		Action:            req.Action,
		Duration:          req.Duration,
		OperatorType:      operatorType,
		OperatorSensitive: req.OperatorSensitive,
		OperatorOperand:   req.OperatorOperand,
		OperatorData:      req.OperatorData,
		OperatorJSON:      operatorJSON,
		Description:       strings.TrimSpace(req.Description),
		Nolog:             req.Nolog,
	}
}

func buildTemplateRuleResponse(rule *db.TemplateRule) (*templateRuleResponse, error) {
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
	operator, err := ruleutil.DBRuleOperator(view)
	if err != nil {
		return nil, err
	}

	return &templateRuleResponse{
		ID:                rule.ID,
		TemplateID:        rule.TemplateID,
		Position:          rule.Position,
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
		CreatedAt:         rule.CreatedAt,
		UpdatedAt:         rule.UpdatedAt,
	}, nil
}

func (a *API) validateAttachmentRequest(req *templateAttachmentRequest) error {
	req.TargetType = strings.TrimSpace(req.TargetType)
	req.TargetRef = strings.TrimSpace(req.TargetRef)
	switch req.TargetType {
	case "node":
		if req.TargetRef == "" {
			return errors.New("target_ref is required")
		}
		if _, err := a.db.GetNode(req.TargetRef); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errors.New("target node not found")
			}
			return err
		}
	case "tag":
		req.TargetRef = db.NormalizeTag(req.TargetRef)
		if req.TargetRef == "" {
			return errors.New("target_ref is required")
		}
	default:
		return errors.New("target_type must be node or tag")
	}

	return nil
}

func parseInt64Param(value string) (int64, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, false
	}
	return parsed, true
}
