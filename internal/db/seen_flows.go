package db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

const seenFlowTimeLayout = "2006-01-02 15:04:05"

type SeenFlowKey struct {
	Node               string
	Process            string
	Protocol           string
	DstPort            int
	DestinationOperand string
	Destination        string
}

type SeenFlow struct {
	ID                 int64  `json:"id"`
	Node               string `json:"node"`
	Process            string `json:"process"`
	Protocol           string `json:"protocol"`
	DstPort            int    `json:"dst_port"`
	DestinationOperand string `json:"destination_operand"`
	Destination        string `json:"destination"`
	Action             string `json:"action"`
	SourceRuleName     string `json:"-"`
	FirstSeen          string `json:"first_seen"`
	LastSeen           string `json:"last_seen"`
	ExpiresAt          string `json:"-"`
	Count              int    `json:"count"`
}

type SeenFlowFilter struct {
	Node   string
	Action string
	Search string
	Limit  int
	Offset int
}

func (d *Database) GetSeenFlow(key SeenFlowKey) (*SeenFlow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
		SELECT id, node, process, protocol, dst_port, destination_operand, destination, action, source_rule_name, first_seen, last_seen, expires_at, count
		FROM seen_flows
		WHERE node = ? AND process = ? AND protocol = ? AND dst_port = ? AND destination_operand = ? AND destination = ?
	`

	var flow SeenFlow
	err := d.db.QueryRow(query,
		key.Node,
		key.Process,
		key.Protocol,
		key.DstPort,
		key.DestinationOperand,
		key.Destination,
	).Scan(
		&flow.ID,
		&flow.Node,
		&flow.Process,
		&flow.Protocol,
		&flow.DstPort,
		&flow.DestinationOperand,
		&flow.Destination,
		&flow.Action,
		&flow.SourceRuleName,
		&flow.FirstSeen,
		&flow.LastSeen,
		&flow.ExpiresAt,
		&flow.Count,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	return &flow, nil
}

func (f *SeenFlow) ExpiryTime() (time.Time, bool) {
	if f == nil || strings.TrimSpace(f.ExpiresAt) == "" {
		return time.Time{}, false
	}

	expiresAt, err := time.ParseInLocation(seenFlowTimeLayout, f.ExpiresAt, time.Local)
	if err != nil {
		return time.Time{}, false
	}

	return expiresAt, true
}

func (f *SeenFlow) IsExpired(now time.Time) bool {
	if f == nil {
		return false
	}

	expiresAt, ok := f.ExpiryTime()
	if !ok {
		return strings.TrimSpace(f.ExpiresAt) != ""
	}

	return !expiresAt.After(now.In(time.Local))
}

func (d *Database) UpsertSeenFlow(key SeenFlowKey, action, sourceRuleName string, observedAt, expiresAt time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if observedAt.IsZero() {
		observedAt = time.Now()
	}

	timestamp := observedAt.In(time.Local).Format(seenFlowTimeLayout)
	expiresAtValue := ""
	if !expiresAt.IsZero() {
		expiresAtValue = expiresAt.In(time.Local).Format(seenFlowTimeLayout)
	}

	_, err := d.db.Exec(`
		INSERT INTO seen_flows (
			node, process, protocol, dst_port, destination_operand, destination, action, source_rule_name, first_seen, last_seen, expires_at, count
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1)
		ON CONFLICT(node, process, protocol, dst_port, destination_operand, destination) DO UPDATE SET
			action = excluded.action,
			source_rule_name = excluded.source_rule_name,
			last_seen = excluded.last_seen,
			expires_at = excluded.expires_at,
			count = seen_flows.count + 1
	`,
		key.Node,
		key.Process,
		key.Protocol,
		key.DstPort,
		key.DestinationOperand,
		key.Destination,
		action,
		sourceRuleName,
		timestamp,
		timestamp,
		expiresAtValue,
	)
	return err
}

func (d *Database) GetSeenFlows(filter *SeenFlowFilter) ([]SeenFlow, int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	where := []string{"1=1"}
	args := []interface{}{}

	if filter.Node != "" {
		where = append(where, "node = ?")
		args = append(args, filter.Node)
	}
	if filter.Action != "" {
		where = append(where, "action = ?")
		args = append(args, filter.Action)
	}
	if filter.Search != "" {
		search := "%" + filter.Search + "%"
		where = append(where, "(node LIKE ? OR process LIKE ? OR destination LIKE ? OR protocol LIKE ?)")
		args = append(args, search, search, search, search)
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	if err := d.db.QueryRow("SELECT COUNT(*) FROM seen_flows WHERE "+whereClause, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf(`
		SELECT id, node, process, protocol, dst_port, destination_operand, destination, action, source_rule_name, first_seen, last_seen, expires_at, count
		FROM seen_flows
		WHERE %s
		ORDER BY last_seen DESC, id DESC
		LIMIT ? OFFSET ?
	`, whereClause)
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	flows := []SeenFlow{}
	for rows.Next() {
		var flow SeenFlow
		if err := rows.Scan(
			&flow.ID,
			&flow.Node,
			&flow.Process,
			&flow.Protocol,
			&flow.DstPort,
			&flow.DestinationOperand,
			&flow.Destination,
			&flow.Action,
			&flow.SourceRuleName,
			&flow.FirstSeen,
			&flow.LastSeen,
			&flow.ExpiresAt,
			&flow.Count,
		); err != nil {
			return nil, 0, err
		}
		flows = append(flows, flow)
	}

	return flows, total, rows.Err()
}

func (d *Database) DeleteSeenFlow(key SeenFlowKey) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		DELETE FROM seen_flows
		WHERE node = ? AND process = ? AND protocol = ? AND dst_port = ? AND destination_operand = ? AND destination = ?
	`,
		key.Node,
		key.Process,
		key.Protocol,
		key.DstPort,
		key.DestinationOperand,
		key.Destination,
	)
	return err
}

func (d *Database) DeleteSeenFlowsBySourceRule(node, ruleName string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return deleteSeenFlowsBySourceRuleExec(d.db, node, ruleName)
}

type seenFlowExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func deleteSeenFlowsBySourceRuleExec(exec seenFlowExecer, node, ruleName string) error {
	ruleName = strings.TrimSpace(ruleName)
	if ruleName == "" {
		return nil
	}

	query := "DELETE FROM seen_flows WHERE source_rule_name = ?"
	args := []interface{}{ruleName}
	if strings.TrimSpace(node) != "" {
		query += " AND node = ?"
		args = append(args, node)
	}

	_, err := exec.Exec(query, args...)
	return err
}
