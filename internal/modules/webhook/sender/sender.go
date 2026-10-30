package sender

import "context"

// WebhookType 发送器类型
type WebhookType string

const (
	TypeFeishu   WebhookType = "feishu"
	TypeDingtalk WebhookType = "dingtalk"
	TypeWecom    WebhookType = "wecom"
	TypeGitHub   WebhookType = "github"
	TypeGeneric  WebhookType = "generic"
)

// Config 发送目标配置（sender 视角的子集，由 webhook.WebhookConfig 转换而来）
type Config struct {
	URL           string
	SigningKey    string
	TimeoutMS     int
	AuthType      string
	AuthValue     string
	CustomHeaders map[string]string
}

// Message 待发送消息
type Message struct {
	Title   string
	Content string
	Source  string
	MsgType string
	Data    map[string]any
}

// Sender 发送器接口
type Sender interface {
	Type() WebhookType
	Send(ctx context.Context, cfg *Config, msg *Message) (int, string, error)
}

// Factory 发送器工厂
type Factory struct {
	senders map[WebhookType]Sender
}

// NewFactory 创建工厂，注册全部内置发送器
func NewFactory() *Factory {
	return &Factory{
		senders: map[WebhookType]Sender{
			TypeFeishu:   &FeishuSender{},
			TypeDingtalk: &DingtalkSender{},
			TypeWecom:    &WecomSender{},
			TypeGitHub:   &GitHubSender{},
			TypeGeneric:  &GenericSender{},
		},
	}
}

// Get 按类型获取发送器
func (f *Factory) Get(t WebhookType) Sender {
	return f.senders[t]
}

// Register 注册自定义发送器
func (f *Factory) Register(t WebhookType, s Sender) {
	f.senders[t] = s
}
