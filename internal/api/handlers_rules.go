package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/evilsocket/opensnitch-web/internal/db"
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

func (a *API) handleGetRules(w http.ResponseWriter, r *http.Request) {
	node := r.URL.Query().Get("node")
	rules, err := a.db.GetRules(node)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if rules == nil {
		rules = []db.DBRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (a *API) handleCreateRule(w http.ResponseWriter, r *http.Request) {
	var req ruleRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	dbRule := &db.DBRule{
		Time:              now,
		Node:              req.Node,
		Name:              req.Name,
		Enabled:           req.Enabled,
		Precedence:        req.Precedence,
		Action:            req.Action,
		Duration:          req.Duration,
		OperatorType:      req.OperatorType,
		OperatorSensitive: req.OperatorSensitive,
		OperatorOperand:   req.OperatorOperand,
		OperatorData:      req.OperatorData,
		Description:       req.Description,
		Nolog:             req.Nolog,
		Created:           now,
	}

	if err := a.db.UpsertRule(dbRule); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Push to daemon
	a.pushRuleToDaemon(req.Node, &req, pb.Action_CHANGE_RULE)

	a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]interface{}{
		"action": "created",
		"rule":   dbRule,
	})

	writeJSON(w, http.StatusCreated, dbRule)
}

func (a *API) handleUpdateRule(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	var req ruleRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	req.Name = name

	now := time.Now().Format("2006-01-02 15:04:05")
	dbRule := &db.DBRule{
		Time:              now,
		Node:              req.Node,
		Name:              req.Name,
		Enabled:           req.Enabled,
		Precedence:        req.Precedence,
		Action:            req.Action,
		Duration:          req.Duration,
		OperatorType:      req.OperatorType,
		OperatorSensitive: req.OperatorSensitive,
		OperatorOperand:   req.OperatorOperand,
		OperatorData:      req.OperatorData,
		Description:       req.Description,
		Nolog:             req.Nolog,
	}

	if err := a.db.UpsertRule(dbRule); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	a.pushRuleToDaemon(req.Node, &req, pb.Action_CHANGE_RULE)

	a.hub.BroadcastEvent(ws.EventRuleChanged, map[string]interface{}{
		"action": "updated",
		"rule":   dbRule,
	})

	writeJSON(w, http.StatusOK, dbRule)
}

func (a *API) handleDeleteRule(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	node := r.URL.Query().Get("node")

	if err := a.db.DeleteRule(node, name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	// Send DELETE_RULE notification to daemon
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

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}

func (a *API) pushRuleToDaemon(nodeAddr string, req *ruleRequest, action pb.Action) {
	rule := &pb.Rule{
		Name:        req.Name,
		Enabled:     req.Enabled,
		Precedence:  req.Precedence,
		Action:      req.Action,
		Duration:    req.Duration,
		Description: req.Description,
		Nolog:       req.Nolog,
		Operator: &pb.Operator{
			Type:      req.OperatorType,
			Operand:   req.OperatorOperand,
			Data:      req.OperatorData,
			Sensitive: req.OperatorSensitive,
		},
	}

	notif := &pb.Notification{
		Id:    a.nodes.NextID(),
		Type:  action,
		Rules: []*pb.Rule{rule},
	}

	if nodeAddr != "" {
		a.nodes.SendNotification(nodeAddr, notif)
	} else {
		a.nodes.BroadcastNotification(notif)
	}
}
