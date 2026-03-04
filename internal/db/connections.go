package db

import (
	"fmt"
	"strings"
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
