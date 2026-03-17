package db

import (
	"fmt"
	"strings"
	"time"
)

type Connection struct {
	ID          int64  `json:"id"`
	Time        string `json:"time"`
	Node        string `json:"node"`
	Action      string `json:"action"`
	Protocol    string `json:"protocol"`
	SrcIP       string `json:"src_ip"`
	SrcPort     int    `json:"src_port"`
	DstIP       string `json:"dst_ip"`
	DstHost     string `json:"dst_host"`
	DstPort     int    `json:"dst_port"`
	UID         int    `json:"uid"`
	PID         int    `json:"pid"`
	Process     string `json:"process"`
	ProcessArgs string `json:"process_args"`
	ProcessCwd  string `json:"process_cwd"`
	Rule        string `json:"rule"`
}

type ConnectionFilter struct {
	Node     string
	Action   string
	Protocol string
	DstHost  string
	DstIP    string
	DstPort  int
	Process  string
	Rule     string
	Search   string
	Limit    int
	Offset   int
}

type Flow struct {
	Node                string `json:"node"`
	Process             string `json:"process"`
	Protocol            string `json:"protocol"`
	DstPort             int    `json:"dst_port"`
	Destination         string `json:"destination"`
	DestinationOperand  string `json:"destination_operand"`
	Hits                int    `json:"hits"`
	FirstSeen           string `json:"first_seen"`
	LastSeen            string `json:"last_seen"`
}

func (d *Database) InsertConnection(c *Connection) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT OR REPLACE INTO connections (time, node, action, protocol, src_ip, src_port, dst_ip, dst_host, dst_port, uid, pid, process, process_args, process_cwd, rule)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Time, c.Node, c.Action, c.Protocol, c.SrcIP, c.SrcPort,
		c.DstIP, c.DstHost, c.DstPort, c.UID, c.PID,
		c.Process, c.ProcessArgs, c.ProcessCwd, c.Rule,
	)
	return err
}

func (d *Database) GetConnections(f *ConnectionFilter) ([]Connection, int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	where := []string{"1=1"}
	args := []interface{}{}

	if f.Node != "" {
		where = append(where, "node = ?")
		args = append(args, f.Node)
	}
	if f.Action != "" {
		where = append(where, "action = ?")
		args = append(args, f.Action)
	}
	if f.Protocol != "" {
		where = append(where, "protocol = ?")
		args = append(args, f.Protocol)
	}
	if f.DstHost != "" {
		where = append(where, "dst_host LIKE ?")
		args = append(args, "%"+f.DstHost+"%")
	}
	if f.DstIP != "" {
		where = append(where, "dst_ip = ?")
		args = append(args, f.DstIP)
	}
	if f.DstPort > 0 {
		where = append(where, "dst_port = ?")
		args = append(args, f.DstPort)
	}
	if f.Process != "" {
		where = append(where, "process LIKE ?")
		args = append(args, "%"+f.Process+"%")
	}
	if f.Rule != "" {
		where = append(where, "rule = ?")
		args = append(args, f.Rule)
	}
	if f.Search != "" {
		where = append(where, "(dst_host LIKE ? OR process LIKE ? OR dst_ip LIKE ? OR rule LIKE ?)")
		s := "%" + f.Search + "%"
		args = append(args, s, s, s, s)
	}

	whereClause := strings.Join(where, " AND ")

	// Count total
	var total int
	countArgs := make([]interface{}, len(args))
	copy(countArgs, args)
	err := d.db.QueryRow("SELECT COUNT(*) FROM connections WHERE "+whereClause, countArgs...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	limit := f.Limit
	if limit <= 0 {
		limit = 50
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}

	query := fmt.Sprintf("SELECT id, time, node, action, protocol, src_ip, src_port, dst_ip, dst_host, dst_port, uid, pid, process, process_args, process_cwd, rule FROM connections WHERE %s ORDER BY id DESC LIMIT ? OFFSET ?", whereClause)
	args = append(args, limit, offset)

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var conns []Connection
	for rows.Next() {
		var c Connection
		if err := rows.Scan(&c.ID, &c.Time, &c.Node, &c.Action, &c.Protocol, &c.SrcIP, &c.SrcPort, &c.DstIP, &c.DstHost, &c.DstPort, &c.UID, &c.PID, &c.Process, &c.ProcessArgs, &c.ProcessCwd, &c.Rule); err != nil {
			return nil, 0, err
		}
		conns = append(conns, c)
	}
	return conns, total, nil
}

func (d *Database) PurgeConnections() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM connections")
	return err
}

func (d *Database) GetUniqueFlows(node string, since, until time.Time, excludeProcesses []string) ([]Flow, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	where := []string{
		"node = ?",
		"action = 'allow'",
		"rule = 'silent-allow'",
		"time >= ?",
		"time <= ?",
		"process != ''",
		"protocol != ''",
		"dst_port > 0",
		"(TRIM(dst_host) != '' OR TRIM(dst_ip) != '')",
	}
	args := []interface{}{
		node,
		since.In(time.Local).Format("2006-01-02 15:04:05"),
		until.In(time.Local).Format("2006-01-02 15:04:05"),
	}

	trimmedExclusions := make([]string, 0, len(excludeProcesses))
	for _, process := range excludeProcesses {
		process = strings.TrimSpace(process)
		if process == "" {
			continue
		}
		trimmedExclusions = append(trimmedExclusions, process)
	}

	if len(trimmedExclusions) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(trimmedExclusions)), ",")
		where = append(where, "process NOT IN ("+placeholders+")")
		for _, process := range trimmedExclusions {
			args = append(args, process)
		}
	}

	query := fmt.Sprintf(`
		SELECT
			process,
			protocol,
			dst_port,
			CASE WHEN TRIM(dst_host) != '' THEN dst_host ELSE dst_ip END AS destination,
			CASE WHEN TRIM(dst_host) != '' THEN 'dest.host' ELSE 'dest.ip' END AS destination_operand,
			COUNT(*) AS hits,
			MIN(time) AS first_seen,
			MAX(time) AS last_seen
		FROM connections
		WHERE %s
		GROUP BY process, protocol, dst_port, destination_operand, destination
		ORDER BY hits DESC, process, destination, dst_port, protocol
	`, strings.Join(where, " AND "))

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	flows := []Flow{}
	for rows.Next() {
		var flow Flow
		var hits int64
		if err := rows.Scan(
			&flow.Process,
			&flow.Protocol,
			&flow.DstPort,
			&flow.Destination,
			&flow.DestinationOperand,
			&hits,
			&flow.FirstSeen,
			&flow.LastSeen,
		); err != nil {
			return nil, err
		}
		flow.Node = node
		flow.Hits = int(hits)
		flows = append(flows, flow)
	}

	return flows, rows.Err()
}
