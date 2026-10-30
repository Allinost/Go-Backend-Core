package developmentmanager

import "time"

// 协议
const (
	ProtocolTCP = "tcp"
	ProtocolUDP = "udp"
)

// 端口暴露范围
const (
	ExposureNone     = "none"     // 不暴露（仅容器网络内访问）
	ExposureIntranet = "intranet" // 内网
	ExposureExtranet = "extranet" // 公网
)

// 反代类型（空 = 无反代直连）
const (
	ProxyNginx   = "nginx"
	ProxyCaddy   = "caddy"
	ProxyTraefik = "traefik"
	ProxyOther   = "other"
)

// 访问地址网络区域
const (
	ZoneIntranet = "intranet"
	ZoneExtranet = "extranet"
	ZoneProxyNet = "proxy_net"
)

// 卷挂载模式
const (
	VolumeModeRW = "rw"
	VolumeModeRO = "ro"
)

// 备份类型
const (
	BackupTypeFull   = "full"
	BackupTypeDB     = "db"
	BackupTypeVolume = "volume"
)

// 备份最近执行状态
const (
	BackupStatusSuccess = "success"
	BackupStatusFailed  = "failed"
	BackupStatusRunning = "running"
)

// ValidProtocols 协议合法集合
var ValidProtocols = map[string]bool{ProtocolTCP: true, ProtocolUDP: true}

// ValidExposures 暴露范围合法集合
var ValidExposures = map[string]bool{ExposureNone: true, ExposureIntranet: true, ExposureExtranet: true}

// ValidProxyTypes 反代类型合法集合（含空字符串 = 直连）
var ValidProxyTypes = map[string]bool{"": true, ProxyNginx: true, ProxyCaddy: true, ProxyTraefik: true, ProxyOther: true}

// ValidZones 网络区域合法集合
var ValidZones = map[string]bool{ZoneIntranet: true, ZoneExtranet: true, ZoneProxyNet: true}

// ValidVolumeModes 挂载模式合法集合
var ValidVolumeModes = map[string]bool{VolumeModeRW: true, VolumeModeRO: true}

// ValidBackupTypes 备份类型合法集合
var ValidBackupTypes = map[string]bool{BackupTypeFull: true, BackupTypeDB: true, BackupTypeVolume: true}

// ValidBackupStatuses 备份状态合法集合（含空字符串 = 未执行）
var ValidBackupStatuses = map[string]bool{"": true, BackupStatusSuccess: true, BackupStatusFailed: true, BackupStatusRunning: true}

// Container 项目关联容器
type Container struct {
	ID            int64             `json:"id"`
	ProjectID     int64             `json:"project_id"`
	ContainerName string            `json:"container_name"`
	Image         string            `json:"image"`
	ComposeRef    string            `json:"compose_ref"`
	Replicas      int               `json:"replicas"`
	NetworkMode   string            `json:"network_mode"`
	EnvNotes      map[string]string `json:"env_notes,omitempty"`
	StatusNote    string            `json:"status_note"`
	Deleted       bool              `json:"deleted"`
	DeletedAt     *time.Time        `json:"deleted_at,omitempty"`
	CreatedBy     string            `json:"created_by"`
	UpdatedBy     string            `json:"updated_by"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// ResourceAlloc 容器资源分配（与容器 1:1）
type ResourceAlloc struct {
	ID             int64     `json:"id"`
	ContainerID    int64     `json:"container_id"`
	CPULimit       string    `json:"cpu_limit"`
	CPUShares      int       `json:"cpu_shares"`
	MemLimit       string    `json:"mem_limit"`
	MemReservation string    `json:"mem_reservation"`
	GPUNote        string    `json:"gpu_note"`
	StorageQuota   string    `json:"storage_quota"`
	Notes          string    `json:"notes"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// VolumeMapping 卷映射
type VolumeMapping struct {
	ID            int64      `json:"id"`
	ContainerID   int64      `json:"container_id"`
	HostPath      string     `json:"host_path"`
	ContainerPath string     `json:"container_path"`
	Mode          string     `json:"mode"`
	Purpose       string     `json:"purpose"`
	InBackup      bool       `json:"in_backup"`
	Deleted       bool       `json:"deleted"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// AccessPoint 网络接入点（端口 + 反代 + 访问地址 + 白名单引用）
type AccessPoint struct {
	ID            int64      `json:"id"`
	ProjectID     int64      `json:"project_id"`
	ContainerID   *int64     `json:"container_id"`
	ListenPort    int        `json:"listen_port"`
	HostPort      *int       `json:"host_port"`
	Protocol      string     `json:"protocol"`
	Purpose       string     `json:"purpose"`
	Exposure      string     `json:"exposure"`
	ProxyType     string     `json:"proxy_type"`
	Domain        string     `json:"domain"`
	UpstreamURL   string     `json:"upstream_url"`
	TLS           bool       `json:"tls"`
	NetworkZone   string     `json:"network_zone"`
	URL           *string    `json:"url"`
	Reachable     bool       `json:"reachable"`
	Notes         string     `json:"notes"`
	WhitelistID   *int64     `json:"whitelist_id"`
	WhitelistName string     `json:"whitelist_name,omitempty"`
	Deleted       bool       `json:"deleted"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	CreatedBy     string     `json:"created_by"`
	UpdatedBy     string     `json:"updated_by"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// Whitelist 常用白名单组（允许 0 成员）
type Whitelist struct {
	ID          int64      `json:"id"`
	ProjectID   int64      `json:"project_id"`
	Name        string     `json:"name"`
	Note        string     `json:"note"`
	MemberCount int        `json:"member_count"`
	Deleted     bool       `json:"deleted"`
	DeletedAt   *time.Time `json:"deleted_at,omitempty"`
	CreatedBy   string     `json:"created_by"`
	UpdatedBy   string     `json:"updated_by"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// WhitelistMember 白名单成员（组 → 允许访问的容器）
type WhitelistMember struct {
	ID            int64     `json:"id"`
	WhitelistID   int64     `json:"whitelist_id"`
	ContainerID   int64     `json:"container_id"`
	ContainerName string    `json:"container_name,omitempty"`
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

// Backup 备份记录
type Backup struct {
	ID            int64      `json:"id"`
	ProjectID     int64      `json:"project_id"`
	ContainerID   *int64     `json:"container_id"`
	VolumeID      *int64     `json:"volume_id"`
	BackupType    string     `json:"backup_type"`
	StrategyCron  string     `json:"strategy_cron"`
	Destination   string     `json:"destination"`
	RetentionDays int        `json:"retention_days"`
	LastRunAt     *time.Time `json:"last_run_at"`
	LastStatus    string     `json:"last_status"`
	LastError     string     `json:"last_error"`
	RestoreNote   string     `json:"restore_note"`
	Enabled       bool       `json:"enabled"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

// ---------- 请求结构 ----------

// CreateContainerReq 新建/更新容器请求
type CreateContainerReq struct {
	ContainerName string            `json:"container_name" binding:"required"`
	Image         string            `json:"image"`
	ComposeRef    string            `json:"compose_ref"`
	Replicas      int               `json:"replicas"`
	NetworkMode   string            `json:"network_mode"`
	EnvNotes      map[string]string `json:"env_notes"`
	StatusNote    string            `json:"status_note"`
}

// UpdateContainerReq 更新容器请求（全量）
type UpdateContainerReq = CreateContainerReq

// UpsertResourceReq 资源分配写入请求（1:1 upsert）
type UpsertResourceReq struct {
	CPULimit       string `json:"cpu_limit"`
	CPUShares      int    `json:"cpu_shares"`
	MemLimit       string `json:"mem_limit"`
	MemReservation string `json:"mem_reservation"`
	GPUNote        string `json:"gpu_note"`
	StorageQuota   string `json:"storage_quota"`
	Notes          string `json:"notes"`
}

// CreateVolumeReq 新建/更新卷映射请求
type CreateVolumeReq struct {
	HostPath      string `json:"host_path" binding:"required"`
	ContainerPath string `json:"container_path" binding:"required"`
	Mode          string `json:"mode"`
	Purpose       string `json:"purpose"`
	InBackup      bool   `json:"in_backup"`
}

// UpdateVolumeReq 更新卷映射请求（全量）
type UpdateVolumeReq = CreateVolumeReq

// CreateAccessPointReq 新建/更新接入点请求
type CreateAccessPointReq struct {
	ContainerID *int64  `json:"container_id"`
	ListenPort  int     `json:"listen_port" binding:"required"`
	HostPort    *int    `json:"host_port"`
	Protocol    string  `json:"protocol"`
	Purpose     string  `json:"purpose"`
	Exposure    string  `json:"exposure"`
	ProxyType   string  `json:"proxy_type"`
	Domain      string  `json:"domain"`
	UpstreamURL string  `json:"upstream_url"`
	TLS         bool    `json:"tls"`
	NetworkZone string  `json:"network_zone"`
	URL         *string `json:"url"`
	WhitelistID *int64  `json:"whitelist_id"`
	Notes       string  `json:"notes"`
}

// UpdateAccessPointReq 更新接入点请求（全量）
type UpdateAccessPointReq = CreateAccessPointReq

// CreateWhitelistReq 新建/更新白名单组请求
type CreateWhitelistReq struct {
	Name string `json:"name" binding:"required"`
	Note string `json:"note"`
}

// UpdateWhitelistReq 更新白名单组请求（全量）
type UpdateWhitelistReq = CreateWhitelistReq

// CreateWhitelistMemberReq 添加白名单成员请求
type CreateWhitelistMemberReq struct {
	ContainerID int64  `json:"container_id" binding:"required"`
	Note        string `json:"note"`
}

// CreateBackupReq 新建备份记录请求
type CreateBackupReq struct {
	ContainerID   *int64 `json:"container_id"`
	VolumeID      *int64 `json:"volume_id"`
	BackupType    string `json:"backup_type" binding:"required"`
	StrategyCron  string `json:"strategy_cron"`
	Destination   string `json:"destination"`
	RetentionDays int    `json:"retention_days"`
	RestoreNote   string `json:"restore_note"`
	Enabled       *bool  `json:"enabled"`
}

// UpdateBackupReq 更新备份记录请求（含最近执行结果回报）
type UpdateBackupReq struct {
	CreateBackupReq
	LastStatus string     `json:"last_status"`
	LastError  string     `json:"last_error"`
	LastRunAt  *time.Time `json:"last_run_at"`
}

// ListDeploymentReq 部署资产列表筛选
type ListDeploymentReq struct {
	Exposure string `form:"exposure"`
	Zone     string `form:"network_zone"`
	Deleted  *bool  `form:"deleted"`
}

// IncludeDeleted 是否包含已删除项
func (r *ListDeploymentReq) IncludeDeleted() bool {
	return r.Deleted != nil && *r.Deleted
}
