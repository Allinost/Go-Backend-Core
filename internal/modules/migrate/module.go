package migrate

import (
	"context"
	"fmt"
	"time"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	appErrors "github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	svc "github.com/Allinost/go-backend-core/internal/services/migrate"
	"github.com/gin-gonic/gin"
)

type Module struct {
	adapter    svc.DBAdapter
	migrator   *svc.SchemaMigrator
	migrateDir string
}

func (m *Module) Name() string {
	return "migrate"
}

func (m *Module) Init(cfg *config.Config) error {
	if database.DB != nil {
		if pool, ok := database.DB.MySQL["main"]; ok && pool.DB != nil {
			m.adapter = svc.NewMySQLAdapter(pool.DB)
			m.migrateDir = "migrations/mysql"
		} else if pool, ok := database.DB.Postgres["main"]; ok && pool.Pool != nil {
			m.adapter = svc.NewPostgresAdapter(pool.Pool)
			m.migrateDir = "migrations/postgres"
		}
	}

	if m.adapter != nil {
		m.migrator = svc.NewSchemaMigrator(m.adapter, m.migrateDir)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = m.migrator.Init(ctx)
	}

	return nil
}

func (m *Module) Close() error {
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/tables", m.listTables)
	r.GET("/tables/:name/schema", m.tableSchema)
	r.POST("/dump", m.dump)
	r.POST("/restore", m.restore)
	r.POST("/transfer", m.transfer)
	r.POST("/backup", m.createBackup)
	r.POST("/backup/restore", m.restoreBackup)
	r.GET("/backups", m.listBackups)
	r.DELETE("/backups/:name", m.deleteBackup)
	r.GET("/schema/migrations", m.migrationStatus)
	r.POST("/schema/migrate", m.applyMigrations)
	r.POST("/schema/create", m.createMigration)
	r.GET("/schema/dump", m.schemaDump)
}

func (m *Module) getAdapter(c *gin.Context) (svc.DBAdapter, bool) {
	if m.adapter == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "数据库未连接"))
		return nil, false
	}
	return m.adapter, true
}

// listTables 获取表列表
// @Summary      获取数据库表列表
// @Description  返回当前数据库中的所有表名
// @Tags         migrate
// @Produce      json
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/tables [get]
func (m *Module) listTables(c *gin.Context) {
	adapter, ok := m.getAdapter(c)
	if !ok {
		return
	}
	tables, err := adapter.GetTables(c.Request.Context())
	if err != nil {
		response.FailCode(c, appErrors.CodeSystemErr)
		return
	}
	response.Success(c, gin.H{"tables": tables})
}

// tableSchema 获取表结构
// @Summary      获取指定表结构
// @Description  返回指定表的字段定义和索引信息
// @Tags         migrate
// @Produce      json
// @Param        name  path  string  true  "表名"
// @Success      200   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /migrate/tables/{name}/schema [get]
func (m *Module) tableSchema(c *gin.Context) {
	adapter, ok := m.getAdapter(c)
	if !ok {
		return
	}
	schema, err := adapter.GetTableSchema(c.Request.Context(), c.Param("name"))
	if err != nil {
		response.FailCode(c, appErrors.CodeNotFound)
		return
	}
	response.Success(c, schema)
}

// dump 数据导出
// @Summary      导出数据库数据
// @Description  按指定格式导出表数据，支持 JSON/SQL/CSV 格式
// @Tags         migrate
// @Accept       json
// @Produce      application/octet-stream
// @Param        body  body  object{tables=[]string,format=string,where=object,batch_size=int,schema_only=bool}  true  "导出选项"
// @Success      200  {file}  binary
// @Failure      500  {object}  response.Response
// @Router       /migrate/dump [post]
func (m *Module) dump(c *gin.Context) {
	adapter, ok := m.getAdapter(c)
	if !ok {
		return
	}

	var req struct {
		Tables     []string          `json:"tables"`
		Format     string            `json:"format"`
		Where      map[string]string `json:"where"`
		BatchSize  int               `json:"batch_size"`
		SchemaOnly bool              `json:"schema_only"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误")
		return
	}

	format := svc.Format(req.Format)
	if format == "" {
		format = svc.FormatJSON
	}

	result, err := svc.Dump(c.Request.Context(), adapter, svc.DumpOptions{
		Tables:     req.Tables,
		Format:     format,
		Where:      req.Where,
		BatchSize:  req.BatchSize,
		SchemaOnly: req.SchemaOnly,
	})
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	c.Header("Content-Type", "application/octet-stream")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="dump_%s.%s"`, adapter.Type(), format))
	c.Data(200, "application/octet-stream", result.Data)
}

// restore 数据导入
// @Summary      导入数据库数据
// @Description  通过上传文件导入数据到数据库
// @Tags         migrate
// @Accept       multipart/form-data
// @Produce      json
// @Param        file     formData  file    true  "数据文件"
// @Param        format   formData  string  false  "文件格式"
// @Param        truncate formData  string  false  "是否先清空表"
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/restore [post]
func (m *Module) restore(c *gin.Context) {
	adapter, ok := m.getAdapter(c)
	if !ok {
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.ParamErr(c, "请上传文件")
		return
	}

	fp, err := file.Open()
	if err != nil {
		response.FailCode(c, appErrors.CodeSystemErr)
		return
	}
	defer fp.Close()

	data := make([]byte, file.Size)
	if _, err := fp.Read(data); err != nil {
		response.FailCode(c, appErrors.CodeSystemErr)
		return
	}

	format := svc.Format(c.PostForm("format"))
	truncate := c.PostForm("truncate") == "true"

	result, err := svc.Restore(c.Request.Context(), adapter, data, svc.RestoreOptions{
		Format:   format,
		Truncate: truncate,
	})
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, result)
}

// transfer 跨库迁移
// @Summary      跨数据库迁移数据
// @Description  将数据从当前数据库迁移到另一个数据库
// @Tags         migrate
// @Accept       json
// @Produce      json
// @Param        body  body  object{target_db_type=string,target_name=string,tables=[]string,create_table=bool}  true  "迁移选项"
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/transfer [post]
func (m *Module) transfer(c *gin.Context) {
	adapter, ok := m.getAdapter(c)
	if !ok {
		return
	}

	var req struct {
		TargetDBType string   `json:"target_db_type"`
		TargetName   string   `json:"target_name"`
		Tables       []string `json:"tables"`
		CreateTable  bool     `json:"create_table"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误")
		return
	}

	var targetAdapter svc.DBAdapter
	if req.TargetDBType == "mysql" || req.TargetDBType == "postgres" {
		if pool, ok := database.DB.MySQL["main"]; ok && pool.DB != nil && req.TargetDBType == "mysql" {
			targetAdapter = svc.NewMySQLAdapter(pool.DB)
		} else if pool, ok := database.DB.Postgres["main"]; ok && pool.Pool != nil && req.TargetDBType == "postgres" {
			targetAdapter = svc.NewPostgresAdapter(pool.Pool)
		}
	}

	if targetAdapter == nil {
		response.Fail(c, appErrors.New(appErrors.CodeParamErr, "目标数据库未连接或类型不支持"))
		return
	}

	result, err := svc.Transfer(c.Request.Context(), adapter, targetAdapter, svc.TransferOptions{
		Tables:      req.Tables,
		CreateTable: req.CreateTable,
	})
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, result)
}

// createBackup 创建备份
// @Summary      创建数据库备份
// @Description  将数据库表数据备份到文件，支持压缩和加密
// @Tags         migrate
// @Accept       json
// @Produce      json
// @Param        body  body  object{tables=[]string,compress_algo=string,encrypt_key=string,output_dir=string}  true  "备份选项"
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/backup [post]
func (m *Module) createBackup(c *gin.Context) {
	adapter, ok := m.getAdapter(c)
	if !ok {
		return
	}

	var req struct {
		Tables       []string `json:"tables"`
		CompressAlgo string   `json:"compress_algo"`
		EncryptKey   string   `json:"encrypt_key"`
		OutputDir    string   `json:"output_dir"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误")
		return
	}

	meta, err := svc.CreateBackup(c.Request.Context(), adapter, svc.BackupOptions{
		Tables:       req.Tables,
		CompressAlgo: req.CompressAlgo,
		EncryptKey:   req.EncryptKey,
		OutputDir:    req.OutputDir,
	})
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, meta)
}

// restoreBackup 从备份恢复
// @Summary      从备份文件恢复数据
// @Description  从指定的备份文件中恢复数据库数据
// @Tags         migrate
// @Accept       json
// @Produce      json
// @Param        body  body  object{path=string,password=string}  true  "恢复选项"
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/backup/restore [post]
func (m *Module) restoreBackup(c *gin.Context) {
	adapter, ok := m.getAdapter(c)
	if !ok {
		return
	}

	var req struct {
		Path     string `json:"path"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.ParamErr(c, "请求格式错误")
		return
	}

	if req.Path == "" {
		response.ParamErr(c, "请指定备份文件路径")
		return
	}

	result, err := svc.RestoreBackup(c.Request.Context(), adapter, req.Path, req.Password)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, result)
}

// listBackups 备份列表
// @Summary      列出备份文件
// @Description      列出指定目录下的所有备份文件
// @Tags         migrate
// @Produce      json
// @Param        dir  query  string  false  "备份目录路径（默认当前目录）"
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/backups [get]
func (m *Module) listBackups(c *gin.Context) {
	dir := c.Query("dir")
	if dir == "" {
		dir = "."
	}

	backups, err := svc.ListBackups(dir)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, gin.H{"backups": backups})
}

// deleteBackup 删除备份
// @Summary      删除备份文件
// @Description  删除指定名称的备份文件
// @Tags         migrate
// @Produce      json
// @Param        name  path   string  true  "备份文件名"
// @Param        dir   query  string  false  "备份目录路径"
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/backups/{name} [delete]
func (m *Module) deleteBackup(c *gin.Context) {
	dir := c.Query("dir")
	if dir == "" {
		dir = "."
	}

	if err := svc.DeleteBackup(dir, c.Param("name")); err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, gin.H{"message": "备份已删除"})
}

// migrationStatus 迁移状态
// @Summary      查看迁移状态
// @Description  查看所有数据库迁移脚本的应用状态
// @Tags         migrate
// @Produce      json
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/schema/migrations [get]
func (m *Module) migrationStatus(c *gin.Context) {
	if m.migrator == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "迁移工具未初始化"))
		return
	}

	status, err := m.migrator.Status(c.Request.Context())
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, status)
}

// applyMigrations 执行迁移
// @Summary      执行未应用的迁移
// @Description  自动应用所有未执行的数据库迁移脚本
// @Tags         migrate
// @Produce      json
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/schema/migrate [post]
func (m *Module) applyMigrations(c *gin.Context) {
	if m.migrator == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "迁移工具未初始化"))
		return
	}

	applied, err := m.migrator.ApplyAll(c.Request.Context())
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, gin.H{
		"applied": len(applied),
		"files":   applied,
	})
}

// createMigration 创建迁移文件
// @Summary      创建新的迁移文件
// @Description  创建一个新的数据库迁移 SQL 文件
// @Tags         migrate
// @Accept       json
// @Produce      json
// @Param        body  body  object{name=string}  true  "迁移名称"
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /migrate/schema/create [post]
func (m *Module) createMigration(c *gin.Context) {
	if m.migrator == nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, "迁移工具未初始化"))
		return
	}

	var req struct {
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Name == "" {
		response.ParamErr(c, "请提供迁移名称")
		return
	}

	filepath, err := m.migrator.CreateMigrationFile(req.Name)
	if err != nil {
		response.Fail(c, appErrors.New(appErrors.CodeSystemErr, err.Error()))
		return
	}

	response.Success(c, gin.H{"file": filepath})
}

// schemaDump 导出表结构
// @Summary      导出数据库表结构
// @Description  以 JSON 格式导出所有表的 DDL 结构定义
// @Tags         migrate
// @Produce      json
// @Success      200  {string}  string
// @Failure      500  {object}  response.Response
// @Router       /migrate/schema/dump [get]
func (m *Module) schemaDump(c *gin.Context) {
	adapter, ok := m.getAdapter(c)
	if !ok {
		return
	}

	data, err := svc.SchemaDump(c.Request.Context(), adapter)
	if err != nil {
		response.FailCode(c, appErrors.CodeSystemErr)
		return
	}

	c.Header("Content-Type", "application/json")
	c.Data(200, "application/json", data)
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
