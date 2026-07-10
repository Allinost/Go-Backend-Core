package scheduler

import (
	"fmt"
	"strconv"

	"github.com/Allinost/go-backend-core/internal/config"
	"github.com/Allinost/go-backend-core/internal/database"
	"github.com/Allinost/go-backend-core/internal/pkg/errors"
	"github.com/Allinost/go-backend-core/internal/pkg/response"
	"github.com/Allinost/go-backend-core/internal/services/monitor"
	"github.com/gin-gonic/gin"
	"github.com/hibiken/asynq"
)

type Module struct {
	scheduler *Scheduler
}

func (m *Module) Name() string {
	return "scheduler"
}

func (m *Module) Init(cfg *config.Config) error {
	sc := cfg.Scheduler

	var store Store
	var asynqClient *asynq.Client
	var asynqServer *asynq.Server

	if pool, ok := database.DB.MySQL["main"]; ok && pool.DB != nil {
		var err error
		store, err = NewMySQLStoreFromDB(pool.DB)
		if err != nil {
			return fmt.Errorf("scheduler: DB 初始化失败: %w", err)
		}
		if err := store.(*MySQLStore).AutoMigrate(); err != nil {
			return fmt.Errorf("scheduler: 自动迁移失败: %w", err)
		}

		if rdb := database.GetRedis("main"); rdb != nil {
			asynqClient = newAsynqClient(rdb)
			asynqServer = newAsynqServer(rdb, sc.WorkerConcurrency, sc.DefaultTimeout)
		}
	} else {
		store = NewInMemoryStore()
	}

	cfgScheduler := SchedulerConfig{
		Enabled:           sc.Enabled,
		WorkerConcurrency: sc.WorkerConcurrency,
		DefaultTimeout:    sc.DefaultTimeout,
		DefaultMaxRetries: sc.DefaultMaxRetries,
		LogRetentionDays:  sc.LogRetentionDays,
	}

	s := New(store, cfgScheduler, asynqClient, asynqServer)

	reg := monitor.NewMetricsRegistry("go_backend_core", "scheduler")
	s.SetMetrics(newPromMetrics(reg))

	if err := s.Start(); err != nil {
		return fmt.Errorf("scheduler: 启动失败: %w", err)
	}

	m.scheduler = s
	return nil
}

func (m *Module) Close() error {
	if m.scheduler != nil {
		m.scheduler.Stop()
	}
	return nil
}

func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/tasks", m.listTasks)
	r.POST("/tasks", m.createTask)
	r.GET("/tasks/:id", m.getTask)
	r.PUT("/tasks/:id", m.updateTask)
	r.DELETE("/tasks/:id", m.deleteTask)
	r.POST("/tasks/:id/pause", m.pauseTask)
	r.POST("/tasks/:id/resume", m.resumeTask)
	r.POST("/tasks/:id/run", m.runTaskNow)
	r.GET("/tasks/:id/logs", m.taskLogs)
	r.GET("/handlers", m.listHandlers)
}

// listTasks 任务列表
// @Summary      获取所有定时任务
// @Description  返回所有已注册的定时任务列表
// @Tags         scheduler
// @Produce      json
// @Success      200  {object}  response.Response
// @Failure      500  {object}  response.Response
// @Router       /scheduler/tasks [get]
func (m *Module) listTasks(c *gin.Context) {
	tasks, err := m.scheduler.ListTasks()
	if err != nil {
		response.Fail(c, errors.New(errors.CodeSystemErr, err.Error()))
		return
	}
	response.Success(c, tasks)
}

// createTask 创建任务
// @Summary      创建定时任务
// @Description  创建一个新的定时任务，需指定名称和处理器
// @Tags         scheduler
// @Accept       json
// @Produce      json
// @Param        body  body  Task  true  "任务信息"
// @Success      200   {object}  response.Response
// @Failure      400   {object}  response.Response
// @Failure      409   {object}  response.Response
// @Router       /scheduler/tasks [post]
func (m *Module) createTask(c *gin.Context) {
	var t Task
	if err := c.ShouldBindJSON(&t); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}
	if t.Name == "" {
		response.ParamErr(c, "任务名称不能为空")
		return
	}
	if t.Handler == "" {
		response.ParamErr(c, "处理器名称不能为空")
		return
	}
	if GetHandler(t.Handler) == nil {
		response.ParamErr(c, fmt.Sprintf("处理器 %s 未注册", t.Handler))
		return
	}

	if err := m.scheduler.AddTask(&t); err != nil {
		response.Fail(c, errors.New(errors.CodeConflict, err.Error()))
		return
	}

	created, _ := m.scheduler.GetTask(t.ID)
	response.Success(c, created)
}

// getTask 获取任务详情
// @Summary      获取指定任务详情
// @Description  根据任务 ID 获取任务详细信息
// @Tags         scheduler
// @Produce      json
// @Param        id  path  int  true  "任务 ID"
// @Success      200  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Router       /scheduler/tasks/{id} [get]
func (m *Module) getTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的任务 ID")
		return
	}
	t, err := m.scheduler.GetTask(uint(id))
	if err != nil {
		response.FailCode(c, errors.CodeNotFound)
		return
	}
	response.Success(c, t)
}

// updateTask 更新任务
// @Summary      更新定时任务
// @Description  更新指定 ID 的定时任务配置
// @Tags         scheduler
// @Accept       json
// @Produce      json
// @Param        id    path  int   true  "任务 ID"
// @Param        body  body  Task  true  "任务信息"
// @Success      200   {object}  response.Response
// @Failure      404   {object}  response.Response
// @Router       /scheduler/tasks/{id} [put]
func (m *Module) updateTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的任务 ID")
		return
	}

	var t Task
	if err := c.ShouldBindJSON(&t); err != nil {
		response.ParamErr(c, "请求格式错误: "+err.Error())
		return
	}
	t.ID = uint(id)

	if err := m.scheduler.UpdateTask(&t); err != nil {
		response.FailCode(c, errors.CodeNotFound)
		return
	}

	updated, _ := m.scheduler.GetTask(t.ID)
	response.Success(c, updated)
}

// deleteTask 删除任务
// @Summary      删除定时任务
// @Description  删除指定 ID 的定时任务
// @Tags         scheduler
// @Produce      json
// @Param        id  path  int  true  "任务 ID"
// @Success      200  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Router       /scheduler/tasks/{id} [delete]
func (m *Module) deleteTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的任务 ID")
		return
	}
	if err := m.scheduler.DeleteTask(uint(id)); err != nil {
		response.FailCode(c, errors.CodeNotFound)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// pauseTask 暂停任务
// @Summary      暂停定时任务
// @Description  暂停指定 ID 的定时任务调度
// @Tags         scheduler
// @Produce      json
// @Param        id  path  int  true  "任务 ID"
// @Success      200  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Router       /scheduler/tasks/{id}/pause [post]
func (m *Module) pauseTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的任务 ID")
		return
	}
	if err := m.scheduler.PauseTask(uint(id)); err != nil {
		response.FailCode(c, errors.CodeNotFound)
		return
	}
	updated, _ := m.scheduler.GetTask(uint(id))
	response.Success(c, updated)
}

// resumeTask 恢复任务
// @Summary      恢复定时任务
// @Description  恢复指定 ID 的暂停任务调度
// @Tags         scheduler
// @Produce      json
// @Param        id  path  int  true  "任务 ID"
// @Success      200  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Router       /scheduler/tasks/{id}/resume [post]
func (m *Module) resumeTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的任务 ID")
		return
	}
	if err := m.scheduler.ResumeTask(uint(id)); err != nil {
		response.FailCode(c, errors.CodeNotFound)
		return
	}
	updated, _ := m.scheduler.GetTask(uint(id))
	response.Success(c, updated)
}

// runTaskNow 立即执行任务
// @Summary      立即执行任务
// @Description  立即触发执行指定 ID 的任务，不受调度计划约束
// @Tags         scheduler
// @Produce      json
// @Param        id  path  int  true  "任务 ID"
// @Success      200  {object}  response.Response
// @Failure      404  {object}  response.Response
// @Router       /scheduler/tasks/{id}/run [post]
func (m *Module) runTaskNow(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的任务 ID")
		return
	}
	if err := m.scheduler.RunTaskNow(uint(id)); err != nil {
		response.FailCode(c, errors.CodeNotFound)
		return
	}
	response.Success(c, gin.H{"message": "任务已触发"})
}

// taskLogs 任务日志
// @Summary      获取任务执行日志
// @Description  获取指定任务的执行历史日志
// @Tags         scheduler
// @Produce      json
// @Param        id  path  int  true  "任务 ID"
// @Success      200  {object}  response.Response
// @Router       /scheduler/tasks/{id}/logs [get]
func (m *Module) taskLogs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.ParamErr(c, "无效的任务 ID")
		return
	}
	logs, err := m.scheduler.TaskLogs(uint(id))
	if err != nil {
		logs = []TaskLog{}
	}
	response.Success(c, logs)
}

// listHandlers 处理器列表
// @Summary      获取已注册的处理器列表
// @Description  返回所有已注册的任务处理器名称列表
// @Tags         scheduler
// @Produce      json
// @Success      200  {object}  response.Response
// @Router       /scheduler/handlers [get]
func (m *Module) listHandlers(c *gin.Context) {
	response.Success(c, ListHandlers())
}

var _ interface {
	Name() string
	Init(*config.Config) error
	Close() error
	RegisterRoutes(*gin.RouterGroup)
} = (*Module)(nil)
