package config

type File struct {
	Alerts   []Alert   `yaml:"alerts,omitempty"`
	Monitors []Monitor `yaml:"monitors,omitempty"`
}

type Alert struct {
	Name     string            `yaml:"name"`
	Type     string            `yaml:"type"`
	Settings map[string]string `yaml:"settings"`
}

type Monitor struct {
	Name            string    `yaml:"name"`
	Type            string    `yaml:"type"`
	URL             string    `yaml:"url,omitempty"`
	Interval        int       `yaml:"interval,omitempty"`
	Alert           string    `yaml:"alert,omitempty"`
	CheckSSL        bool      `yaml:"check_ssl,omitempty"`
	ExpiryThreshold int       `yaml:"expiry_threshold,omitempty"`
	MaxRetries      int       `yaml:"max_retries,omitempty"`
	Hostname        string    `yaml:"hostname,omitempty"`
	Port            int       `yaml:"port,omitempty"`
	Timeout         int       `yaml:"timeout,omitempty"`
	Method          string    `yaml:"method,omitempty"`
	Description     string    `yaml:"description,omitempty"`
	AcceptedCodes   string    `yaml:"accepted_codes,omitempty"`
	DNSResolveType  string    `yaml:"dns_resolve_type,omitempty"`
	DNSServer       string    `yaml:"dns_server,omitempty"`
	IgnoreTLS       bool      `yaml:"ignore_tls,omitempty"`
	Paused          bool      `yaml:"paused,omitempty"`
	Regions         string    `yaml:"regions,omitempty"`
	Monitors        []Monitor `yaml:"monitors,omitempty"`
}
