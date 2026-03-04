package db

type DBAlert struct {
	ID       int64  `json:"id"`
	Time     string `json:"time"`
	Node     string `json:"node"`
	Type     int    `json:"type"`
	Action   int    `json:"action"`
	Priority int    `json:"priority"`
	What     int    `json:"what"`
	Body     string `json:"body"`
	Status   string `json:"status"`
}

func (d *Database) InsertAlert(a *DBAlert) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`INSERT INTO alerts (time, node, type, action, priority, what, body, status) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		a.Time, a.Node, a.Type, a.Action, a.Priority, a.What, a.Body, a.Status,
	)
	return err
}

func (d *Database) GetAlerts(limit, offset int) ([]DBAlert, int, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var total int
	if err := d.db.QueryRow("SELECT COUNT(*) FROM alerts").Scan(&total); err != nil {
		return nil, 0, err
	}

	if limit <= 0 {
		limit = 50
	}

	rows, err := d.db.Query("SELECT id, time, node, type, action, priority, what, body, status FROM alerts ORDER BY id DESC LIMIT ? OFFSET ?", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var alerts []DBAlert
	for rows.Next() {
		var a DBAlert
		if err := rows.Scan(&a.ID, &a.Time, &a.Node, &a.Type, &a.Action, &a.Priority, &a.What, &a.Body, &a.Status); err != nil {
			return nil, 0, err
		}
		alerts = append(alerts, a)
	}
	return alerts, total, nil
}

func (d *Database) DeleteAlert(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM alerts WHERE id = ?", id)
	return err
}
