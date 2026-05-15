package store

import (
	"database/sql"
	"encoding/json"
	"go-upkeep/internal/models"

	_ "github.com/lib/pq"
)

type PostgresStore struct {
	ConnStr string
	db      *sql.DB
}

func (p *PostgresStore) Init() error {
	var err error
	p.db, err = sql.Open("postgres", p.ConnStr)
	if err != nil {
		return err
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS alerts (
			id SERIAL PRIMARY KEY,
			name TEXT,
			type TEXT,
			settings TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS sites (
			id SERIAL PRIMARY KEY,
			name TEXT DEFAULT 'New Monitor',
			url TEXT,
			type TEXT DEFAULT 'http',
			token TEXT,
			interval INTEGER,
			alert_id INTEGER,
			check_ssl BOOLEAN DEFAULT FALSE,
			threshold INTEGER DEFAULT 7,
			max_retries INTEGER DEFAULT 0,
			hostname TEXT DEFAULT '',
			port INTEGER DEFAULT 0,
			timeout INTEGER DEFAULT 0,
			method TEXT DEFAULT 'GET',
			description TEXT DEFAULT '',
			parent_id INTEGER DEFAULT 0,
			accepted_codes TEXT DEFAULT '200-299',
			dns_resolve_type TEXT DEFAULT '',
			dns_server TEXT DEFAULT '',
			ignore_tls BOOLEAN DEFAULT FALSE,
			paused BOOLEAN DEFAULT FALSE
		);`,
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			username TEXT NOT NULL,
			public_key TEXT NOT NULL,
			role TEXT DEFAULT 'user'
		);`,
		`CREATE TABLE IF NOT EXISTS check_history (
			id SERIAL PRIMARY KEY,
			site_id INTEGER NOT NULL,
			latency_ns BIGINT,
			is_up BOOLEAN,
			checked_at TIMESTAMP DEFAULT NOW()
		);`,
	}
	for _, q := range queries {
		if _, err := p.db.Exec(q); err != nil {
			return err
		}
	}

	p.db.Exec("CREATE INDEX IF NOT EXISTS idx_check_history_site ON check_history(site_id, checked_at DESC)")

	migrations := []string{
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
	for _, m := range migrations {
		p.db.Exec(m)
	}

	return nil
}

// ... [CRUD Methods are identical to Phase 4, keeping them concise here] ...
func (p *PostgresStore) GetSites() []models.Site {
	rows, err := p.db.Query("SELECT id, COALESCE(name, url), url, COALESCE(type, 'http'), COALESCE(token, ''), interval, alert_id, check_ssl, threshold, max_retries, COALESCE(hostname, ''), COALESCE(port, 0), COALESCE(timeout, 0), COALESCE(method, 'GET'), COALESCE(description, ''), COALESCE(parent_id, 0), COALESCE(accepted_codes, '200-299'), COALESCE(dns_resolve_type, ''), COALESCE(dns_server, ''), COALESCE(ignore_tls, FALSE), COALESCE(paused, FALSE) FROM sites")
	if err != nil {
		return []models.Site{}
	}
	defer rows.Close()
	var sites []models.Site
	for rows.Next() {
		var s models.Site
		rows.Scan(&s.ID, &s.Name, &s.URL, &s.Type, &s.Token, &s.Interval, &s.AlertID, &s.CheckSSL, &s.ExpiryThreshold, &s.MaxRetries,
			&s.Hostname, &s.Port, &s.Timeout, &s.Method, &s.Description, &s.ParentID, &s.AcceptedCodes, &s.DNSResolveType, &s.DNSServer, &s.IgnoreTLS, &s.Paused)
		sites = append(sites, s)
	}
	return sites
}
func (p *PostgresStore) AddSite(site models.Site) {
	token := ""
	if site.Type == "push" {
		token = generateToken()
	}
	p.db.Exec("INSERT INTO sites (name, url, type, token, interval, alert_id, check_ssl, threshold, max_retries, hostname, port, timeout, method, description, parent_id, accepted_codes, dns_resolve_type, dns_server, ignore_tls, paused) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)",
		site.Name, site.URL, site.Type, token, site.Interval, site.AlertID, site.CheckSSL, site.ExpiryThreshold, site.MaxRetries,
		site.Hostname, site.Port, site.Timeout, site.Method, site.Description, site.ParentID, site.AcceptedCodes, site.DNSResolveType, site.DNSServer, site.IgnoreTLS, site.Paused)
}
func (p *PostgresStore) UpdateSite(site models.Site) {
	var existingToken string
	p.db.QueryRow("SELECT token FROM sites WHERE id=$1", site.ID).Scan(&existingToken)
	if site.Type == "push" && existingToken == "" {
		existingToken = generateToken()
	}
	p.db.Exec("UPDATE sites SET name=$1, url=$2, type=$3, token=$4, interval=$5, alert_id=$6, check_ssl=$7, threshold=$8, max_retries=$9, hostname=$10, port=$11, timeout=$12, method=$13, description=$14, parent_id=$15, accepted_codes=$16, dns_resolve_type=$17, dns_server=$18, ignore_tls=$19, paused=$20 WHERE id=$21",
		site.Name, site.URL, site.Type, existingToken, site.Interval, site.AlertID, site.CheckSSL, site.ExpiryThreshold, site.MaxRetries,
		site.Hostname, site.Port, site.Timeout, site.Method, site.Description, site.ParentID, site.AcceptedCodes, site.DNSResolveType, site.DNSServer, site.IgnoreTLS, site.Paused, site.ID)
}
func (p *PostgresStore) UpdateSitePaused(id int, paused bool) {
	p.db.Exec("UPDATE sites SET paused=$1 WHERE id=$2", paused, id)
}
func (p *PostgresStore) DeleteSite(id int) { p.db.Exec("DELETE FROM sites WHERE id=$1", id) }
func (p *PostgresStore) GetAllAlerts() []models.AlertConfig {
	rows, err := p.db.Query("SELECT id, name, type, settings FROM alerts")
	if err != nil {
		return []models.AlertConfig{}
	}
	defer rows.Close()
	var alerts []models.AlertConfig
	for rows.Next() {
		var a models.AlertConfig
		var settingsJSON string
		rows.Scan(&a.ID, &a.Name, &a.Type, &settingsJSON)
		json.Unmarshal([]byte(settingsJSON), &a.Settings)
		alerts = append(alerts, a)
	}
	return alerts
}
func (p *PostgresStore) GetAlert(id int) (models.AlertConfig, bool) {
	var a models.AlertConfig
	var settingsJSON string
	err := p.db.QueryRow("SELECT id, name, type, settings FROM alerts WHERE id = $1", id).Scan(&a.ID, &a.Name, &a.Type, &settingsJSON)
	if err != nil {
		return a, false
	}
	json.Unmarshal([]byte(settingsJSON), &a.Settings)
	return a, true
}
func (p *PostgresStore) AddAlert(name, aType string, settings map[string]string) {
	jsonBytes, _ := json.Marshal(settings)
	p.db.Exec("INSERT INTO alerts (name, type, settings) VALUES ($1, $2, $3)", name, aType, string(jsonBytes))
}
func (p *PostgresStore) UpdateAlert(id int, name, aType string, settings map[string]string) {
	jsonBytes, _ := json.Marshal(settings)
	p.db.Exec("UPDATE alerts SET name=$1, type=$2, settings=$3 WHERE id=$4", name, aType, string(jsonBytes), id)
}
func (p *PostgresStore) DeleteAlert(id int) { p.db.Exec("DELETE FROM alerts WHERE id=$1", id) }
func (p *PostgresStore) GetAllUsers() []models.User {
	rows, err := p.db.Query("SELECT id, username, public_key, role FROM users")
	if err != nil {
		return []models.User{}
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		rows.Scan(&u.ID, &u.Username, &u.PublicKey, &u.Role)
		users = append(users, u)
	}
	return users
}
func (p *PostgresStore) AddUser(username, publicKey, role string) error {
	_, err := p.db.Exec("INSERT INTO users (username, public_key, role) VALUES ($1, $2, $3)", username, publicKey, role)
	return err
}
func (p *PostgresStore) UpdateUser(id int, username, publicKey, role string) error {
	_, err := p.db.Exec("UPDATE users SET username=$1, public_key=$2, role=$3 WHERE id=$4", username, publicKey, role, id)
	return err
}
func (p *PostgresStore) DeleteUser(id int) error {
	_, err := p.db.Exec("DELETE FROM users WHERE id=$1", id)
	return err
}

func (p *PostgresStore) SaveCheck(siteID int, latencyNs int64, isUp bool) {
	p.db.Exec("INSERT INTO check_history (site_id, latency_ns, is_up) VALUES ($1, $2, $3)", siteID, latencyNs, isUp)
	p.db.Exec(`DELETE FROM check_history WHERE site_id = $1 AND id NOT IN (
		SELECT id FROM check_history WHERE site_id = $1 ORDER BY checked_at DESC LIMIT 1000
	)`, siteID)
}

func (p *PostgresStore) LoadAllHistory(limit int) map[int][]models.CheckRecord {
	result := make(map[int][]models.CheckRecord)
	rows, err := p.db.Query(`
		SELECT site_id, latency_ns, is_up FROM (
			SELECT site_id, latency_ns, is_up,
				ROW_NUMBER() OVER (PARTITION BY site_id ORDER BY checked_at DESC) AS rn
			FROM check_history
		) sub WHERE rn <= $1`, limit)
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var r models.CheckRecord
		rows.Scan(&r.SiteID, &r.LatencyNs, &r.IsUp)
		result[r.SiteID] = append(result[r.SiteID], r)
	}
	for id, records := range result {
		for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
			records[i], records[j] = records[j], records[i]
		}
		result[id] = records
	}
	return result
}

func (p *PostgresStore) ExportData() models.Backup {
	return models.Backup{
		Sites:  p.GetSites(),
		Alerts: p.GetAllAlerts(),
		Users:  p.GetAllUsers(),
	}
}

func (p *PostgresStore) ImportData(data models.Backup) error {
	tx, err := p.db.Begin()
	if err != nil {
		return err
	}

	tx.Exec("TRUNCATE TABLE sites RESTART IDENTITY CASCADE")
	tx.Exec("TRUNCATE TABLE alerts RESTART IDENTITY CASCADE")
	tx.Exec("TRUNCATE TABLE users RESTART IDENTITY CASCADE")

	for _, u := range data.Users {
		tx.Exec("INSERT INTO users (username, public_key, role) VALUES ($1, $2, $3)", u.Username, u.PublicKey, u.Role)
	}
	for _, a := range data.Alerts {
		jsonBytes, _ := json.Marshal(a.Settings)
		tx.Exec("INSERT INTO alerts (id, name, type, settings) VALUES ($1, $2, $3, $4)", a.ID, a.Name, a.Type, string(jsonBytes))
	}
	for _, st := range data.Sites {
		tx.Exec("INSERT INTO sites (id, name, url, type, token, interval, alert_id, check_ssl, threshold, max_retries, hostname, port, timeout, method, description, parent_id, accepted_codes, dns_resolve_type, dns_server, ignore_tls, paused) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)",
			st.ID, st.Name, st.URL, st.Type, st.Token, st.Interval, st.AlertID, st.CheckSSL, st.ExpiryThreshold, st.MaxRetries,
			st.Hostname, st.Port, st.Timeout, st.Method, st.Description, st.ParentID, st.AcceptedCodes, st.DNSResolveType, st.DNSServer, st.IgnoreTLS, st.Paused)
	}

	tx.Exec("SELECT setval('sites_id_seq', (SELECT MAX(id) FROM sites))")
	tx.Exec("SELECT setval('alerts_id_seq', (SELECT MAX(id) FROM alerts))")
	tx.Exec("SELECT setval('users_id_seq', (SELECT MAX(id) FROM users))")

	return tx.Commit()
}
