package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	db *sql.DB
	mu sync.RWMutex
}

func New(dbPath string) (*Database, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	sqlDB, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&cache=shared")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	sqlDB.SetMaxOpenConns(1)

	d := &Database{db: sqlDB}
	if err := d.migrate(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return d, nil
}

func (d *Database) Close() error {
	return d.db.Close()
}

func (d *Database) DB() *sql.DB {
	return d.db
}

func (d *Database) Lock() {
	d.mu.Lock()
}

func (d *Database) Unlock() {
	d.mu.Unlock()
}

func (d *Database) RLock() {
	d.mu.RLock()
}

func (d *Database) RUnlock() {
	d.mu.RUnlock()
}

func (d *Database) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS connections (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		time TEXT NOT NULL,
		node TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL DEFAULT '',
		protocol TEXT NOT NULL DEFAULT '',
		src_ip TEXT NOT NULL DEFAULT '',
		src_port INTEGER NOT NULL DEFAULT 0,
		dst_ip TEXT NOT NULL DEFAULT '',
		dst_host TEXT NOT NULL DEFAULT '',
		dst_port INTEGER NOT NULL DEFAULT 0,
		uid INTEGER NOT NULL DEFAULT 0,
		pid INTEGER NOT NULL DEFAULT 0,
		process TEXT NOT NULL DEFAULT '',
		process_args TEXT NOT NULL DEFAULT '',
		process_cwd TEXT NOT NULL DEFAULT '',
		rule TEXT NOT NULL DEFAULT '',
		UNIQUE(node, action, protocol, src_ip, src_port, dst_ip, dst_port, uid, pid, process, process_args)
	);

	CREATE INDEX IF NOT EXISTS idx_connections_time ON connections(time);
	CREATE INDEX IF NOT EXISTS idx_connections_action ON connections(action);
	CREATE INDEX IF NOT EXISTS idx_connections_protocol ON connections(protocol);
	CREATE INDEX IF NOT EXISTS idx_connections_dst_host ON connections(dst_host);
	CREATE INDEX IF NOT EXISTS idx_connections_process ON connections(process);
	CREATE INDEX IF NOT EXISTS idx_connections_dst_ip ON connections(dst_ip);
	CREATE INDEX IF NOT EXISTS idx_connections_dst_port ON connections(dst_port);
	CREATE INDEX IF NOT EXISTS idx_connections_rule ON connections(rule);
	CREATE INDEX IF NOT EXISTS idx_connections_node ON connections(node);

	CREATE TABLE IF NOT EXISTS nodes (
		addr TEXT PRIMARY KEY,
		hostname TEXT NOT NULL DEFAULT '',
		daemon_version TEXT NOT NULL DEFAULT '',
		daemon_uptime INTEGER NOT NULL DEFAULT 0,
		daemon_rules INTEGER NOT NULL DEFAULT 0,
		cons INTEGER NOT NULL DEFAULT 0,
		cons_dropped INTEGER NOT NULL DEFAULT 0,
		version TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'offline',
		last_connection TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		time TEXT NOT NULL DEFAULT '',
		node TEXT NOT NULL DEFAULT '',
		name TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		precedence INTEGER NOT NULL DEFAULT 0,
		action TEXT NOT NULL DEFAULT '',
		duration TEXT NOT NULL DEFAULT '',
		operator_type TEXT NOT NULL DEFAULT '',
		operator_sensitive INTEGER NOT NULL DEFAULT 0,
		operator_operand TEXT NOT NULL DEFAULT '',
		operator_data TEXT NOT NULL DEFAULT '',
		operator_json TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		nolog INTEGER NOT NULL DEFAULT 0,
		created TEXT NOT NULL DEFAULT '',
		UNIQUE(node, name)
	);

	CREATE INDEX IF NOT EXISTS idx_rules_time ON rules(time);

	CREATE TABLE IF NOT EXISTS alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		time TEXT NOT NULL DEFAULT '',
		node TEXT NOT NULL DEFAULT '',
		type INTEGER NOT NULL DEFAULT 0,
		action INTEGER NOT NULL DEFAULT 0,
		priority INTEGER NOT NULL DEFAULT 0,
		what INTEGER NOT NULL DEFAULT 0,
		body TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'new'
	);

	CREATE TABLE IF NOT EXISTS hosts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		what TEXT NOT NULL DEFAULT '',
		hits INTEGER NOT NULL DEFAULT 0,
		node TEXT NOT NULL DEFAULT '',
		UNIQUE(what, node)
	);

	CREATE TABLE IF NOT EXISTS procs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		what TEXT NOT NULL DEFAULT '',
		hits INTEGER NOT NULL DEFAULT 0,
		node TEXT NOT NULL DEFAULT '',
		UNIQUE(what, node)
	);

	CREATE TABLE IF NOT EXISTS addrs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		what TEXT NOT NULL DEFAULT '',
		hits INTEGER NOT NULL DEFAULT 0,
		node TEXT NOT NULL DEFAULT '',
		UNIQUE(what, node)
	);

	CREATE TABLE IF NOT EXISTS ports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		what TEXT NOT NULL DEFAULT '',
		hits INTEGER NOT NULL DEFAULT 0,
		node TEXT NOT NULL DEFAULT '',
		UNIQUE(what, node)
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		what TEXT NOT NULL DEFAULT '',
		hits INTEGER NOT NULL DEFAULT 0,
		node TEXT NOT NULL DEFAULT '',
		UNIQUE(what, node)
	);

	CREATE TABLE IF NOT EXISTS web_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);

	CREATE TABLE IF NOT EXISTS web_config (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS blocklists (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		url TEXT NOT NULL,
		category TEXT NOT NULL DEFAULT '',
		description TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 0,
		domain_count INTEGER NOT NULL DEFAULT 0,
		last_synced TEXT NOT NULL DEFAULT '',
		UNIQUE(url)
	);

	CREATE TABLE IF NOT EXISTS blocklist_domains (
		blocklist_id INTEGER NOT NULL,
		domain TEXT NOT NULL,
		UNIQUE(blocklist_id, domain),
		FOREIGN KEY (blocklist_id) REFERENCES blocklists(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_blocklist_domains_domain ON blocklist_domains(domain);

	CREATE TABLE IF NOT EXISTS process_trust (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node TEXT NOT NULL DEFAULT '',
		process_path TEXT NOT NULL DEFAULT '',
		trust_level TEXT NOT NULL DEFAULT 'default',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		UNIQUE(node, process_path)
	);
	CREATE INDEX IF NOT EXISTS idx_process_trust_lookup ON process_trust(node, process_path);
	`

	_, err := d.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	// Add mode column to nodes (safe: ignores error if already exists)
	d.db.Exec("ALTER TABLE nodes ADD COLUMN mode TEXT NOT NULL DEFAULT 'ask'")
	d.db.Exec("ALTER TABLE rules ADD COLUMN operator_json TEXT NOT NULL DEFAULT ''")

	// Pre-seed default blocklists (disabled by default)
	defaultBlocklists := []struct {
		name, url, category string
	}{
		{"Hagezi Light", "https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/hosts/light.txt", "ads"},
		{"Hagezi Pro", "https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/hosts/pro.txt", "ads"},
		{"Hagezi TIF", "https://cdn.jsdelivr.net/gh/hagezi/dns-blocklists@latest/hosts/tif.txt", "malware"},
		{"OISD Small", "https://small.oisd.nl/domainswild2", "ads"},
		{"OISD Big", "https://big.oisd.nl/domainswild2", "ads"},
		{"Steven Black", "https://raw.githubusercontent.com/StevenBlack/hosts/master/hosts", "ads"},
		{"Firebog Ads", "https://v.firebog.net/hosts/AdguardDNS.txt", "ads"},
		{"Firebog Trackers", "https://v.firebog.net/hosts/Easyprivacy.txt", "telemetry"},
		{"Firebog Malicious", "https://urlhaus.abuse.ch/downloads/hostfile/", "malware"},
	}
	for _, bl := range defaultBlocklists {
		d.db.Exec("INSERT OR IGNORE INTO blocklists (name, url, category) VALUES (?, ?, ?)", bl.name, bl.url, bl.category)
	}

	// Pre-seed default trusted processes (only if table is empty)
	defaultTrustedProcesses := []string{
		"/usr/bin/curl", "/usr/bin/wget", "/usr/bin/apt", "/usr/bin/apt-get",
		"/usr/lib/apt/methods/http", "/usr/lib/apt/methods/https",
		"/usr/bin/dpkg", "/usr/bin/snap", "/usr/bin/flatpak",
		"/usr/lib/systemd/systemd-resolved", "/usr/lib/systemd/systemd-timesyncd",
		"/usr/lib/systemd/systemd-networkd", "/usr/sbin/NetworkManager",
		"/usr/bin/node", "/usr/bin/git", "/usr/bin/ssh", "/usr/bin/gpg",
		"/usr/bin/gpgv", "/usr/lib/gnupg/dirmngr", "/usr/bin/python3",
	}
	var trustCount int
	d.db.QueryRow("SELECT COUNT(*) FROM process_trust").Scan(&trustCount)
	if trustCount == 0 {
		for _, p := range defaultTrustedProcesses {
			d.db.Exec("INSERT OR IGNORE INTO process_trust (node, process_path, trust_level) VALUES ('*', ?, 'trusted')", p)
		}
	}

	log.Println("[db] Schema migration completed")
	return nil
}
