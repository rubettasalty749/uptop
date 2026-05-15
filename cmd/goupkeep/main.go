package main

import (
	"flag"
	"fmt"
	"go-upkeep/internal/cluster"
	"go-upkeep/internal/importer"
	"go-upkeep/internal/models"
	"go-upkeep/internal/monitor"
	"go-upkeep/internal/server"
	"go-upkeep/internal/store"
	"go-upkeep/internal/tui"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/mattn/go-isatty"
)

func main() {
	log.SetOutput(os.Stderr)

	portVal := 23234
	dbType := "sqlite"
	dbDSN := "upkeep.db"
	httpPort := 8080
	enableStatus := false
	statusTitle := "System Status"
	clusterMode := "leader"
	clusterPeer := ""
	clusterKey := ""

	if v := os.Getenv("UPKEEP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			portVal = p
		}
	}
	if v := os.Getenv("UPKEEP_DB_TYPE"); v != "" {
		dbType = v
	}
	if v := os.Getenv("UPKEEP_DB_DSN"); v != "" {
		dbDSN = v
	}
	if v := os.Getenv("UPKEEP_HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			httpPort = p
		}
	}
	if v := os.Getenv("UPKEEP_STATUS_ENABLED"); v == "true" {
		enableStatus = true
	}
	if v := os.Getenv("UPKEEP_STATUS_TITLE"); v != "" {
		statusTitle = v
	}

	if v := os.Getenv("UPKEEP_CLUSTER_MODE"); v != "" {
		clusterMode = v
	}
	if v := os.Getenv("UPKEEP_PEER_URL"); v != "" {
		clusterPeer = v
	}
	if v := os.Getenv("UPKEEP_CLUSTER_SECRET"); v != "" {
		clusterKey = v
	}
	if os.Getenv("UPKEEP_INSECURE_SKIP_VERIFY") == "true" {
		monitor.SetInsecureSkipVerify(true)
	}

	port := flag.Int("port", portVal, "SSH Port")
	flagDBType := flag.String("db-type", dbType, "Database type")
	flagDSN := flag.String("dsn", dbDSN, "Database DSN")
	demo := flag.Bool("demo", false, "Seed demo data")
	importKuma := flag.String("import-kuma", "", "Import Uptime Kuma backup JSON file")
	flag.Parse()

	var s store.Store
	var dbErr error
	if *flagDBType == "postgres" {
		s, dbErr = store.NewPostgresStore(*flagDSN)
		fmt.Printf("Using PostgreSQL: %s\n", *flagDSN)
	} else {
		s, dbErr = store.NewSQLiteStore(*flagDSN)
		fmt.Printf("Using SQLite: %s\n", *flagDSN)
	}
	if dbErr != nil {
		fmt.Printf("Database connection error: %v\n", dbErr)
		os.Exit(1)
	}

	if err := s.Init(); err != nil {
		fmt.Printf("Database init error: %v\n", err)
		os.Exit(1)
	}
	store.SetGlobal(s)

	if *demo {
		seedDemoData(s)
	}

	if *importKuma != "" {
		kb, err := importer.LoadKumaFile(*importKuma)
		if err != nil {
			fmt.Printf("Kuma import error: %v\n", err)
			os.Exit(1)
		}
		backup := importer.ConvertKuma(kb)
		if err := s.ImportData(backup); err != nil {
			fmt.Printf("Import failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Imported %d monitors and %d alerts from Uptime Kuma v%s\n", len(backup.Sites), len(backup.Alerts), kb.Version)
	}

	monitor.InitHistoryFromStore()
	monitor.StartEngine()

	server.Start(server.ServerConfig{
		Port:         httpPort,
		EnableStatus: enableStatus,
		Title:        statusTitle,
		ClusterKey:   clusterKey,
	})

	cluster.Start(cluster.Config{
		Mode:      clusterMode,
		PeerURL:   clusterPeer,
		SharedKey: clusterKey,
	})

	startSSHServer(*port)

	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		p := tea.NewProgram(tui.InitialModel(true), tea.WithAltScreen(), tea.WithMouseCellMotion())
		if _, err := p.Run(); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	} else {
		fmt.Println("Go-Upkeep running in HEADLESS mode")
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		<-done
		fmt.Println("Shutting down...")
	}
}

func startSSHServer(port int) {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			return isKeyAllowed(key)
		}),
		wish.WithMiddleware(
			bm.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				return tui.InitialModel(false), []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
			}),
		),
	)
	if err != nil {
		fmt.Printf("SSH server error: %v\n", err)
		return
	}
	go func() {
		if err := s.ListenAndServe(); err != nil {
			log.Fatalf("SSH server failed: %v", err)
		}
	}()
}

func seedDemoData(s store.Store) {
	if existing := s.GetSites(); len(existing) > 0 {
		return
	}
	fmt.Println("Seeding demo data...")

	s.AddAlert("Discord Ops", "discord", map[string]string{"url": "https://discord.com/api/webhooks/demo/token"})
	s.AddAlert("Slack Infra", "slack", map[string]string{"url": "https://hooks.slack.com/services/DEMO/WEBHOOK"})
	s.AddAlert("Email Oncall", "email", map[string]string{
		"host": "smtp.example.com", "port": "587",
		"user": "oncall@example.com", "pass": "replace-me",
		"from": "oncall@example.com", "to": "team@example.com",
	})

	alerts := s.GetAllAlerts()
	alertID := 0
	if len(alerts) > 0 {
		alertID = alerts[0].ID
	}

	s.AddSite(models.Site{Name: "Google", URL: "https://www.google.com", Type: "http", Interval: 30, AlertID: alertID, CheckSSL: true, ExpiryThreshold: 14, MaxRetries: 2})
	s.AddSite(models.Site{Name: "GitHub", URL: "https://github.com", Type: "http", Interval: 30, AlertID: alertID, CheckSSL: true, ExpiryThreshold: 7, MaxRetries: 3})
	s.AddSite(models.Site{Name: "Cloudflare DNS", URL: "https://1.1.1.1", Type: "http", Interval: 60, AlertID: alertID, ExpiryThreshold: 7, MaxRetries: 1})
	s.AddSite(models.Site{Name: "JSON Placeholder", URL: "https://jsonplaceholder.typicode.com/posts/1", Type: "http", Interval: 45, AlertID: alertID, ExpiryThreshold: 7, MaxRetries: 2})
	s.AddSite(models.Site{Name: "Nonexistent Site", URL: "https://this-domain-does-not-exist-12345.com", Type: "http", Interval: 30, AlertID: alertID, ExpiryThreshold: 7, MaxRetries: 3})
	s.AddSite(models.Site{Name: "Bad Port", URL: "https://localhost:19999", Type: "http", Interval: 30, ExpiryThreshold: 7, MaxRetries: 1})
	s.AddSite(models.Site{Name: "Backup Cron", Type: "push", Interval: 300, AlertID: alertID, ExpiryThreshold: 7})
	s.AddSite(models.Site{Name: "DB Healthcheck", Type: "push", Interval: 120, AlertID: alertID, ExpiryThreshold: 7})
	s.AddSite(models.Site{Name: "Gateway", Type: "ping", Interval: 30, AlertID: alertID, Hostname: "10.0.0.1", Timeout: 5, ExpiryThreshold: 7})
	s.AddSite(models.Site{Name: "SSH Server", Type: "port", Interval: 60, AlertID: alertID, Hostname: "10.0.0.1", Port: 22, Timeout: 5, ExpiryThreshold: 7})
}

func isKeyAllowed(incomingKey ssh.PublicKey) bool {
	users := store.Get().GetAllUsers()
	for _, u := range users {
		allowedKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(u.PublicKey))
		if err != nil {
			continue
		}
		if ssh.KeysEqual(allowedKey, incomingKey) {
			return true
		}
	}
	return false
}
