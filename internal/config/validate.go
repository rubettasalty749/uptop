package config

import "fmt"

var validMonitorTypes = map[string]bool{
	"http":  true,
	"push":  true,
	"ping":  true,
	"port":  true,
	"dns":   true,
	"group": true,
}

func Validate(f *File) error {
	alertNames := make(map[string]bool, len(f.Alerts))
	for _, a := range f.Alerts {
		if a.Name == "" {
			return fmt.Errorf("alert missing name")
		}
		if alertNames[a.Name] {
			return fmt.Errorf("duplicate alert name %q", a.Name)
		}
		alertNames[a.Name] = true
		if a.Type == "" {
			return fmt.Errorf("alert %q: missing type", a.Name)
		}
	}

	monitorNames := make(map[string]bool)
	for _, m := range f.Monitors {
		if err := validateMonitor(m, monitorNames, false); err != nil {
			return err
		}
	}
	return nil
}

func validateMonitor(m Monitor, names map[string]bool, nested bool) error {
	if m.Name == "" {
		return fmt.Errorf("monitor missing name")
	}
	if names[m.Name] {
		return fmt.Errorf("duplicate monitor name %q", m.Name)
	}
	names[m.Name] = true

	if !validMonitorTypes[m.Type] {
		return fmt.Errorf("monitor %q: invalid type %q", m.Name, m.Type)
	}

	if m.Type == "group" && nested {
		return fmt.Errorf("monitor %q: groups cannot be nested inside other groups", m.Name)
	}

	switch m.Type {
	case "http":
		if m.URL == "" {
			return fmt.Errorf("monitor %q: url is required for type http", m.Name)
		}
	case "ping":
		if m.Hostname == "" {
			return fmt.Errorf("monitor %q: hostname is required for type ping", m.Name)
		}
	case "port":
		if m.Hostname == "" {
			return fmt.Errorf("monitor %q: hostname is required for type port", m.Name)
		}
		if m.Port == 0 {
			return fmt.Errorf("monitor %q: port is required for type port", m.Name)
		}
	case "dns":
		if m.Hostname == "" {
			return fmt.Errorf("monitor %q: hostname is required for type dns", m.Name)
		}
	}

	if m.Type == "group" {
		for _, child := range m.Monitors {
			if err := validateMonitor(child, names, true); err != nil {
				return err
			}
		}
	} else if len(m.Monitors) > 0 {
		return fmt.Errorf("monitor %q: only groups can have nested monitors", m.Name)
	}

	return nil
}
