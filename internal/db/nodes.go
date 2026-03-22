package db

import "fmt"

type Node struct {
	Addr          string `json:"addr"`
	Hostname      string `json:"hostname"`
	DaemonVersion string `json:"daemon_version"`
	DaemonUptime  int64  `json:"daemon_uptime"`
	DaemonRules   int64  `json:"daemon_rules"`
	Cons          int64  `json:"cons"`
	ConsDropped   int64  `json:"cons_dropped"`
	Version       string `json:"version"`
	Status        string `json:"status"`
	LastConn      string `json:"last_connection"`
	Mode          string `json:"mode"`
	SourceType    string `json:"source_type"`
}

func (d *Database) UpsertNode(n *Node) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	sourceType := n.SourceType
	if sourceType == "" {
		sourceType = "opensnitch"
	}

	_, err := d.db.Exec(`
		INSERT INTO nodes (addr, hostname, daemon_version, daemon_uptime, daemon_rules, cons, cons_dropped, version, status, last_connection, mode, source_type)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'ask', ?)
		ON CONFLICT(addr) DO UPDATE SET
			hostname=excluded.hostname,
			daemon_version=excluded.daemon_version,
			daemon_uptime=excluded.daemon_uptime,
			daemon_rules=excluded.daemon_rules,
			cons=excluded.cons,
			cons_dropped=excluded.cons_dropped,
			version=excluded.version,
			status=excluded.status,
			last_connection=excluded.last_connection,
			source_type=excluded.source_type`,
		n.Addr, n.Hostname, n.DaemonVersion, n.DaemonUptime, n.DaemonRules,
		n.Cons, n.ConsDropped, n.Version, n.Status, n.LastConn, sourceType,
	)
	return err
}

func (d *Database) GetNodes() ([]Node, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT addr, hostname, daemon_version, daemon_uptime, daemon_rules, cons, cons_dropped, version, status, last_connection, mode, source_type FROM nodes ORDER BY addr")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.Addr, &n.Hostname, &n.DaemonVersion, &n.DaemonUptime, &n.DaemonRules, &n.Cons, &n.ConsDropped, &n.Version, &n.Status, &n.LastConn, &n.Mode, &n.SourceType); err != nil {
			return nil, err
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (d *Database) GetNode(addr string) (*Node, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var n Node
	err := d.db.QueryRow("SELECT addr, hostname, daemon_version, daemon_uptime, daemon_rules, cons, cons_dropped, version, status, last_connection, mode, source_type FROM nodes WHERE addr = ?", addr).
		Scan(&n.Addr, &n.Hostname, &n.DaemonVersion, &n.DaemonUptime, &n.DaemonRules, &n.Cons, &n.ConsDropped, &n.Version, &n.Status, &n.LastConn, &n.Mode, &n.SourceType)
	if err != nil {
		return nil, err
	}
	return &n, nil
}

func (d *Database) SetNodeStatus(addr, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("UPDATE nodes SET status = ? WHERE addr = ?", status, addr)
	return err
}

func (d *Database) SetNodeMode(addr, mode string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("UPDATE nodes SET mode = ? WHERE addr = ?", mode, addr)
	return err
}

// UpsertRouterNode registers or updates a router node without overwriting
// connection counters. This avoids the bug where UpsertNode resets cons to 0
// on every ingest POST because the caller passes a zero-value Node.
func (d *Database) UpsertRouterNode(addr, hostname, version, status, lastConn string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO nodes (addr, hostname, daemon_version, daemon_uptime, daemon_rules, cons, cons_dropped, version, status, last_connection, mode, source_type)
		VALUES (?, ?, ?, 0, 0, 0, 0, '', ?, ?, 'ask', 'router')
		ON CONFLICT(addr) DO UPDATE SET
			hostname=excluded.hostname,
			daemon_version=excluded.daemon_version,
			status=excluded.status,
			last_connection=excluded.last_connection,
			source_type='router'`,
		addr, hostname, version, status, lastConn,
	)
	return err
}

// IncrementNodeCons atomically adds to the connection counter and updates
// the node's status and last_connection timestamp.
func (d *Database) IncrementNodeCons(addr string, count int, lastConn string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		"UPDATE nodes SET cons = cons + ?, status = 'online', last_connection = ? WHERE addr = ?",
		count, lastConn, addr,
	)
	return err
}

func (d *Database) GetNodeMode(addr string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var mode string
	err := d.db.QueryRow("SELECT mode FROM nodes WHERE addr = ?", addr).Scan(&mode)
	if err != nil {
		return "ask", nil
	}
	return mode, nil
}

func (d *Database) DeleteNode(addr string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	statements := []struct {
		query string
		args  []any
	}{
		{query: "DELETE FROM template_attachments WHERE target_type = 'node' AND target_ref = ?", args: []any{addr}},
		{query: "DELETE FROM node_tags WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM node_template_sync WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM process_trust WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM rules WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM alerts WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM seen_flows WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM dns_domains WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM connections WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM hosts WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM procs WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM addrs WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM ports WHERE node = ?", args: []any{addr}},
		{query: "DELETE FROM users WHERE node = ?", args: []any{addr}},
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt.query, stmt.args...); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	result, err := tx.Exec("DELETE FROM nodes WHERE addr = ?", addr)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	if rowsAffected == 0 {
		_ = tx.Rollback()
		return fmt.Errorf("node %s not found", addr)
	}

	return tx.Commit()
}
