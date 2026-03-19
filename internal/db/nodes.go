package db

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
