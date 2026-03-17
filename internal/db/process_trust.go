package db

import "fmt"

type ProcessTrust struct {
	ID          int64  `json:"id"`
	Node        string `json:"node"`
	ProcessPath string `json:"process_path"`
	TrustLevel  string `json:"trust_level"`
	CreatedAt   string `json:"created_at"`
}

// GetProcessTrustLevel returns the effective trust level for a process on a node.
// Node-specific entries take priority over wildcard ("*") entries.
// Returns "default" if no matching entry exists.
func (d *Database) GetProcessTrustLevel(node, processPath string) string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	// Check node-specific first
	var level string
	err := d.db.QueryRow(
		"SELECT trust_level FROM process_trust WHERE node = ? AND process_path = ?",
		node, processPath,
	).Scan(&level)
	if err == nil {
		return level
	}

	// Fall back to wildcard
	err = d.db.QueryRow(
		"SELECT trust_level FROM process_trust WHERE node = '*' AND process_path = ?",
		processPath,
	).Scan(&level)
	if err == nil {
		return level
	}

	return "default"
}

// GetProcessTrustList returns all trust entries that apply to a node (node-specific + wildcard).
func (d *Database) GetProcessTrustList(node string) ([]ProcessTrust, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(
		"SELECT id, node, process_path, trust_level, created_at FROM process_trust WHERE node = ? OR node = '*' ORDER BY process_path",
		node,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []ProcessTrust
	for rows.Next() {
		var pt ProcessTrust
		if err := rows.Scan(&pt.ID, &pt.Node, &pt.ProcessPath, &pt.TrustLevel, &pt.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, pt)
	}
	return list, nil
}

// AddProcessTrust creates a new trust entry. Returns conflict error if duplicate.
func (d *Database) AddProcessTrust(node, processPath, trustLevel string) (*ProcessTrust, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec(
		"INSERT INTO process_trust (node, process_path, trust_level) VALUES (?, ?, ?)",
		node, processPath, trustLevel,
	)
	if err != nil {
		return nil, fmt.Errorf("insert process trust: %w", err)
	}

	id, _ := result.LastInsertId()
	return &ProcessTrust{
		ID:          id,
		Node:        node,
		ProcessPath: processPath,
		TrustLevel:  trustLevel,
	}, nil
}

// UpdateProcessTrust changes the trust level of an existing entry.
func (d *Database) UpdateProcessTrust(id int64, trustLevel string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("UPDATE process_trust SET trust_level = ? WHERE id = ?", trustLevel, id)
	return err
}

// DeleteProcessTrust removes a trust entry by ID.
func (d *Database) DeleteProcessTrust(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM process_trust WHERE id = ?", id)
	return err
}
