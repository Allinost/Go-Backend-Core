package ops

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
)

const tableHost = "zzz_ops_host"

type HostRepository struct {
	db *sql.DB
}

func NewHostRepository(db *sql.DB) *HostRepository {
	return &HostRepository{db: db}
}

const hostColumns = `id, name, role, environment, address, ssh_port, username, auth_type,
	credential_ciphertext, credential_hint, os, tags, notes, status, last_checked_at, last_error,
	deleted, created_by, updated_by, created_at, updated_at`

func scanHost(row interface{ Scan(dest ...any) error }) (*Host, error) {
	var h Host
	var tags []byte
	var lastChecked sql.NullTime
	if err := row.Scan(&h.ID, &h.Name, &h.Role, &h.Environment, &h.Address, &h.SSHPort, &h.Username,
		&h.AuthType, &h.credentialCiphertext, &h.CredentialHint, &h.OS, &tags, &h.Notes,
		&h.Status, &lastChecked, &h.LastError, &h.Deleted, &h.CreatedBy, &h.UpdatedBy, &h.CreatedAt, &h.UpdatedAt); err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		_ = json.Unmarshal(tags, &h.Tags)
	}
	if lastChecked.Valid {
		t := lastChecked.Time
		h.LastCheckedAt = &t
	}
	h.HasCredential = h.credentialCiphertext != ""
	return &h, nil
}

func (r *HostRepository) List(ctx context.Context, req *ListHostsReq) ([]Host, bool, int, error) {
	req.Normalize()
	conditions := []string{"deleted = false"}
	args := []any{}
	if req.Role != "" {
		conditions = append(conditions, "role = ?")
		args = append(args, req.Role)
	}
	if req.Environment != "" {
		conditions = append(conditions, "environment = ?")
		args = append(args, req.Environment)
	}
	if req.Keyword != "" {
		like := "%" + req.Keyword + "%"
		conditions = append(conditions, "(name ILIKE ? OR address ILIKE ? OR notes ILIKE ?)")
		args = append(args, like, like, like)
	}
	where := strings.Join(conditions, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, dbutil.Rebind("SELECT COUNT(*) FROM "+tableHost+" WHERE "+where), args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}
	query := "SELECT " + hostColumns + " FROM " + tableHost + " WHERE " + where + " ORDER BY id DESC LIMIT ? OFFSET ?"
	rows, err := r.db.QueryContext(ctx, dbutil.Rebind(query), append(args, req.PageSize+1, req.Offset())...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()
	var items []Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, false, 0, err
		}
		items = append(items, *h)
	}
	if err := rows.Err(); err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > req.PageSize
	if hasMore {
		items = items[:req.PageSize]
	}
	return items, hasMore, total, nil
}

func (r *HostRepository) Get(ctx context.Context, id int64) (*Host, error) {
	h, err := scanHost(r.db.QueryRowContext(ctx, dbutil.Rebind("SELECT "+hostColumns+" FROM "+tableHost+" WHERE id = ?"), id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrHostNotFound
	}
	return h, err
}

func (r *HostRepository) NameExists(ctx context.Context, name string, excludeID int64) (bool, error) {
	var exists bool
	err := r.db.QueryRowContext(ctx, dbutil.Rebind(`SELECT EXISTS(SELECT 1 FROM `+tableHost+` WHERE name = ? AND id <> ? AND deleted = false)`), name, excludeID).Scan(&exists)
	return exists, err
}

func (r *HostRepository) Create(ctx context.Context, h *Host) (*Host, error) {
	tags, _ := json.Marshal(h.Tags)
	var id int64
	err := r.db.QueryRowContext(ctx, dbutil.Rebind(`INSERT INTO `+tableHost+`
		(name, role, environment, address, ssh_port, username, auth_type, credential_ciphertext, credential_hint, os, tags, notes, status, created_by, updated_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`),
		h.Name, h.Role, h.Environment, h.Address, h.SSHPort, h.Username, h.AuthType,
		h.credentialCiphertext, h.CredentialHint, h.OS, string(tags), h.Notes, h.Status, h.CreatedBy, h.UpdatedBy).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrHostExists
		}
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *HostRepository) Update(ctx context.Context, id int64, updates map[string]any, actor string) (*Host, error) {
	allowed := map[string]bool{
		"name": true, "role": true, "environment": true, "address": true, "ssh_port": true,
		"username": true, "auth_type": true, "credential_ciphertext": true, "credential_hint": true,
		"os": true, "tags": true, "notes": true,
	}
	keys := make([]string, 0, len(updates))
	for k := range updates {
		if allowed[k] {
			keys = append(keys, k)
		}
	}
	if len(keys) == 0 {
		return r.Get(ctx, id)
	}
	sort.Strings(keys)
	sets := make([]string, 0, len(keys)+2)
	args := make([]any, 0, len(keys)+2)
	for _, k := range keys {
		sets = append(sets, k+" = ?")
		args = append(args, updates[k])
	}
	sets = append(sets, "updated_by = ?", "updated_at = now()")
	args = append(args, actor, id)
	res, err := r.db.ExecContext(ctx, dbutil.Rebind("UPDATE "+tableHost+" SET "+strings.Join(sets, ", ")+" WHERE id = ? AND deleted = false"), args...)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrHostExists
		}
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrHostNotFound
	}
	return r.Get(ctx, id)
}

func (r *HostRepository) SoftDelete(ctx context.Context, id int64, actor string) error {
	res, err := r.db.ExecContext(ctx, dbutil.Rebind(`UPDATE `+tableHost+` SET deleted = true, updated_by = ?, updated_at = now() WHERE id = ? AND deleted = false`), actor, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrHostNotFound
	}
	return nil
}

func (r *HostRepository) UpdateStatus(ctx context.Context, id int64, status, lastError string, checkedAt time.Time) error {
	_, err := r.db.ExecContext(ctx, dbutil.Rebind(`UPDATE `+tableHost+`
		SET status = ?, last_error = ?, last_checked_at = ?, updated_at = now() WHERE id = ?`),
		status, lastError, checkedAt, id)
	return err
}

var (
	ErrHostNotFound   = errors.New("主机不存在")
	ErrHostExists     = errors.New("主机名已存在")
	ErrHostInvalid    = errors.New("主机参数非法")
	ErrHostCredential = errors.New("主机凭据不可用")
)

func hostValidationError(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrHostInvalid, fmt.Sprintf(format, args...))
}
