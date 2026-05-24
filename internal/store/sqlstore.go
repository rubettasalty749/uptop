package store

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go-upkeep/internal/models"
	"time"
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

func generateToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("crypto/rand failed: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func (s *SQLStore) Close() error {
	return s.db.Close()
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
		"SELECT id, COALESCE(name, url), url, COALESCE(type, 'http'), COALESCE(token, ''), interval, alert_id, check_ssl, threshold, max_retries, COALESCE(hostname, ''), COALESCE(port, 0), COALESCE(timeout, 0), COALESCE(method, 'GET'), COALESCE(description, ''), COALESCE(parent_id, 0), COALESCE(accepted_codes, '200-299'), COALESCE(dns_resolve_type, ''), COALESCE(dns_server, ''), COALESCE(ignore_tls, %s), COALESCE(paused, %s), COALESCE(regions, '') FROM sites",
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
			&st.DNSServer, &st.IgnoreTLS, &st.Paused, &st.Regions); err != nil {
			return sites, err
		}
		sites = append(sites, st)
	}
	return sites, rows.Err()
}

func (s *SQLStore) AddSite(site models.Site) error {
	token := ""
	if site.Type == "push" {
		var err error
		token, err = generateToken()
		if err != nil {
			return fmt.Errorf("generate push token: %w", err)
		}
	}
	_, err := s.db.Exec(s.q("INSERT INTO sites (name, url, type, token, interval, alert_id, check_ssl, threshold, max_retries, hostname, port, timeout, method, description, parent_id, accepted_codes, dns_resolve_type, dns_server, ignore_tls, paused, regions) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"),
		site.Name, site.URL, site.Type, token, site.Interval, site.AlertID, site.CheckSSL, site.ExpiryThreshold, site.MaxRetries,
		site.Hostname, site.Port, site.Timeout, site.Method, site.Description, site.ParentID, site.AcceptedCodes, site.DNSResolveType, site.DNSServer, site.IgnoreTLS, site.Paused, site.Regions)
	return err
}

func (s *SQLStore) UpdateSite(site models.Site) error {
	var existingToken string
	s.db.QueryRow(s.q("SELECT token FROM sites WHERE id=?"), site.ID).Scan(&existingToken)
	if site.Type == "push" && existingToken == "" {
		var err error
		existingToken, err = generateToken()
		if err != nil {
			return fmt.Errorf("generate push token: %w", err)
		}
	}
	_, err := s.db.Exec(s.q("UPDATE sites SET name=?, url=?, type=?, token=?, interval=?, alert_id=?, check_ssl=?, threshold=?, max_retries=?, hostname=?, port=?, timeout=?, method=?, description=?, parent_id=?, accepted_codes=?, dns_resolve_type=?, dns_server=?, ignore_tls=?, paused=?, regions=? WHERE id=?"),
		site.Name, site.URL, site.Type, existingToken, site.Interval, site.AlertID, site.CheckSSL, site.ExpiryThreshold, site.MaxRetries,
		site.Hostname, site.Port, site.Timeout, site.Method, site.Description, site.ParentID, site.AcceptedCodes, site.DNSResolveType, site.DNSServer, site.IgnoreTLS, site.Paused, site.Regions, site.ID)
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
		"SELECT id, COALESCE(name, url), url, COALESCE(type, 'http'), COALESCE(token, ''), interval, alert_id, check_ssl, threshold, max_retries, COALESCE(hostname, ''), COALESCE(port, 0), COALESCE(timeout, 0), COALESCE(method, 'GET'), COALESCE(description, ''), COALESCE(parent_id, 0), COALESCE(accepted_codes, '200-299'), COALESCE(dns_resolve_type, ''), COALESCE(dns_server, ''), COALESCE(ignore_tls, %s), COALESCE(paused, %s), COALESCE(regions, '') FROM sites WHERE name = %s",
		bf, bf, s.q("?"),
	)
	var st models.Site
	err := s.db.QueryRow(query, name).Scan(&st.ID, &st.Name, &st.URL, &st.Type, &st.Token, &st.Interval, &st.AlertID,
		&st.CheckSSL, &st.ExpiryThreshold, &st.MaxRetries, &st.Hostname, &st.Port, &st.Timeout,
		&st.Method, &st.Description, &st.ParentID, &st.AcceptedCodes, &st.DNSResolveType,
		&st.DNSServer, &st.IgnoreTLS, &st.Paused, &st.Regions)
	return st, err
}

func (s *SQLStore) GetAlertByName(name string) (models.AlertConfig, error) {
	var a models.AlertConfig
	var settingsJSON string
	err := s.db.QueryRow(s.q("SELECT id, name, type, settings FROM alerts WHERE name = ?"), name).Scan(&a.ID, &a.Name, &a.Type, &settingsJSON)
	if err != nil {
		return a, err
	}
	if err := json.Unmarshal([]byte(settingsJSON), &a.Settings); err != nil {
		return a, fmt.Errorf("unmarshal alert settings: %w", err)
	}
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
		if err := json.Unmarshal([]byte(settingsJSON), &a.Settings); err != nil {
			return alerts, fmt.Errorf("unmarshal alert settings for %q: %w", a.Name, err)
		}
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
	if err := json.Unmarshal([]byte(settingsJSON), &a.Settings); err != nil {
		return a, fmt.Errorf("unmarshal alert settings: %w", err)
	}
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
	return s.SaveCheckFromNode(siteID, "", latencyNs, isUp)
}

func (s *SQLStore) SaveCheckFromNode(siteID int, nodeID string, latencyNs int64, isUp bool) error {
	_, err := s.db.Exec(s.q("INSERT INTO check_history (site_id, node_id, latency_ns, is_up) VALUES (?, ?, ?, ?)"), siteID, nodeID, latencyNs, isUp)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(s.q(`DELETE FROM check_history WHERE site_id = ? AND id NOT IN (
		SELECT id FROM check_history WHERE site_id = ? ORDER BY checked_at DESC LIMIT 1000
	)`), siteID, siteID)
	return err
}

func (s *SQLStore) RegisterNode(node models.ProbeNode) error {
	_, err := s.db.Exec(s.dialect.UpsertNodeSQL(), node.ID, node.Name, node.Region, node.Version)
	return err
}

func (s *SQLStore) GetNode(id string) (models.ProbeNode, error) {
	var n models.ProbeNode
	err := s.db.QueryRow(s.q("SELECT id, name, region, last_seen, version FROM nodes WHERE id = ?"), id).
		Scan(&n.ID, &n.Name, &n.Region, &n.LastSeen, &n.Version)
	return n, err
}

func (s *SQLStore) GetAllNodes() ([]models.ProbeNode, error) {
	rows, err := s.db.Query("SELECT id, name, region, last_seen, version FROM nodes ORDER BY region, name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var nodes []models.ProbeNode
	for rows.Next() {
		var n models.ProbeNode
		if err := rows.Scan(&n.ID, &n.Name, &n.Region, &n.LastSeen, &n.Version); err != nil {
			return nodes, err
		}
		nodes = append(nodes, n)
	}
	return nodes, rows.Err()
}

func (s *SQLStore) UpdateNodeLastSeen(id string) error {
	_, err := s.db.Exec(s.q("UPDATE nodes SET last_seen = CURRENT_TIMESTAMP WHERE id = ?"), id)
	return err
}

func (s *SQLStore) DeleteNode(id string) error {
	_, err := s.db.Exec(s.q("DELETE FROM nodes WHERE id = ?"), id)
	return err
}

func (s *SQLStore) SaveLog(message string) error {
	_, err := s.db.Exec(s.q("INSERT INTO logs (message) VALUES (?)"), message)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(s.q(`DELETE FROM logs WHERE id NOT IN (
		SELECT id FROM logs ORDER BY created_at DESC LIMIT 200
	)`))
	return err
}

func (s *SQLStore) LoadLogs(limit int) ([]string, error) {
	rows, err := s.db.Query(s.q("SELECT message FROM logs ORDER BY created_at DESC LIMIT ?"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []string
	for rows.Next() {
		var msg string
		if err := rows.Scan(&msg); err != nil {
			return logs, err
		}
		logs = append(logs, msg)
	}
	return logs, rows.Err()
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

func (s *SQLStore) scanMaintenanceWindow(rows *sql.Rows) (models.MaintenanceWindow, error) {
	var mw models.MaintenanceWindow
	var endTime sql.NullTime
	if err := rows.Scan(&mw.ID, &mw.MonitorID, &mw.Title, &mw.Description, &mw.Type, &mw.StartTime, &endTime, &mw.CreatedBy, &mw.CreatedAt); err != nil {
		return mw, err
	}
	if endTime.Valid {
		mw.EndTime = endTime.Time
	}
	return mw, nil
}

func (s *SQLStore) GetActiveMaintenanceWindows() ([]models.MaintenanceWindow, error) {
	rows, err := s.db.Query(s.q("SELECT id, monitor_id, title, description, type, start_time, end_time, created_by, created_at FROM maintenance_windows WHERE start_time <= CURRENT_TIMESTAMP AND (end_time IS NULL OR end_time > CURRENT_TIMESTAMP) ORDER BY start_time DESC"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var windows []models.MaintenanceWindow
	for rows.Next() {
		mw, err := s.scanMaintenanceWindow(rows)
		if err != nil {
			return windows, err
		}
		windows = append(windows, mw)
	}
	return windows, rows.Err()
}

func (s *SQLStore) GetAllMaintenanceWindows(limit int) ([]models.MaintenanceWindow, error) {
	rows, err := s.db.Query(s.q("SELECT id, monitor_id, title, description, type, start_time, end_time, created_by, created_at FROM maintenance_windows ORDER BY created_at DESC LIMIT ?"), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var windows []models.MaintenanceWindow
	for rows.Next() {
		mw, err := s.scanMaintenanceWindow(rows)
		if err != nil {
			return windows, err
		}
		windows = append(windows, mw)
	}
	return windows, rows.Err()
}

func (s *SQLStore) AddMaintenanceWindow(mw models.MaintenanceWindow) error {
	if mw.StartTime.IsZero() {
		mw.StartTime = time.Now()
	}
	_, err := s.db.Exec(s.q("INSERT INTO maintenance_windows (monitor_id, title, description, type, start_time, end_time, created_by) VALUES (?, ?, ?, ?, ?, ?, ?)"),
		mw.MonitorID, mw.Title, mw.Description, mw.Type, mw.StartTime, sql.NullTime{Time: mw.EndTime, Valid: !mw.EndTime.IsZero()}, mw.CreatedBy)
	return err
}

func (s *SQLStore) EndMaintenanceWindow(id int) error {
	_, err := s.db.Exec(s.q("UPDATE maintenance_windows SET end_time = CURRENT_TIMESTAMP WHERE id = ?"), id)
	return err
}

func (s *SQLStore) DeleteMaintenanceWindow(id int) error {
	_, err := s.db.Exec(s.q("DELETE FROM maintenance_windows WHERE id = ?"), id)
	if err != nil {
		return err
	}
	s.dialect.ResetSequenceOnEmpty(s.db, "maintenance_windows")
	return nil
}

func (s *SQLStore) IsMonitorInMaintenance(monitorID int) (bool, error) {
	var count int
	err := s.db.QueryRow(s.q(`SELECT COUNT(*) FROM maintenance_windows
		WHERE type = 'maintenance'
		AND start_time <= CURRENT_TIMESTAMP
		AND (end_time IS NULL OR end_time > CURRENT_TIMESTAMP)
		AND (monitor_id = 0 OR monitor_id = ?
			OR monitor_id IN (SELECT parent_id FROM sites WHERE id = ? AND parent_id > 0))`),
		monitorID, monitorID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (s *SQLStore) GetPreference(key string) (string, error) {
	var value string
	err := s.db.QueryRow(s.q("SELECT value FROM preferences WHERE key = ?"), key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *SQLStore) SetPreference(key, value string) error {
	if s.dollar {
		_, err := s.db.Exec(s.q("INSERT INTO preferences (key, value) VALUES (?, ?) ON CONFLICT (key) DO UPDATE SET value = ?"), key, value, value)
		return err
	}
	_, err := s.db.Exec("INSERT OR REPLACE INTO preferences (key, value) VALUES (?, ?)", key, value)
	return err
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
	windows, err := s.GetAllMaintenanceWindows(1000)
	if err != nil {
		return models.Backup{}, err
	}
	return models.Backup{Sites: sites, Alerts: alerts, Users: users, MaintenanceWindows: windows}, nil
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
		if _, err := tx.Exec(s.q("INSERT INTO sites (id, name, url, type, token, interval, alert_id, check_ssl, threshold, max_retries, hostname, port, timeout, method, description, parent_id, accepted_codes, dns_resolve_type, dns_server, ignore_tls, paused, regions) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)"),
			st.ID, st.Name, st.URL, st.Type, st.Token, st.Interval, st.AlertID, st.CheckSSL, st.ExpiryThreshold, st.MaxRetries,
			st.Hostname, st.Port, st.Timeout, st.Method, st.Description, st.ParentID, st.AcceptedCodes, st.DNSResolveType, st.DNSServer, st.IgnoreTLS, st.Paused, st.Regions); err != nil {
			return err
		}
	}

	for _, mw := range data.MaintenanceWindows {
		if _, err := tx.Exec(s.q("INSERT INTO maintenance_windows (id, monitor_id, title, description, type, start_time, end_time, created_by) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"),
			mw.ID, mw.MonitorID, mw.Title, mw.Description, mw.Type, mw.StartTime, sql.NullTime{Time: mw.EndTime, Valid: !mw.EndTime.IsZero()}, mw.CreatedBy); err != nil {
			return err
		}
	}

	s.dialect.ImportResetSequences(tx)

	return tx.Commit()
}
