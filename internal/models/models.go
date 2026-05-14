package models

import "time"

type Site struct {
	ID              int
	Name            string
	URL             string
	Type            string // "http", "push", "ping", "port", "dns", "group"
	Token           string
	Interval        int
	AlertID         int
	CheckSSL        bool
	ExpiryThreshold int
	MaxRetries      int

	Hostname       string
	Port           int
	Timeout        int
	Method         string
	Description    string
	ParentID       int
	AcceptedCodes  string
	DNSResolveType string
	DNSServer      string
	IgnoreTLS      bool
	Paused         bool

	FailureCount   int
	Status         string
	StatusCode     int
	Latency        time.Duration
	CertExpiry     time.Time
	HasSSL         bool
	LastCheck      time.Time
	SentSSLWarning bool
}

type AlertConfig struct {
	ID       int
	Name     string
	Type     string
	Settings map[string]string
}

type User struct {
	ID        int
	Username  string
	PublicKey string
	Role      string
}

// Phase 5: Backup Structure
type Backup struct {
	Sites  []Site        `json:"sites"`
	Alerts []AlertConfig `json:"alerts"`
	Users  []User        `json:"users"`
}
