package ops

import (
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	appErr "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/fileref"
	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// ListFiles 列出文件
// GET /api/v1/ops/rustfs/files?prefix=...&offset=0&limit=20
func (h *Handler) ListFiles(c *gin.Context) {
	prefix := c.Query("prefix")
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit < 1 || limit > 1000 {
		limit = 100
	}

	files, err := h.svc.ListFile(c.Request.Context(), prefix, offset, limit)
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "获取文件列表失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{
		"files":  files,
		"offset": offset,
		"limit":  limit,
		"total":  len(files),
	})
}

// GetFileInfo 获取文件信息
// GET /api/v1/ops/rustfs/info?path=goodser/images/xxx.jpg
func (h *Handler) GetFileInfo(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		response.ParamErr(c, "path 参数不能为空")
		return
	}
	info, err := h.svc.GetFileInfo(c.Request.Context(), filePath)
	if err != nil {
		if err == ErrFileNotFound {
			response.Fail(c, ErrFileNotFound)
			return
		}
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "获取文件信息失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, info)
}

// GetSignedURL 获取文件签名 URL
// GET /api/v1/ops/rustfs/url?path=goodser/images/xxx.jpg&expire=3600
func (h *Handler) GetSignedURL(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		response.ParamErr(c, "path 参数不能为空")
		return
	}
	expireSec, _ := strconv.Atoi(c.DefaultQuery("expire", "3600"))
	if expireSec < 60 {
		expireSec = 60
	}
	if expireSec > 604800 {
		expireSec = 604800
	}
	expire := time.Duration(expireSec) * time.Second

	urlStr, err := h.svc.GetSignedURL(c.Request.Context(), filePath, expire)
	if err != nil {
		if err == ErrFileNotFound {
			response.Fail(c, ErrFileNotFound)
			return
		}
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "获取签名 URL 失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{
		"url":    urlStr,
		"path":   filePath,
		"expire": expireSec,
	})
}

// DownloadFile 代理下载文件
// GET /api/v1/ops/rustfs/download?path=goodser/images/xxx.jpg&filename=xxx.jpg
func (h *Handler) DownloadFile(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		response.ParamErr(c, "path 参数不能为空")
		return
	}

	reader, info, err := h.svc.DownloadFile(c.Request.Context(), filePath)
	if err != nil {
		if err == ErrFileNotFound {
			response.Fail(c, ErrFileNotFound)
			return
		}
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "下载文件失败").WithDetail(err.Error()))
		return
	}
	defer reader.Close()

	filename := c.DefaultQuery("filename", info.Name)
	contentType := info.ContentType
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	c.Header("Content-Type", contentType)
	c.Header("Content-Length", strconv.FormatInt(info.Size, 10))
	_, _ = io.Copy(c.Writer, reader)
}

// UploadFile 上传文件
// POST /api/v1/ops/rustfs/upload
// Content-Type: multipart/form-data
// Fields: file (the file), path (optional object key prefix), content_type (optional)
func (h *Handler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.ParamErr(c, "file 字段不能为空")
		return
	}
	defer file.Close()

	pathPrefix := strings.Trim(c.PostForm("path"), "/")
	contentType := c.PostForm("content_type")

	// 构建对象路径
	objectKey := header.Filename
	if pathPrefix != "" {
		objectKey = pathPrefix + "/" + objectKey
	}

	info, err := h.svc.UploadFile(c.Request.Context(), objectKey, file, contentType)
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "上传文件失败").WithDetail(err.Error()))
		return
	}

	response.Success(c, gin.H{
		"file": info,
	})
}

// DeleteFile 删除文件
// DELETE /api/v1/ops/rustfs/delete?path=goodser/images/xxx.jpg
func (h *Handler) DeleteFile(c *gin.Context) {
	filePath := c.Query("path")
	if filePath == "" {
		response.ParamErr(c, "path 参数不能为空")
		return
	}

	if err := h.svc.DeleteFile(c.Request.Context(), filePath); err != nil {
		if err == ErrFileNotFound {
			response.Fail(c, ErrFileNotFound)
			return
		}
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "删除文件失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{"path": filePath})
}

// BatchDelete 批量删除文件
// POST /api/v1/ops/rustfs/batch-delete
// Body: {"paths": ["goodser/images/a.jpg", "goodser/images/b.jpg"]}
func (h *Handler) BatchDelete(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求参数错误，需要 paths 数组")
		return
	}
	if len(req.Paths) == 0 {
		response.ParamErr(c, "paths 不能为空")
		return
	}

	result, err := h.svc.BatchDeleteFiles(c.Request.Context(), req.Paths)
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "批量删除失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, result)
}

// ScanFileUsage 扫描文件使用情况
// GET /api/v1/ops/rustfs/scan?targets=table1.col1,table2.col2
// targets 可选，不传则从 scan_targets 表读取配置
func (h *Handler) ScanFileUsage(c *gin.Context) {
	targetsParam := c.Query("targets")
	var targets []ScanTarget
	if targetsParam != "" {
		targets = parseScanTargets(targetsParam)
	}

	result, err := h.svc.ScanFileUsage(c.Request.Context(), targets)
	if err != nil {
		if err == ErrScanFailed {
			response.Fail(c, ErrScanFailed)
			return
		}
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "扫描失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, result)
}

// CleanupOrphans 清理孤立文件
// POST /api/v1/ops/rustfs/cleanup
// Body: {"paths": ["goodser/images/orphan1.jpg"]}  或 {} 全自动清理
func (h *Handler) CleanupOrphans(c *gin.Context) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Paths = nil
	}

	result, err := h.svc.CleanupOrphans(c.Request.Context(), req.Paths)
	if err != nil {
		if err == ErrCleanupFailed {
			response.Fail(c, ErrCleanupFailed)
			return
		}
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "清理失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, result)
}

// BucketStats 获取存储统计信息
// GET /api/v1/ops/rustfs/stats
func (h *Handler) BucketStats(c *gin.Context) {
	stats, err := h.svc.GetBucketStats(c.Request.Context())
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "获取统计信息失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, stats)
}

// parseScanTargets 解析扫描目标字符串 "table1.col1,table2.col2"
func parseScanTargets(s string) []ScanTarget {
	if s == "" {
		return []ScanTarget{
			{Table: "zzz_goodser_products", Column: "image_url"},
			{Table: "zzz_goodser_products", Column: "images"},
		}
	}
	var targets []ScanTarget
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		seg := strings.SplitN(part, ".", 2)
		if len(seg) == 2 {
			targets = append(targets, ScanTarget{Table: seg[0], Column: seg[1]})
		}
	}
	if len(targets) == 0 {
		targets = []ScanTarget{
			{Table: "zzz_goodser_products", Column: "image_url"},
			{Table: "zzz_goodser_products", Column: "images"},
		}
	}
	return targets
}

// RegisterReferences 注册文件引用
// POST /api/v1/ops/references
// Body: {"references": [{"object_key": "...", "module_name": "...", "table_name": "...", ...}]}
func (h *Handler) RegisterReferences(c *gin.Context) {
	var req struct {
		References []fileref.ReferenceRecord `json:"references" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求参数错误")
		return
	}
	if err := h.svc.RegisterReferences(c.Request.Context(), req.References); err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "注册引用失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{"count": len(req.References)})
}

// RemoveReference 删除文件引用
// DELETE /api/v1/ops/references/:id
func (h *Handler) RemoveReference(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的 ID")
		return
	}
	if err := h.svc.RemoveReference(c.Request.Context(), id); err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "删除引用失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{"id": id})
}

// ListReferences 列出文件引用
// GET /api/v1/ops/references?module_name=...&object_key=...&offset=0&limit=100
func (h *Handler) ListReferences(c *gin.Context) {
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))

	filter := fileref.ReferenceFilter{
		StorageName: c.Query("storage_name"),
		ObjectKey:   c.Query("object_key"),
		ModuleName:  c.Query("module_name"),
		TableName:   c.Query("table_name"),
		RecordID:    c.Query("record_id"),
		RefType:     c.Query("reference_type"),
		Offset:      offset,
		Limit:       limit,
	}

	records, total, err := h.svc.ListReferences(c.Request.Context(), filter)
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "查询引用失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{
		"records": records,
		"total":   total,
		"offset":  offset,
		"limit":   limit,
	})
}

// ReferenceStats 引用统计
// GET /api/v1/ops/references/stats
func (h *Handler) ReferenceStats(c *gin.Context) {
	stats, err := h.svc.ReferenceStats(c.Request.Context())
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "获取引用统计失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, stats)
}

// SyncReferences 从数据库扫描并同步文件引用
// POST /api/v1/ops/references/sync
// Body: {"targets": "table1.col1,table2.col2"} or empty for defaults
func (h *Handler) SyncReferences(c *gin.Context) {
	var req struct {
		Targets string `json:"targets"`
	}
	_ = c.ShouldBindJSON(&req)

	targets := parseScanTargets(req.Targets)
	result, err := h.svc.SyncReferences(c.Request.Context(), targets)
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "同步引用失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, result)
}

// ListScanTargets 列出扫描目标
// GET /api/v1/ops/scan-targets?storage_name=rustfs&enabled_only=true
func (h *Handler) ListScanTargets(c *gin.Context) {
	storageName := c.DefaultQuery("storage_name", "rustfs")
	enabledOnly := c.DefaultQuery("enabled_only", "false") == "true"

	targets, err := h.svc.ListScanTargets(c.Request.Context(), storageName, enabledOnly)
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "查询扫描目标失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{"targets": targets})
}

// CreateScanTarget 创建扫描目标
// POST /api/v1/ops/scan-targets
func (h *Handler) CreateScanTarget(c *gin.Context) {
	var t fileref.ScanTarget
	if err := c.ShouldBindJSON(&t); err != nil {
		response.ParamErr(c, "请求参数错误")
		return
	}
	if t.TableName == "" || t.ColumnName == "" {
		response.ParamErr(c, "table_name 和 column_name 不能为空")
		return
	}
	if err := h.svc.CreateScanTarget(c.Request.Context(), &t); err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "创建扫描目标失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, t)
}

// UpdateScanTarget 更新扫描目标
// PUT /api/v1/ops/scan-targets/:id
func (h *Handler) UpdateScanTarget(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的 ID")
		return
	}
	var t fileref.ScanTarget
	if err := c.ShouldBindJSON(&t); err != nil {
		response.ParamErr(c, "请求参数错误")
		return
	}
	t.ID = id
	if err := h.svc.UpdateScanTarget(c.Request.Context(), &t); err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "更新扫描目标失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, t)
}

// DeleteScanTarget 删除扫描目标
// DELETE /api/v1/ops/scan-targets/:id
func (h *Handler) DeleteScanTarget(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的 ID")
		return
	}
	if err := h.svc.RemoveScanTarget(c.Request.Context(), id); err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "删除扫描目标失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{"id": id})
}

// handleS3Unavailable 用于路由检查，返回 503
func handleS3Unavailable(c *gin.Context) {
	response.Fail(c, ErrStorageNotAvailable)
}
