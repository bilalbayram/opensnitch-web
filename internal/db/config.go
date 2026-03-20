package db

import "strings"

// GetWebConfig retrieves a single value from the web_config table.
func (d *Database) GetWebConfig(key string) (string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var value string
	err := d.db.QueryRow("SELECT value FROM web_config WHERE key = ?", key).Scan(&value)
	return value, err
}

// SetWebConfig inserts or updates a key-value pair in web_config.
func (d *Database) SetWebConfig(key, value string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec(
		"INSERT INTO web_config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value = ?",
		key, value, value,
	)
	return err
}

// DeleteWebConfig removes a key from web_config.
func (d *Database) DeleteWebConfig(key string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("DELETE FROM web_config WHERE key = ?", key)
	return err
}

// GetWebConfigByPrefix returns all entries whose key starts with the given prefix.
func (d *Database) GetWebConfigByPrefix(prefix string) (map[string]string, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query("SELECT key, value FROM web_config WHERE key LIKE ?", prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		// Strip the prefix so callers get the node address as key
		result[strings.TrimPrefix(k, prefix)] = v
	}
	return result, rows.Err()
}
