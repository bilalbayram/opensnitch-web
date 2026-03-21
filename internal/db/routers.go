package db

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
)

type Router struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	Addr           string `json:"addr"`
	SSHPort        int    `json:"ssh_port"`
	SSHUser        string `json:"ssh_user"`
	APIKey         string `json:"-"`
	LANSubnet      string `json:"lan_subnet"`
	DaemonMode     string `json:"daemon_mode"`
	LinkedNodeAddr string `json:"linked_node_addr"`
	Status         string `json:"status"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at"`
}

const (
	RouterDaemonModeConntrackAgent = "conntrack-agent"
	RouterDaemonModeRouterDaemon   = "router-daemon"
)

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

	if strings.TrimSpace(r.DaemonMode) == "" {
		r.DaemonMode = RouterDaemonModeConntrackAgent
	}

	res, err := d.db.Exec(`
		INSERT INTO routers (name, addr, ssh_port, ssh_user, api_key, lan_subnet, daemon_mode, linked_node_addr, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.Name, r.Addr, r.SSHPort, r.SSHUser, r.APIKey, r.LANSubnet, r.DaemonMode, r.LinkedNodeAddr, r.Status,
	)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	r.ID = id
	return nil
}

func (d *Database) UpsertRouter(r *Router) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if strings.TrimSpace(r.DaemonMode) == "" {
		r.DaemonMode = RouterDaemonModeConntrackAgent
	}

	_, err := d.db.Exec(`
		INSERT INTO routers (name, addr, ssh_port, ssh_user, api_key, lan_subnet, daemon_mode, linked_node_addr, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(addr) DO UPDATE SET
			name=excluded.name,
			ssh_port=excluded.ssh_port,
			ssh_user=excluded.ssh_user,
			lan_subnet=excluded.lan_subnet,
			daemon_mode=excluded.daemon_mode,
			linked_node_addr=excluded.linked_node_addr,
			status=excluded.status,
			updated_at=datetime('now')`,
		r.Name, r.Addr, r.SSHPort, r.SSHUser, r.APIKey, r.LANSubnet, r.DaemonMode, r.LinkedNodeAddr, r.Status,
	)
	if err != nil {
		return err
	}

	err = d.db.QueryRow(`
		SELECT id, created_at, updated_at, api_key, daemon_mode, linked_node_addr
		FROM routers WHERE addr = ?`, r.Addr).
		Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt, &r.APIKey, &r.DaemonMode, &r.LinkedNodeAddr)
	if err != nil {
		return err
	}

	return nil
}

func (d *Database) GetRouterByAPIKey(key string) (*Router, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var r Router
	err := d.db.QueryRow(`
		SELECT id, name, addr, ssh_port, ssh_user, api_key, lan_subnet, daemon_mode, linked_node_addr, status, created_at, updated_at
		FROM routers WHERE api_key = ?`, key).
		Scan(&r.ID, &r.Name, &r.Addr, &r.SSHPort, &r.SSHUser, &r.APIKey, &r.LANSubnet, &r.DaemonMode, &r.LinkedNodeAddr, &r.Status, &r.CreatedAt, &r.UpdatedAt)
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
		SELECT id, name, addr, ssh_port, ssh_user, api_key, lan_subnet, daemon_mode, linked_node_addr, status, created_at, updated_at
		FROM routers WHERE addr = ?`, addr).
		Scan(&r.ID, &r.Name, &r.Addr, &r.SSHPort, &r.SSHUser, &r.APIKey, &r.LANSubnet, &r.DaemonMode, &r.LinkedNodeAddr, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *Database) GetRouterByLinkedNodeAddr(nodeAddr string) (*Router, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var r Router
	err := d.db.QueryRow(`
		SELECT id, name, addr, ssh_port, ssh_user, api_key, lan_subnet, daemon_mode, linked_node_addr, status, created_at, updated_at
		FROM routers WHERE linked_node_addr = ?`, nodeAddr).
		Scan(&r.ID, &r.Name, &r.Addr, &r.SSHPort, &r.SSHUser, &r.APIKey, &r.LANSubnet, &r.DaemonMode, &r.LinkedNodeAddr, &r.Status, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (d *Database) GetRouters() ([]Router, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.db.Query(`
		SELECT id, name, addr, ssh_port, ssh_user, api_key, lan_subnet, daemon_mode, linked_node_addr, status, created_at, updated_at
		FROM routers ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var routers []Router
	for rows.Next() {
		var r Router
		if err := rows.Scan(&r.ID, &r.Name, &r.Addr, &r.SSHPort, &r.SSHUser, &r.APIKey, &r.LANSubnet, &r.DaemonMode, &r.LinkedNodeAddr, &r.Status, &r.CreatedAt, &r.UpdatedAt); err != nil {
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

func (d *Database) UpdateRouterRuntime(addr, daemonMode, linkedNodeAddr string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if strings.TrimSpace(daemonMode) == "" {
		daemonMode = RouterDaemonModeConntrackAgent
	}

	_, err := d.db.Exec(
		"UPDATE routers SET daemon_mode = ?, linked_node_addr = ?, updated_at = datetime('now') WHERE addr = ?",
		daemonMode,
		strings.TrimSpace(linkedNodeAddr),
		addr,
	)
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
