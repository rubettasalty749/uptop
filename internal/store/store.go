package store

import (
	"go-upkeep/internal/models"
)

type Store interface {
	Init() error

	// Sites
	GetSites() ([]models.Site, error)
	AddSite(site models.Site) error
	UpdateSite(site models.Site) error
	UpdateSitePaused(id int, paused bool) error
	DeleteSite(id int) error

	// Alerts
	GetAllAlerts() ([]models.AlertConfig, error)
	GetAlert(id int) (models.AlertConfig, error)
	AddAlert(name, aType string, settings map[string]string) error
	UpdateAlert(id int, name, aType string, settings map[string]string) error
	DeleteAlert(id int) error

	// Users
	GetAllUsers() ([]models.User, error)
	AddUser(username, publicKey, role string) error
	UpdateUser(id int, username, publicKey, role string) error
	DeleteUser(id int) error

	// History
	SaveCheck(siteID int, latencyNs int64, isUp bool) error
	LoadAllHistory(limit int) (map[int][]models.CheckRecord, error)

	// Backup & Restore
	ExportData() (models.Backup, error)
	ImportData(data models.Backup) error
}
