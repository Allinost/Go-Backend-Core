// Package servicenav 数据访问层 — MySQL

package servicenav

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Repository 数据访问层，封装 zzz_servicenav_item 与 zzz_servicenav_type 的 CRUD 操作
type Repository struct {
	db *sql.DB
}

// NewRepository 创建 Repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ---------- 服务类型 ----------

// ListTypes 查询全部服务类型，按名称升序
func (r *Repository) ListTypes(ctx context.Context) ([]ServiceType, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, name, remark, created_at, updated_at FROM `+tableType+` ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ServiceType
	for rows.Next() {
		var t ServiceType
		if err := rows.Scan(&t.ID, &t.Name, &t.Remark, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

// GetType 根据 ID 获取单个服务类型
func (r *Repository) GetType(ctx context.Context, id string) (*ServiceType, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, remark, created_at, updated_at FROM `+tableType+` WHERE id = ?`, id)
	var t ServiceType
	if err := row.Scan(&t.ID, &t.Name, &t.Remark, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTypeByName 根据名称获取单个服务类型
func (r *Repository) GetTypeByName(ctx context.Context, name string) (*ServiceType, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, name, remark, created_at, updated_at FROM `+tableType+` WHERE name = ?`, name)
	var t ServiceType
	if err := row.Scan(&t.ID, &t.Name, &t.Remark, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return nil, err
	}
	return &t, nil
}

// CreateType 创建服务类型并返回完整对象
func (r *Repository) CreateType(ctx context.Context, req *CreateTypeReq) (*ServiceType, error) {
	now := time.Now()
	t := &ServiceType{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Remark:    req.Remark,
		CreatedAt: now,
		UpdatedAt: now,
	}
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableType+` (id, name, remark, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`,
		t.ID, t.Name, t.Remark, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// UpdateType 更新服务类型（仅更新非 nil 字段），返回更新后的对象
func (r *Repository) UpdateType(ctx context.Context, id string, req *UpdateTypeReq) (*ServiceType, error) {
	t, err := r.GetType(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Name != nil {
		t.Name = *req.Name
	}
	if req.Remark != nil {
		t.Remark = *req.Remark
	}
	t.UpdatedAt = time.Now()

	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableType+` SET name = ?, remark = ?, updated_at = ? WHERE id = ?`,
		t.Name, t.Remark, t.UpdatedAt, t.ID)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteType 删除服务类型
func (r *Repository) DeleteType(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableType+` WHERE id = ?`, id)
	return err
}

// TypeNameExists 判断服务类型名称是否存在（排除指定 ID）
func (r *Repository) TypeNameExists(ctx context.Context, name, excludeID string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableType+` WHERE name = ? AND id <> ?`, name, excludeID).Scan(&count)
	return count > 0, err
}

// CountItemsByType 统计引用了指定类型名称的服务条目数量
func (r *Repository) CountItemsByType(ctx context.Context, name string) (int, error) {
	typeJSON, _ := json.Marshal(name)
	var count int
	err := r.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+tableItem+` WHERE JSON_CONTAINS(types, ?)`, string(typeJSON)).Scan(&count)
	return count, err
}

// ValidateTypes 校验类型名称集合是否都存在，返回不存在的名称列表
func (r *Repository) ValidateTypes(ctx context.Context, names []string) ([]string, error) {
	if len(names) == 0 {
		return names, nil
	}
	placeholders := make([]string, len(names))
	args := make([]any, len(names))
	for i, n := range names {
		placeholders[i] = "?"
		args[i] = n
	}
	query := fmt.Sprintf(`SELECT name FROM `+tableType+` WHERE name IN (%s)`, strings.Join(placeholders, ","))
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	found := make(map[string]struct{})
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			return nil, err
		}
		found[n] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var missing []string
	for _, n := range names {
		if _, ok := found[n]; !ok {
			missing = append(missing, n)
		}
	}
	return missing, nil
}

// ---------- 服务条目 ----------

const (
	projectTable   = "zzz_development_manager_project"
	containerTable = "zzz_development_manager_container"

	// itemSelect 服务条目查询（LEFT JOIN 项目/容器用于展示关联信息）
	itemSelect = `SELECT i.id, i.name, i.types, i.protocol, i.address, i.intranet_port, i.easytier_port,
		i.account, i.password_hint, i.remark, i.project_id, i.container_id, i.access_point_id,
		p.project_key, p.name, c.container_name, i.created_at, i.updated_at
		FROM ` + tableItem + ` i
		LEFT JOIN ` + projectTable + ` p ON p.id = i.project_id
		LEFT JOIN ` + containerTable + ` c ON c.id = i.container_id`
)

// ListItems 分页查询服务条目，支持按 keyword 模糊搜索、type 筛选、项目筛选、排序与分页
func (r *Repository) ListItems(ctx context.Context, req *ListItemsReq) ([]ServiceItem, bool, int, error) {
	var conditions []string
	var args []any

	conditions = append(conditions, "1=1")

	if req.Keyword != "" {
		conditions = append(conditions, "i.name LIKE ?")
		args = append(args, "%"+req.Keyword+"%")
	}
	if req.Type != "" {
		typeJSON, _ := json.Marshal(req.Type)
		conditions = append(conditions, "JSON_CONTAINS(i.types, ?)")
		args = append(args, string(typeJSON))
	}
	if req.ProjectID != nil {
		conditions = append(conditions, "i.project_id = ?")
		args = append(args, *req.ProjectID)
	}

	where := strings.Join(conditions, " AND ")

	var total int
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM %s i WHERE %s", tableItem, where)
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, false, 0, err
	}

	limit := req.PageSize
	offset := req.Offset()
	sortBy, sortOrder := sanitizeSort(req.SortBy, req.SortOrder)

	dataArgs := append(args, limit+1, offset)
	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf("%s WHERE %s ORDER BY i.%s %s LIMIT ? OFFSET ?", itemSelect, where, sortBy, sortOrder),
		dataArgs...)
	if err != nil {
		return nil, false, 0, err
	}
	defer rows.Close()

	items, err := scanItems(rows)
	if err != nil {
		return nil, false, 0, err
	}
	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}
	return items, hasMore, total, nil
}

// GetItem 根据 ID 获取单个服务条目
func (r *Repository) GetItem(ctx context.Context, id string) (*ServiceItem, error) {
	row := r.db.QueryRowContext(ctx, itemSelect+` WHERE i.id = ?`, id)
	return scanItem(row)
}

// GetItemByName 根据名称获取单个服务条目
func (r *Repository) GetItemByName(ctx context.Context, name string) (*ServiceItem, error) {
	row := r.db.QueryRowContext(ctx, itemSelect+` WHERE i.name = ?`, name)
	return scanItem(row)
}

// GetItemByAccessPointID 根据网络接入点 ID 查询已导入条目（用于导入幂等）
func (r *Repository) GetItemByAccessPointID(ctx context.Context, accessPointID int64) (*ServiceItem, error) {
	row := r.db.QueryRowContext(ctx, itemSelect+` WHERE i.access_point_id = ?`, accessPointID)
	return scanItem(row)
}

// CreateItem 创建服务条目并返回完整对象
func (r *Repository) CreateItem(ctx context.Context, req *CreateItemReq) (*ServiceItem, error) {
	now := time.Now()
	protocol := strings.TrimSpace(req.Protocol)
	if protocol == "" {
		protocol = "http"
	}
	item := &ServiceItem{
		ID:            uuid.New().String(),
		Name:          req.Name,
		Types:         req.Types,
		Protocol:      protocol,
		Address:       req.Address,
		IntranetPort:  req.IntranetPort,
		EasyTierPort:  req.EasyTierPort,
		Account:       req.Account,
		PasswordHint:  req.PasswordHint,
		Remark:        req.Remark,
		ProjectID:     req.ProjectID,
		ContainerID:   req.ContainerID,
		AccessPointID: req.AccessPointID,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	typesJSON, _ := json.Marshal(req.Types)
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO `+tableItem+` (id, name, types, protocol, address, intranet_port, easytier_port, account, password_hint, remark,
		 project_id, container_id, access_point_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.ID, item.Name, typesJSON, item.Protocol, item.Address, item.IntranetPort, item.EasyTierPort,
		item.Account, item.PasswordHint, item.Remark, item.ProjectID, item.ContainerID, item.AccessPointID,
		item.CreatedAt, item.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// UpdateItem 更新服务条目（仅更新非 nil 字段），返回更新后的对象
func (r *Repository) UpdateItem(ctx context.Context, id string, req *UpdateItemReq) (*ServiceItem, error) {
	item, err := r.GetItem(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		item.Name = *req.Name
	}
	if req.Types != nil {
		item.Types = req.Types
	}
	if req.Protocol != nil {
		protocol := strings.TrimSpace(*req.Protocol)
		if protocol == "" {
			protocol = "http"
		}
		item.Protocol = protocol
	}
	if req.Address != nil {
		item.Address = *req.Address
	}
	if req.ClearIntranetPort {
		item.IntranetPort = nil
	} else if req.IntranetPort != nil {
		item.IntranetPort = req.IntranetPort
	}
	if req.ClearEasyTierPort {
		item.EasyTierPort = nil
	} else if req.EasyTierPort != nil {
		item.EasyTierPort = req.EasyTierPort
	}
	if req.Account != nil {
		item.Account = *req.Account
	}
	if req.PasswordHint != nil {
		item.PasswordHint = *req.PasswordHint
	}
	if req.Remark != nil {
		item.Remark = *req.Remark
	}
	if req.ClearProject {
		item.ProjectID = nil
	} else if req.ProjectID != nil {
		item.ProjectID = req.ProjectID
	}
	if req.ClearContainer {
		item.ContainerID = nil
	} else if req.ContainerID != nil {
		item.ContainerID = req.ContainerID
	}
	if req.ClearAccessPoint {
		item.AccessPointID = nil
	} else if req.AccessPointID != nil {
		item.AccessPointID = req.AccessPointID
	}
	item.UpdatedAt = time.Now()

	typesJSON, _ := json.Marshal(item.Types)
	_, err = r.db.ExecContext(ctx,
		`UPDATE `+tableItem+` SET name = ?, types = ?, protocol = ?, address = ?, intranet_port = ?, easytier_port = ?,
		 account = ?, password_hint = ?, remark = ?, project_id = ?, container_id = ?, access_point_id = ?,
		 updated_at = ? WHERE id = ?`,
		item.Name, typesJSON, item.Protocol, item.Address, item.IntranetPort, item.EasyTierPort,
		item.Account, item.PasswordHint, item.Remark, item.ProjectID, item.ContainerID, item.AccessPointID,
		item.UpdatedAt, item.ID)
	if err != nil {
		return nil, err
	}
	return item, nil
}

// DeleteItem 删除服务条目
func (r *Repository) DeleteItem(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM `+tableItem+` WHERE id = ?`, id)
	return err
}

// ---------- 通用解析 ----------

// scanItems 遍历 rows 解析为 ServiceItem 列表
func scanItems(rows *sql.Rows) ([]ServiceItem, error) {
	var items []ServiceItem
	for rows.Next() {
		var it ServiceItem
		if err := scanItemRow(rows, &it); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// scanItem 解析单行服务条目
func scanItem(row *sql.Row) (*ServiceItem, error) {
	var it ServiceItem
	if err := scanItemRow(row, &it); err != nil {
		return nil, err
	}
	return &it, nil
}

// scanItemRow 通用行解析，支持 sql.Row 和 sql.Rows，处理 JSON 字段与可空端口
func scanItemRow(scanner interface {
	Scan(dest ...any) error
}, it *ServiceItem) error {
	var typesJSON sql.NullString
	var intranetPort, easytierPort sql.NullInt64
	var projectID, containerID, accessPointID sql.NullInt64
	var projectKey, projectName, containerName sql.NullString

	err := scanner.Scan(
		&it.ID, &it.Name, &typesJSON, &it.Protocol, &it.Address,
		&intranetPort, &easytierPort,
		&it.Account, &it.PasswordHint, &it.Remark,
		&projectID, &containerID, &accessPointID,
		&projectKey, &projectName, &containerName,
		&it.CreatedAt, &it.UpdatedAt)
	if err != nil {
		return err
	}

	if typesJSON.Valid {
		json.Unmarshal([]byte(typesJSON.String), &it.Types)
	}
	if intranetPort.Valid {
		v := int(intranetPort.Int64)
		it.IntranetPort = &v
	}
	if easytierPort.Valid {
		v := int(easytierPort.Int64)
		it.EasyTierPort = &v
	}
	if projectID.Valid {
		v := projectID.Int64
		it.ProjectID = &v
	}
	if containerID.Valid {
		v := containerID.Int64
		it.ContainerID = &v
	}
	if accessPointID.Valid {
		v := accessPointID.Int64
		it.AccessPointID = &v
	}
	it.ProjectKey = projectKey.String
	it.ProjectName = projectName.String
	it.ContainerName = containerName.String
	return nil
}

// sanitizeSort 校验排序字段白名单，防 SQL 注入，非法时回退到 name ASC
func sanitizeSort(sortBy, sortOrder string) (string, string) {
	switch sortBy {
	case "name":
		sortBy = "name"
	case "created_at":
		sortBy = "created_at"
	default:
		sortBy = "name"
	}
	switch sortOrder {
	case "asc":
		sortOrder = "ASC"
	case "desc":
		sortOrder = "DESC"
	default:
		sortOrder = "ASC"
	}
	return sortBy, sortOrder
}
