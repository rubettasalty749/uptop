package cluster

import (
	"fmt"
	"go-upkeep/internal/monitor"
	"net/http"
	"strings"
	"time"
)

type Config struct {
	Mode      string // "leader" or "follower"
	PeerURL   string // URL of the Leader (e.g., http://primary:8080)
	SharedKey string // Security Key
}

func Start(cfg Config) {
	if cfg.Mode == "leader" {
		fmt.Println("Cluster: Running as LEADER (Active)")
		if cfg.SharedKey != "" {
			fmt.Println("WARNING: Cluster mode enabled. Ensure the HTTP server is behind a TLS-terminating proxy.")
		}
		monitor.SetEngineActive(true)
		return
	}

	if cfg.Mode == "follower" {
		fmt.Println("Cluster: Running as FOLLOWER (Passive)")
		if cfg.PeerURL != "" && !strings.HasPrefix(cfg.PeerURL, "https://") {
			fmt.Println("WARNING: Cluster peer URL is not HTTPS. Cluster secret will be sent in cleartext.")
		}
		monitor.SetEngineActive(false)
		go runFollowerLoop(cfg)
	}
}

func runFollowerLoop(cfg Config) {
	client := http.Client{Timeout: 2 * time.Second}

	// Failover Configuration
	failures := 0
	threshold := 3

	for {
		time.Sleep(5 * time.Second)

		req, _ := http.NewRequest("GET", cfg.PeerURL+"/api/health", nil)
		if cfg.SharedKey != "" {
			req.Header.Set("X-Upkeep-Secret", cfg.SharedKey)
		}

		resp, err := client.Do(req)
		isLeaderHealthy := false

		if err == nil && resp.StatusCode == 200 {
			isLeaderHealthy = true
			resp.Body.Close()
		}

		if isLeaderHealthy {
			failures = 0
			if monitor.IsEngineActive() {
				// Leader is back, yield
				monitor.SetEngineActive(false)
				monitor.AddLog("Cluster: Leader detected. Switching to PASSIVE.")
			}
		} else {
			failures++
			// If failures exceed threshold, take over
			if failures >= threshold && !monitor.IsEngineActive() {
				monitor.SetEngineActive(true)
				monitor.AddLog("Cluster: Leader Unreachable. Switching to ACTIVE.")
			}
		}
	}
}
