package alert

import (
	"encoding/json"
	"go-upkeep/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHTTPProviderDiscord(t *testing.T) {
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	p := GetProvider(models.AlertConfig{Type: "discord", Settings: map[string]string{"url": srv.URL}})
	if err := p.Send("Test Title", "Test Body"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if received["content"] != "**Test Title**\nTest Body" {
		t.Errorf("unexpected payload: %s", received["content"])
	}
}

func TestHTTPProviderSlack(t *testing.T) {
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	p := GetProvider(models.AlertConfig{Type: "slack", Settings: map[string]string{"url": srv.URL}})
	if err := p.Send("Alert", "Message"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if received["text"] != "*Alert*\nMessage" {
		t.Errorf("unexpected payload: %s", received["text"])
	}
}

func TestHTTPProviderWebhook(t *testing.T) {
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	p := GetProvider(models.AlertConfig{Type: "webhook", Settings: map[string]string{"url": srv.URL}})
	if err := p.Send("Title", "Body"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if received["title"] != "Title" || received["message"] != "Body" || received["status"] != "alert" {
		t.Errorf("unexpected webhook payload: %v", received)
	}
}

func TestHTTPProviderErrorOnHTTP4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
	}))
	defer srv.Close()

	p := GetProvider(models.AlertConfig{Type: "discord", Settings: map[string]string{"url": srv.URL}})
	if err := p.Send("Test", "Test"); err == nil {
		t.Fatal("expected error on 403 response")
	}
}

func TestNtfyProvider(t *testing.T) {
	var title, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		title = r.Header.Get("Title")
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		body = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer srv.Close()

	p := GetProvider(models.AlertConfig{Type: "ntfy", Settings: map[string]string{
		"url":   srv.URL,
		"topic": "test",
	}})
	if err := p.Send("Alert Title", "Alert Body"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if title != "Alert Title" {
		t.Errorf("expected title 'Alert Title', got '%s'", title)
	}
	if body != "Alert Body" {
		t.Errorf("expected body 'Alert Body', got '%s'", body)
	}
}

func TestGetProviderUnknown(t *testing.T) {
	p := GetProvider(models.AlertConfig{Type: "unknown"})
	if p != nil {
		t.Error("expected nil for unknown provider type")
	}
}
