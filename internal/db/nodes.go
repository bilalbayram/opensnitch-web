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
}

func (d *Database) UpsertNode(n *Node) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO nodes (addr, hostname, daemon_version, daemon_uptime, daemon_rules, cons, cons_dropped, version, status, last_connection)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(addr) DO UPDATE SET
			hostname=excluded.hostname,
			daemon_version=excluded.daemon_version,
			daemon_uptime=excluded.daemon_uptime,
			daemon_rules=excluded.daemon_rules,
			cons=excluded.cons,
			cons_dropped=excluded.cons_dropped,
			version=excluded.version,
			status=excluded.status,
			last_connection=excluded.last_connection`,
		n.Addr, n.Hostname, n.DaemonVersion, n.DaemonUptime, n.DaemonRules,
		n.Cons, n.ConsDropped, n.Version, n.Status, n.LastConn,
	)
	return err
}

func (d *Database) GetNodes() ([]Node, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT addr, hostname, daemon_version, daemon_uptime, daemon_rules, cons, cons_dropped, version, status, last_connection FROM nodes ORDER BY addr")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var nodes []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.Addr, &n.Hostname, &n.DaemonVersion, &n.DaemonUptime, &n.DaemonRules, &n.Cons, &n.ConsDropped, &n.Version, &n.Status, &n.LastConn); err != nil {
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
	err := d.db.QueryRow("SELECT addr, hostname, daemon_version, daemon_uptime, daemon_rules, cons, cons_dropped, version, status, last_connection FROM nodes WHERE addr = ?", addr).
		Scan(&n.Addr, &n.Hostname, &n.DaemonVersion, &n.DaemonUptime, &n.DaemonRules, &n.Cons, &n.ConsDropped, &n.Version, &n.Status, &n.LastConn)
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
