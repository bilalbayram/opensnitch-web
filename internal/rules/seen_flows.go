package rules

import (
	"fmt"
	"strconv"
	"strings"

	pb "github.com/evilsocket/opensnitch-web/proto"
)

func LearningKeyFromConnection(conn *pb.Connection) (LearningKey, bool) {
	if conn == nil {
		return LearningKey{}, false
	}

	process := strings.TrimSpace(conn.GetProcessPath())
	protocol := strings.ToLower(strings.TrimSpace(conn.GetProtocol()))
	if process == "" || protocol == "" || conn.GetDstPort() == 0 {
		return LearningKey{}, false
	}

	destinationType := "dest.ip"
	destination := strings.TrimSpace(conn.GetDstIp())
	if host := strings.TrimSpace(conn.GetDstHost()); host != "" {
		destinationType = "dest.host"
		destination = host
	}
	if destination == "" {
		return LearningKey{}, false
	}

	return LearningKey{
		Process:         process,
		DestinationType: destinationType,
		Destination:     destination,
		DstPort:         int(conn.GetDstPort()),
		Protocol:        protocol,
	}, true
}

func BuildSeenFlowRule(key LearningKey, action string) *pb.Rule {
	protocol := strings.ToLower(strings.TrimSpace(key.Protocol))
	return &pb.Rule{
		Name:        fmt.Sprintf("seen-flow-%s-%s", strings.ToLower(strings.TrimSpace(action)), FingerprintForKey(key)[:8]),
		Description: "Auto-applied from a remembered user decision.",
		Enabled:     true,
		Action:      action,
		Duration:    "once",
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
