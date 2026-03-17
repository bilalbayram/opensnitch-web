package db

import (
	"database/sql"
	"errors"
	"strings"
)

type DBRule struct {
	ID                int64  `json:"id"`
	Time              string `json:"time"`
	Node              string `json:"node"`
	Name              string `json:"name"`
	Enabled           bool   `json:"enabled"`
	Precedence        bool   `json:"precedence"`
	Action            string `json:"action"`
	Duration          string `json:"duration"`
	OperatorType      string `json:"operator_type"`
	OperatorSensitive bool   `json:"operator_sensitive"`
	OperatorOperand   string `json:"operator_operand"`
	OperatorData      string `json:"operator_data"`
	OperatorJSON      string `json:"-"`
	Description       string `json:"description"`
	Nolog             bool   `json:"nolog"`
	Created           string `json:"created"`
}

const upsertRuleQuery = `
	INSERT INTO rules (time, node, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, operator_json, description, nolog, created)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
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
		operator_json=excluded.operator_json,
		description=excluded.description,
		nolog=excluded.nolog`

type ruleExecer interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
}

func (d *Database) UpsertRule(r *DBRule) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	return upsertRule(d.db, r)
}

func (d *Database) ApplyGeneratedRules(node string, rules []*DBRule, mode string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return "", err
	}

	previousMode, err := getNodeModeTx(tx, node)
	if err != nil {
		_ = tx.Rollback()
		return "", err
	}

	for _, rule := range rules {
		if err := upsertRule(tx, rule); err != nil {
			_ = tx.Rollback()
			return "", err
		}
	}

	if _, err := tx.Exec("UPDATE nodes SET mode = ? WHERE addr = ?", mode, node); err != nil {
		_ = tx.Rollback()
		return "", err
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return previousMode, nil
}

func (d *Database) RevertGeneratedRules(node string, ruleNames []string, mode string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	trimmedNames := make([]string, 0, len(ruleNames))
	for _, name := range ruleNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		trimmedNames = append(trimmedNames, name)
	}

	if len(trimmedNames) > 0 {
		placeholders := strings.TrimRight(strings.Repeat("?,", len(trimmedNames)), ",")
		args := make([]interface{}, 0, len(trimmedNames)+1)
		args = append(args, node)
		for _, name := range trimmedNames {
			args = append(args, name)
		}

		if _, err := tx.Exec("DELETE FROM rules WHERE node = ? AND name IN ("+placeholders+")", args...); err != nil {
			_ = tx.Rollback()
			return err
		}
	}

	if _, err := tx.Exec("UPDATE nodes SET mode = ? WHERE addr = ?", mode, node); err != nil {
		_ = tx.Rollback()
		return err
	}

	return tx.Commit()
}

func upsertRule(exec ruleExecer, r *DBRule) error {
	_, err := exec.Exec(upsertRuleQuery,
		r.Time, r.Node, r.Name, r.Enabled, r.Precedence,
		r.Action, r.Duration, r.OperatorType, r.OperatorSensitive,
		r.OperatorOperand, r.OperatorData, r.OperatorJSON, r.Description, r.Nolog, r.Created,
	)
	return err
}

func getNodeModeTx(tx *sql.Tx, addr string) (string, error) {
	var mode string
	err := tx.QueryRow("SELECT mode FROM nodes WHERE addr = ?", addr).Scan(&mode)
	if errors.Is(err, sql.ErrNoRows) {
		return "ask", nil
	}
	if err != nil {
		return "", err
	}
	return mode, nil
}

func (d *Database) GetRules(node string) ([]DBRule, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := "SELECT id, time, node, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, operator_json, description, nolog, created FROM rules"
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
		if err := rows.Scan(&r.ID, &r.Time, &r.Node, &r.Name, &r.Enabled, &r.Precedence, &r.Action, &r.Duration, &r.OperatorType, &r.OperatorSensitive, &r.OperatorOperand, &r.OperatorData, &r.OperatorJSON, &r.Description, &r.Nolog, &r.Created); err != nil {
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
	err := d.db.QueryRow("SELECT id, time, node, name, enabled, precedence, action, duration, operator_type, operator_sensitive, operator_operand, operator_data, operator_json, description, nolog, created FROM rules WHERE node = ? AND name = ?", node, name).
		Scan(&r.ID, &r.Time, &r.Node, &r.Name, &r.Enabled, &r.Precedence, &r.Action, &r.Duration, &r.OperatorType, &r.OperatorSensitive, &r.OperatorOperand, &r.OperatorData, &r.OperatorJSON, &r.Description, &r.Nolog, &r.Created)
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
