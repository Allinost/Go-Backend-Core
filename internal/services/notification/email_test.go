package notification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEmailSender_Name(t *testing.T) {
	s := NewEmailSender("test-sender", "smtp.example.com", 587, "user", "pass", "noreply@example.com")
	assert.Equal(t, "test-sender", s.Name())
	assert.Equal(t, ChannelEmail, s.Channel())
}

func TestEmailSender_SendNoRecipients(t *testing.T) {
	s := NewEmailSender("test", "host", 25, "u", "p", "from@test.com")
	_, err := s.Send(context.Background(), Message{})
	assert.Error(t, err)
}

func TestEmailSender_SendBadHost(t *testing.T) {
	s := NewEmailSender("test", "nonexistent.smtp.local", 25, "u", "p", "from@test.com")
	result, err := s.Send(context.Background(), Message{
		To:      []string{"test@example.com"},
		Subject: "Hello",
		Body:    "World",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Error, "no such host")
}

func TestEmailSender_Close(t *testing.T) {
	s := NewEmailSender("t", "host", 25, "u", "p", "f")
	assert.NoError(t, s.Close())
}

func TestEmailSender_HtmlBody(t *testing.T) {
	s := NewEmailSender("test", "invalid.local", 25, "u", "p", "from@test.com")
	result, err := s.Send(context.Background(), Message{
		To:       []string{"test@example.com"},
		Subject:  "HTML",
		Body:     "text",
		HTMLBody: "<b>html</b>",
	})
	assert.NoError(t, err)
	assert.False(t, result.Success)
}