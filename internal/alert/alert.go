package alert

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go-upkeep/internal/models"
	"net/http"
	"net/smtp"
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
	default:
		return nil
	}
}

// --- DISCORD ---
type DiscordProvider struct{ URL string }

func (d *DiscordProvider) Send(title, message string) error {
	payload := map[string]string{"content": fmt.Sprintf("**%s**\n%s", title, message)}
	jsonValue, _ := json.Marshal(payload)
	resp, err := alertClient.Post(d.URL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// --- SLACK ---
type SlackProvider struct{ URL string }

func (s *SlackProvider) Send(title, message string) error {
	payload := map[string]string{"text": fmt.Sprintf("*%s*\n%s", title, message)}
	jsonValue, _ := json.Marshal(payload)
	resp, err := alertClient.Post(s.URL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	resp.Body.Close()
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
	jsonValue, _ := json.Marshal(payload)
	resp, err := alertClient.Post(w.URL, "application/json", bytes.NewBuffer(jsonValue))
	if err != nil {
		return err
	}
	resp.Body.Close()
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
