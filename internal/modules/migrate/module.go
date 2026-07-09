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
	adapter     svc.DBAdapter
	migrator    *svc.SchemaMigrator
	migrateDir  string
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
