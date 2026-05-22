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

	// Declarative config support
	GetSiteByName(name string) (models.Site, error)
	GetAlertByName(name string) (models.AlertConfig, error)
	AddSiteReturningID(site models.Site) (int, error)
	AddAlertReturningID(name, aType string, settings map[string]string) (int, error)

	// Users
	GetAllUsers() ([]models.User, error)
	AddUser(username, publicKey, role string) error
	UpdateUser(id int, username, publicKey, role string) error
	DeleteUser(id int) error

	// History
	SaveCheck(siteID int, latencyNs int64, isUp bool) error
	SaveCheckFromNode(siteID int, nodeID string, latencyNs int64, isUp bool) error
	LoadAllHistory(limit int) (map[int][]models.CheckRecord, error)

	// Nodes
	RegisterNode(node models.ProbeNode) error
	GetNode(id string) (models.ProbeNode, error)
	GetAllNodes() ([]models.ProbeNode, error)
	UpdateNodeLastSeen(id string) error
	DeleteNode(id string) error

	// Logs
	SaveLog(message string) error
	LoadLogs(limit int) ([]string, error)

	// Maintenance Windows
	GetActiveMaintenanceWindows() ([]models.MaintenanceWindow, error)
	GetAllMaintenanceWindows(limit int) ([]models.MaintenanceWindow, error)
	AddMaintenanceWindow(mw models.MaintenanceWindow) error
	EndMaintenanceWindow(id int) error
	DeleteMaintenanceWindow(id int) error
	IsMonitorInMaintenance(monitorID int) (bool, error)

	// Backup & Restore
	ExportData() (models.Backup, error)
	ImportData(data models.Backup) error
}
