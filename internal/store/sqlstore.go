package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-upkeep/internal/models"
)

type SQLStore struct {
	db      *sql.DB
	dialect Dialect
	dollar  bool
}

func NewSQLStore(driverName, dsn string, dialect Dialect) (*SQLStore, error) {
	db, err := sql.Open(driverName, dsn)
	if err != nil {
		return nil, err
	}
	_, isDollar := dialect.(*PostgresDialect)
	return &SQLStore{db: db, dialect: dialect, dollar: isDollar}, nil
}

func (s *SQLStore) q(query string) string {
	return rewritePlaceholders(query, s.dollar)
}

func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func (s *SQLStore) Init() error {
	for _, stmt := range s.dialect.CreateTablesSQL() {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	for _, m := range s.dialect.MigrationsSQL() {
		s.db.Exec(m)
	}
	return nil
}

func (s *SQLStore) GetSites() ([]models.Site, error) {
	bf := s.dialect.BoolFalse()
	query := fmt.Sprintf(
		"SELECT id, COALESCE(name, url), url, COALESCE(type, 'http'), COALESCE(token, ''), interval, alert_id, check_ssl, threshold, max_retries, COALESCE(hostname, ''), COALESCE(port, 0), COALESCE(timeout, 0), COALESCE(method, 'GET'), COALESCE(description, ''), COALESCE(parent_id, 0), COALESCE(accepted_codes, '200-299'), COALESCE(dns_resolve_type, ''), COALESCE(dns_server, ''), COALESCE(ignore_tls, %s), COALESCE(paused, %s) FROM sites",
		bf, bf,
	)
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sites []models.Site
	for rows.Next() {
		var st models.Site
		if err := rows.Scan(&st.ID, &st.Name, &st.URL, &st.Type, &st.Token, &st.Interval, &st.AlertID,
			&st.CheckSSL, &st.ExpiryThreshold, &st.MaxRetries, &st.Hostname, &st.Port, &st.Timeout,
			&st.Method, &st.Description, &st.ParentID, &st.AcceptedCodes, &st.DNSResolveType,
			&st.DNSServer, &st.IgnoreTLS, &st.Paused); err != nil {
			return sites, err
		}
		sites = append(sites, st)
	}
	return sites, rows.Err()
}

func (s *SQLStore) AddSite(site models.Site) error {
	token := ""
	if site.Type == "push" {
		token = generateToken()
	}
	_, err := s.db.Exec(s.q("INSERT INTO sites (name, url, type, token, interval, alert_id, check_ssl, threshold, max_retries, hostname, port, timeout, method, description, parent_id, accepted_codes, dns_resolve_type, dns_server, ignore_tls, paused) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"),
		site.Name, site.URL, site.Type, token, site.Interval, site.AlertID, site.CheckSSL, site.ExpiryThreshold, site.MaxRetries,
		site.Hostname, site.Port, site.Timeout, site.Method, site.Description, site.ParentID, site.AcceptedCodes, site.DNSResolveType, site.DNSServer, site.IgnoreTLS, site.Paused)
	return err
}

func (s *SQLStore) UpdateSite(site models.Site) error {
	var existingToken string
	s.db.QueryRow(s.q("SELECT token FROM sites WHERE id=?"), site.ID).Scan(&existingToken)
	if site.Type == "push" && existingToken == "" {
		existingToken = generateToken()
	}
	_, err := s.db.Exec(s.q("UPDATE sites SET name=?, url=?, type=?, token=?, interval=?, alert_id=?, check_ssl=?, threshold=?, max_retries=?, hostname=?, port=?, timeout=?, method=?, description=?, parent_id=?, accepted_codes=?, dns_resolve_type=?, dns_server=?, ignore_tls=?, paused=? WHERE id=?"),
		site.Name, site.URL, site.Type, existingToken, site.Interval, site.AlertID, site.CheckSSL, site.ExpiryThreshold, site.MaxRetries,
		site.Hostname, site.Port, site.Timeout, site.Method, site.Description, site.ParentID, site.AcceptedCodes, site.DNSResolveType, site.DNSServer, site.IgnoreTLS, site.Paused, site.ID)
	return err
}

func (s *SQLStore) UpdateSitePaused(id int, paused bool) error {
	_, err := s.db.Exec(s.q("UPDATE sites SET paused=? WHERE id=?"), paused, id)
	return err
}

func (s *SQLStore) DeleteSite(id int) error {
	_, err := s.db.Exec(s.q("DELETE FROM sites WHERE id=?"), id)
	if err != nil {
		return err
	}
	s.dialect.ResetSequenceOnEmpty(s.db, "sites")
	return nil
}

func (s *SQLStore) GetSiteByName(name string) (models.Site, error) {
	bf := s.dialect.BoolFalse()
	query := fmt.Sprintf(
		"SELECT id, COALESCE(name, url), url, COALESCE(type, 'http'), COALESCE(token, ''), interval, alert_id, check_ssl, threshold, max_retries, COALESCE(hostname, ''), COALESCE(port, 0), COALESCE(timeout, 0), COALESCE(method, 'GET'), COALESCE(description, ''), COALESCE(parent_id, 0), COALESCE(accepted_codes, '200-299'), COALESCE(dns_resolve_type, ''), COALESCE(dns_server, ''), COALESCE(ignore_tls, %s), COALESCE(paused, %s) FROM sites WHERE name = %s",
		bf, bf, s.q("?"),
	)
	var st models.Site
	err := s.db.QueryRow(query, name).Scan(&st.ID, &st.Name, &st.URL, &st.Type, &st.Token, &st.Interval, &st.AlertID,
		&st.CheckSSL, &st.ExpiryThreshold, &st.MaxRetries, &st.Hostname, &st.Port, &st.Timeout,
		&st.Method, &st.Description, &st.ParentID, &st.AcceptedCodes, &st.DNSResolveType,
		&st.DNSServer, &st.IgnoreTLS, &st.Paused)
	return st, err
}

func (s *SQLStore) GetAlertByName(name string) (models.AlertConfig, error) {
	var a models.AlertConfig
	var settingsJSON string
	err := s.db.QueryRow(s.q("SELECT id, name, type, settings FROM alerts WHERE name = ?"), name).Scan(&a.ID, &a.Name, &a.Type, &settingsJSON)
	if err != nil {
		return a, err
	}
	json.Unmarshal([]byte(settingsJSON), &a.Settings)
	return a, nil
}

func (s *SQLStore) AddSiteReturningID(site models.Site) (int, error) {
	if err := s.AddSite(site); err != nil {
		return 0, err
	}
	created, err := s.GetSiteByName(site.Name)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (s *SQLStore) AddAlertReturningID(name, aType string, settings map[string]string) (int, error) {
	if err := s.AddAlert(name, aType, settings); err != nil {
		return 0, err
	}
	created, err := s.GetAlertByName(name)
	if err != nil {
		return 0, err
	}
	return created.ID, nil
}

func (s *SQLStore) GetAllAlerts() ([]models.AlertConfig, error) {
	rows, err := s.db.Query("SELECT id, name, type, settings FROM alerts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []models.AlertConfig
	for rows.Next() {
		var a models.AlertConfig
		var settingsJSON string
		if err := rows.Scan(&a.ID, &a.Name, &a.Type, &settingsJSON); err != nil {
			return alerts, err
		}
		json.Unmarshal([]byte(settingsJSON), &a.Settings)
		alerts = append(alerts, a)
	}
	return alerts, rows.Err()
}

func (s *SQLStore) GetAlert(id int) (models.AlertConfig, error) {
	var a models.AlertConfig
	var settingsJSON string
	err := s.db.QueryRow(s.q("SELECT id, name, type, settings FROM alerts WHERE id = ?"), id).Scan(&a.ID, &a.Name, &a.Type, &settingsJSON)
	if err != nil {
		return a, err
	}
	json.Unmarshal([]byte(settingsJSON), &a.Settings)
	return a, nil
}

func (s *SQLStore) AddAlert(name, aType string, settings map[string]string) error {
	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(s.q("INSERT INTO alerts (name, type, settings) VALUES (?, ?, ?)"), name, aType, string(jsonBytes))
	return err
}

func (s *SQLStore) UpdateAlert(id int, name, aType string, settings map[string]string) error {
	jsonBytes, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(s.q("UPDATE alerts SET name=?, type=?, settings=? WHERE id=?"), name, aType, string(jsonBytes), id)
	return err
}

func (s *SQLStore) DeleteAlert(id int) error {
	_, err := s.db.Exec(s.q("DELETE FROM alerts WHERE id=?"), id)
	if err != nil {
		return err
	}
	s.dialect.ResetSequenceOnEmpty(s.db, "alerts")
	return nil
}

func (s *SQLStore) GetAllUsers() ([]models.User, error) {
	rows, err := s.db.Query("SELECT id, username, public_key, role FROM users")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.PublicKey, &u.Role); err != nil {
			return users, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (s *SQLStore) AddUser(username, publicKey, role string) error {
	_, err := s.db.Exec(s.q("INSERT INTO users (username, public_key, role) VALUES (?, ?, ?)"), username, publicKey, role)
	return err
}

func (s *SQLStore) UpdateUser(id int, username, publicKey, role string) error {
	_, err := s.db.Exec(s.q("UPDATE users SET username=?, public_key=?, role=? WHERE id=?"), username, publicKey, role, id)
	return err
}

func (s *SQLStore) DeleteUser(id int) error {
	_, err := s.db.Exec(s.q("DELETE FROM users WHERE id=?"), id)
	return err
}

func (s *SQLStore) SaveCheck(siteID int, latencyNs int64, isUp bool) error {
	_, err := s.db.Exec(s.q("INSERT INTO check_history (site_id, latency_ns, is_up) VALUES (?, ?, ?)"), siteID, latencyNs, isUp)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(s.q(`DELETE FROM check_history WHERE site_id = ? AND id NOT IN (
		SELECT id FROM check_history WHERE site_id = ? ORDER BY checked_at DESC LIMIT 1000
	)`), siteID, siteID)
	return err
}

func (s *SQLStore) LoadAllHistory(limit int) (map[int][]models.CheckRecord, error) {
	result := make(map[int][]models.CheckRecord)
	rows, err := s.db.Query(s.q(`
		SELECT site_id, latency_ns, is_up FROM (
			SELECT site_id, latency_ns, is_up,
				ROW_NUMBER() OVER (PARTITION BY site_id ORDER BY checked_at DESC) AS rn
			FROM check_history
		) sub WHERE rn <= ?`), limit)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var r models.CheckRecord
		if err := rows.Scan(&r.SiteID, &r.LatencyNs, &r.IsUp); err != nil {
			return result, err
		}
		result[r.SiteID] = append(result[r.SiteID], r)
	}
	for id, records := range result {
		for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
			records[i], records[j] = records[j], records[i]
		}
		result[id] = records
	}
	return result, rows.Err()
}

func (s *SQLStore) ExportData() (models.Backup, error) {
	sites, err := s.GetSites()
	if err != nil {
		return models.Backup{}, err
	}
	alerts, err := s.GetAllAlerts()
	if err != nil {
		return models.Backup{}, err
	}
	users, err := s.GetAllUsers()
	if err != nil {
		return models.Backup{}, err
	}
	return models.Backup{Sites: sites, Alerts: alerts, Users: users}, nil
}

func (s *SQLStore) ImportData(data models.Backup) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	s.dialect.ImportWipe(tx)

	for _, u := range data.Users {
		if _, err := tx.Exec(s.q("INSERT INTO users (username, public_key, role) VALUES (?, ?, ?)"), u.Username, u.PublicKey, u.Role); err != nil {
			return err
		}
	}
	for _, a := range data.Alerts {
		jsonBytes, err := json.Marshal(a.Settings)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(s.q("INSERT INTO alerts (id, name, type, settings) VALUES (?, ?, ?, ?)"), a.ID, a.Name, a.Type, string(jsonBytes)); err != nil {
			return err
		}
	}
	for _, st := range data.Sites {
		if _, err := tx.Exec(s.q("INSERT INTO sites (id, name, url, type, token, interval, alert_id, check_ssl, threshold, max_retries, hostname, port, timeout, method, description, parent_id, accepted_codes, dns_resolve_type, dns_server, ignore_tls, paused) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"),
			st.ID, st.Name, st.URL, st.Type, st.Token, st.Interval, st.AlertID, st.CheckSSL, st.ExpiryThreshold, st.MaxRetries,
			st.Hostname, st.Port, st.Timeout, st.Method, st.Description, st.ParentID, st.AcceptedCodes, st.DNSResolveType, st.DNSServer, st.IgnoreTLS, st.Paused); err != nil {
			return err
		}
	}

	s.dialect.ImportResetSequences(tx)

	return tx.Commit()
}
