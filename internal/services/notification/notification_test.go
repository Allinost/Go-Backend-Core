package notification

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockSender struct {
	name    string
	channel Channel
	fail    bool
}

func (m *mockSender) Name() string { return m.name }
func (m *mockSender) Channel() Channel { return m.channel }
func (m *mockSender) Send(ctx context.Context, msg Message) (*Result, error) {
	if m.fail {
		return &Result{MessageID: msg.ID, Channel: m.channel, Success: false, Error: "mock failure"}, nil
	}
	return &Result{MessageID: msg.ID, Channel: m.channel, Success: true}, nil
}
func (m *mockSender) Close() error { return nil }

func TestService_RegisterSender(t *testing.T) {
	s := NewService()
	s.RegisterSender(&mockSender{name: "test", channel: ChannelEmail})
	assert.NotNil(t, s)
}

func TestService_Send(t *testing.T) {
	s := NewService()
	s.RegisterSender(&mockSender{name: "email-sender", channel: ChannelEmail})

	result, err := s.Send(context.Background(), Message{
		Channel: ChannelEmail,
		To:      []string{"test@example.com"},
		Subject: "Hello",
		Body:    "World",
	})
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestService_SendNoSender(t *testing.T) {
	s := NewService()
	_, err := s.Send(context.Background(), Message{Channel: ChannelSMS})
	assert.Error(t, err)
}

func TestService_SendToAll(t *testing.T) {
	s := NewService()
	s.RegisterSender(&mockSender{name: "s1", channel: ChannelPush})
	s.RegisterSender(&mockSender{name: "s2", channel: ChannelPush})

	results := s.SendToAll(context.Background(), Message{Channel: ChannelPush})
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.True(t, r.Success)
	}
}

func TestService_SendToAllNoSender(t *testing.T) {
	s := NewService()
	results := s.SendToAll(context.Background(), Message{Channel: "unknown"})
	assert.Len(t, results, 1)
	assert.False(t, results[0].Success)
}

func TestService_SendFailover(t *testing.T) {
	s := NewService()
	s.RegisterSender(&mockSender{name: "failing", channel: ChannelEmail, fail: true})
	s.RegisterSender(&mockSender{name: "working", channel: ChannelEmail})

	result, err := s.Send(context.Background(), Message{Channel: ChannelEmail})
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestService_Close(t *testing.T) {
	s := NewService()
	s.RegisterSender(&mockSender{name: "t", channel: ChannelEmail})
	s.Close()
}

func TestLogSender(t *testing.T) {
	l := NewLogSender("test")
	assert.Equal(t, "test", l.Name())
	assert.Equal(t, ChannelEmail, l.Channel())

	result, err := l.Send(context.Background(), Message{})
	require.NoError(t, err)
	assert.True(t, result.Success)
}

func TestMessagePriority(t *testing.T) {
	m := Message{Priority: PriorityHigh}
	assert.Equal(t, PriorityHigh, m.Priority)
}

func TestService_SendAllFail(t *testing.T) {
	s := NewService()
	s.RegisterSender(&mockSender{name: "f1", channel: ChannelEmail, fail: true})
	s.RegisterSender(&mockSender{name: "f2", channel: ChannelEmail, fail: true})

	result, err := s.Send(context.Background(), Message{Channel: ChannelEmail})
	assert.Error(t, err)
	assert.Nil(t, result)
}
