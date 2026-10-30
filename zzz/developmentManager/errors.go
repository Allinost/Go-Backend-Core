package developmentmanager

import "fmt"

// 模块业务错误码（developmentManager 占用 46001+ 区段）
const (
	ErrProjectNotFound   = 46001
	ErrRelationNotFound  = 46002
	ErrAssetNotFound     = 46003
	ErrInvalidProjectKey = 46100
	ErrInvalidProjectType = 46101
	ErrDuplicateProjectKey = 46102
	ErrInvalidRelationType = 46103
	ErrSelfRelation        = 46104
	ErrTargetProjectMissing = 46105
	ErrProjectHasDependents = 46106
	ErrInvalidAssetCategory = 46107
	ErrInvalidProjectName   = 46108
)

var errMsgs = map[int]string{
	ErrProjectNotFound:      "项目不存在",
	ErrRelationNotFound:     "项目关联不存在",
	ErrAssetNotFound:        "资产不存在",
	ErrInvalidProjectKey:    "无效的项目唯一键（仅允许小写字母/数字/中划线，2-64 位）",
	ErrInvalidProjectType:   "无效的项目类型（base/template/backend_module/frontend_module）",
	ErrDuplicateProjectKey:  "项目唯一键已存在",
	ErrInvalidRelationType:  "无效的关联类型（uses_base/uses_template/binds_backend_module/related）",
	ErrSelfRelation:         "不能将项目关联到自身",
	ErrTargetProjectMissing: "关联目标项目不存在",
	ErrProjectHasDependents: "项目存在启用中的被依赖关联，需强制删除",
	ErrInvalidAssetCategory: "无效的资产类别（requirement/code/doc/asset）",
	ErrInvalidProjectName:   "项目名称不能为空（2-128 字符）",
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
