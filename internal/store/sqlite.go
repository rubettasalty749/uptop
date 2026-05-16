package store

import (
	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDialect struct{}

func NewSQLiteStore(path string) (*SQLStore, error) {
	return NewSQLStore("sqlite3", path, &SQLiteDialect{})
}

func (d *SQLiteDialect) DriverName() string { return "sqlite3" }
func (d *SQLiteDialect) BoolFalse() string  { return "0" }

func (d *SQLiteDialect) CreateTablesSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS alerts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT, type TEXT, settings TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT DEFAULT 'New Monitor', url TEXT, type TEXT DEFAULT 'http',
			token TEXT, interval INTEGER, alert_id INTEGER,
			check_ssl BOOLEAN DEFAULT 0, threshold INTEGER DEFAULT 7,
			max_retries INTEGER DEFAULT 0, hostname TEXT DEFAULT '',
			port INTEGER DEFAULT 0, timeout INTEGER DEFAULT 0,
			method TEXT DEFAULT 'GET', description TEXT DEFAULT '',
			parent_id INTEGER DEFAULT 0, accepted_codes TEXT DEFAULT '200-299',
			dns_resolve_type TEXT DEFAULT '', dns_server TEXT DEFAULT '',
			ignore_tls BOOLEAN DEFAULT 0, paused BOOLEAN DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL, public_key TEXT NOT NULL,
			role TEXT DEFAULT 'user'
		)`,
		`CREATE TABLE IF NOT EXISTS check_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			site_id INTEGER NOT NULL, latency_ns INTEGER,
			is_up BOOLEAN, checked_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_check_history_site ON check_history(site_id, checked_at DESC)`,
		`CREATE TABLE IF NOT EXISTS nodes (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			region TEXT DEFAULT '',
			last_seen DATETIME DEFAULT CURRENT_TIMESTAMP,
			version TEXT DEFAULT ''
		)`,
	}
}

func (d *SQLiteDialect) MigrationsSQL() []string {
	return []string{
		"ALTER TABLE sites ADD COLUMN hostname TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN port INTEGER DEFAULT 0",
		"ALTER TABLE sites ADD COLUMN timeout INTEGER DEFAULT 0",
		"ALTER TABLE sites ADD COLUMN method TEXT DEFAULT 'GET'",
		"ALTER TABLE sites ADD COLUMN description TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN parent_id INTEGER DEFAULT 0",
		"ALTER TABLE sites ADD COLUMN accepted_codes TEXT DEFAULT '200-299'",
		"ALTER TABLE sites ADD COLUMN dns_resolve_type TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN dns_server TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN ignore_tls BOOLEAN DEFAULT 0",
		"ALTER TABLE sites ADD COLUMN paused BOOLEAN DEFAULT 0",
		"ALTER TABLE check_history ADD COLUMN node_id TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN regions TEXT DEFAULT ''",
	}
}

func (d *SQLiteDialect) UpsertNodeSQL() string {
	return "INSERT OR REPLACE INTO nodes (id, name, region, last_seen, version) VALUES (?, ?, ?, CURRENT_TIMESTAMP, ?)"
}

func (d *SQLiteDialect) ResetSequenceOnEmpty(db *sql.DB, table string) {
	var count int
	db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
	if count == 0 {
		db.Exec("DELETE FROM sqlite_sequence WHERE name=?", table)
	}
}

func (d *SQLiteDialect) ImportWipe(tx *sql.Tx) {
	tx.Exec("DELETE FROM sites")
	tx.Exec("DELETE FROM sqlite_sequence WHERE name='sites'")
	tx.Exec("DELETE FROM alerts")
	tx.Exec("DELETE FROM sqlite_sequence WHERE name='alerts'")
	tx.Exec("DELETE FROM users")
	tx.Exec("DELETE FROM sqlite_sequence WHERE name='users'")
}

func (d *SQLiteDialect) ImportResetSequences(tx *sql.Tx) {}
