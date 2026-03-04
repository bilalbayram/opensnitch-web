package db

type StatEntry struct {
	What string `json:"what"`
	Hits int64  `json:"hits"`
	Node string `json:"node"`
}

func (d *Database) UpsertStat(table, what, node string, hits int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	allowed := map[string]bool{"hosts": true, "procs": true, "addrs": true, "ports": true, "users": true}
	if !allowed[table] {
		return nil
	}

	_, err := d.db.Exec(
		"INSERT INTO "+table+" (what, hits, node) VALUES (?, ?, ?) ON CONFLICT(what, node) DO UPDATE SET hits=excluded.hits",
		what, hits, node,
	)
	return err
}

func (d *Database) GetStats(table string, limit int) ([]StatEntry, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	allowed := map[string]bool{"hosts": true, "procs": true, "addrs": true, "ports": true, "users": true}
	if !allowed[table] {
		return nil, nil
	}

	if limit <= 0 {
		limit = 100
	}

	rows, err := d.db.Query("SELECT what, hits, node FROM "+table+" ORDER BY hits DESC LIMIT ?", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []StatEntry
	for rows.Next() {
		var e StatEntry
		if err := rows.Scan(&e.What, &e.Hits, &e.Node); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}
