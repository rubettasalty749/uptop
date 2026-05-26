package monitor

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gitea.lerkolabs.com/lerko/uptop/internal/models"

	"github.com/miekg/dns"
	probing "github.com/prometheus-community/pro-bing"
)

type CheckResult struct {
	SiteID     int
	Status     string // "UP", "DOWN", "SSL EXP"
	StatusCode int
	LatencyNs  int64
	HasSSL     bool
	CertExpiry time.Time
}

func RunCheck(site models.Site, strict, insecure *http.Client, globalInsecure bool, allowPrivate ...bool) CheckResult {
	private := len(allowPrivate) > 0 && allowPrivate[0]

	if site.Type != "http" && site.Type != "dns" && !private {
		host := site.Hostname
		if host == "" {
			host = site.URL
		}
		if host != "" {
			if ips, err := net.LookupIP(host); err == nil {
				for _, ip := range ips {
					if isPrivateIP(ip) {
						return CheckResult{SiteID: site.ID, Status: "DOWN"}
					}
				}
			}
		}
	}

	switch site.Type {
	case "http":
		return runHTTPCheck(site, strict, insecure, globalInsecure)
	case "ping":
		return runPingCheck(site)
	case "port":
		return runPortCheck(site)
	case "dns":
		return runDNSCheck(site)
	default:
		return CheckResult{SiteID: site.ID, Status: "DOWN"}
	}
}

func runHTTPCheck(site models.Site, strict, insecure *http.Client, globalInsecure bool) CheckResult {
	method := site.Method
	if method == "" {
		method = "GET"
	}

	timeout := siteTimeout(site)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, site.URL, nil)
	if err != nil {
		return CheckResult{SiteID: site.ID, Status: "DOWN"}
	}

	client := strict
	if globalInsecure || site.IgnoreTLS {
		client = insecure
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	result := CheckResult{
		SiteID:    site.ID,
		Status:    "UP",
		LatencyNs: latency.Nanoseconds(),
	}

	if err != nil {
		result.Status = "DOWN"
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	if !isCodeAccepted(resp.StatusCode, site.AcceptedCodes) {
		result.Status = "DOWN"
	}

	if site.CheckSSL && resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		result.HasSSL = true
		cert := resp.TLS.PeerCertificates[0]
		result.CertExpiry = cert.NotAfter
		if time.Now().After(cert.NotAfter) {
			result.Status = "SSL EXP"
		}
	}

	return result
}

func runPingCheck(site models.Site) CheckResult {
	host := site.Hostname
	if host == "" {
		host = site.URL
	}

	pinger, err := probing.NewPinger(host)
	if err != nil {
		return CheckResult{SiteID: site.ID, Status: "DOWN"}
	}
	pinger.Count = 1
	pinger.Timeout = siteTimeout(site)
	pinger.SetPrivileged(false)

	start := time.Now()
	err = pinger.Run()
	latency := time.Since(start)

	if err != nil || pinger.Statistics().PacketsRecv == 0 {
		return CheckResult{SiteID: site.ID, Status: "DOWN", LatencyNs: latency.Nanoseconds()}
	}

	stats := pinger.Statistics()
	return CheckResult{SiteID: site.ID, Status: "UP", LatencyNs: stats.AvgRtt.Nanoseconds()}
}

func runPortCheck(site models.Site) CheckResult {
	host := site.Hostname
	if host == "" {
		host = site.URL
	}
	addr := net.JoinHostPort(host, strconv.Itoa(site.Port))
	timeout := siteTimeout(site)

	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	latency := time.Since(start)

	if err != nil {
		return CheckResult{SiteID: site.ID, Status: "DOWN", LatencyNs: latency.Nanoseconds()}
	}
	_ = conn.Close()
	return CheckResult{SiteID: site.ID, Status: "UP", LatencyNs: latency.Nanoseconds()}
}

func runDNSCheck(site models.Site) CheckResult {
	host := site.Hostname
	if host == "" {
		host = site.URL
	}

	server := site.DNSServer
	if server == "" {
		server = "1.1.1.1"
	}
	if _, _, err := net.SplitHostPort(server); err != nil {
		server = net.JoinHostPort(server, "53")
	}

	qtype := dns.TypeA
	switch site.DNSResolveType {
	case "AAAA":
		qtype = dns.TypeAAAA
	case "MX":
		qtype = dns.TypeMX
	case "CNAME":
		qtype = dns.TypeCNAME
	case "TXT":
		qtype = dns.TypeTXT
	case "NS":
		qtype = dns.TypeNS
	case "SOA":
		qtype = dns.TypeSOA
	case "SRV":
		qtype = dns.TypeSRV
	case "PTR":
		qtype = dns.TypePTR
	}

	m := new(dns.Msg)
	m.SetQuestion(dns.Fqdn(host), qtype)

	c := new(dns.Client)
	c.Timeout = siteTimeout(site)

	start := time.Now()
	r, _, err := c.Exchange(m, server)
	latency := time.Since(start)

	if err != nil {
		return CheckResult{SiteID: site.ID, Status: "DOWN", LatencyNs: latency.Nanoseconds()}
	}
	if r.Rcode != dns.RcodeSuccess {
		return CheckResult{SiteID: site.ID, Status: "DOWN", StatusCode: r.Rcode, LatencyNs: latency.Nanoseconds()}
	}
	return CheckResult{SiteID: site.ID, Status: "UP", LatencyNs: latency.Nanoseconds()}
}

func siteTimeout(site models.Site) time.Duration {
	if site.Timeout > 0 {
		return time.Duration(site.Timeout) * time.Second
	}
	return 5 * time.Second
}

func isCodeAccepted(code int, accepted string) bool {
	if accepted == "" {
		return code >= 200 && code < 300
	}
	for _, part := range strings.Split(accepted, ",") {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			bounds := strings.SplitN(part, "-", 2)
			lo, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			hi, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 == nil && err2 == nil && code >= lo && code <= hi {
				return true
			}
		} else {
			if v, err := strconv.Atoi(part); err == nil && code == v {
				return true
			}
		}
	}
	return false
}
