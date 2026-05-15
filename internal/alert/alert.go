package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-upkeep/internal/models"
	"net/http"
	"net/smtp"
	"strings"
	"time"
)

var alertClient = &http.Client{Timeout: 10 * time.Second}

type Provider interface {
	Send(title, message string) error
}

func GetProvider(cfg models.AlertConfig) Provider {
	switch cfg.Type {
	case "discord":
		return &DiscordProvider{URL: cfg.Settings["url"]}
	case "slack":
		return &SlackProvider{URL: cfg.Settings["url"]}
	case "webhook":
		// Generic Webhook
		return &WebhookProvider{URL: cfg.Settings["url"]}
	case "email":
		port := "25"
		if p, ok := cfg.Settings["port"]; ok {
			port = p
		}
		return &EmailProvider{
			Host: cfg.Settings["host"],
			Port: port,
			User: cfg.Settings["user"],
			Pass: cfg.Settings["pass"],
			To:   cfg.Settings["to"],
			From: cfg.Settings["from"],
		}
	case "ntfy":
		priority := "3"
		if p, ok := cfg.Settings["priority"]; ok && p != "" {
			priority = p
		}
		return &NtfyProvider{
			ServerURL: cfg.Settings["url"],
			Topic:     cfg.Settings["topic"],
			Priority:  priority,
			Username:  cfg.Settings["username"],
			Password:  cfg.Settings["password"],
		}
	default:
		return nil
	}
}

// --- DISCORD ---
type DiscordProvider struct{ URL string }

func (d *DiscordProvider) Send(title, message string) error {
	payload := map[string]string{"content": fmt.Sprintf("**%s**\n%s", title, message)}
	jsonValue, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := alertClient.Post(d.URL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// --- SLACK ---
type SlackProvider struct{ URL string }

func (s *SlackProvider) Send(title, message string) error {
	payload := map[string]string{"text": fmt.Sprintf("*%s*\n%s", title, message)}
	jsonValue, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := alertClient.Post(s.URL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// --- GENERIC WEBHOOK ---
type WebhookProvider struct{ URL string }

func (w *WebhookProvider) Send(title, message string) error {
	payload := map[string]string{
		"title":   title,
		"message": message,
		"status":  "alert",
	}
	jsonValue, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	resp, err := alertClient.Post(w.URL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

// --- EMAIL ---
type EmailProvider struct {
	Host, Port, User, Pass, To, From string
}

func (e *EmailProvider) Send(title, message string) error {
	auth := smtp.PlainAuth("", e.User, e.Pass, e.Host)
	msg := []byte("To: " + e.To + "\r\n" +
		"Subject: Go-Upkeep: " + title + "\r\n" +
		"\r\n" +
		message + "\r\n")
	return smtp.SendMail(e.Host+":"+e.Port, auth, e.From, []string{e.To}, msg)
}

type NtfyProvider struct {
	ServerURL string
	Topic     string
	Priority  string
	Username  string
	Password  string
}

func (n *NtfyProvider) Send(title, message string) error {
	url := strings.TrimRight(n.ServerURL, "/") + "/" + n.Topic
	req, err := http.NewRequest("POST", url, strings.NewReader(message))
	if err != nil {
		return err
	}
	req.Header.Set("Title", title)
	req.Header.Set("Priority", n.Priority)
	if n.Username != "" && n.Password != "" {
		req.SetBasicAuth(n.Username, n.Password)
	}
	resp, err := alertClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}
