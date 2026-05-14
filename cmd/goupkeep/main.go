package main

import (
	"flag"
	"fmt"
	"go-upkeep/internal/cluster"
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
	flag.Parse()

	var s store.Store
	if *flagDBType == "postgres" {
		s = &store.PostgresStore{ConnStr: *flagDSN}
		fmt.Printf("Using PostgreSQL: %s\n", *flagDSN)
	} else {
		s = &store.SQLiteStore{DBPath: *flagDSN}
		fmt.Printf("Using SQLite: %s\n", *flagDSN)
	}

	if err := s.Init(); err != nil {
		fmt.Printf("Database Init Error: %v\n", err)
		os.Exit(1)
	}
	store.SetGlobal(s)

	if *demo {
		seedDemoData(s)
	}

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
	go func() { s.ListenAndServe() }()
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

	s.AddSite("Google", "https://www.google.com", "http", 30, alertID, true, 14, 2)
	s.AddSite("GitHub", "https://github.com", "http", 30, alertID, true, 7, 3)
	s.AddSite("Cloudflare DNS", "https://1.1.1.1", "http", 60, alertID, false, 7, 1)
	s.AddSite("JSON Placeholder", "https://jsonplaceholder.typicode.com/posts/1", "http", 45, alertID, false, 7, 2)
	s.AddSite("Nonexistent Site", "https://this-domain-does-not-exist-12345.com", "http", 30, alertID, false, 7, 3)
	s.AddSite("Bad Port", "https://localhost:19999", "http", 30, 0, false, 7, 1)
	s.AddSite("Backup Cron", "", "push", 300, alertID, false, 7, 0)
	s.AddSite("DB Healthcheck", "", "push", 120, alertID, false, 7, 0)
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
