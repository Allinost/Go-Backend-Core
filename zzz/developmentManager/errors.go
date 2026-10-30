package developmentmanager

import "fmt"

// 模块业务错误码（developmentManager 占用 46001+ 区段）
const (
	ErrProjectNotFound         = 46001
	ErrRelationNotFound        = 46002
	ErrAssetNotFound           = 46003
	ErrContainerNotFound       = 46004
	ErrResourceNotFound        = 46005
	ErrVolumeNotFound          = 46006
	ErrAccessPointNotFound     = 46007
	ErrWhitelistNotFound       = 46008
	ErrWhitelistMemberNotFound = 46009
	ErrBackupNotFound          = 46010
	ErrTableNotFound           = 46011
	ErrColumnNotFound          = 46012
	ErrInvalidProjectKey       = 46100
	ErrInvalidProjectType      = 46101
	ErrDuplicateProjectKey     = 46102
	ErrInvalidRelationType     = 46103
	ErrSelfRelation            = 46104
	ErrTargetProjectMissing    = 46105
	ErrProjectHasDependents    = 46106
	ErrInvalidAssetCategory    = 46107
	ErrInvalidProjectName      = 46108

	// 部署资产校验（P2）
	ErrInvalidContainerName   = 46200
	ErrDuplicateContainerName = 46201
	ErrInvalidPort            = 46202
	ErrInvalidProtocol        = 46203
	ErrInvalidExposure        = 46204
	ErrInvalidProxyType       = 46205
	ErrInvalidNetworkZone     = 46206
	ErrCrossProjectRef        = 46207
	ErrInvalidVolumePath      = 46208
	ErrInvalidBackupType      = 46209
	ErrInvalidBackupStatus    = 46210
	ErrDuplicateWhitelistName = 46211
	ErrInvalidVolumeMode      = 46212
	ErrContainerInWhitelist   = 46213
	ErrWhitelistInUse         = 46214
	ErrInvalidWhitelistName   = 46215
	ErrInvalidReplicas        = 46216
	ErrInvalidResourceAlloc   = 46217

	// 数据字典校验（P3）
	ErrInvalidDBType      = 46300
	ErrInvalidTableName   = 46301
	ErrDuplicateTable     = 46302
	ErrInvalidTableStatus = 46303
	ErrInvalidColumnName  = 46304
	ErrDuplicateColumn    = 46305
	ErrScanUnsupported    = 46306
	ErrScanFailed         = 46307
)

var errMsgs = map[int]string{
	ErrProjectNotFound:      "项目不存在",
	ErrRelationNotFound:     "项目关联不存在",
	ErrAssetNotFound:        "资产不存在",
	ErrInvalidProjectKey:    "无效的项目唯一键（仅允许小写字母/数字/中划线，2-64 位）",
	ErrInvalidProjectType:   "无效的项目类型（base/template/backend_module/frontend_module/frontend_base）",
	ErrDuplicateProjectKey:  "项目唯一键已存在",
	ErrInvalidRelationType:  "无效的关联类型（uses_base/uses_template/binds_backend_module/related）",
	ErrSelfRelation:         "不能将项目关联到自身",
	ErrTargetProjectMissing: "关联目标项目不存在",
	ErrProjectHasDependents: "项目存在启用中的被依赖关联，需强制删除",
	ErrInvalidAssetCategory: "无效的资产类别（requirement/code/doc/asset）",
	ErrInvalidProjectName:   "项目名称不能为空（2-128 字符）",

	ErrContainerNotFound:       "容器不存在",
	ErrResourceNotFound:        "容器资源分配不存在",
	ErrVolumeNotFound:          "卷映射不存在",
	ErrAccessPointNotFound:     "网络接入点不存在",
	ErrWhitelistNotFound:       "白名单组不存在",
	ErrWhitelistMemberNotFound: "白名单成员不存在",
	ErrBackupNotFound:          "备份记录不存在",

	ErrInvalidContainerName:   "容器名不能为空（1-128 字符）",
	ErrDuplicateContainerName: "容器名在项目内已存在",
	ErrInvalidPort:            "端口非法（listen_port 必填 1-65535，host_port 为 1-65535 或留空）",
	ErrInvalidProtocol:        "无效的协议（tcp/udp）",
	ErrInvalidExposure:        "无效的暴露范围（none/intranet/extranet）",
	ErrInvalidProxyType:       "无效的反代类型（nginx/caddy/traefik/other，留空=直连）",
	ErrInvalidNetworkZone:     "无效的网络区域（intranet/extranet/proxy_net）",
	ErrCrossProjectRef:        "引用的资源不属于同一项目",
	ErrInvalidVolumePath:      "卷映射的宿主路径与容器路径不能为空",
	ErrInvalidBackupType:      "无效的备份类型（full/db/volume）",
	ErrInvalidBackupStatus:    "无效的备份状态（success/failed/running，留空=未执行）",
	ErrDuplicateWhitelistName: "白名单组名在项目内已存在",
	ErrInvalidVolumeMode:      "无效的挂载模式（rw/ro）",
	ErrContainerInWhitelist:   "容器仍被白名单组引用，需先移除成员",
	ErrWhitelistInUse:         "白名单组仍被网络接入点引用，需先解除引用",
	ErrInvalidWhitelistName:   "白名单组名不能为空（1-128 字符）",
	ErrInvalidReplicas:        "副本数不能为负数",
	ErrInvalidResourceAlloc:   "资源分配参数非法（CPU 权重不能为负数）",

	ErrTableNotFound:      "数据表不存在",
	ErrColumnNotFound:     "字段不存在",
	ErrInvalidDBType:      "无效的数据库类型（mysql/postgres）",
	ErrInvalidTableName:   "表名非法（1-128 位，仅允许字母/数字/下划线）",
	ErrDuplicateTable:     "同一库内该表已登记",
	ErrInvalidTableStatus: "无效的表状态（active/deprecated/dropped）",
	ErrInvalidColumnName:  "字段名非法（1-128 位，仅允许字母/数字/下划线）",
	ErrDuplicateColumn:    "同表内该字段已存在",
	ErrScanUnsupported:    "当前数据库类型不支持扫描导入（本期仅支持 mysql）",
	ErrScanFailed:         "扫描 information_schema 失败",
}

// DevelopmentManagerError 模块业务错误，携带模块级错误码
type DevelopmentManagerError struct {
	Code    int
	Message string
	Detail  string // 附加说明（开发环境可见，如被依赖关联列表）
}

func (e *DevelopmentManagerError) Error() string {
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

// WithDetail 附加详细说明
func (e *DevelopmentManagerError) WithDetail(detail string) *DevelopmentManagerError {
	e.Detail = detail
	return e
}

// NewDevelopmentManagerError 按错误码创建业务错误，支持 fmt.Sprintf 参数
func NewDevelopmentManagerError(code int, args ...any) *DevelopmentManagerError {
	msg := errMsgs[code]
	if len(args) > 0 {
		msg = fmt.Sprintf(msg, args...)
	}
	return &DevelopmentManagerError{Code: code, Message: msg}
}
