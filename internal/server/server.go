package server

import (
	"encoding/json"
	"fmt"
	"go-upkeep/internal/importer"
	"go-upkeep/internal/models"
	"go-upkeep/internal/monitor"
	"go-upkeep/internal/store"
	"html/template"
	"log"
	"net/http"
	"sort"
)

var statusTpl = template.Must(template.New("status").Parse(`
<!DOCTYPE html>
<html>
<head>
	<title>{{.Title}}</title>
	<meta name="viewport" content="width=device-width, initial-scale=1.0">
	<style>
		body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif; background: #1a1b26; color: #a9b1d6; padding: 20px; margin: 0; }
		h1 { text-align: center; color: #7aa2f7; margin-bottom: 30px; }
		.container { max-width: 800px; margin: 0 auto; }
		.card { background: #24283b; padding: 20px; margin-bottom: 15px; border-radius: 8px; display: flex; align-items: center; justify-content: space-between; box-shadow: 0 4px 6px rgba(0,0,0,0.1); }
		.info { display: flex; flex-direction: column; }
		.name { font-size: 1.2em; font-weight: bold; color: #c0caf5; margin-bottom: 5px; }
		.meta { font-size: 0.85em; color: #565f89; }
		.status { font-weight: bold; padding: 6px 12px; border-radius: 6px; min-width: 60px; text-align: center; }
		.UP { background: #9ece6a; color: #1a1b26; }
		.DOWN { background: #f7768e; color: #1a1b26; }
		.PENDING { background: #e0af68; color: #1a1b26; }
		.SSL-EXP { background: #e0af68; color: #1a1b26; }
		.PAUSED { background: #565f89; color: #c0caf5; }
		.summary { display: flex; justify-content: center; gap: 16px; margin-bottom: 24px; font-size: 0.95em; font-weight: 600; }
		.summary span { padding: 4px 12px; border-radius: 6px; }
		.summary .s-up { color: #9ece6a; }
		.summary .s-down { color: #f7768e; }
		.summary .s-paused { color: #565f89; }
		.summary .s-total { color: #7aa2f7; }
		.stale-bar { text-align: center; font-size: 0.8em; color: #565f89; margin-bottom: 16px; transition: color 0.3s; }
		.stale-bar.warn { color: #e0af68; }
		.stale-bar.error { color: #f7768e; }
	</style>
</head>
<body>
	<div class="container">
		<h1>{{.Title}}</h1>
		<div id="summary" class="summary"></div>
		<div id="stale" class="stale-bar"></div>
		<div id="cards"></div>
		<div style="text-align: center; margin-top: 40px; color: #565f89; font-size: 0.8em;">Powered by Go-Upkeep</div>
	</div>
	<script>
		var lastUpdate = null;

		function esc(s) {
			var d = document.createElement('div');
			d.appendChild(document.createTextNode(s));
			return d.innerHTML;
		}

		function cssClass(status) {
			return status.replace(/\s+/g, '-');
		}

		function renderSummary(sites) {
			var up = 0, down = 0, paused = 0, total = sites.length;
			for (var i = 0; i < sites.length; i++) {
				if (sites[i].Paused) { paused++; continue; }
				if (sites[i].Status === 'UP') up++;
				else if (sites[i].Status === 'DOWN') down++;
			}
			var el = document.getElementById('summary');
			var parts = ['<span class="s-total">' + up + '/' + total + ' UP</span>'];
			if (down > 0) parts.push('<span class="s-down">' + down + ' DOWN</span>');
			if (paused > 0) parts.push('<span class="s-paused">' + paused + ' PAUSED</span>');
			el.innerHTML = parts.join('<span style="color:#383838">·</span>');
		}

		function renderStale() {
			var el = document.getElementById('stale');
			if (!lastUpdate) { el.textContent = ''; return; }
			var ago = Math.round((Date.now() - lastUpdate) / 1000);
			el.className = 'stale-bar';
			if (ago < 10) {
				el.textContent = 'Updated just now';
			} else if (ago < 30) {
				el.textContent = 'Updated ' + ago + 's ago';
				el.className = 'stale-bar warn';
			} else {
				el.textContent = 'Stale — last update ' + ago + 's ago';
				el.className = 'stale-bar error';
			}
		}

		function render(sites) {
			var c = document.getElementById('cards');
			var html = '';
			sites.sort(function(a, b) {
				if (a.Status !== b.Status) {
					if (a.Status === 'DOWN') return -1;
					if (b.Status === 'DOWN') return 1;
				}
				return a.Name < b.Name ? -1 : a.Name > b.Name ? 1 : 0;
			});
			renderSummary(sites);
			for (var i = 0; i < sites.length; i++) {
				var s = sites[i];
				var st = s.Paused ? 'PAUSED' : s.Status;
				var cls = cssClass(st);
				var meta = esc(s.Type) + ' | ' + (s.Type === 'http' ? esc(s.URL) : 'Heartbeat Monitor');
				var lc = s.LastCheck ? new Date(s.LastCheck).toLocaleTimeString('en-GB', {hour12: false}) : '—';
				html += '<div class="card"><div class="info">' +
					'<div class="name">' + esc(s.Name) + '</div>' +
					'<div class="meta">' + meta + '</div>' +
					'<div class="meta" style="margin-top:4px;">Last Check: ' + lc + '</div>' +
					'</div><div class="status ' + cls + '">' + esc(st) + '</div></div>';
			}
			c.innerHTML = html;
		}

		function refresh() {
			fetch('/status/json')
				.then(function(r) { return r.json(); })
				.then(function(data) {
					var sites = [];
					for (var k in data) sites.push(data[k]);
					lastUpdate = Date.now();
					render(sites);
				})
				.catch(function() {});
			renderStale();
			setTimeout(refresh, 5000);
		}

		setInterval(renderStale, 1000);
		refresh();
	</script>
</body>
</html>`))

type ServerConfig struct {
	Port         int
	EnableStatus bool
	Title        string
	ClusterKey   string // Shared Secret for Security
}

func Start(cfg ServerConfig, s store.Store) {
	if cfg.ClusterKey == "" {
		fmt.Println("WARNING: No UPKEEP_CLUSTER_SECRET set. Cluster API endpoints are unauthenticated.")
	}
	mux := http.NewServeMux()

	// 1. Push Heartbeat
	mux.HandleFunc("/api/push", func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Missing token", 400)
			return
		}
		if monitor.RecordHeartbeat(token) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		} else {
			http.Error(w, "Invalid Token", 404)
		}
	})

	// 2. Health Check (For Cluster Follower)
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if cfg.ClusterKey != "" && r.Header.Get("X-Upkeep-Secret") != cfg.ClusterKey {
			http.Error(w, "Unauthorized", 401)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 3. Config Export
	mux.HandleFunc("/api/backup/export", func(w http.ResponseWriter, r *http.Request) {
		if cfg.ClusterKey == "" || r.Header.Get("X-Upkeep-Secret") != cfg.ClusterKey {
			http.Error(w, "Unauthorized: UPKEEP_CLUSTER_SECRET required", 401)
			return
		}
		data, err := s.ExportData()
		if err != nil {
			log.Printf("Export failed: %v", err)
			http.Error(w, "Export failed", 500)
			return
		}
		json.NewEncoder(w).Encode(data)
	})

	// 4. Config Import
	mux.HandleFunc("/api/backup/import", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		if cfg.ClusterKey == "" || r.Header.Get("X-Upkeep-Secret") != cfg.ClusterKey {
			http.Error(w, "Unauthorized", 401)
			return
		}
		var data models.Backup
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			http.Error(w, "Invalid JSON", 400)
			return
		}
		if err := s.ImportData(data); err != nil {
			log.Printf("Import failed: %v", err)
			http.Error(w, "Import failed", 500)
			return
		}
		w.Write([]byte("Import Successful"))
	})

	// 5. Kuma Import
	mux.HandleFunc("/api/import/kuma", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "POST required", 405)
			return
		}
		if cfg.ClusterKey == "" || r.Header.Get("X-Upkeep-Secret") != cfg.ClusterKey {
			http.Error(w, "Unauthorized", 401)
			return
		}
		var kb importer.KumaBackup
		if err := json.NewDecoder(r.Body).Decode(&kb); err != nil {
			log.Printf("Invalid Kuma JSON: %v", err)
			http.Error(w, "Invalid Kuma JSON", 400)
			return
		}
		backup := importer.ConvertKuma(&kb)
		if err := s.ImportData(backup); err != nil {
			log.Printf("Kuma import failed: %v", err)
			http.Error(w, "Import failed", 500)
			return
		}
		w.Write([]byte(fmt.Sprintf("Imported %d monitors, %d alerts from Kuma v%s", len(backup.Sites), len(backup.Alerts), kb.Version)))
	})

	// 6. Status Page
	if cfg.EnableStatus {
		mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) { renderStatusPage(w, cfg.Title) })
		mux.HandleFunc("/status/json", func(w http.ResponseWriter, r *http.Request) {
			monitor.Mutex.RLock()
			defer monitor.Mutex.RUnlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(monitor.LiveState)
		})
	}

	go func() {
		addr := fmt.Sprintf(":%d", cfg.Port)
		fmt.Printf("HTTP Server listening on %s\n", addr)
		if err := http.ListenAndServe(addr, mux); err != nil {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()
}

func renderStatusPage(w http.ResponseWriter, title string) {
	monitor.Mutex.RLock()
	var sites []models.Site
	for _, s := range monitor.LiveState {
		sites = append(sites, s)
	}
	monitor.Mutex.RUnlock()

	sort.Slice(sites, func(i, j int) bool {
		if sites[i].Status != sites[j].Status {
			if sites[i].Status == "DOWN" {
				return true
			}
			if sites[j].Status == "DOWN" {
				return false
			}
		}
		return sites[i].Name < sites[j].Name
	})

	data := struct {
		Title string
		Sites []models.Site
	}{Title: title, Sites: sites}
	statusTpl.Execute(w, data)
}
