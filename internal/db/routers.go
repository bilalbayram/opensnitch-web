package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
)

type Router struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	Addr      string `json:"addr"`
	SSHPort   int    `json:"ssh_port"`
	SSHUser   string `json:"ssh_user"`
	APIKey    string `json:"api_key"`
	LANSubnet string `json:"lan_subnet"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func GenerateAPIKey() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (d *Database) InsertRouter(r *Router) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.db.Exec(`
		INSERT INTO routers (name, addr, ssh_port, ssh_user, api_key, lan_subnet, status)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.Addr, r.SSHPort, r.SSHUser, r.APIKey, r.LANSubnet, r.Status,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	r.ID = id
	return nil
}

func (d *Database) GetRouterByAPIKey(key string) (*Router, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var r Router
	err := d.db.QueryRow(`
		SELECT id, name, addr, ssh_port, ssh_user, api_key, lan_subnet, status, created_at, updated_at
		FROM routers WHERE api_key = ?`, key).
		Scan(&r.ID, &r.Name, &r.Addr, &r.SSHPort, &r.SSHUser, &r.APIKey, &r.LANSubnet, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *Database) GetRouterByAddr(addr string) (*Router, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var r Router
	err := d.db.QueryRow(`
		SELECT id, name, addr, ssh_port, ssh_user, api_key, lan_subnet, status, created_at, updated_at
		FROM routers WHERE addr = ?`, addr).
		Scan(&r.ID, &r.Name, &r.Addr, &r.SSHPort, &r.SSHUser, &r.APIKey, &r.LANSubnet, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *Database) GetRouters() ([]Router, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, name, addr, ssh_port, ssh_user, api_key, lan_subnet, status, created_at, updated_at
		FROM routers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routers []Router
	for rows.Next() {
		var r Router
		if err := rows.Scan(&r.ID, &r.Name, &r.Addr, &r.SSHPort, &r.SSHUser, &r.APIKey, &r.LANSubnet, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		routers = append(routers, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return routers, nil
}

func (d *Database) UpdateRouterStatus(addr, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, err := d.db.Exec("UPDATE routers SET status = ?, updated_at = datetime('now') WHERE addr = ?", status, addr)
	return err
}

func (d *Database) DeleteRouter(addr string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	res, err := d.db.Exec("DELETE FROM routers WHERE addr = ?", addr)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("router %s not found", addr)
	}
	return nil
}

func (d *Database) IsRouterAddr(addr string) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM routers WHERE addr = ?", addr).Scan(&count)
	if err != nil && err != sql.ErrNoRows {
		return false
	}
	return count > 0
}
