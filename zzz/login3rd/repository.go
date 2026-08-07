package login3rd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Repository 数据库操作层
type Repository struct {
	db *sql.DB
}

// NewRepository 创建 Repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ---------- Platform ----------

// ListPlatforms 列出所有平台
func (r *Repository) ListPlatforms(ctx context.Context) ([]Platform, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, category, login_type, has_auto, login_url, home_url, form_config, icon_url, created_at
		 FROM login3rd_platforms ORDER BY category, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPlatforms(rows)
}

// GetPlatform 获取单个平台
func (r *Repository) GetPlatform(ctx context.Context, id string) (*Platform, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, category, login_type, has_auto, login_url, home_url, form_config, icon_url, created_at
		 FROM login3rd_platforms WHERE id = ?`, id)
	return scanPlatform(row)
}

// SeedPlatforms 初始化平台数据
func (r *Repository) SeedPlatforms(ctx context.Context) error {
	for _, p := range seedPlatforms {
		formJSON, _ := json.Marshal(p.FormCfg)
		var formStr *string
		if string(formJSON) != "null" {
			s := string(formJSON)
			formStr = &s
		}
		_, err := r.db.ExecContext(ctx,
			`INSERT INTO login3rd_platforms
			 (id, name, category, login_type, has_auto, login_url, home_url, form_config)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)
			 ON DUPLICATE KEY UPDATE
			 has_auto    = VALUES(has_auto),
			 form_config = VALUES(form_config),
			 login_url   = VALUES(login_url),
			 home_url    = VALUES(home_url)`,
			p.ID, p.Name, p.Category, p.LoginType, p.HasAuto, p.LoginURL, p.HomeURL, formStr)
		if err != nil {
			return fmt.Errorf("seed platform %s 失败: %w", p.ID, err)
		}
	}
	return nil
}

// seedPlatforms 预置平台数据
var seedPlatforms = []struct {
	ID        string
	Name      string
	Category  string
	LoginType string
	HasAuto   bool
	LoginURL  string
	HomeURL   string
	FormCfg   *FormConfig
}{
	{
		ID: "jd", Name: "京东", Category: "shopping", LoginType: "browser", HasAuto: false,
		LoginURL: "https://passport.jd.com/new/login.aspx?ReturnUrl=https%3A%2F%2Fwww.jd.com",
		HomeURL:  "https://www.jd.com",
		FormCfg: &FormConfig{
			SuccessCookies:   []string{"pt_key", "pt_pin"},
			SuccessURLPrefix: "https://www.jd.com",
			MaxAutoRetry:     3,
		},
	},
	{
		ID: "taobao", Name: "淘宝", Category: "shopping", LoginType: "browser", HasAuto: false,
		LoginURL: "https://login.taobao.com",
		HomeURL:  "https://www.taobao.com",
	},
	{
		ID: "pdd", Name: "拼多多", Category: "shopping", LoginType: "browser", HasAuto: false,
		LoginURL: "https://mms.pinduoduo.com/login",
		HomeURL:  "https://mms.pinduoduo.com",
	},
	{
		ID: "_1688", Name: "1688", Category: "shopping", LoginType: "browser", HasAuto: true,
		LoginURL: "https://login.1688.com",
		HomeURL:  "https://www.1688.com",
		FormCfg: &FormConfig{
			UsernameSelector: `#username`,
			PasswordSelector: `#password`,
			SubmitSelector:   `#login-btn`,
			SuccessURLPrefix: "https://www.1688.com",
			MaxAutoRetry:     3,
		},
	},
	{
		ID: "tjupt", Name: "TJUPT", Category: "pt", LoginType: "browser", HasAuto: true,
		LoginURL: "https://tjupt.org/login.php",
		HomeURL:  "https://tjupt.org",
		FormCfg: &FormConfig{
			UsernameSelector: `#username`,
			PasswordSelector: `#password`,
			SubmitSelector:   `input[type="submit"]`,
			SuccessURLPrefix: "https://tjupt.org/index",
			MaxAutoRetry:     2,
		},
	},
}

// ---------- Account ----------

// CreateAccount 创建账号
func (r *Repository) CreateAccount(ctx context.Context, a *Account) (*Account, error) {
	webhookJSON, _ := json.Marshal(a.WebhookIDs)
	sessionJSON, _ := json.Marshal(a.SessionData)
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO login3rd_accounts
		 (platform_id, label, username, encrypted_password, encrypted_token, webhook_ids,
		  session_data, login_method, session_expires_at, last_verified_at, verified_status,
		  status, last_login_at, last_error)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.PlatformID, a.Label, a.Username, a.EncryptedPwd, a.EncryptedToken,
		nullJSON(webhookJSON), nullJSON(sessionJSON),
		a.LoginMethod, a.SessionExpiresAt, a.LastVerifiedAt, a.VerifiedStatus,
		a.Status, a.LastLoginAt, a.LastError)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	a.ID = id
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	return a, nil
}

// GetAccount 获取单个账号
func (r *Repository) GetAccount(ctx context.Context, id int64) (*Account, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, platform_id, label, username, encrypted_password, encrypted_token,
		       webhook_ids, session_data, login_method, session_expires_at,
		       last_verified_at, verified_status, status, last_login_at, last_error,
		       created_at, updated_at
		FROM login3rd_accounts WHERE id = ?`, id)
	return scanAccount(row)
}

// GetAccountByPlatform 按平台标识获取账号列表
func (r *Repository) GetAccountByPlatform(ctx context.Context, platformID string) ([]Account, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, platform_id, label, username, encrypted_password, encrypted_token,
		       webhook_ids, session_data, login_method, session_expires_at,
		       last_verified_at, verified_status, status, last_login_at, last_error,
		       created_at, updated_at
		FROM login3rd_accounts WHERE platform_id = ? ORDER BY id`, platformID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanAccounts(rows)
}

// ListAccounts 分页查询账号列表
func (r *Repository) ListAccounts(ctx context.Context, req *ListAccountsReq) ([]Account, bool, int, error) {
	where := "WHERE 1=1"
	args := []any{}
	if req.PlatformID != "" {
		where += " AND platform_id = ?"
		args = append(args, req.PlatformID)
	}
	if req.Status != "" {
		where += " AND status = ?"
		args = append(args, req.Status)
	}
	if req.Keyword != "" {
		where += " AND (label LIKE ? OR username LIKE ?)"
		kw := "%" + req.Keyword + "%"
		args = append(args, kw, kw)
	}

	var total int
	r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM login3rd_accounts `+where, args...).Scan(&total)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, platform_id, label, username, encrypted_password, encrypted_token,
		       webhook_ids, session_data, login_method, session_expires_at,
		       last_verified_at, verified_status, status, last_login_at, last_error,
		       created_at, updated_at
		FROM login3rd_accounts `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, req.PageSize, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	items, err := scanAccounts(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) >= req.PageSize
	return items, hasMore, total, nil
}

// UpdateAccount 更新账号信息
func (r *Repository) UpdateAccount(ctx context.Context, id int64, updates map[string]any) error {
	if len(updates) == 0 {
		return nil
	}
	setClauses := ""
	args := []any{}
	for k, v := range updates {
		if setClauses != "" {
			setClauses += ", "
		}
		setClauses += k + " = ?"
		args = append(args, v)
	}
	args = append(args, id)
	_, err := r.db.ExecContext(ctx, `UPDATE login3rd_accounts SET `+setClauses+` WHERE id = ?`, args...)
	return err
}

// DeleteAccount 删除账号
func (r *Repository) DeleteAccount(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM login3rd_accounts WHERE id = ?`, id)
	return err
}

// UpdateSession 更新 session 数据
func (r *Repository) UpdateSession(ctx context.Context, id int64, sessionData *SessionData, expiresAt *time.Time, method string) error {
	sessionJSON, _ := json.Marshal(sessionData)
	_, err := r.db.ExecContext(ctx,
		`UPDATE login3rd_accounts SET session_data = ?, login_method = ?,
		 session_expires_at = ?, last_login_at = NOW(), verified_status = 'valid', last_error = ''
		 WHERE id = ?`,
		nullJSON(sessionJSON), method, expiresAt, id)
	return err
}

// UpdateVerifiedStatus 更新验证状态
func (r *Repository) UpdateVerifiedStatus(ctx context.Context, id int64, status string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE login3rd_accounts SET verified_status = ?, last_verified_at = NOW() WHERE id = ?`,
		status, id)
	return err
}

// MarkExpired 标记账号 session 为过期
func (r *Repository) MarkExpired(ctx context.Context, id int64, errMsg string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE login3rd_accounts SET verified_status = 'expired', status = 'expired', last_error = ? WHERE id = ?`,
		errMsg, id)
	return err
}

// ---------- scanner helpers ----------

func scanPlatforms(rows *sql.Rows) ([]Platform, error) {
	var items []Platform
	for rows.Next() {
		var p Platform
		var formStr *string
		var iconURL *string
		if err := rows.Scan(&p.ID, &p.Name, &p.Category, &p.LoginType, &p.HasAuto,
			&p.LoginURL, &p.HomeURL, &formStr, &iconURL, &p.CreatedAt); err != nil {
			return nil, err
		}
		if formStr != nil && *formStr != "" {
			json.Unmarshal([]byte(*formStr), &p.FormCfg)
		}
		if iconURL != nil {
			p.IconURL = *iconURL
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

func scanPlatform(row *sql.Row) (*Platform, error) {
	var p Platform
	var formStr *string
	var iconURL *string
	if err := row.Scan(&p.ID, &p.Name, &p.Category, &p.LoginType, &p.HasAuto,
		&p.LoginURL, &p.HomeURL, &formStr, &iconURL, &p.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if formStr != nil && *formStr != "" {
		json.Unmarshal([]byte(*formStr), &p.FormCfg)
	}
	if iconURL != nil {
		p.IconURL = *iconURL
	}
	return &p, nil
}

func scanAccounts(rows *sql.Rows) ([]Account, error) {
	var items []Account
	for rows.Next() {
		a, err := scanAccountFromRow(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *a)
	}
	return items, rows.Err()
}

func scanAccount(row *sql.Row) (*Account, error) {
	var a Account
	return scanAccountFromScanner(row, &a)
}

type scanner interface {
	Scan(dest ...any) error
}

func scanAccountFromScanner(s scanner, a *Account) (*Account, error) {
	var webhookJSON, sessionJSON *string
	var loginMethod, lastError *string
	var sessionExpiresAt, lastVerifiedAt, lastLoginAt *time.Time
	if err := s.Scan(&a.ID, &a.PlatformID, &a.Label, &a.Username, &a.EncryptedPwd, &a.EncryptedToken,
		&webhookJSON, &sessionJSON, &loginMethod, &sessionExpiresAt,
		&lastVerifiedAt, &a.VerifiedStatus, &a.Status, &lastLoginAt, &lastError,
		&a.CreatedAt, &a.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if webhookJSON != nil && *webhookJSON != "" {
		json.Unmarshal([]byte(*webhookJSON), &a.WebhookIDs)
	}
	if sessionJSON != nil && *sessionJSON != "" {
		json.Unmarshal([]byte(*sessionJSON), &a.SessionData)
	}
	if loginMethod != nil {
		a.LoginMethod = *loginMethod
	}
	if sessionExpiresAt != nil {
		a.SessionExpiresAt = sessionExpiresAt
	}
	if lastVerifiedAt != nil {
		a.LastVerifiedAt = lastVerifiedAt
	}
	if lastLoginAt != nil {
		a.LastLoginAt = lastLoginAt
	}
	if lastError != nil {
		a.LastError = *lastError
	}
	return a, nil
}

func scanAccountFromRow(rows *sql.Rows) (*Account, error) {
	var a Account
	var webhookJSON, sessionJSON *string
	var loginMethod, lastError *string
	var sessionExpiresAt, lastVerifiedAt, lastLoginAt *time.Time
	if err := rows.Scan(&a.ID, &a.PlatformID, &a.Label, &a.Username, &a.EncryptedPwd, &a.EncryptedToken,
		&webhookJSON, &sessionJSON, &loginMethod, &sessionExpiresAt,
		&lastVerifiedAt, &a.VerifiedStatus, &a.Status, &lastLoginAt, &lastError,
		&a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	if webhookJSON != nil && *webhookJSON != "" {
		json.Unmarshal([]byte(*webhookJSON), &a.WebhookIDs)
	}
	if sessionJSON != nil && *sessionJSON != "" {
		json.Unmarshal([]byte(*sessionJSON), &a.SessionData)
	}
	if loginMethod != nil {
		a.LoginMethod = *loginMethod
	}
	if sessionExpiresAt != nil {
		a.SessionExpiresAt = sessionExpiresAt
	}
	if lastVerifiedAt != nil {
		a.LastVerifiedAt = lastVerifiedAt
	}
	if lastLoginAt != nil {
		a.LastLoginAt = lastLoginAt
	}
	if lastError != nil {
		a.LastError = *lastError
	}
	return &a, nil
}

// nullJSON returns the JSON string or nil for SQL NULL
func nullJSON(data []byte) any {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}
	return string(data)
}
