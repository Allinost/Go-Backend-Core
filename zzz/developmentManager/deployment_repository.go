package developmentmanager

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

const (
	tableContainer       = "zzz_development_manager_container"
	tableResourceAlloc   = "zzz_development_manager_resource_alloc"
	tableVolumeMapping   = "zzz_development_manager_volume_mapping"
	tableAccessPoint     = "zzz_development_manager_access_point"
	tableWhitelist       = "zzz_development_manager_whitelist"
	tableWhitelistMember = "zzz_development_manager_whitelist_member"
	tableBackup          = "zzz_development_manager_backup"

	containerColumns = "id, project_id, container_name, image, compose_ref, replicas, network_mode, " +
		"env_notes, status_note, deleted, deleted_at, created_by, updated_by, created_at, updated_at"

	resourceColumns = "id, container_id, cpu_limit, cpu_shares, mem_limit, mem_reservation, gpu_note, " +
		"storage_quota, notes, created_at, updated_at"

	volumeColumns = "id, container_id, host_path, container_path, mode, purpose, in_backup, deleted, " +
		"deleted_at, created_at, updated_at"

	accessPointColumns = "a.id, a.project_id, a.container_id, a.listen_port, a.host_port, a.protocol, a.purpose, " +
		"a.exposure, a.proxy_type, a.domain, a.upstream_url, a.tls, a.network_zone, a.url, a.reachable, " +
		"a.notes, a.whitelist_id, a.deleted, a.deleted_at, a.created_by, a.updated_by, a.created_at, a.updated_at"

	whitelistColumns = "w.id, w.project_id, w.name, w.note, w.deleted, w.deleted_at, w.created_by, " +
		"w.updated_by, w.created_at, w.updated_at, " +
		"(SELECT COUNT(*) FROM zzz_development_manager_whitelist_member m WHERE m.whitelist_id = w.id)"

	memberColumns = "m.id, m.whitelist_id, m.container_id, m.note, m.created_at, c.container_name"

	backupColumns = "id, project_id, container_id, volume_id, backup_type, strategy_cron, destination, " +
		"retention_days, last_run_at, last_status, last_error, restore_note, enabled, created_at, updated_at"
)

// ---------- 扫描 ----------

func scanContainer(row rowScanner) (*Container, error) {
	var c Container
	var env sql.NullString
	var deletedAt sql.NullTime
	if err := row.Scan(&c.ID, &c.ProjectID, &c.ContainerName, &c.Image, &c.ComposeRef, &c.Replicas,
		&c.NetworkMode, &env, &c.StatusNote, &c.Deleted, &deletedAt, &c.CreatedBy, &c.UpdatedBy,
		&c.CreatedAt, &c.UpdatedAt); err != nil {
		return nil, err
	}
	if env.Valid && env.String != "" && env.String != "null" {
		_ = json.Unmarshal([]byte(env.String), &c.EnvNotes)
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		c.DeletedAt = &t
	}
	return &c, nil
}

func scanResourceAlloc(row rowScanner) (*ResourceAlloc, error) {
	var a ResourceAlloc
	if err := row.Scan(&a.ID, &a.ContainerID, &a.CPULimit, &a.CPUShares, &a.MemLimit, &a.MemReservation,
		&a.GPUNote, &a.StorageQuota, &a.Notes, &a.CreatedAt, &a.UpdatedAt); err != nil {
		return nil, err
	}
	return &a, nil
}

func scanVolume(row rowScanner) (*VolumeMapping, error) {
	var v VolumeMapping
	var deletedAt sql.NullTime
	if err := row.Scan(&v.ID, &v.ContainerID, &v.HostPath, &v.ContainerPath, &v.Mode, &v.Purpose,
		&v.InBackup, &v.Deleted, &deletedAt, &v.CreatedAt, &v.UpdatedAt); err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		v.DeletedAt = &t
	}
	return &v, nil
}

func scanAccessPoint(row rowScanner) (*AccessPoint, error) {
	var a AccessPoint
	var containerID, hostPort, whitelistID sql.NullInt64
	var url sql.NullString
	var deletedAt sql.NullTime
	var whitelistName sql.NullString
	if err := row.Scan(&a.ID, &a.ProjectID, &containerID, &a.ListenPort, &hostPort, &a.Protocol,
		&a.Purpose, &a.Exposure, &a.ProxyType, &a.Domain, &a.UpstreamURL, &a.TLS, &a.NetworkZone,
		&url, &a.Reachable, &a.Notes, &whitelistID, &a.Deleted, &deletedAt, &a.CreatedBy, &a.UpdatedBy,
		&a.CreatedAt, &a.UpdatedAt, &whitelistName); err != nil {
		return nil, err
	}
	if containerID.Valid {
		v := containerID.Int64
		a.ContainerID = &v
	}
	if hostPort.Valid {
		v := int(hostPort.Int64)
		a.HostPort = &v
	}
	if whitelistID.Valid {
		v := whitelistID.Int64
		a.WhitelistID = &v
	}
	if url.Valid {
		v := url.String
		a.URL = &v
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		a.DeletedAt = &t
	}
	a.WhitelistName = whitelistName.String
	return &a, nil
}

func scanWhitelist(row rowScanner) (*Whitelist, error) {
	var w Whitelist
	var deletedAt sql.NullTime
	if err := row.Scan(&w.ID, &w.ProjectID, &w.Name, &w.Note, &w.Deleted, &deletedAt,
		&w.CreatedBy, &w.UpdatedBy, &w.CreatedAt, &w.UpdatedAt, &w.MemberCount); err != nil {
		return nil, err
	}
	if deletedAt.Valid {
		t := deletedAt.Time
		w.DeletedAt = &t
	}
	return &w, nil
}

func scanWhitelistMember(row rowScanner) (*WhitelistMember, error) {
	var m WhitelistMember
	var containerName sql.NullString
	if err := row.Scan(&m.ID, &m.WhitelistID, &m.ContainerID, &m.Note, &m.CreatedAt, &containerName); err != nil {
		return nil, err
	}
	m.ContainerName = containerName.String
	return &m, nil
}

func scanBackup(row rowScanner) (*Backup, error) {
	var b Backup
	var containerID, volumeID sql.NullInt64
	var lastRunAt sql.NullTime
	if err := row.Scan(&b.ID, &b.ProjectID, &containerID, &volumeID, &b.BackupType, &b.StrategyCron,
		&b.Destination, &b.RetentionDays, &lastRunAt, &b.LastStatus, &b.LastError, &b.RestoreNote,
		&b.Enabled, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}
	if containerID.Valid {
		v := containerID.Int64
		b.ContainerID = &v
	}
	if volumeID.Valid {
		v := volumeID.Int64
		b.VolumeID = &v
	}
	if lastRunAt.Valid {
		t := lastRunAt.Time
		b.LastRunAt = &t
	}
	return &b, nil
}

// ---------- 容器 ----------

// ListContainers 查询项目下容器
func (r *Repository) ListContainers(ctx context.Context, projectID int64, includeDeleted bool) ([]Container, error) {
	query := "SELECT " + containerColumns + " FROM " + tableContainer + " WHERE project_id = ?"
	if !includeDeleted {
		query += " AND deleted = 0"
	}
	query += " ORDER BY id ASC"
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Container
	for rows.Next() {
		c, err := scanContainer(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *c)
	}
	return items, rows.Err()
}

// GetContainer 按 ID 查询容器（含已删除，由 deleted 字段区分）
func (r *Repository) GetContainer(ctx context.Context, id int64) (*Container, error) {
	c, err := scanContainer(r.db.QueryRowContext(ctx,
		"SELECT "+containerColumns+" FROM "+tableContainer+" WHERE id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrContainerNotFound)
		}
		return nil, err
	}
	return c, nil
}

// GetContainerByName 查询项目内未删除的同名容器
func (r *Repository) GetContainerByName(ctx context.Context, projectID int64, name string) (*Container, error) {
	c, err := scanContainer(r.db.QueryRowContext(ctx,
		"SELECT "+containerColumns+" FROM "+tableContainer+" WHERE project_id = ? AND container_name = ? AND deleted = 0",
		projectID, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrContainerNotFound)
		}
		return nil, err
	}
	return c, nil
}

// CreateContainer 新建容器
func (r *Repository) CreateContainer(ctx context.Context, c *Container) (*Container, error) {
	env := "null"
	if len(c.EnvNotes) > 0 {
		b, _ := json.Marshal(c.EnvNotes)
		env = string(b)
	}
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableContainer+
			" (project_id, container_name, image, compose_ref, replicas, network_mode, env_notes, status_note, created_by, updated_by)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		c.ProjectID, c.ContainerName, c.Image, c.ComposeRef, c.Replicas, c.NetworkMode, env, c.StatusNote,
		c.CreatedBy, c.UpdatedBy)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetContainer(ctx, id)
}

// UpdateContainer 全量更新容器
func (r *Repository) UpdateContainer(ctx context.Context, c *Container) (*Container, error) {
	env := "null"
	if len(c.EnvNotes) > 0 {
		b, _ := json.Marshal(c.EnvNotes)
		env = string(b)
	}
	if _, err := r.db.ExecContext(ctx,
		"UPDATE "+tableContainer+" SET container_name=?, image=?, compose_ref=?, replicas=?, network_mode=?, "+
			"env_notes=?, status_note=?, updated_by=? WHERE id=? AND deleted=0",
		c.ContainerName, c.Image, c.ComposeRef, c.Replicas, c.NetworkMode, env, c.StatusNote, c.UpdatedBy, c.ID); err != nil {
		return nil, err
	}
	return r.GetContainer(ctx, c.ID)
}

// DeleteContainerTx 事务内软删除容器，并级联软删除其卷映射与网络接入点
func (r *Repository) DeleteContainerTx(ctx context.Context, containerID int64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"UPDATE "+tableContainer+" SET deleted=1, deleted_at=? WHERE id=? AND deleted=0", now, containerID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE "+tableVolumeMapping+" SET deleted=1, deleted_at=? WHERE container_id=? AND deleted=0", now, containerID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE "+tableAccessPoint+" SET deleted=1, deleted_at=? WHERE container_id=? AND deleted=0", now, containerID); err != nil {
		return err
	}
	return tx.Commit()
}

// WhitelistGroupsOfContainer 查询引用了该容器的白名单组名（用于删除前提示）
func (r *Repository) WhitelistGroupsOfContainer(ctx context.Context, containerID int64) ([]string, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT w.name FROM "+tableWhitelistMember+" m JOIN "+tableWhitelist+" w ON w.id = m.whitelist_id "+
			"WHERE m.container_id = ? AND w.deleted = 0 ORDER BY w.name", containerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// ---------- 资源分配 ----------

// GetResourceAlloc 查询容器资源分配
func (r *Repository) GetResourceAlloc(ctx context.Context, containerID int64) (*ResourceAlloc, error) {
	a, err := scanResourceAlloc(r.db.QueryRowContext(ctx,
		"SELECT "+resourceColumns+" FROM "+tableResourceAlloc+" WHERE container_id = ?", containerID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrResourceNotFound)
		}
		return nil, err
	}
	return a, nil
}

// UpsertResourceAlloc 写入容器资源分配（1:1，存在则更新）
func (r *Repository) UpsertResourceAlloc(ctx context.Context, a *ResourceAlloc) (*ResourceAlloc, error) {
	if _, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableResourceAlloc+
			" (container_id, cpu_limit, cpu_shares, mem_limit, mem_reservation, gpu_note, storage_quota, notes)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?)"+
			" ON DUPLICATE KEY UPDATE cpu_limit=VALUES(cpu_limit), cpu_shares=VALUES(cpu_shares),"+
			" mem_limit=VALUES(mem_limit), mem_reservation=VALUES(mem_reservation), gpu_note=VALUES(gpu_note),"+
			" storage_quota=VALUES(storage_quota), notes=VALUES(notes)",
		a.ContainerID, a.CPULimit, a.CPUShares, a.MemLimit, a.MemReservation, a.GPUNote, a.StorageQuota, a.Notes); err != nil {
		return nil, err
	}
	return r.GetResourceAlloc(ctx, a.ContainerID)
}

// ---------- 卷映射 ----------

// ListVolumes 查询容器卷映射
func (r *Repository) ListVolumes(ctx context.Context, containerID int64, includeDeleted bool) ([]VolumeMapping, error) {
	query := "SELECT " + volumeColumns + " FROM " + tableVolumeMapping + " WHERE container_id = ?"
	if !includeDeleted {
		query += " AND deleted = 0"
	}
	query += " ORDER BY id ASC"
	rows, err := r.db.QueryContext(ctx, query, containerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []VolumeMapping
	for rows.Next() {
		v, err := scanVolume(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *v)
	}
	return items, rows.Err()
}

// GetVolume 按 ID 查询卷映射
func (r *Repository) GetVolume(ctx context.Context, id int64) (*VolumeMapping, error) {
	v, err := scanVolume(r.db.QueryRowContext(ctx,
		"SELECT "+volumeColumns+" FROM "+tableVolumeMapping+" WHERE id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrVolumeNotFound)
		}
		return nil, err
	}
	return v, nil
}

// CreateVolume 新建卷映射
func (r *Repository) CreateVolume(ctx context.Context, v *VolumeMapping) (*VolumeMapping, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableVolumeMapping+
			" (container_id, host_path, container_path, mode, purpose, in_backup) VALUES (?, ?, ?, ?, ?, ?)",
		v.ContainerID, v.HostPath, v.ContainerPath, v.Mode, v.Purpose, v.InBackup)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetVolume(ctx, id)
}

// UpdateVolume 更新卷映射
func (r *Repository) UpdateVolume(ctx context.Context, v *VolumeMapping) (*VolumeMapping, error) {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE "+tableVolumeMapping+" SET host_path=?, container_path=?, mode=?, purpose=?, in_backup=? "+
			"WHERE id=? AND deleted=0",
		v.HostPath, v.ContainerPath, v.Mode, v.Purpose, v.InBackup, v.ID); err != nil {
		return nil, err
	}
	return r.GetVolume(ctx, v.ID)
}

// SoftDeleteVolume 软删除卷映射
func (r *Repository) SoftDeleteVolume(ctx context.Context, id int64, now time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE "+tableVolumeMapping+" SET deleted=1, deleted_at=? WHERE id=? AND deleted=0", now, id)
	return err
}

// ---------- 网络接入点 ----------

// ListAccessPoints 查询项目网络接入点（可按暴露范围/网络区域筛选）
func (r *Repository) ListAccessPoints(ctx context.Context, projectID int64, req *ListDeploymentReq) ([]AccessPoint, error) {
	query := "SELECT " + accessPointColumns + ", wl.name FROM " + tableAccessPoint + " a " +
		"LEFT JOIN " + tableWhitelist + " wl ON wl.id = a.whitelist_id WHERE a.project_id = ?"
	args := []any{projectID}
	if !req.IncludeDeleted() {
		query += " AND a.deleted = 0"
	}
	if req.Exposure != "" {
		query += " AND a.exposure = ?"
		args = append(args, req.Exposure)
	}
	if req.Zone != "" {
		query += " AND a.network_zone = ?"
		args = append(args, req.Zone)
	}
	query += " ORDER BY a.listen_port ASC, a.id ASC"
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []AccessPoint
	for rows.Next() {
		a, err := scanAccessPoint(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *a)
	}
	return items, rows.Err()
}

// GetAccessPoint 按 ID 查询接入点（含白名单组名）
func (r *Repository) GetAccessPoint(ctx context.Context, id int64) (*AccessPoint, error) {
	query := "SELECT " + accessPointColumns + ", wl.name FROM " + tableAccessPoint + " a " +
		"LEFT JOIN " + tableWhitelist + " wl ON wl.id = a.whitelist_id WHERE a.id = ?"
	a, err := scanAccessPoint(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrAccessPointNotFound)
		}
		return nil, err
	}
	return a, nil
}

// CreateAccessPoint 新建接入点
func (r *Repository) CreateAccessPoint(ctx context.Context, a *AccessPoint) (*AccessPoint, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableAccessPoint+
			" (project_id, container_id, listen_port, host_port, protocol, purpose, exposure, proxy_type, domain, upstream_url, tls, network_zone, url, reachable, notes, whitelist_id, created_by, updated_by)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		a.ProjectID, a.ContainerID, a.ListenPort, a.HostPort, a.Protocol, a.Purpose, a.Exposure,
		a.ProxyType, a.Domain, a.UpstreamURL, a.TLS, a.NetworkZone, a.URL, a.Reachable, a.Notes,
		a.WhitelistID, a.CreatedBy, a.UpdatedBy)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetAccessPoint(ctx, id)
}

// UpdateAccessPoint 全量更新接入点
func (r *Repository) UpdateAccessPoint(ctx context.Context, a *AccessPoint) (*AccessPoint, error) {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE "+tableAccessPoint+" SET container_id=?, listen_port=?, host_port=?, protocol=?, purpose=?, "+
			"exposure=?, proxy_type=?, domain=?, upstream_url=?, tls=?, network_zone=?, url=?, reachable=?, "+
			"notes=?, whitelist_id=?, updated_by=? WHERE id=? AND deleted=0",
		a.ContainerID, a.ListenPort, a.HostPort, a.Protocol, a.Purpose, a.Exposure, a.ProxyType,
		a.Domain, a.UpstreamURL, a.TLS, a.NetworkZone, a.URL, a.Reachable, a.Notes, a.WhitelistID,
		a.UpdatedBy, a.ID); err != nil {
		return nil, err
	}
	return r.GetAccessPoint(ctx, a.ID)
}

// SoftDeleteAccessPoint 软删除接入点
func (r *Repository) SoftDeleteAccessPoint(ctx context.Context, id int64, now time.Time) error {
	_, err := r.db.ExecContext(ctx,
		"UPDATE "+tableAccessPoint+" SET deleted=1, deleted_at=? WHERE id=? AND deleted=0", now, id)
	return err
}

// ---------- 白名单 ----------

// ListWhitelists 查询项目白名单组（含成员数）
func (r *Repository) ListWhitelists(ctx context.Context, projectID int64, includeDeleted bool) ([]Whitelist, error) {
	query := "SELECT " + whitelistColumns + " FROM " + tableWhitelist + " w WHERE w.project_id = ?"
	if !includeDeleted {
		query += " AND w.deleted = 0"
	}
	query += " ORDER BY w.id ASC"
	rows, err := r.db.QueryContext(ctx, query, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Whitelist
	for rows.Next() {
		w, err := scanWhitelist(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *w)
	}
	return items, rows.Err()
}

// GetWhitelist 按 ID 查询白名单组
func (r *Repository) GetWhitelist(ctx context.Context, id int64) (*Whitelist, error) {
	w, err := scanWhitelist(r.db.QueryRowContext(ctx,
		"SELECT "+whitelistColumns+" FROM "+tableWhitelist+" w WHERE w.id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrWhitelistNotFound)
		}
		return nil, err
	}
	return w, nil
}

// GetWhitelistByName 查询项目内未删除的同名白名单组
func (r *Repository) GetWhitelistByName(ctx context.Context, projectID int64, name string) (*Whitelist, error) {
	w, err := scanWhitelist(r.db.QueryRowContext(ctx,
		"SELECT "+whitelistColumns+" FROM "+tableWhitelist+" w WHERE w.project_id = ? AND w.name = ? AND w.deleted = 0",
		projectID, name))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrWhitelistNotFound)
		}
		return nil, err
	}
	return w, nil
}

// CreateWhitelist 新建白名单组
func (r *Repository) CreateWhitelist(ctx context.Context, w *Whitelist) (*Whitelist, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableWhitelist+" (project_id, name, note, created_by, updated_by) VALUES (?, ?, ?, ?, ?)",
		w.ProjectID, w.Name, w.Note, w.CreatedBy, w.UpdatedBy)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetWhitelist(ctx, id)
}

// UpdateWhitelist 更新白名单组
func (r *Repository) UpdateWhitelist(ctx context.Context, w *Whitelist) (*Whitelist, error) {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE "+tableWhitelist+" SET name=?, note=?, updated_by=? WHERE id=? AND deleted=0",
		w.Name, w.Note, w.UpdatedBy, w.ID); err != nil {
		return nil, err
	}
	return r.GetWhitelist(ctx, w.ID)
}

// DeleteWhitelistTx 事务内删除白名单成员并软删除组
func (r *Repository) DeleteWhitelistTx(ctx context.Context, whitelistID int64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx,
		"DELETE FROM "+tableWhitelistMember+" WHERE whitelist_id = ?", whitelistID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"UPDATE "+tableWhitelist+" SET deleted=1, deleted_at=? WHERE id=? AND deleted=0", now, whitelistID); err != nil {
		return err
	}
	return tx.Commit()
}

// CountAccessPointRefs 统计引用该白名单组的未删除接入点数
func (r *Repository) CountAccessPointRefs(ctx context.Context, whitelistID int64) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM "+tableAccessPoint+" WHERE whitelist_id = ? AND deleted = 0", whitelistID).Scan(&n)
	return n, err
}

// ListWhitelistMembers 查询白名单组成员（含容器名）
func (r *Repository) ListWhitelistMembers(ctx context.Context, whitelistID int64) ([]WhitelistMember, error) {
	query := "SELECT " + memberColumns + " FROM " + tableWhitelistMember + " m " +
		"LEFT JOIN " + tableContainer + " c ON c.id = m.container_id WHERE m.whitelist_id = ? ORDER BY m.id ASC"
	rows, err := r.db.QueryContext(ctx, query, whitelistID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []WhitelistMember
	for rows.Next() {
		m, err := scanWhitelistMember(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *m)
	}
	return items, rows.Err()
}

// GetWhitelistMember 按 ID 查询白名单成员
func (r *Repository) GetWhitelistMember(ctx context.Context, id int64) (*WhitelistMember, error) {
	query := "SELECT " + memberColumns + " FROM " + tableWhitelistMember + " m " +
		"LEFT JOIN " + tableContainer + " c ON c.id = m.container_id WHERE m.id = ?"
	m, err := scanWhitelistMember(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrWhitelistMemberNotFound)
		}
		return nil, err
	}
	return m, nil
}

// WhitelistMemberExists 判断组内是否已有该容器
func (r *Repository) WhitelistMemberExists(ctx context.Context, whitelistID, containerID int64) (bool, error) {
	var n int
	err := r.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM "+tableWhitelistMember+" WHERE whitelist_id=? AND container_id=?",
		whitelistID, containerID).Scan(&n)
	return n > 0, err
}

// CreateWhitelistMember 添加白名单成员
func (r *Repository) CreateWhitelistMember(ctx context.Context, m *WhitelistMember) (*WhitelistMember, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableWhitelistMember+" (whitelist_id, container_id, note) VALUES (?, ?, ?)",
		m.WhitelistID, m.ContainerID, m.Note)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return nil, NewDevelopmentManagerError(ErrContainerInWhitelist)
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetWhitelistMember(ctx, id)
}

// DeleteWhitelistMember 删除白名单成员（硬删除）
func (r *Repository) DeleteWhitelistMember(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx,
		"DELETE FROM "+tableWhitelistMember+" WHERE id = ?", id); err != nil {
		return err
	}
	return nil
}

// ---------- 备份 ----------

// ListBackups 查询项目备份记录
func (r *Repository) ListBackups(ctx context.Context, projectID int64) ([]Backup, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+backupColumns+" FROM "+tableBackup+" WHERE project_id = ? ORDER BY id DESC", projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Backup
	for rows.Next() {
		b, err := scanBackup(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *b)
	}
	return items, rows.Err()
}

// GetBackup 按 ID 查询备份记录
func (r *Repository) GetBackup(ctx context.Context, id int64) (*Backup, error) {
	b, err := scanBackup(r.db.QueryRowContext(ctx,
		"SELECT "+backupColumns+" FROM "+tableBackup+" WHERE id = ?", id))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, NewDevelopmentManagerError(ErrBackupNotFound)
		}
		return nil, err
	}
	return b, nil
}

// CreateBackup 新建备份记录
func (r *Repository) CreateBackup(ctx context.Context, b *Backup) (*Backup, error) {
	res, err := r.db.ExecContext(ctx,
		"INSERT INTO "+tableBackup+
			" (project_id, container_id, volume_id, backup_type, strategy_cron, destination, retention_days, restore_note, enabled)"+
			" VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		b.ProjectID, b.ContainerID, b.VolumeID, b.BackupType, b.StrategyCron, b.Destination,
		b.RetentionDays, b.RestoreNote, b.Enabled)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return r.GetBackup(ctx, id)
}

// UpdateBackup 更新备份记录（含最近执行结果）
func (r *Repository) UpdateBackup(ctx context.Context, b *Backup) (*Backup, error) {
	if _, err := r.db.ExecContext(ctx,
		"UPDATE "+tableBackup+" SET container_id=?, volume_id=?, backup_type=?, strategy_cron=?, destination=?, "+
			"retention_days=?, last_run_at=?, last_status=?, last_error=?, restore_note=?, enabled=? WHERE id=?",
		b.ContainerID, b.VolumeID, b.BackupType, b.StrategyCron, b.Destination, b.RetentionDays,
		b.LastRunAt, b.LastStatus, b.LastError, b.RestoreNote, b.Enabled, b.ID); err != nil {
		return nil, err
	}
	return r.GetBackup(ctx, b.ID)
}

// DeleteBackup 删除备份记录（硬删除）
func (r *Repository) DeleteBackup(ctx context.Context, id int64) error {
	if _, err := r.db.ExecContext(ctx, "DELETE FROM "+tableBackup+" WHERE id = ?", id); err != nil {
		return err
	}
	return nil
}
