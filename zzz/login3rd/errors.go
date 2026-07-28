package login3rd

import "errors"

// Login3rdError 模块自定义错误
type Login3rdError struct {
	Code    int
	Message string
	Err     error
}

func (e *Login3rdError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *Login3rdError) Unwrap() error {
	return e.Err
}

// 错误码
const (
	ErrCodePlatformNotFound = iota + 1001
	ErrCodeAccountNotFound
	ErrCodeAccountDisabled
	ErrCodeSessionExpired
	ErrCodeAutoLoginNotSupported
	ErrCodeAutoLoginFailed
	ErrCodeBrowserLoginFailed
	ErrCodeSessionVerifyFailed
	ErrCodeRemoteSessionCreateFailed
	ErrCodeRemoteSessionNotFound
	ErrCodeRemoteSessionExpired
	ErrCodeWebhookSendFailed
	ErrCodeEncryptFailed
	ErrCodeDecryptFailed
)

// 错误构造函数
func errPlatformNotFound() *Login3rdError {
	return &Login3rdError{Code: ErrCodePlatformNotFound, Message: "平台不存在"}
}

func errAccountNotFound() *Login3rdError {
	return &Login3rdError{Code: ErrCodeAccountNotFound, Message: "账号不存在"}
}

func errAccountDisabled() *Login3rdError {
	return &Login3rdError{Code: ErrCodeAccountDisabled, Message: "账号已禁用"}
}

func errSessionExpired() *Login3rdError {
	return &Login3rdError{Code: ErrCodeSessionExpired, Message: "登录会话已过期"}
}

func errAutoLoginNotSupported() *Login3rdError {
	return &Login3rdError{Code: ErrCodeAutoLoginNotSupported, Message: "该平台不支持自动登录"}
}

func errAutoLoginFailed(err error) *Login3rdError {
	return &Login3rdError{Code: ErrCodeAutoLoginFailed, Message: "自动登录失败", Err: err}
}

func errBrowserLoginFailed(err error) *Login3rdError {
	return &Login3rdError{Code: ErrCodeBrowserLoginFailed, Message: "浏览器登录失败", Err: err}
}

func errRemoteSessionCreateFailed(err error) *Login3rdError {
	return &Login3rdError{Code: ErrCodeRemoteSessionCreateFailed, Message: "创建远程浏览器会话失败", Err: err}
}

func errRemoteSessionNotFound() *Login3rdError {
	return &Login3rdError{Code: ErrCodeRemoteSessionNotFound, Message: "远程浏览器会话不存在"}
}

func errRemoteSessionExpired() *Login3rdError {
	return &Login3rdError{Code: ErrCodeRemoteSessionExpired, Message: "远程浏览器会话已过期"}
}

func errEncryptFailed(err error) *Login3rdError {
	return &Login3rdError{Code: ErrCodeEncryptFailed, Message: "加密失败", Err: err}
}

func errDecryptFailed(err error) *Login3rdError {
	return &Login3rdError{Code: ErrCodeDecryptFailed, Message: "解密失败", Err: err}
}

// 标准错误
var (
	ErrNoMySQLConnection = errors.New("login3rd: MySQL 连接不可用")
	ErrNoWebhookModule   = errors.New("login3rd: webhook 模块未注册")
)
