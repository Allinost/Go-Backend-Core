package ops

import "time"

// 主机角色
const (
	HostRoleCloud  = "cloud"
	HostRoleHost   = "host"
	HostRoleBackup = "backup"
)

// 主机状态
const (
	HostStatusUnknown = "unknown"
	HostStatusOnline  = "online"
	HostStatusOffline = "offline"
	HostStatusError   = "error"
)

var ValidHostRoles = map[string]bool{HostRoleCloud: true, HostRoleHost: true, HostRoleBackup: true}
var ValidHostEnvs = map[string]bool{"prod": true, "staging": true, "dev": true}
var ValidHostAuthTypes = map[string]bool{"password": true, "key": true}

// Host 主机台账。
type Host struct {
	ID                   int64      `json:"id"`
	Name                 string     `json:"name"`
	Role                 string     `json:"role"`
	Environment          string     `json:"environment"`
	Address              string     `json:"address"`
	SSHPort              int        `json:"ssh_port"`
	Username             string     `json:"username"`
	AuthType             string     `json:"auth_type"`
	CredentialHint       string     `json:"credential_hint"`
	HasCredential        bool       `json:"has_credential"`
	OS                   string     `json:"os"`
	Tags                 []string   `json:"tags"`
	Notes                string     `json:"notes"`
	Status               string     `json:"status"`
	LastCheckedAt        *time.Time `json:"last_checked_at,omitempty"`
	LastError            string     `json:"last_error"`
	Deleted              bool       `json:"deleted"`
	CreatedBy            string     `json:"created_by"`
	UpdatedBy            string     `json:"updated_by"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
	credentialCiphertext string
}

type ListHostsReq struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"page_size"`
	Role        string `form:"role"`
	Environment string `form:"environment"`
	Keyword     string `form:"keyword"`
}

func (r *ListHostsReq) Normalize() {
	if r.Page < 1 {
		r.Page = 1
	}
	if r.PageSize < 1 || r.PageSize > 100 {
		r.PageSize = 20
	}
}

func (r *ListHostsReq) Offset() int { return (r.Page - 1) * r.PageSize }

type CreateHostReq struct {
	Name        string   `json:"name" binding:"required"`
	Role        string   `json:"role" binding:"required"`
	Environment string   `json:"environment"`
	Address     string   `json:"address" binding:"required"`
	SSHPort     int      `json:"ssh_port"`
	Username    string   `json:"username"`
	AuthType    string   `json:"auth_type"`
	Password    string   `json:"password"`
	PrivateKey  string   `json:"private_key"`
	OS          string   `json:"os"`
	Tags        []string `json:"tags"`
	Notes       string   `json:"notes"`
}

type UpdateHostReq struct {
	Name        *string  `json:"name"`
	Role        *string  `json:"role"`
	Environment *string  `json:"environment"`
	Address     *string  `json:"address"`
	SSHPort     *int     `json:"ssh_port"`
	Username    *string  `json:"username"`
	AuthType    *string  `json:"auth_type"`
	Password    *string  `json:"password"`
	PrivateKey  *string  `json:"private_key"`
	OS          *string  `json:"os"`
	Tags        []string `json:"tags"`
	Notes       *string  `json:"notes"`
}

// HostTestResult SSH 连通测试结果。
type HostTestResult struct {
	Online     bool   `json:"online"`
	Status     string `json:"status"`
	Error      string `json:"error,omitempty"`
	DurationMS int64  `json:"duration_ms"`
	RemoteAddr string `json:"remote_addr,omitempty"`
	ServerInfo string `json:"server_info,omitempty"`
}

// HostStatusResult 只读状态采集。
type HostStatusResult struct {
	Online      bool      `json:"online"`
	Status      string    `json:"status"`
	Error       string    `json:"error,omitempty"`
	Uptime      string    `json:"uptime,omitempty"`
	Disk        string    `json:"disk,omitempty"`
	Memory      string    `json:"memory,omitempty"`
	CollectedAt time.Time `json:"collected_at"`
}
