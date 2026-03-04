package db

type DBRule struct {
	ID               int64  `json:"id"`
	Time             string `json:"time"`
	Node             string `json:"node"`
	Name             string `json:"name"`
	Enabled          bool   `json:"enabled"`
	Precedence       bool   `json:"precedence"`
	Action           string `json:"action"`
	Duration         string `json:"duration"`
	OperatorType     string `json:"operator_type"`
	OperatorSensitive bool  `json:"operator_sensitive"`
	OperatorOperand  string `json:"operator_operand"`
	OperatorData     string `json:"operator_data"`
	Description      string `json:"description"`
	Nolog            bool   `json:"nolog"`
	Created          string `json:"created"`
}

func (d *Database) UpsertRule(r *DBRule) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(`
		INSERT INTO rules (time, node, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, description, nolog, created)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(node, name) DO UPDATE SET
			time=excluded.time,
			enabled=excluded.enabled,
			precedence=excluded.precedence,
			action=excluded.action,
			duration=excluded.duration,
			operator_type=excluded.operator_type,
			operator_sensitive=excluded.operator_sensitive,
			operator_operand=excluded.operator_operand,
			operator_data=excluded.operator_data,
			description=excluded.description,
			nolog=excluded.nolog`,
		r.Time, r.Node, r.Name, r.Enabled, r.Precedence,
		r.Action, r.Duration, r.OperatorType, r.OperatorSensitive,
		r.OperatorOperand, r.OperatorData, r.Description, r.Nolog, r.Created,
	)
	return err
}

func (d *Database) GetRules(node string) ([]DBRule, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := "SELECT id, time, node, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, description, nolog, created FROM rules"
	args := []interface{}{}
	if node != "" {
		query += " WHERE node = ?"
		args = append(args, node)
	}
	query += " ORDER BY name"

	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []DBRule
	for rows.Next() {
		var r DBRule
		if err := rows.Scan(&r.ID, &r.Time, &r.Node, &r.Name, &r.Enabled, &r.Precedence, &r.Action, &r.Duration, &r.OperatorType, &r.OperatorSensitive, &r.OperatorOperand, &r.OperatorData, &r.Description, &r.Nolog, &r.Created); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (d *Database) GetRule(node, name string) (*DBRule, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var r DBRule
	err := d.db.QueryRow("SELECT id, time, node, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, description, nolog, created FROM rules WHERE node = ? AND name = ?", node, name).
		Scan(&r.ID, &r.Time, &r.Node, &r.Name, &r.Enabled, &r.Precedence, &r.Action, &r.Duration, &r.OperatorType, &r.OperatorSensitive, &r.OperatorOperand, &r.OperatorData, &r.Description, &r.Nolog, &r.Created)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *Database) DeleteRule(node, name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM rules WHERE node = ? AND name = ?", node, name)
	return err
}
