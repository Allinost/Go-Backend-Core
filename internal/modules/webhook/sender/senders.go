package sender

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// FeishuSender 飞书机器人 Webhook 发送器
type FeishuSender struct{}

func (s *FeishuSender) Type() WebhookType { return TypeFeishu }

func (s *FeishuSender) Send(ctx context.Context, cfg *Config, msg *Message) (int, string, error) {
	body := s.buildMessage(msg.Title, msg.Content, msg.MsgType)
	jsonBody, _ := json.Marshal(body)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(jsonBody))
	if err != nil {
		return 0, "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	if cfg.SigningKey != "" {
		timestamp := time.Now().Unix()
		sign := s.sign(cfg.SigningKey, timestamp)
		httpReq.Header.Set("X-Timestamp", strconv.FormatInt(timestamp, 10))
		httpReq.Header.Set("X-Sign", sign)
	}

	timeout := cfg.TimeoutMS
	if timeout <= 0 {
		timeout = 10000
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, "", fmt.Errorf("飞书发送失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

func (s *FeishuSender) sign(secret string, timestamp int64) string {
	stringToSign := fmt.Sprintf("%d\n%s", timestamp, secret)
	mac := hmac.New(sha256.New, []byte(stringToSign))
	mac.Write([]byte(""))
	return hex.EncodeToString(mac.Sum(nil))
}

func (s *FeishuSender) buildMessage(title, content, msgType string) map[string]any {
	if msgType == "card" || msgType == "interactive" {
		return map[string]any{
			"msg_type": "interactive",
			"card": map[string]any{
				"header": map[string]any{
					"title":    map[string]any{"tag": "plain_text", "content": title},
					"template": "red",
				},
				"elements": []map[string]any{
					{"tag": "div", "text": map[string]any{"tag": "lark_md", "content": content}},
				},
			},
		}
	}
	return map[string]any{
		"msg_type": "text",
		"content":  map[string]any{"text": title + "\n" + content},
	}
}

// DingtalkSender 钉钉机器人 Webhook 发送器
type DingtalkSender struct{}

func (s *DingtalkSender) Type() WebhookType { return TypeDingtalk }

func (s *DingtalkSender) Send(ctx context.Context, cfg *Config, msg *Message) (int, string, error) {
	body := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]any{
			"title": msg.Title,
			"text":  msg.Content,
		},
	}
	return doPost(ctx, cfg.URL, body, cfg.TimeoutMS)
}

// WecomSender 企业微信机器人 Webhook 发送器
type WecomSender struct{}

func (s *WecomSender) Type() WebhookType { return TypeWecom }

func (s *WecomSender) Send(ctx context.Context, cfg *Config, msg *Message) (int, string, error) {
	body := map[string]any{
		"msgtype": "markdown",
		"markdown": map[string]any{
			"content": msg.Title + "\n" + msg.Content,
		},
	}
	return doPost(ctx, cfg.URL, body, cfg.TimeoutMS)
}

// GitHubSender GitHub Webhook 发送器
type GitHubSender struct{}

func (s *GitHubSender) Type() WebhookType { return TypeGitHub }

func (s *GitHubSender) Send(ctx context.Context, cfg *Config, msg *Message) (int, string, error) {
	payload := map[string]any{
		"ref": "refs/heads/main",
		"commits": []map[string]any{
			{
				"id":      sha256Hex(msg.Content)[:12],
				"message": msg.Title + "\n\n" + msg.Content,
			},
		},
	}
	jsonBody, _ := json.Marshal(payload)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-GitHub-Event", "push")
	httpReq.Header.Set("X-GitHub-Delivery", newUUID())

	if cfg.SigningKey != "" {
		mac := hmac.New(sha256.New, []byte(cfg.SigningKey))
		mac.Write(jsonBody)
		sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		httpReq.Header.Set("X-Hub-Signature-256", sig)
	}

	timeout := cfg.TimeoutMS
	if timeout <= 0 {
		timeout = 10000
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, "", fmt.Errorf("GitHub 发送失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// GenericSender 通用 HTTP Webhook 发送器
type GenericSender struct{}

func (s *GenericSender) Type() WebhookType { return TypeGeneric }

func (s *GenericSender) Send(ctx context.Context, cfg *Config, msg *Message) (int, string, error) {
	body := map[string]any{
		"title":   msg.Title,
		"content": msg.Content,
		"source":  msg.Source,
		"type":    msg.MsgType,
		"data":    msg.Data,
	}
	jsonBody, _ := json.Marshal(body)

	httpReq, _ := http.NewRequestWithContext(ctx, "POST", cfg.URL, bytes.NewReader(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")

	for k, v := range cfg.CustomHeaders {
		httpReq.Header.Set(k, v)
	}

	switch cfg.AuthType {
	case "basic":
		httpReq.Header.Set("Authorization", "Basic "+cfg.AuthValue)
	case "bearer":
		httpReq.Header.Set("Authorization", "Bearer "+cfg.AuthValue)
	case "header":
		if cfg.AuthValue != "" {
			httpReq.Header.Set("X-Auth", cfg.AuthValue)
		}
	}

	timeout := cfg.TimeoutMS
	if timeout <= 0 {
		timeout = 10000
	}
	client := &http.Client{Timeout: time.Duration(timeout) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, "", fmt.Errorf("通用 Webhook 发送失败: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

func doPost(ctx context.Context, url string, body any, timeoutMS int) (int, string, error) {
	jsonBody, _ := json.Marshal(body)
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	httpReq.Header.Set("Content-Type", "application/json")
	if timeoutMS <= 0 {
		timeoutMS = 10000
	}
	client := &http.Client{Timeout: time.Duration(timeoutMS) * time.Millisecond}
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", h)
}

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
