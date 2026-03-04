package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	pb "github.com/evilsocket/opensnitch-web/proto"
)

type promptReplyRequest struct {
	Action   string `json:"action"`
	Duration string `json:"duration"`
	Name     string `json:"name"`
	Operand  string `json:"operand"`
	Data     string `json:"data"`
	Operator string `json:"operator"`
}

func (a *API) handleGetPendingPrompts(w http.ResponseWriter, r *http.Request) {
	pending := a.prompter.GetPending()

	type promptResponse struct {
		ID        string      `json:"id"`
		NodeAddr  string      `json:"node_addr"`
		CreatedAt string      `json:"created_at"`
		Process   string      `json:"process"`
		DstHost   string      `json:"dst_host"`
		DstIP     string      `json:"dst_ip"`
		DstPort   uint32      `json:"dst_port"`
		Protocol  string      `json:"protocol"`
		SrcIP     string      `json:"src_ip"`
		SrcPort   uint32      `json:"src_port"`
		UID       uint32      `json:"uid"`
		PID       uint32      `json:"pid"`
		Args      []string    `json:"args"`
		Cwd       string      `json:"cwd"`
		Checksums interface{} `json:"checksums"`
	}

	result := make([]promptResponse, 0, len(pending))
	for _, p := range pending {
		conn := p.Connection
		result = append(result, promptResponse{
			ID:        p.ID,
			NodeAddr:  p.NodeAddr,
			CreatedAt: p.CreatedAt.Format("2006-01-02 15:04:05"),
			Process:   conn.GetProcessPath(),
			DstHost:   conn.GetDstHost(),
			DstIP:     conn.GetDstIp(),
			DstPort:   conn.GetDstPort(),
			Protocol:  conn.GetProtocol(),
			SrcIP:     conn.GetSrcIp(),
			SrcPort:   conn.GetSrcPort(),
			UID:       conn.GetUserId(),
			PID:       conn.GetProcessId(),
			Args:      conn.GetProcessArgs(),
			Cwd:       conn.GetProcessCwd(),
			Checksums: conn.GetProcessChecksums(),
		})
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handlePromptReply(w http.ResponseWriter, r *http.Request) {
	promptID := chi.URLParam(r, "id")

	var req promptReplyRequest
	if err := readJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}

	operatorType := req.Operator
	if operatorType == "" {
		operatorType = "simple"
	}

	rule := &pb.Rule{
		Name:     req.Name,
		Action:   req.Action,
		Duration: req.Duration,
		Enabled:  true,
		Operator: &pb.Operator{
			Type:    operatorType,
			Operand: req.Operand,
			Data:    req.Data,
		},
	}

	if err := a.prompter.Reply(promptID, rule); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
