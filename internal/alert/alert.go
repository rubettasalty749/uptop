package alert

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"gitea.lerkolabs.com/lerko/uptop/internal/models"
	"net/http"
	"net/smtp"
	"strconv"
	"strings"
	"time"
)

var alertClient = &http.Client{Timeout: 10 * time.Second}

type Provider interface {
	Send(ctx context.Context, title, message string) error
}

type PayloadFunc func(title, message string) ([]byte, error)

type HTTPProvider struct {
	URL     string
	Payload PayloadFunc
}

func (h *HTTPProvider) Send(ctx context.Context, title, message string) error {
	body, err := h.Payload(title, message)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", h.URL, bytes.NewBuffer(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := alertClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("alert webhook returned HTTP %d", resp.StatusCode)
	}
	return nil
}

func discordPayload(title, message string) ([]byte, error) {
	return json.Marshal(map[string]string{"content": fmt.Sprintf("**%s**\n%s", title, message)})
}

func slackPayload(title, message string) ([]byte, error) {
	return json.Marshal(map[string]string{"text": fmt.Sprintf("*%s*\n%s", title, message)})
}

func webhookPayload(title, message string) ([]byte, error) {
	return json.Marshal(map[string]string{"title": title, "message": message, "status": "alert"})
}

func telegramPayload(chatID string) PayloadFunc {
	return func(title, message string) ([]byte, error) {
		return json.Marshal(map[string]string{
			"chat_id":    chatID,
			"text":       fmt.Sprintf("*%s*\n%s", title, message),
			"parse_mode": "Markdown",
		})
	}
}

func pagerdutyPayload(routingKey, severity string) PayloadFunc {
	return func(title, message string) ([]byte, error) {
		return json.Marshal(map[string]any{
			"routing_key":  routingKey,
			"event_action": "trigger",
			"payload": map[string]string{
				"summary":  fmt.Sprintf("%s: %s", title, message),
				"source":   "uptop",
				"severity": severity,
			},
		})
	}
}

func pushoverPayload(token, user string) PayloadFunc {
	return func(title, message string) ([]byte, error) {
		return json.Marshal(map[string]string{
			"token":   token,
			"user":    user,
			"title":   title,
			"message": message,
		})
	}
}

func gotifyPayload(priority string) PayloadFunc {
	return func(title, message string) ([]byte, error) {
		pri, _ := strconv.Atoi(priority)
		return json.Marshal(map[string]any{
			"title":    title,
			"message":  message,
			"priority": pri,
		})
	}
}

func GetProvider(cfg models.AlertConfig) Provider {
	switch cfg.Type {
	case "discord":
		return &HTTPProvider{URL: cfg.Settings["url"], Payload: discordPayload}
	case "slack":
		return &HTTPProvider{URL: cfg.Settings["url"], Payload: slackPayload}
	case "webhook":
		return &HTTPProvider{URL: cfg.Settings["url"], Payload: webhookPayload}
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
	case "telegram":
		return &HTTPProvider{
			URL:     fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", cfg.Settings["token"]),
			Payload: telegramPayload(cfg.Settings["chat_id"]),
		}
	case "pagerduty":
		severity := "critical"
		if s, ok := cfg.Settings["severity"]; ok && s != "" {
			severity = s
		}
		return &HTTPProvider{
			URL:     "https://events.pagerduty.com/v2/enqueue",
			Payload: pagerdutyPayload(cfg.Settings["routing_key"], severity),
		}
	case "pushover":
		return &HTTPProvider{
			URL:     "https://api.pushover.net/1/messages.json",
			Payload: pushoverPayload(cfg.Settings["token"], cfg.Settings["user"]),
		}
	case "gotify":
		priority := "5"
		if p, ok := cfg.Settings["priority"]; ok && p != "" {
			priority = p
		}
		serverURL := strings.TrimRight(cfg.Settings["url"], "/")
		return &HTTPProvider{
			URL:     fmt.Sprintf("%s/message?token=%s", serverURL, cfg.Settings["token"]),
			Payload: gotifyPayload(priority),
		}
	default:
		return nil
	}
}

type EmailProvider struct {
	Host, Port, User, Pass, To, From string
}

func (e *EmailProvider) Send(ctx context.Context, title, message string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	auth := smtp.PlainAuth("", e.User, e.Pass, e.Host)
	msg := []byte("To: " + e.To + "\r\n" +
		"Subject: uptop: " + title + "\r\n" +
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

func (n *NtfyProvider) Send(ctx context.Context, title, message string) error {
	url := strings.TrimRight(n.ServerURL, "/") + "/" + n.Topic
	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(message))
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
	if resp.StatusCode >= 400 {
		return fmt.Errorf("ntfy returned HTTP %d", resp.StatusCode)
	}
	return nil
}
