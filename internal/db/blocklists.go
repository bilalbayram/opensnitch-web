package db

import (
	"fmt"
	"time"
)

type Blocklist struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	URL         string `json:"url"`
	Category    string `json:"category"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	DomainCount int64  `json:"domain_count"`
	LastSynced  string `json:"last_synced"`
}

func (d *Database) GetBlocklists() ([]Blocklist, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT id, name, url, category, description, enabled, domain_count, last_synced FROM blocklists ORDER BY name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []Blocklist
	for rows.Next() {
		var b Blocklist
		var enabled int
		if err := rows.Scan(&b.ID, &b.Name, &b.URL, &b.Category, &b.Description, &enabled, &b.DomainCount, &b.LastSynced); err != nil {
			return nil, err
		}
		b.Enabled = enabled == 1
		lists = append(lists, b)
	}
	return lists, nil
}

func (d *Database) GetBlocklist(id int64) (*Blocklist, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var b Blocklist
	var enabled int
	err := d.db.QueryRow("SELECT id, name, url, category, description, enabled, domain_count, last_synced FROM blocklists WHERE id = ?", id).
		Scan(&b.ID, &b.Name, &b.URL, &b.Category, &b.Description, &enabled, &b.DomainCount, &b.LastSynced)
	if err != nil {
		return nil, err
	}
	b.Enabled = enabled == 1
	return &b, nil
}

func (d *Database) CreateBlocklist(name, url, category string) (*Blocklist, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	result, err := d.db.Exec("INSERT INTO blocklists (name, url, category) VALUES (?, ?, ?)", name, url, category)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &Blocklist{
		ID:       id,
		Name:     name,
		URL:      url,
		Category: category,
	}, nil
}

func (d *Database) DeleteBlocklist(id int64) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Delete domains first (FK may not cascade with pragma off)
	d.db.Exec("DELETE FROM blocklist_domains WHERE blocklist_id = ?", id)
	_, err := d.db.Exec("DELETE FROM blocklists WHERE id = ?", id)
	return err
}

func (d *Database) ToggleBlocklist(id int64, enabled bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	val := 0
	if enabled {
		val = 1
	}
	_, err := d.db.Exec("UPDATE blocklists SET enabled = ? WHERE id = ?", val, id)
	return err
}

func (d *Database) ReplaceBlocklistDomains(id int64, domains []string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM blocklist_domains WHERE blocklist_id = ?", id); err != nil {
		return fmt.Errorf("delete old domains: %w", err)
	}

	// Batch insert 500 at a time
	const batchSize = 500
	for i := 0; i < len(domains); i += batchSize {
		end := i + batchSize
		if end > len(domains) {
			end = len(domains)
		}
		batch := domains[i:end]

		stmt, err := tx.Prepare("INSERT OR IGNORE INTO blocklist_domains (blocklist_id, domain) VALUES (?, ?)")
		if err != nil {
			return fmt.Errorf("prepare: %w", err)
		}
		for _, domain := range batch {
			if _, err := stmt.Exec(id, domain); err != nil {
				stmt.Close()
				return fmt.Errorf("insert domain: %w", err)
			}
		}
		stmt.Close()
	}

	// Update metadata
	now := time.Now().Format("2006-01-02 15:04:05")
	if _, err := tx.Exec("UPDATE blocklists SET domain_count = ?, last_synced = ? WHERE id = ?", len(domains), now, id); err != nil {
		return fmt.Errorf("update metadata: %w", err)
	}

	return tx.Commit()
}

func (d *Database) IsDomainBlocked(domain string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var exists int
	err := d.db.QueryRow(`
		SELECT 1 FROM blocklist_domains bd
		JOIN blocklists b ON b.id = bd.blocklist_id
		WHERE bd.domain = ? AND b.enabled = 1
		LIMIT 1`, domain).Scan(&exists)
	return err == nil && exists == 1
}
