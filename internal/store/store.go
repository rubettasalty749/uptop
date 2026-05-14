package store

import (
	"go-upkeep/internal/models"
)

type Store interface {
	Init() error

	// Sites
	GetSites() []models.Site
	AddSite(site models.Site)
	UpdateSite(site models.Site)
	UpdateSitePaused(id int, paused bool)
	DeleteSite(id int)

	// Alerts
	GetAllAlerts() []models.AlertConfig
	GetAlert(id int) (models.AlertConfig, bool)
	AddAlert(name, aType string, settings map[string]string)
	UpdateAlert(id int, name, aType string, settings map[string]string)
	DeleteAlert(id int)

	// Users
	GetAllUsers() []models.User
	AddUser(username, publicKey, role string) error
	UpdateUser(id int, username, publicKey, role string) error
	DeleteUser(id int) error

	// Phase 5: Backup & Restore
	ExportData() models.Backup
	ImportData(data models.Backup) error
}

var Current Store

func SetGlobal(s Store) {
	Current = s
}

func Get() Store {
	return Current
}
