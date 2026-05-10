// Package telegram is a thin wrapper around the Telegram Bot sendMessage API.
package telegram

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

const apiURL = "https://api.telegram.org/bot%s/sendMessage"

type Client struct {
	BotToken string
	ChatID   string
	HTTP     *http.Client
}

func NewClient(token, chatID string) *Client {
	return &Client{
		BotToken: token,
		ChatID:   chatID,
		HTTP:     &http.Client{Timeout: 10 * time.Second},
	}
}

// Send posts an HTML-formatted message to the configured chat.
// Returns false on any error (network, non-2xx, missing creds) — never panics.
func (c *Client) Send(text string) bool {
	if c.BotToken == "" || c.ChatID == "" {
		return false
	}
	body, _ := json.Marshal(map[string]string{
		"chat_id":    c.ChatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	url := "https://api.telegram.org/bot" + c.BotToken + "/sendMessage"
	resp, err := c.HTTP.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("telegram send error: %v", err)
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		log.Printf("telegram non-2xx: %d", resp.StatusCode)
		return false
	}
	return true
}
