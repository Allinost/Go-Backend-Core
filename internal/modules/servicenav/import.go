package servicenav

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

// importableProjectTypes 允许导入导航的项目类型（不含模块：模块挂载在基座上运行）
var importableProjectTypes = []string{"base", "frontend_base", "container_project", "template"}

// ImportFromProjectsReq 从项目部署接入点快速导入导航
type ImportFromProjectsReq struct {
	ProjectIDs     []int64  `json:"project_ids"`     // 为空表示全部可导入项目
	Types          []string `json:"types"`           // 可选：统一附加的服务类型
	UpdateExisting bool     `json:"update_existing"` // 已导入条目是否更新
}

// ImportFromProjectsResp 导入结果
type ImportFromProjectsResp struct {
	Projects     int `json:"projects"`
	AccessPoints int `json:"access_points"`
	Created      int `json:"created"`
	Updated      int `json:"updated"`
	Skipped      int `json:"skipped"`
}

// ImportCandidate 可导入的接入点行
type ImportCandidate struct {
	ProjectID     int64
	ProjectKey    string
	ProjectName   string
	ProjectType   string
	AccessPointID int64
	ContainerID   *int64
	ContainerName string
	ListenPort    int
	Purpose       string
	Exposure      string
	URL           string
}

// ListImportCandidates 查询可导入的接入点（项目未删除、接入点未删除且 url 非空、项目类型在允许范围）
func (r *Repository) ListImportCandidates(ctx context.Context, projectIDs []int64) ([]ImportCandidate, error) {
	query := `SELECT p.id, p.project_key, p.name, p.type, ap.id, ap.container_id, c.container_name,
		ap.listen_port, ap.purpose, ap.exposure, ap.url
		FROM ` + projectTable + ` p
		JOIN zzz_development_manager_access_point ap ON ap.project_id = p.id
		LEFT JOIN ` + containerTable + ` c ON c.id = ap.container_id
		WHERE p.deleted = 0 AND ap.deleted = 0 AND ap.url IS NOT NULL AND ap.url <> ''
		  AND p.type IN (?, ?, ?, ?)`
	args := []any{importableProjectTypes[0], importableProjectTypes[1], importableProjectTypes[2], importableProjectTypes[3]}
	if len(projectIDs) > 0 {
		placeholders := make([]string, len(projectIDs))
		for i, id := range projectIDs {
			placeholders[i] = "?"
			args = append(args, id)
		}
		query += " AND p.id IN (" + strings.Join(placeholders, ",") + ")"
	}
	query += " ORDER BY p.id ASC, ap.id ASC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ImportCandidate
	for rows.Next() {
		var c ImportCandidate
		var containerID sql.NullInt64
		var containerName sql.NullString
		if err := rows.Scan(&c.ProjectID, &c.ProjectKey, &c.ProjectName, &c.ProjectType, &c.AccessPointID,
			&containerID, &containerName, &c.ListenPort, &c.Purpose, &c.Exposure, &c.URL); err != nil {
			return nil, err
		}
		if containerID.Valid {
			v := containerID.Int64
			c.ContainerID = &v
		}
		c.ContainerName = containerName.String
		items = append(items, c)
	}
	return items, rows.Err()
}

// ImportFromProjects 把项目部署接入点导入为服务导航条目（同一接入点幂等）
func (s *Service) ImportFromProjects(ctx context.Context, req *ImportFromProjectsReq) (*ImportFromProjectsResp, error) {
	if len(req.Types) > 0 {
		if err := s.validateItemTypes(ctx, req.Types); err != nil {
			return nil, err
		}
	}

	candidates, err := s.repo.ListImportCandidates(ctx, req.ProjectIDs)
	if err != nil {
		return nil, err
	}

	resp := &ImportFromProjectsResp{AccessPoints: len(candidates)}
	projectSet := make(map[int64]struct{})
	for _, c := range candidates {
		projectSet[c.ProjectID] = struct{}{}
		protocol, _ := splitNavURL(c.URL)
		// 保留完整 URL 作为地址：前端 withScheme 会原样使用，避免 http 地址被默认补成 https
		address := strings.TrimSpace(c.URL)
		baseName := buildImportName(c)

		existing, err := s.repo.GetItemByAccessPointID(ctx, c.AccessPointID)
		if err != nil && err != sql.ErrNoRows {
			return nil, err
		}

		if existing != nil {
			if !req.UpdateExisting {
				resp.Skipped++
				continue
			}
			name, err := s.uniqueName(ctx, baseName, existing.ID)
			if err != nil {
				return nil, err
			}
			update := &UpdateItemReq{
				Name:          &name,
				Protocol:      &protocol,
				Address:       &address,
				ProjectID:     &c.ProjectID,
				ContainerID:   c.ContainerID,
				AccessPointID: &c.AccessPointID,
			}
			if len(req.Types) > 0 {
				update.Types = req.Types
			}
			if _, err := s.repo.UpdateItem(ctx, existing.ID, update); err != nil {
				return nil, err
			}
			resp.Updated++
			continue
		}

		name, err := s.uniqueName(ctx, baseName, "")
		if err != nil {
			return nil, err
		}
		types := req.Types
		if types == nil {
			types = []string{}
		}
		if _, err := s.repo.CreateItem(ctx, &CreateItemReq{
			Name:          name,
			Types:         types,
			Protocol:      protocol,
			Address:       address,
			Remark:        fmt.Sprintf("导入自项目 %s / 接入点 #%d", c.ProjectKey, c.AccessPointID),
			ProjectID:     &c.ProjectID,
			ContainerID:   c.ContainerID,
			AccessPointID: &c.AccessPointID,
		}); err != nil {
			return nil, err
		}
		resp.Created++
	}
	resp.Projects = len(projectSet)
	return resp, nil
}

// uniqueName 生成不冲突的条目名称（excludeID 用于更新时排除自身）
func (s *Service) uniqueName(ctx context.Context, base, excludeID string) (string, error) {
	name := truncateRunes(base, 100)
	for i := 2; i <= 8; i++ {
		item, err := s.repo.GetItemByName(ctx, name)
		if err == sql.ErrNoRows {
			return name, nil
		}
		if err != nil {
			return "", err
		}
		if excludeID != "" && item.ID == excludeID {
			return name, nil
		}
		name = truncateRunes(fmt.Sprintf("%s #%d", base, i), 100)
	}
	return truncateRunes(fmt.Sprintf("%s #%d", base, time.Now().UnixNano()), 100), nil
}

// splitNavURL 拆分完整访问地址为协议 + 地址（无协议时默认 http，地址原样保留）
func splitNavURL(raw string) (protocol, address string) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "http", raw
	}
	address = u.Host
	if u.Path != "" && u.Path != "/" {
		address += u.Path
	}
	if u.RawQuery != "" {
		address += "?" + u.RawQuery
	}
	return u.Scheme, address
}

// buildImportName 生成导入条目名称：项目名 · 用途/端口（暴露范围）
func buildImportName(c ImportCandidate) string {
	label := strings.TrimSpace(c.Purpose)
	if label == "" {
		label = fmt.Sprintf("端口 %d", c.ListenPort)
	}
	return fmt.Sprintf("%s · %s（%s）", c.ProjectName, label, exposureLabel(c.Exposure))
}

// exposureLabel 暴露范围中文
func exposureLabel(exposure string) string {
	switch exposure {
	case "extranet":
		return "公网"
	case "intranet":
		return "内网"
	case "proxy_net":
		return "反代网络"
	default:
		return "不暴露"
	}
}

// truncateRunes 按字符截断
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max])
}
