package developmentmanager

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// maxExpandFiles 单次目录展开的文件上限，避免误扫大目录
const maxExpandFiles = 2000

// ExpandAssetReq 目录展开请求
type ExpandAssetReq struct {
	AssetID int64 `json:"asset_id" binding:"required"`
}

// ExpandAssetResp 目录展开结果
type ExpandAssetResp struct {
	AssetID int64 `json:"asset_id,omitempty"`
	Scanned int   `json:"scanned"`
	Created int   `json:"created"`
	Skipped int   `json:"skipped"`
	Failed  int   `json:"failed"`
}

// ExpandAssetDirectory 把一个目录型资产展开为文件级资产索引（幂等：已登记 URI 跳过）
func (s *Service) ExpandAssetDirectory(ctx context.Context, projectID, assetID int64) (*ExpandAssetResp, error) {
	project, err := s.requireProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	asset, err := s.repo.GetAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	if asset.ProjectID != projectID {
		return nil, NewDevelopmentManagerError(ErrCrossProjectRef)
	}
	if asset.Deleted {
		return nil, NewDevelopmentManagerError(ErrAssetNotFound)
	}

	resp := &ExpandAssetResp{AssetID: assetID}
	if err := s.expandAssetDir(ctx, project, asset, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// ExpandProjectDirectories 展开项目下全部目录型资产
func (s *Service) ExpandProjectDirectories(ctx context.Context, projectID int64) (*ExpandAssetResp, error) {
	project, err := s.requireProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	assets, err := s.repo.ListDirectoryAssets(ctx, projectID)
	if err != nil {
		return nil, err
	}

	resp := &ExpandAssetResp{}
	for _, asset := range assets {
		if err := s.expandAssetDir(ctx, project, &asset, resp); err != nil {
			resp.Failed++
		}
	}
	return resp, nil
}

// expandAssetDir 扫描目录并登记文件级资产
func (s *Service) expandAssetDir(ctx context.Context, project *Project, asset *ProjectAsset, resp *ExpandAssetResp) error {
	dir, err := resolveAssetDir(project, asset.URI)
	if err != nil {
		return NewDevelopmentManagerError(ErrAssetDirectoryInvalid).WithDetail(err.Error())
	}

	parentURI := strings.TrimSuffix(strings.ReplaceAll(asset.URI, "\\", "/"), "/") + "/"
	fileCount := 0

	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() {
			if path != dir {
				name := d.Name()
				if name == "node_modules" || name == "dist" || name == ".git" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
			}
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		if fileCount >= maxExpandFiles {
			return fmt.Errorf("目录文件数超过上限 %d", maxExpandFiles)
		}
		fileCount++
		resp.Scanned++

		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		uri := parentURI + rel

		exists, err := s.repo.AssetExists(ctx, project.ID, uri)
		if err != nil {
			return err
		}
		if exists {
			resp.Skipped++
			return nil
		}

		format := strings.TrimPrefix(filepath.Ext(rel), ".")
		if _, err := s.repo.CreateAsset(ctx, &ProjectAsset{
			ProjectID: project.ID,
			Category:  asset.Category,
			Title:     rel,
			URI:       uri,
			Format:    format,
			Size:      info.Size(),
			Note:      "目录展开导入：" + asset.Title,
		}); err != nil {
			resp.Failed++
			return nil
		}
		resp.Created++
		return nil
	})
	if walkErr != nil {
		return NewDevelopmentManagerError(ErrAssetDirectoryInvalid).WithDetail(walkErr.Error())
	}
	return nil
}

// resolveAssetDir 解析目录型资产的真实路径：
// 先按后端仓库根目录解析，再按项目 repo_path 解析（兼容 NKUI 等兄弟仓库），并限制不得越出根目录
func resolveAssetDir(project *Project, uri string) (string, error) {
	rel := strings.TrimSpace(strings.ReplaceAll(uri, "\\", "/"))
	rel = strings.TrimSuffix(rel, "/")
	if rel == "" {
		return "", fmt.Errorf("资产 URI 为空")
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("非法目录路径：%s", uri)
	}

	roots := []string{"."}
	if project.RepoPath != "" && project.RepoPath != "." {
		roots = append(roots, project.RepoPath)
	}
	for _, root := range roots {
		base, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		candidate := filepath.Join(base, clean)
		if !withinDir(base, candidate) {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("目录不存在或不可访问：%s", uri)
}

// withinDir 判断 target 是否位于 base 目录内
func withinDir(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}
