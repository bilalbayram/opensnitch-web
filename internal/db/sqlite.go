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
	`

	_, err := d.db.Exec(schema)
	if err != nil {
		return fmt.Errorf("execute schema: %w", err)
	}

	log.Println("[db] Schema migration completed")
	return nil
}
