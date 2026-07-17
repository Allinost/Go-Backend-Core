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
	"github.com/Allinost/go-backend-core/internal/services/storage"
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
// @Summary      列出 S3 文件
// @Description  按前缀列出 S3 存储中的文件，支持分页。返回当前层级文件和子目录
// @Tags         ops-S3
// @Produce      json
// @Param        prefix  query  string  false  "文件路径前缀过滤"
// @Param        offset  query  int     false  "分页偏移量"  default(0)
// @Param        limit   query  int     false  "每页数量(1-1000)"  default(100)
// @Success      200  {object}  response.Response{data=object{files=[]storage.FileInfo,dirs=[]string,offset=int,limit=int,total=int}}
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/files [get]
func (h *Handler) ListFiles(c *gin.Context) {
	prefix := c.Query("prefix")
	// 补上尾随 / 以确保 S3 delimiter 正确返回子目录而非当前目录
	if prefix != "" && !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if limit < 1 || limit > 1000 {
		limit = 100
	}

	allFiles, err := h.svc.ListFile(c.Request.Context(), prefix, offset, limit)
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "获取文件列表失败").WithDetail(err.Error()))
		return
	}

	var files []storage.FileInfo
	var dirs []string
	for _, f := range allFiles {
		if f.Path == prefix {
			continue
		}
		if strings.HasSuffix(f.Path, "/") && f.Size == 0 {
			// 返回相对于当前 prefix 的子目录名
			dirs = append(dirs, strings.TrimPrefix(f.Path, prefix))
		} else {
			files = append(files, f)
		}
	}

	response.Success(c, gin.H{
		"files":  files,
		"dirs":   dirs,
		"offset": offset,
		"limit":  limit,
		"total":  len(files) + len(dirs),
	})
}

// GetFileInfo 获取文件信息
// @Summary      获取文件信息
// @Description  获取 S3 文件的元信息，含 ContentType、ETag 等
// @Tags         ops-S3
// @Produce      json
// @Param        path  query  string  true  "文件路径"
// @Success      200  {object}  response.Response{data=ops.FileInfoDetail}
// @Failure      400  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/info [get]
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
// @Summary      获取签名 URL
// @Description  获取 S3 文件的临时签名 URL（可指定过期时长）
// @Tags         ops-S3
// @Produce      json
// @Param        path    query  string  true  "文件路径"
// @Param        expire  query  int     false  "过期时间(秒，60-604800)"  default(3600)
// @Success      200  {object}  response.Response{data=object{url=string,path=string,expire=int}}
// @Failure      400  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/url [get]
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
// @Summary      下载文件
// @Description  代理下载 S3 文件到本地
// @Tags         ops-S3
// @Produce      application/octet-stream
// @Param        path      query  string  true  "文件路径"
// @Param        filename  query  string  false  "下载文件名"
// @Success      200  {file}  binary
// @Failure      400  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/download [get]
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
// @Summary      上传文件
// @Description  上传文件到 S3 存储
// @Tags         ops-S3
// @Accept       multipart/form-data
// @Produce      json
// @Param        file         formData  file    true   "文件内容"
// @Param        path         formData  string  false  "路径前缀"
// @Param        content_type  formData  string  false  "文件 MIME 类型"
// @Success      200  {object}  response.Response{data=object{file=storage.FileInfo}}
// @Failure      400  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/upload [post]
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
// @Summary      删除文件
// @Description  删除 S3 存储中的单个文件
// @Tags         ops-S3
// @Produce      json
// @Param        path  query  string  true  "文件路径"
// @Success      200  {object}  response.Response{data=object{path=string}}
// @Failure      400  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/delete [delete]
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
// @Summary      批量删除文件
// @Description  批量删除 S3 存储中的多个文件
// @Tags         ops-S3
// @Accept       json
// @Produce      json
// @Param        body  body  object{paths=[]string}  true  "待删除的文件路径列表"
// @Success      200  {object}  response.Response{data=ops.BatchDeleteResult}
// @Failure      400  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/batch-delete [post]
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
// @Summary      扫描文件引用
// @Description  扫描 S3 文件在数据库中的引用情况，区分引用文件和孤立文件
// @Tags         ops-扫描
// @Produce      json
// @Param        targets  query  string  false  "扫描目标，格式: table1.col1,table2.col2"
// @Success      200  {object}  response.Response{data=ops.ScanResult}
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/scan [get]
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
// @Summary      清理孤立文件
// @Description  清理未被任何数据库记录引用的孤立 S3 文件。body 为空时自动扫描全量
// @Tags         ops-扫描
// @Accept       json
// @Produce      json
// @Param        body  body  object{paths=[]string}  false  "指定清理的路径列表（为空则全量扫描后清理）"
// @Success      200  {object}  response.Response{data=ops.CleanupResult}
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/cleanup [post]
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
// @Summary      存储统计
// @Description  获取 S3 存储桶的统计信息（文件总数、总大小、按前缀分布）
// @Tags         ops-S3
// @Produce      json
// @Success      200  {object}  response.Response{data=ops.BucketStats}
// @Failure      500  {object}  response.Response
// @Router       /ops/rustfs/stats [get]
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
// @Summary      注册文件引用
// @Description  注册文件引用记录到全局引用注册表
// @Tags         ops-引用
// @Accept       json
// @Produce      json
// @Param        body  body  object{references=[]fileref.ReferenceRecord}  true  "引用记录列表"
// @Success      200  {object}  response.Response{data=object{count=int}}
// @Failure      400  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/references [post]
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
// @Summary      删除文件引用
// @Description  按 ID 删除文件引用记录
// @Tags         ops-引用
// @Produce      json
// @Param        id  path  int  true  "引用记录 ID"
// @Success      200  {object}  response.Response{data=object{id=int}}
// @Failure      400  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/references/{id} [delete]
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
// @Summary      列出文件引用
// @Description  按条件查询文件引用记录
// @Tags         ops-引用
// @Produce      json
// @Param        storage_name  query  string  false  "存储名称"
// @Param        object_key    query  string  false  "对象键"
// @Param        module_name   query  string  false  "模块名称"
// @Param        table_name    query  string  false  "表名"
// @Param        record_id     query  string  false  "记录 ID"
// @Param        reference_type  query  string  false  "引用类型"
// @Param        offset        query  int     false  "分页偏移"  default(0)
// @Param        limit         query  int     false  "每页数量"  default(100)
// @Success      200  {object}  response.Response{data=object{records=[]fileref.ReferenceRecord,total=int,offset=int,limit=int}}
// @Failure      500  {object}  response.Response
// @Router       /ops/references [get]
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
// @Summary      引用统计
// @Description  获取文件引用全局统计信息
// @Tags         ops-引用
// @Produce      json
// @Success      200  {object}  response.Response{data=fileref.UsageStats}
// @Failure      500  {object}  response.Response
// @Router       /ops/references/stats [get]
func (h *Handler) ReferenceStats(c *gin.Context) {
	stats, err := h.svc.ReferenceStats(c.Request.Context())
	if err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "获取引用统计失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, stats)
}

// SyncReferences 从数据库扫描并同步文件引用
// @Summary      同步文件引用
// @Description  扫描数据库表并同步文件引用到全局引用注册表
// @Tags         ops-引用
// @Accept       json
// @Produce      json
// @Param        body  body  object{targets=string}  false  "扫描目标，格式: 'table1.col1,table2.col2'（为空则使用默认值）"
// @Success      200  {object}  response.Response{data=ops.SyncResult}
// @Failure      500  {object}  response.Response
// @Router       /ops/references/sync [post]
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
// @Summary      列出扫描目标
// @Description  列出所有启用的扫描目标配置
// @Tags         ops-扫描
// @Produce      json
// @Param        storage_name  query  string  false  "存储名称"  default(rustfs)
// @Param        enabled_only  query  bool    false  "仅显示已启用的"  default(false)
// @Success      200  {object}  response.Response{data=object{targets=[]fileref.ScanTarget}}
// @Failure      500  {object}  response.Response
// @Router       /ops/scan-targets [get]
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
// @Summary      创建扫描目标
// @Description  创建新的扫描目标配置
// @Tags         ops-扫描
// @Accept       json
// @Produce      json
// @Param        body  body  fileref.ScanTarget  true  "扫描目标配置"
// @Success      200  {object}  response.Response{data=fileref.ScanTarget}
// @Failure      400  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/scan-targets [post]
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
// @Summary      更新扫描目标
// @Description  更新指定扫描目标的配置
// @Tags         ops-扫描
// @Accept       json
// @Produce      json
// @Param        id    path  int              true  "扫描目标 ID"
// @Param        body  body  fileref.ScanTarget  true  "更新后的配置"
// @Success      200  {object}  response.Response{data=fileref.ScanTarget}
// @Failure      400  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/scan-targets/{id} [put]
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
// @Summary      删除扫描目标
// @Description  删除指定的扫描目标配置
// @Tags         ops-扫描
// @Produce      json
// @Param        id  path  int  true  "扫描目标 ID"
// @Success      200  {object}  response.Response{data=object{id=int}}
// @Failure      400  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/scan-targets/{id} [delete]
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

// Status 服务状态诊断
// @Summary      服务状态
// @Description  获取 S3、MySQL、文件引用服务的连接状态诊断
// @Tags         ops-诊断
// @Produce      json
// @Success      200  {object}  response.Response{data=ops.ComponentStatus}
// @Router       /ops/status [get]
func (h *Handler) Status(c *gin.Context) {
	response.Success(c, h.svc.Status(c.Request.Context()))
}

// ReinitTables 手动重试初始化数据表
// @Summary      重建设表
// @Description  手动重试初始化 fileref 数据表（幂等操作）
// @Tags         ops-诊断
// @Produce      json
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /ops/reinit [post]
func (h *Handler) ReinitTables(c *gin.Context) {
	if err := h.svc.ReinitTables(c.Request.Context()); err != nil {
		response.Fail(c, appErr.New(appErr.CodeSystemErr, "初始化失败").WithDetail(err.Error()))
		return
	}
	response.Success(c, gin.H{"message": "数据表初始化成功"})
}

// handleS3Unavailable 用于路由检查，返回 503
func handleS3Unavailable(c *gin.Context) {
	response.Fail(c, ErrStorageNotAvailable)
}
