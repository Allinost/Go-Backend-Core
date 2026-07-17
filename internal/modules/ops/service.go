package ops

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/internal/services/fileref"
	"github.com/Allinost/go-backend-core/internal/services/storage"
)

// Service 运维业务逻辑层
type Service struct {
	db          *sql.DB
	minioClient *minio.Client
	bucket      string
	store       storage.Storage
	endpoint    string
	useSSL      bool
	refSvc      *fileref.Service
}

func NewService(db *sql.DB, minioClient *minio.Client, bucket string, store storage.Storage, endpoint string, useSSL bool, refSvc *fileref.Service) *Service {
	return &Service{
		db:          db,
		minioClient: minioClient,
		bucket:      bucket,
		store:       store,
		endpoint:    endpoint,
		useSSL:      useSSL,
		refSvc:      refSvc,
	}
}

func (s *Service) baseURL() string {
	scheme := "https"
	if !s.useSSL {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s/%s/", scheme, s.endpoint, s.bucket)
}

// ListFile 列出文件
func (s *Service) ListFile(ctx context.Context, prefix string, offset, limit int) ([]storage.FileInfo, error) {
	var opts []storage.ListOption
	if offset > 0 {
		opts = append(opts, storage.WithOffset(offset))
	}
	if limit > 0 {
		opts = append(opts, storage.WithLimit(limit))
	}
	files, err := s.store.List(ctx, prefix, opts...)
	if err != nil {
		return nil, fmt.Errorf("列出文件失败: %w", err)
	}
	return files, nil
}

// FileInfo 增强文件信息
type FileInfoDetail struct {
	storage.FileInfo
	SignedURL string `json:"signed_url,omitempty"`
}

// GetFileInfo 获取文件详细信息
func (s *Service) GetFileInfo(ctx context.Context, filePath string) (*FileInfoDetail, error) {
	obj, err := s.minioClient.StatObject(ctx, s.bucket, filePath, minio.StatObjectOptions{})
	if err != nil {
		errResp := minio.ToErrorResponse(err)
		if errResp.Code == "NoSuchKey" {
			return nil, ErrFileNotFound
		}
		return nil, fmt.Errorf("获取文件信息失败: %w", err)
	}
	detail := &FileInfoDetail{
		FileInfo: storage.FileInfo{
			Name:         path.Base(obj.Key),
			Path:         obj.Key,
			Size:         obj.Size,
			ContentType:  obj.ContentType,
			LastModified: obj.LastModified,
			ETag:         obj.ETag,
		},
	}
	return detail, nil
}

// GetSignedURL 获取文件签名 URL
func (s *Service) GetSignedURL(ctx context.Context, filePath string, expire time.Duration) (string, error) {
	exists, err := s.store.Exists(ctx, filePath)
	if err != nil {
		return "", fmt.Errorf("检查文件失败: %w", err)
	}
	if !exists {
		return "", ErrFileNotFound
	}
	urlStr, err := s.store.SignedURL(ctx, filePath, expire)
	if err != nil {
		return "", fmt.Errorf("生成签名 URL 失败: %w", err)
	}
	return urlStr, nil
}

// DownloadFile 下载文件
func (s *Service) DownloadFile(ctx context.Context, filePath string) (io.ReadCloser, *storage.FileInfo, error) {
	info, err := s.GetFileInfo(ctx, filePath)
	if err != nil {
		return nil, nil, err
	}
	reader, err := s.store.Download(ctx, filePath)
	if err != nil {
		return nil, nil, fmt.Errorf("下载文件失败: %w", err)
	}
	fi := info.FileInfo
	return reader, &fi, nil
}

// UploadFile 上传文件
func (s *Service) UploadFile(ctx context.Context, filePath string, reader io.Reader, contentType string) (*storage.FileInfo, error) {
	opts := []storage.UploadOption{}
	if contentType != "" {
		opts = append(opts, storage.WithContentType(contentType))
	}
	info, err := s.store.Upload(ctx, filePath, reader, opts...)
	if err != nil {
		return nil, fmt.Errorf("上传文件失败: %w", err)
	}
	return info, nil
}

// DeleteFile 删除文件
func (s *Service) DeleteFile(ctx context.Context, filePath string) error {
	exists, err := s.store.Exists(ctx, filePath)
	if err != nil {
		return fmt.Errorf("检查文件失败: %w", err)
	}
	if !exists {
		return ErrFileNotFound
	}
	if err := s.store.Delete(ctx, filePath); err != nil {
		return fmt.Errorf("删除文件失败: %w", err)
	}
	return nil
}

// BatchDeleteFiles 批量删除文件
type BatchDeleteResult struct {
	SuccessCount int      `json:"success_count"`
	FailCount    int      `json:"fail_count"`
	FailedPaths  []string `json:"failed_paths,omitempty"`
}

func (s *Service) BatchDeleteFiles(ctx context.Context, paths []string) (*BatchDeleteResult, error) {
	result := &BatchDeleteResult{}
	for _, p := range paths {
		if err := s.store.Delete(ctx, p); err != nil {
			result.FailCount++
			result.FailedPaths = append(result.FailedPaths, p)
			logger.Warn().Str("path", p).Err(err).Msg("批量删除文件失败")
		} else {
			result.SuccessCount++
		}
	}
	return result, nil
}

// ScanTarget 扫描目标：表名.列名
type ScanTarget struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

// ScanResult 扫描结果
type ScanResult struct {
	TotalFiles      int                `json:"total_files"`
	TotalSize       int64              `json:"total_size"`
	ReferencedFiles int                `json:"referenced_files"`
	OrphanFiles     int                `json:"orphan_files"`
	ScanTargets     []TargetScanReport `json:"scan_targets"`
	Orphans         []storage.FileInfo `json:"orphans,omitempty"`
	AllReferenced   []storage.FileInfo `json:"all_referenced,omitempty"`
}

// TargetScanReport 单个目标的扫描报告
type TargetScanReport struct {
	Table           string `json:"table"`
	Column          string `json:"column"`
	ReferencedCount int    `json:"referenced_count"`
	ScannedRows     int    `json:"scanned_rows"`
}

// getScanTargets 获取扫描目标：优先从 scan_targets 表读取，失败时返回默认值
func (s *Service) getScanTargets(ctx context.Context) []ScanTarget {
	if s.refSvc != nil {
		registered, err := s.refSvc.ListScanTargets(ctx, "rustfs", true)
		if err == nil && len(registered) > 0 {
			converted := make([]ScanTarget, len(registered))
			for i, t := range registered {
				converted[i] = ScanTarget{
					Table:  t.TableName,
					Column: t.ColumnName,
				}
			}
			return converted
		}
	}
	// 默认回退
	return []ScanTarget{
		{Table: "zzz_goodser_products", Column: "image_url"},
		{Table: "zzz_goodser_products", Column: "images"},
	}
}

// ScanFileUsage 扫描文件在数据库中的使用情况
// 优先使用 file_references 表（精确），再按 scan_targets 配置的列扫描（启发式）
func (s *Service) ScanFileUsage(ctx context.Context, targets []ScanTarget) (*ScanResult, error) {
	// 1. 列出存储中所有文件
	allFiles, err := s.store.List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("%w: 列出存储文件失败: %v", ErrScanFailed, err)
	}

	fileMap := make(map[string]storage.FileInfo)
	totalSize := int64(0)
	for _, f := range allFiles {
		fileMap[f.Path] = f
		totalSize += f.Size
	}

	referencedSet := make(map[string]bool)
	var targetReports []TargetScanReport

	// 2a. 优先使用 file_references 表（精确匹配）
	if s.refSvc != nil {
		keys, err := s.refSvc.AllKeys(ctx, "rustfs")
		if err != nil {
			logger.Warn().Err(err).Msg("读取文件引用表失败，降级到列扫描")
		} else {
			logger.Info().Int("registered_keys", len(keys)).Msg("从 file_references 表加载引用")
			for _, k := range keys {
				referencedSet[k] = true
			}
			targetReports = append(targetReports, TargetScanReport{
				Table:           "file_references",
				Column:          "object_key",
				ReferencedCount: len(keys),
				ScannedRows:     len(keys),
			})
		}
	}

	// 2b. 按列扫描（补充，可发现未注册的引用）
	if s.db != nil {
		scanTargets := targets
		if len(scanTargets) == 0 {
			scanTargets = s.getScanTargets(ctx)
		}
		for _, target := range scanTargets {
			report, err := s.scanColumn(ctx, target.Table, target.Column, referencedSet)
			if err != nil {
				logger.Warn().Str("table", target.Table).Str("column", target.Column).Err(err).Msg("扫描列失败，跳过")
				continue
			}
			targetReports = append(targetReports, *report)
		}
	}

	// 3. 计算引用和孤立的文件
	var referenced, orphans []storage.FileInfo
	for fpath, fi := range fileMap {
		if referencedSet[fpath] {
			referenced = append(referenced, fi)
		} else {
			orphans = append(orphans, fi)
		}
	}

	return &ScanResult{
		TotalFiles:      len(allFiles),
		TotalSize:       totalSize,
		ReferencedFiles: len(referenced),
		OrphanFiles:     len(orphans),
		ScanTargets:     targetReports,
		Orphans:         orphans,
		AllReferenced:   referenced,
	}, nil
}

// scanColumn 扫描单个列对文件路径的引用
func (s *Service) scanColumn(ctx context.Context, table, column string, referencedSet map[string]bool) (*TargetScanReport, error) {
	query := fmt.Sprintf("SELECT `%s` FROM `%s` WHERE `%s` IS NOT NULL AND `%s` != ''", column, table, column, column)
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("查询 %s.%s 失败: %w", table, column, err)
	}
	defer rows.Close()

	baseURL := s.baseURL()
	report := &TargetScanReport{
		Table:  table,
		Column: column,
	}
	count := 0

	for rows.Next() {
		var val string
		if err := rows.Scan(&val); err != nil {
			return nil, fmt.Errorf("扫描 %s.%s 行失败: %w", table, column, err)
		}
		count++

		// 尝试解析为 JSON 数组
		var jsonVals []string
		if err := json.Unmarshal([]byte(val), &jsonVals); err == nil {
			for _, jv := range jsonVals {
				if key := s.extractKey(jv, baseURL); key != "" {
					referencedSet[key] = true
				}
			}
			continue
		}

		// 尝试解析为包含多个 URL 的 JSON 对象（如 {"urls":[...]}），降级为单值处理
		if key := s.extractKey(val, baseURL); key != "" {
			referencedSet[key] = true
		}
	}
	report.ScannedRows = count
	report.ReferencedCount = len(referencedSet)

	return report, nil
}

// extractKey 从 URL 或路径中提取 Rustfs 对象键
func (s *Service) extractKey(raw, baseURL string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// 如果是以 baseURL 开头的完整 URL，提取对象键
	if strings.HasPrefix(raw, baseURL) {
		trimmed := strings.TrimPrefix(raw, baseURL)
		// 去掉查询参数
		if idx := strings.Index(trimmed, "?"); idx > 0 {
			trimmed = trimmed[:idx]
		}
		decoded, err := url.QueryUnescape(trimmed)
		if err == nil {
			return decoded
		}
		return trimmed
	}
	// 如果包含 endpoint，尝试提取
	if s.endpoint != "" && strings.Contains(raw, s.endpoint) {
		parts := strings.SplitN(raw, s.bucket+"/", 2)
		if len(parts) == 2 {
			after := parts[1]
			if idx := strings.Index(after, "?"); idx > 0 {
				after = after[:idx]
			}
			decoded, err := url.QueryUnescape(after)
			if err == nil {
				return decoded
			}
			return after
		}
	}
	// 直接作为对象键匹配
	return raw
}

// CleanupOrphans 清理孤立文件
type CleanupResult struct {
	TotalOrphans int      `json:"total_orphans"`
	DeletedCount int      `json:"deleted_count"`
	FailCount    int      `json:"fail_count"`
	FailedPaths  []string `json:"failed_paths,omitempty"`
}

func (s *Service) CleanupOrphans(ctx context.Context, paths []string) (*CleanupResult, error) {
	if len(paths) == 0 {
		// 如果没有指定路径，执行全量扫描后清理
		defaultTargets := []ScanTarget{
			{Table: "zzz_goodser_products", Column: "image_url"},
			{Table: "zzz_goodser_products", Column: "images"},
		}
		scanResult, err := s.ScanFileUsage(ctx, defaultTargets)
		if err != nil {
			return nil, err
		}
		paths = make([]string, len(scanResult.Orphans))
		for i, o := range scanResult.Orphans {
			paths[i] = o.Path
		}
	}

	result := &CleanupResult{
		TotalOrphans: len(paths),
	}
	for _, p := range paths {
		if err := s.store.Delete(ctx, p); err != nil {
			result.FailCount++
			result.FailedPaths = append(result.FailedPaths, p)
			logger.Warn().Str("path", p).Err(err).Msg("清理孤立文件失败")
		} else {
			result.DeletedCount++
			logger.Info().Str("path", p).Msg("已清理孤立文件")
		}
	}
	return result, nil
}

// BucketStats 存储桶统计信息
type BucketStats struct {
	TotalFiles  int                `json:"total_files"`
	TotalSize   int64              `json:"total_size"`
	PrefixStats []PrefixStat       `json:"prefix_stats,omitempty"`
	Files       []storage.FileInfo `json:"files,omitempty"`
}

type PrefixStat struct {
	Prefix string `json:"prefix"`
	Count  int    `json:"count"`
	Size   int64  `json:"size"`
}

func (s *Service) GetBucketStats(ctx context.Context) (*BucketStats, error) {
	files, err := s.store.List(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("获取文件列表失败: %w", err)
	}

	stats := &BucketStats{
		TotalFiles: len(files),
	}
	prefixMap := make(map[string]*PrefixStat)

	for _, f := range files {
		stats.TotalSize += f.Size
		prefix := extractPrefix(f.Path)
		if ps, ok := prefixMap[prefix]; ok {
			ps.Count++
			ps.Size += f.Size
		} else {
			prefixMap[prefix] = &PrefixStat{Prefix: prefix, Count: 1, Size: f.Size}
		}
	}

	for _, ps := range prefixMap {
		stats.PrefixStats = append(stats.PrefixStats, *ps)
	}

	return stats, nil
}

// RegisterReferences 注册文件引用
func (s *Service) RegisterReferences(ctx context.Context, refs []fileref.ReferenceRecord) error {
	if s.refSvc == nil {
		return fmt.Errorf("文件引用服务未初始化")
	}
	return s.refSvc.Register(ctx, refs)
}

// RemoveReference 删除文件引用
func (s *Service) RemoveReference(ctx context.Context, id int64) error {
	if s.refSvc == nil {
		return fmt.Errorf("文件引用服务未初始化")
	}
	return s.refSvc.Remove(ctx, id)
}

// ListReferences 列出文件引用
func (s *Service) ListReferences(ctx context.Context, filter fileref.ReferenceFilter) ([]fileref.ReferenceRecord, int64, error) {
	if s.refSvc == nil {
		return nil, 0, fmt.Errorf("文件引用服务未初始化")
	}
	return s.refSvc.List(ctx, filter)
}

// ReferenceStats 获取引用统计
func (s *Service) ReferenceStats(ctx context.Context) (*fileref.UsageStats, error) {
	if s.refSvc == nil {
		return nil, fmt.Errorf("文件引用服务未初始化")
	}
	return s.refSvc.Stats(ctx)
}

// SyncReferences 从数据库扫描并同步文件引用到 file_references 表
func (s *Service) SyncReferences(ctx context.Context, targets []ScanTarget) (*SyncResult, error) {
	if s.refSvc == nil {
		return nil, fmt.Errorf("文件引用服务未初始化")
	}
	if s.db == nil {
		return nil, fmt.Errorf("数据库连接不可用")
	}

	result := &SyncResult{}

	for _, target := range targets {
		query := fmt.Sprintf("SELECT id, `%s` FROM `%s` WHERE `%s` IS NOT NULL AND `%s` != ''", target.Column, target.Table, target.Column, target.Column)
		rows, err := s.db.QueryContext(ctx, query)
		if err != nil {
			logger.Warn().Str("table", target.Table).Str("column", target.Column).Err(err).Msg("同步扫描失败，跳过")
			continue
		}

		baseURL := s.baseURL()
		var batch []fileref.ReferenceRecord

		for rows.Next() {
			var recordID, val string
			if err := rows.Scan(&recordID, &val); err != nil {
				rows.Close()
				return nil, fmt.Errorf("扫描 %s.%s 行失败: %w", target.Table, target.Column, err)
			}

			// 提取所有对象键（单值或 JSON 数组）
			keys := s.extractKeys(val, baseURL)
			for _, key := range keys {
				batch = append(batch, fileref.ReferenceRecord{
					StorageName:   "rustfs",
					ObjectKey:     key,
					ModuleName:    extractModuleName(target.Table),
					TableName:     target.Table,
					RecordID:      recordID,
					ColumnName:    target.Column,
					ReferenceType: "image",
				})
			}
		}
		rows.Close()

		if len(batch) > 0 {
			if err := s.refSvc.Register(ctx, batch); err != nil {
				logger.Warn().Err(err).Msg("同步写入引用失败")
			} else {
				result.TotalRegistered += len(batch)
				result.SyncedTargets++
			}
		}
		result.ScannedTargets++
	}

	return result, nil
}

type SyncResult struct {
	ScannedTargets  int `json:"scanned_targets"`
	SyncedTargets   int `json:"synced_targets"`
	TotalRegistered int `json:"total_registered"`
}

// extractKeys 从值中提取所有可能的对象键
func (s *Service) extractKeys(val, baseURL string) []string {
	val = strings.TrimSpace(val)
	if val == "" {
		return nil
	}

	// 尝试 JSON 数组
	var jsonVals []string
	if err := json.Unmarshal([]byte(val), &jsonVals); err == nil {
		var keys []string
		for _, jv := range jsonVals {
			if k := s.extractKey(jv, baseURL); k != "" {
				keys = append(keys, k)
			}
		}
		return keys
	}

	// 单值
	if k := s.extractKey(val, baseURL); k != "" {
		return []string{k}
	}
	return nil
}

// ListScanTargets 列出扫描目标
func (s *Service) ListScanTargets(ctx context.Context, storageName string, enabledOnly bool) ([]fileref.ScanTarget, error) {
	if s.refSvc == nil {
		return nil, fmt.Errorf("文件引用服务未初始化")
	}
	return s.refSvc.ListScanTargets(ctx, storageName, enabledOnly)
}

// CreateScanTarget 创建扫描目标
func (s *Service) CreateScanTarget(ctx context.Context, t *fileref.ScanTarget) error {
	if s.refSvc == nil {
		return fmt.Errorf("文件引用服务未初始化")
	}
	return s.refSvc.AddScanTarget(ctx, t)
}

// UpdateScanTarget 更新扫描目标
func (s *Service) UpdateScanTarget(ctx context.Context, t *fileref.ScanTarget) error {
	if s.refSvc == nil {
		return fmt.Errorf("文件引用服务未初始化")
	}
	return s.refSvc.UpdateScanTarget(ctx, t)
}

// RemoveScanTarget 删除扫描目标
func (s *Service) RemoveScanTarget(ctx context.Context, id int64) error {
	if s.refSvc == nil {
		return fmt.Errorf("文件引用服务未初始化")
	}
	return s.refSvc.RemoveScanTarget(ctx, id)
}

// extractModuleName 从表名推导模块名
func extractModuleName(tableName string) string {
	parts := strings.SplitN(strings.TrimPrefix(tableName, "zzz_"), "_", 2)
	if len(parts) >= 1 {
		return parts[0]
	}
	return "unknown"
}

func extractPrefix(filePath string) string {
	parts := strings.SplitN(filePath, "/", 2)
	if len(parts) == 2 {
		return parts[0]
	}
	if len(parts) == 1 {
		return "root"
	}
	return "unknown"
}
