package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"go-upkeep/internal/cluster"
	"go-upkeep/internal/config"
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
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	bm "github.com/charmbracelet/wish/bubbletea"
	"github.com/mattn/go-isatty"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	log.SetOutput(os.Stderr)

	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "apply":
			runApply(os.Args[2:])
			return
		case "export":
			runExport(os.Args[2:])
			return
		case "version", "--version", "-v":
			printVersion()
			return
		}
	}
	runServe(os.Args[1:])
}

func printVersion() {
	if version == "dev" {
		fmt.Println("go-upkeep dev")
	} else {
		fmt.Printf("go-upkeep %s (%s, %s)\n", version, commit, date)
	}
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func openStore(dbType, dsn string) store.Store {
	var s store.Store
	var err error
	if dbType == "postgres" {
		s, err = store.NewPostgresStore(dsn)
	} else {
		s, err = store.NewSQLiteStore(dsn)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "database error: %v\n", err)
		os.Exit(1)
	}
	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "database init error: %v\n", err)
		os.Exit(1)
	}
	return s
}

func runApply(args []string) {
	fs := flag.NewFlagSet("apply", flag.ExitOnError)
	filePath := fs.String("f", "", "Path to YAML config file (required)")
	dryRun := fs.Bool("dry-run", false, "Show planned changes without applying")
	prune := fs.Bool("prune", false, "Delete monitors/alerts not in YAML")
	dbType := fs.String("db-type", envOrDefault("UPKEEP_DB_TYPE", "sqlite"), "Database type")
	dsn := fs.String("dsn", envOrDefault("UPKEEP_DB_DSN", "upkeep.db"), "Database DSN")
	_ = fs.Parse(args) // ExitOnError: parse errors exit before returning

	if *filePath == "" {
		fmt.Fprintln(os.Stderr, "error: -f flag is required")
		fs.Usage()
		os.Exit(1)
	}

	s := openStore(*dbType, *dsn)

	f, err := config.LoadFile(*filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	changes, err := config.Apply(s, f, config.ApplyOpts{
		DryRun: *dryRun,
		Prune:  *prune,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Print(config.FormatChanges(changes, *dryRun))
}

func runExport(args []string) {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	outPath := fs.String("o", "-", "Output file path (- for stdout)")
	dbType := fs.String("db-type", envOrDefault("UPKEEP_DB_TYPE", "sqlite"), "Database type")
	dsn := fs.String("dsn", envOrDefault("UPKEEP_DB_DSN", "upkeep.db"), "Database DSN")
	_ = fs.Parse(args) // ExitOnError: parse errors exit before returning

	s := openStore(*dbType, *dsn)

	f, err := config.Export(s)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := config.WriteFile(f, *outPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func runServe(args []string) {
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

	nodeID := os.Getenv("UPKEEP_NODE_ID")
	nodeName := os.Getenv("UPKEEP_NODE_NAME")
	nodeRegion := os.Getenv("UPKEEP_NODE_REGION")
	aggStrategy := os.Getenv("UPKEEP_AGG_STRATEGY")

	if clusterMode == "probe" {
		if nodeID == "" {
			fmt.Fprintln(os.Stderr, "UPKEEP_NODE_ID is required for probe mode")
			os.Exit(1)
		}
		if clusterPeer == "" {
			fmt.Fprintln(os.Stderr, "UPKEEP_PEER_URL is required for probe mode")
			os.Exit(1)
		}

		fmt.Printf("Cluster: Running as PROBE (node=%s, region=%s)\n", nodeID, nodeRegion)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan os.Signal, 1)
		signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-done
			cancel()
		}()

		if err := cluster.RunProbe(ctx, cluster.ProbeConfig{
			NodeID:    nodeID,
			NodeName:  nodeName,
			Region:    nodeRegion,
			LeaderURL: clusterPeer,
			SharedKey: clusterKey,
			Interval:  30,
		}); err != nil {
			fmt.Fprintf(os.Stderr, "Probe error: %v\n", err)
		}
		return
	}

	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", portVal, "SSH Port")
	flagDBType := fs.String("db-type", dbType, "Database type")
	flagDSN := fs.String("dsn", dbDSN, "Database DSN")
	demo := fs.Bool("demo", false, "Seed demo data")
	importKuma := fs.String("import-kuma", "", "Import Uptime Kuma backup JSON file")
	_ = fs.Parse(args) // ExitOnError: parse errors exit before returning

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
	defer s.Close()

	if err := s.Init(); err != nil {
		fmt.Printf("Database init error: %v\n", err)
		os.Exit(1)
	}
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

	eng := monitor.NewEngine(s)
	if os.Getenv("UPKEEP_INSECURE_SKIP_VERIFY") == "true" {
		eng.SetInsecureSkipVerify(true)
	}
	if aggStrategy != "" {
		eng.SetAggStrategy(monitor.AggregationStrategy(aggStrategy))
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eng.InitHistory()
	eng.InitLogs()
	eng.Start(ctx)

	httpSrv := server.Start(server.ServerConfig{
		Port:         httpPort,
		EnableStatus: enableStatus,
		Title:        statusTitle,
		ClusterKey:   clusterKey,
	}, s, eng)

	cluster.Start(ctx, cluster.Config{
		Mode:      clusterMode,
		PeerURL:   clusterPeer,
		SharedKey: clusterKey,
	}, eng)

	sshSrv := startSSHServer(*port, s, eng)

	if isatty.IsTerminal(os.Stdout.Fd()) || isatty.IsCygwinTerminal(os.Stdout.Fd()) {
		p := tea.NewProgram(tui.InitialModel(true, s, eng), tea.WithAltScreen(), tea.WithMouseCellMotion())
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
	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()
	if httpSrv != nil {
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP shutdown error: %v", err)
		}
	}
	if sshSrv != nil {
		if err := sshSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("SSH shutdown error: %v", err)
		}
	}
}

func startSSHServer(port int, db store.Store, eng *monitor.Engine) *ssh.Server {
	s, err := wish.NewServer(
		wish.WithAddress(fmt.Sprintf(":%d", port)),
		wish.WithHostKeyPath(".ssh/id_ed25519"),
		wish.WithPublicKeyAuth(func(ctx ssh.Context, key ssh.PublicKey) bool {
			return isKeyAllowed(db, key)
		}),
		wish.WithMiddleware(
			bm.Middleware(func(s ssh.Session) (tea.Model, []tea.ProgramOption) {
				return tui.InitialModel(false, db, eng), []tea.ProgramOption{tea.WithAltScreen(), tea.WithMouseCellMotion()}
			}),
		),
	)
	if err != nil {
		fmt.Printf("SSH server error: %v\n", err)
		return nil
	}
	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			log.Printf("SSH server error: %v", err)
		}
	}()
	return s
}

func seedDemoData(s store.Store) {
	existing, _ := s.GetSites()
	if len(existing) > 0 {
		return
	}
	fmt.Println("Seeding demo data...")

	if err := s.AddAlert("Discord Ops", "discord", map[string]string{"url": "https://discord.com/api/webhooks/demo/token"}); err != nil {
		log.Printf("demo seed: add alert: %v", err)
		return
	}
	if err := s.AddAlert("Slack Infra", "slack", map[string]string{"url": "https://hooks.slack.com/services/DEMO/WEBHOOK"}); err != nil {
		log.Printf("demo seed: add alert: %v", err)
		return
	}
	if err := s.AddAlert("Email Oncall", "email", map[string]string{
		"host": "smtp.example.com", "port": "587",
		"user": "oncall@example.com", "pass": "replace-me",
		"from": "oncall@example.com", "to": "team@example.com",
	}); err != nil {
		log.Printf("demo seed: add alert: %v", err)
		return
	}

	alerts, _ := s.GetAllAlerts()
	alertID := 0
	if len(alerts) > 0 {
		alertID = alerts[0].ID
	}

	demoSites := []models.Site{
		{Name: "Google", URL: "https://www.google.com", Type: "http", Interval: 30, AlertID: alertID, CheckSSL: true, ExpiryThreshold: 14, MaxRetries: 2},
		{Name: "GitHub", URL: "https://github.com", Type: "http", Interval: 30, AlertID: alertID, CheckSSL: true, ExpiryThreshold: 7, MaxRetries: 3},
		{Name: "Cloudflare DNS", URL: "https://1.1.1.1", Type: "http", Interval: 60, AlertID: alertID, ExpiryThreshold: 7, MaxRetries: 1},
		{Name: "JSON Placeholder", URL: "https://jsonplaceholder.typicode.com/posts/1", Type: "http", Interval: 45, AlertID: alertID, ExpiryThreshold: 7, MaxRetries: 2},
		{Name: "Nonexistent Site", URL: "https://this-domain-does-not-exist-12345.com", Type: "http", Interval: 30, AlertID: alertID, ExpiryThreshold: 7, MaxRetries: 3},
		{Name: "Bad Port", URL: "https://localhost:19999", Type: "http", Interval: 30, ExpiryThreshold: 7, MaxRetries: 1},
		{Name: "Backup Cron", Type: "push", Interval: 300, AlertID: alertID, ExpiryThreshold: 7},
		{Name: "DB Healthcheck", Type: "push", Interval: 120, AlertID: alertID, ExpiryThreshold: 7},
		{Name: "Gateway", Type: "ping", Interval: 30, AlertID: alertID, Hostname: "10.0.0.1", Timeout: 5, ExpiryThreshold: 7},
		{Name: "SSH Server", Type: "port", Interval: 60, AlertID: alertID, Hostname: "10.0.0.1", Port: 22, Timeout: 5, ExpiryThreshold: 7},
	}
	for _, site := range demoSites {
		if err := s.AddSite(site); err != nil {
			log.Printf("demo seed: add site %q: %v", site.Name, err)
		}
	}
}

func isKeyAllowed(db store.Store, incomingKey ssh.PublicKey) bool {
	users, err := db.GetAllUsers()
	if err != nil {
		return false
	}
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
