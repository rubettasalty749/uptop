package config

import (
	"strings"
	"testing"
)

func TestValidateDuplicateAlertNames(t *testing.T) {
	f := &File{
		Alerts: []Alert{
			{Name: "A", Type: "discord"},
			{Name: "A", Type: "slack"},
		},
	}
	err := Validate(f)
	if err == nil || !strings.Contains(err.Error(), "duplicate alert name") {
		t.Fatalf("expected duplicate alert error, got %v", err)
	}
}

func TestValidateDuplicateMonitorNames(t *testing.T) {
	f := &File{
		Monitors: []Monitor{
			{Name: "M", Type: "http", URL: "https://example.com"},
			{Name: "M", Type: "ping", Hostname: "10.0.0.1"},
		},
	}
	err := Validate(f)
	if err == nil || !strings.Contains(err.Error(), "duplicate monitor name") {
		t.Fatalf("expected duplicate monitor error, got %v", err)
	}
}

func TestValidateDuplicateNameAcrossGroups(t *testing.T) {
	f := &File{
		Monitors: []Monitor{
			{Name: "Web", Type: "http", URL: "https://example.com"},
			{
				Name: "Prod", Type: "group",
				Monitors: []Monitor{
					{Name: "Web", Type: "http", URL: "https://prod.example.com"},
				},
			},
		},
	}
	err := Validate(f)
	if err == nil || !strings.Contains(err.Error(), "duplicate monitor name") {
		t.Fatalf("expected duplicate name across group, got %v", err)
	}
}

func TestValidateNestedGroupReject(t *testing.T) {
	f := &File{
		Monitors: []Monitor{
			{
				Name: "Outer", Type: "group",
				Monitors: []Monitor{
					{Name: "Inner", Type: "group"},
				},
			},
		},
	}
	err := Validate(f)
	if err == nil || !strings.Contains(err.Error(), "cannot be nested") {
		t.Fatalf("expected nested group error, got %v", err)
	}
}

func TestValidateRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		monitor Monitor
		wantErr string
	}{
		{"http no url", Monitor{Name: "A", Type: "http"}, "url is required"},
		{"ping no hostname", Monitor{Name: "A", Type: "ping"}, "hostname is required"},
		{"port no hostname", Monitor{Name: "A", Type: "port", Port: 22}, "hostname is required"},
		{"port no port", Monitor{Name: "A", Type: "port", Hostname: "h"}, "port is required"},
		{"dns no hostname", Monitor{Name: "A", Type: "dns"}, "hostname is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &File{Monitors: []Monitor{tt.monitor}}
			err := Validate(f)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestValidateInvalidMonitorType(t *testing.T) {
	f := &File{
		Monitors: []Monitor{
			{Name: "A", Type: "ftp"},
		},
	}
	err := Validate(f)
	if err == nil || !strings.Contains(err.Error(), "invalid type") {
		t.Fatalf("expected invalid type error, got %v", err)
	}
}

func TestValidateNonGroupWithChildren(t *testing.T) {
	f := &File{
		Monitors: []Monitor{
			{
				Name: "A", Type: "http", URL: "https://example.com",
				Monitors: []Monitor{
					{Name: "B", Type: "ping", Hostname: "h"},
				},
			},
		},
	}
	err := Validate(f)
	if err == nil || !strings.Contains(err.Error(), "only groups") {
		t.Fatalf("expected only-groups error, got %v", err)
	}
}

func TestValidateAlertMissingName(t *testing.T) {
	f := &File{
		Alerts: []Alert{{Type: "discord"}},
	}
	err := Validate(f)
	if err == nil || !strings.Contains(err.Error(), "missing name") {
		t.Fatalf("expected missing name error, got %v", err)
	}
}

func TestValidateAlertMissingType(t *testing.T) {
	f := &File{
		Alerts: []Alert{{Name: "A"}},
	}
	err := Validate(f)
	if err == nil || !strings.Contains(err.Error(), "missing type") {
		t.Fatalf("expected missing type error, got %v", err)
	}
}

func TestValidateValidConfig(t *testing.T) {
	f := &File{
		Alerts: []Alert{
			{Name: "Discord", Type: "discord", Settings: map[string]string{"url": "https://example.com"}},
		},
		Monitors: []Monitor{
			{Name: "Web", Type: "http", URL: "https://example.com", Interval: 30, Alert: "Discord"},
			{Name: "Ping", Type: "ping", Hostname: "10.0.0.1", Interval: 30},
			{Name: "SSH", Type: "port", Hostname: "10.0.0.1", Port: 22, Interval: 60},
			{Name: "DNS", Type: "dns", Hostname: "example.com", Interval: 60},
			{Name: "Cron", Type: "push", Interval: 300},
			{
				Name: "Prod", Type: "group",
				Monitors: []Monitor{
					{Name: "Prod Web", Type: "http", URL: "https://prod.example.com", Interval: 15},
				},
			},
		},
	}
	if err := Validate(f); err != nil {
		t.Fatalf("expected valid config, got %v", err)
	}
}
