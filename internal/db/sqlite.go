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
	CREATE INDEX IF NOT EXISTS idx_connections_time_action ON connections(time, action);

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
		display_name TEXT NOT NULL DEFAULT '',
		source_kind TEXT NOT NULL DEFAULT 'manual',
		template_id INTEGER NOT NULL DEFAULT 0,
		template_rule_id INTEGER NOT NULL DEFAULT 0,
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
	CREATE INDEX IF NOT EXISTS idx_rules_node_source ON rules(node, source_kind);
	CREATE INDEX IF NOT EXISTS idx_rules_template_ref ON rules(template_id, template_rule_id);

	CREATE TABLE IF NOT EXISTS rule_templates (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		UNIQUE(name)
	);

	CREATE TABLE IF NOT EXISTS template_rules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		template_id INTEGER NOT NULL,
		position INTEGER NOT NULL DEFAULT 0,
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
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	);
	CREATE INDEX IF NOT EXISTS idx_template_rules_template ON template_rules(template_id, position, id);

	CREATE TABLE IF NOT EXISTS template_attachments (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		template_id INTEGER NOT NULL,
		target_type TEXT NOT NULL DEFAULT '',
		target_ref TEXT NOT NULL DEFAULT '',
		priority INTEGER NOT NULL DEFAULT 100,
		created_at TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT '',
		UNIQUE(template_id, target_type, target_ref)
	);
	CREATE INDEX IF NOT EXISTS idx_template_attachments_target ON template_attachments(target_type, target_ref);

	CREATE TABLE IF NOT EXISTS node_tags (
		node TEXT NOT NULL,
		tag TEXT NOT NULL,
		UNIQUE(node, tag)
	);
	CREATE INDEX IF NOT EXISTS idx_node_tags_tag ON node_tags(tag);

	CREATE TABLE IF NOT EXISTS node_template_sync (
		node TEXT PRIMARY KEY,
		pending INTEGER NOT NULL DEFAULT 0,
		error TEXT NOT NULL DEFAULT '',
		updated_at TEXT NOT NULL DEFAULT ''
	);

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

	CREATE TABLE IF NOT EXISTS seen_flows (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node TEXT NOT NULL DEFAULT '',
		process TEXT NOT NULL DEFAULT '',
		protocol TEXT NOT NULL DEFAULT '',
		dst_port INTEGER NOT NULL DEFAULT 0,
		destination_operand TEXT NOT NULL DEFAULT '',
		destination TEXT NOT NULL DEFAULT '',
		action TEXT NOT NULL DEFAULT '',
		source_rule_name TEXT NOT NULL DEFAULT '',
		first_seen TEXT NOT NULL DEFAULT '',
		last_seen TEXT NOT NULL DEFAULT '',
		expires_at TEXT NOT NULL DEFAULT '',
		count INTEGER NOT NULL DEFAULT 0,
		UNIQUE(node, process, protocol, dst_port, destination_operand, destination)
	);
	CREATE INDEX IF NOT EXISTS idx_seen_flows_last_seen ON seen_flows(last_seen);
	CREATE INDEX IF NOT EXISTS idx_seen_flows_node_action ON seen_flows(node, action);
	CREATE INDEX IF NOT EXISTS idx_seen_flows_source_rule ON seen_flows(node, source_rule_name);

	CREATE TABLE IF NOT EXISTS dns_domains (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node TEXT NOT NULL DEFAULT '',
		domain TEXT NOT NULL DEFAULT '',
		ip TEXT NOT NULL DEFAULT '',
		first_seen TEXT NOT NULL DEFAULT '',
		last_seen TEXT NOT NULL DEFAULT '',
		hit_count INTEGER NOT NULL DEFAULT 1,
		UNIQUE(node, domain, ip)
	);
	CREATE INDEX IF NOT EXISTS idx_dns_domains_domain ON dns_domains(domain);
	CREATE INDEX IF NOT EXISTS idx_dns_domains_ip ON dns_domains(ip);
	CREATE INDEX IF NOT EXISTS idx_dns_domains_node ON dns_domains(node);

	CREATE TABLE IF NOT EXISTS geoip_cache (
		ip TEXT PRIMARY KEY,
		country TEXT NOT NULL DEFAULT '',
		country_code TEXT NOT NULL DEFAULT '',
		city TEXT NOT NULL DEFAULT '',
		lat REAL NOT NULL DEFAULT 0,
		lon REAL NOT NULL DEFAULT 0,
		cached_at TEXT NOT NULL DEFAULT ''
	);

	CREATE TABLE IF NOT EXISTS routers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL DEFAULT '',
		addr TEXT NOT NULL UNIQUE,
		ssh_port INTEGER NOT NULL DEFAULT 22,
		ssh_user TEXT NOT NULL DEFAULT 'root',
		api_key TEXT NOT NULL UNIQUE,
		lan_subnet TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL DEFAULT 'pending',
		created_at TEXT NOT NULL DEFAULT (datetime('now')),
		updated_at TEXT NOT NULL DEFAULT (datetime('now'))
	);
	`

	_, err := d.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	// Legacy-safe column additions for databases created before these columns
	// existed. Must run AFTER CREATE TABLE so tables exist on fresh installs.
	// Errors are ignored — column already exists is expected on newer DBs.
	d.db.Exec("ALTER TABLE nodes ADD COLUMN mode TEXT NOT NULL DEFAULT 'ask'")
	d.db.Exec("ALTER TABLE nodes ADD COLUMN source_type TEXT NOT NULL DEFAULT 'opensnitch'")
	d.db.Exec("ALTER TABLE rules ADD COLUMN operator_json TEXT NOT NULL DEFAULT ''")
	d.db.Exec("ALTER TABLE rules ADD COLUMN display_name TEXT NOT NULL DEFAULT ''")
	d.db.Exec("ALTER TABLE rules ADD COLUMN source_kind TEXT NOT NULL DEFAULT 'manual'")
	d.db.Exec("ALTER TABLE rules ADD COLUMN template_id INTEGER NOT NULL DEFAULT 0")
	d.db.Exec("ALTER TABLE rules ADD COLUMN template_rule_id INTEGER NOT NULL DEFAULT 0")
	d.db.Exec("ALTER TABLE seen_flows ADD COLUMN source_rule_name TEXT NOT NULL DEFAULT ''")
	d.db.Exec("ALTER TABLE seen_flows ADD COLUMN expires_at TEXT NOT NULL DEFAULT ''")

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
