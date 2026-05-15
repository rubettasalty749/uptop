package store

import (
	"database/sql"

	_ "github.com/lib/pq"
)

type PostgresDialect struct{}

func NewPostgresStore(connStr string) (*SQLStore, error) {
	return NewSQLStore("postgres", connStr, &PostgresDialect{})
}

func (d *PostgresDialect) DriverName() string { return "postgres" }
func (d *PostgresDialect) BoolFalse() string  { return "FALSE" }

func (d *PostgresDialect) CreateTablesSQL() []string {
	return []string{
		`CREATE TABLE IF NOT EXISTS alerts (
			id SERIAL PRIMARY KEY,
			name TEXT, type TEXT, settings TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS sites (
			id SERIAL PRIMARY KEY,
			name TEXT DEFAULT 'New Monitor', url TEXT, type TEXT DEFAULT 'http',
			token TEXT, interval INTEGER, alert_id INTEGER,
			check_ssl BOOLEAN DEFAULT FALSE, threshold INTEGER DEFAULT 7,
			max_retries INTEGER DEFAULT 0, hostname TEXT DEFAULT '',
			port INTEGER DEFAULT 0, timeout INTEGER DEFAULT 0,
			method TEXT DEFAULT 'GET', description TEXT DEFAULT '',
			parent_id INTEGER DEFAULT 0, accepted_codes TEXT DEFAULT '200-299',
			dns_resolve_type TEXT DEFAULT '', dns_server TEXT DEFAULT '',
			ignore_tls BOOLEAN DEFAULT FALSE, paused BOOLEAN DEFAULT FALSE
		)`,
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL, public_key TEXT NOT NULL,
			role TEXT DEFAULT 'user'
		)`,
		`CREATE TABLE IF NOT EXISTS check_history (
			id SERIAL PRIMARY KEY,
			site_id INTEGER NOT NULL, latency_ns BIGINT,
			is_up BOOLEAN, checked_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_check_history_site ON check_history(site_id, checked_at DESC)`,
	}
}

func (d *PostgresDialect) MigrationsSQL() []string {
	return []string{
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS hostname TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS port INTEGER DEFAULT 0",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS timeout INTEGER DEFAULT 0",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS method TEXT DEFAULT 'GET'",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS description TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS parent_id INTEGER DEFAULT 0",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS accepted_codes TEXT DEFAULT '200-299'",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS dns_resolve_type TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS dns_server TEXT DEFAULT ''",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS ignore_tls BOOLEAN DEFAULT FALSE",
		"ALTER TABLE sites ADD COLUMN IF NOT EXISTS paused BOOLEAN DEFAULT FALSE",
	}
}

func (d *PostgresDialect) ResetSequenceOnEmpty(db *sql.DB, table string) {}

func (d *PostgresDialect) ImportWipe(tx *sql.Tx) {
	tx.Exec("TRUNCATE TABLE sites RESTART IDENTITY CASCADE")
	tx.Exec("TRUNCATE TABLE alerts RESTART IDENTITY CASCADE")
	tx.Exec("TRUNCATE TABLE users RESTART IDENTITY CASCADE")
}

func (d *PostgresDialect) ImportResetSequences(tx *sql.Tx) {
	tx.Exec("SELECT setval('sites_id_seq', (SELECT COALESCE(MAX(id), 1) FROM sites))")
	tx.Exec("SELECT setval('alerts_id_seq', (SELECT COALESCE(MAX(id), 1) FROM alerts))")
	tx.Exec("SELECT setval('users_id_seq', (SELECT COALESCE(MAX(id), 1) FROM users))")
}
