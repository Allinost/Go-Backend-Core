package developmentmanager

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// PMService 里程碑/任务业务服务。
type PMService struct {
	repo *PMRepository
}

func NewPMService(repo *PMRepository) *PMService {
	return &PMService{repo: repo}
}

func (s *PMService) ensureProject(ctx context.Context, projectID int64) error {
	ok, err := s.repo.ProjectExists(ctx, projectID)
	if err != nil {
		return err
	}
	if !ok {
		return NewDevelopmentManagerError(ErrProjectNotFound)
	}
	return nil
}

func parseDate(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", value)
	if err != nil {
		return nil, fmt.Errorf("日期格式应为 YYYY-MM-DD: %w", err)
	}
	return &t, nil
}

func parseDateTime(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return &t, nil
	}
	t, err := time.Parse("2006-01-02 15:04:05", value)
	if err != nil {
		return nil, fmt.Errorf("时间格式应为 RFC3339 或 YYYY-MM-DD HH:MM:SS: %w", err)
	}
	return &t, nil
}

// ---------- 里程碑 ----------

func (s *PMService) ListMilestones(ctx context.Context, projectID int64, req *ListMilestonesReq) ([]Milestone, bool, int, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, false, 0, err
	}
	if req.Status != "" && !ValidMilestoneStatuses[req.Status] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidMilestoneStatus)
	}
	return s.repo.ListMilestones(ctx, projectID, req)
}

func (s *PMService) GetMilestone(ctx context.Context, id int64) (*Milestone, error) {
	return s.repo.GetMilestone(ctx, id)
}

func (s *PMService) CreateMilestone(ctx context.Context, projectID int64, req *CreateMilestoneReq, actor string) (*Milestone, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		return nil, NewDevelopmentManagerError(ErrMilestoneVersionRequired)
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, NewDevelopmentManagerError(ErrMilestoneNameRequired)
	}
	status := req.Status
	if status == "" {
		status = MilestoneStatusPlanned
	}
	if !ValidMilestoneStatuses[status] {
		return nil, NewDevelopmentManagerError(ErrInvalidMilestoneStatus)
	}
	start, err := parseDate(req.StartDate)
	if err != nil {
		return nil, err
	}
	target, err := parseDate(req.TargetDate)
	if err != nil {
		return nil, err
	}
	m := &Milestone{
		ProjectID: projectID, Version: version, Name: name, Goal: req.Goal, Status: status,
		StartDate: start, TargetDate: target, Owner: strings.TrimSpace(req.Owner),
		CreatedBy: actor, UpdatedBy: actor,
	}
	return s.repo.CreateMilestone(ctx, m)
}

func (s *PMService) UpdateMilestone(ctx context.Context, id int64, req *UpdateMilestoneReq, actor string) (*Milestone, error) {
	existing, err := s.repo.GetMilestone(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Deleted {
		return nil, NewDevelopmentManagerError(ErrMilestoneNotFound)
	}
	updates := map[string]any{"updated_by": actor}
	if req.Version != nil {
		v := strings.TrimSpace(*req.Version)
		if v == "" {
			return nil, NewDevelopmentManagerError(ErrMilestoneVersionRequired)
		}
		updates["version"] = v
	}
	if req.Name != nil {
		v := strings.TrimSpace(*req.Name)
		if v == "" {
			return nil, NewDevelopmentManagerError(ErrMilestoneNameRequired)
		}
		updates["name"] = v
	}
	if req.Goal != nil {
		updates["goal"] = *req.Goal
	}
	if req.Status != nil {
		if !ValidMilestoneStatuses[*req.Status] {
			return nil, NewDevelopmentManagerError(ErrInvalidMilestoneStatus)
		}
		updates["status"] = *req.Status
	}
	if req.Owner != nil {
		updates["owner"] = strings.TrimSpace(*req.Owner)
	}
	if req.StartDate != nil {
		t, err := parseDate(*req.StartDate)
		if err != nil {
			return nil, err
		}
		updates["start_date"] = t
	}
	if req.TargetDate != nil {
		t, err := parseDate(*req.TargetDate)
		if err != nil {
			return nil, err
		}
		updates["target_date"] = t
	}
	if req.ReleasedAt != nil {
		t, err := parseDateTime(*req.ReleasedAt)
		if err != nil {
			return nil, err
		}
		updates["released_at"] = t
	}
	if err := s.repo.UpdateMilestone(ctx, id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetMilestone(ctx, id)
}

func (s *PMService) DeleteMilestone(ctx context.Context, id int64) error {
	return s.repo.DeleteMilestone(ctx, id)
}

func (s *PMService) MilestoneProgress(ctx context.Context, id int64) (*MilestoneProgress, error) {
	if _, err := s.repo.GetMilestone(ctx, id); err != nil {
		return nil, err
	}
	return s.repo.MilestoneProgress(ctx, id)
}

// ---------- 任务 ----------

func (s *PMService) ListTasks(ctx context.Context, projectID int64, req *ListTasksReq) ([]Task, bool, int, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, false, 0, err
	}
	if req.Status != "" && !ValidTaskStatuses[req.Status] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidTaskStatus)
	}
	if req.Type != "" && !ValidTaskTypes[req.Type] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidTaskType)
	}
	if req.Priority != "" && !ValidRequirementPriorities[req.Priority] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidTaskPriority)
	}
	return s.repo.ListTasks(ctx, projectID, req)
}

func (s *PMService) GetTask(ctx context.Context, id int64) (*Task, error) {
	return s.repo.GetTask(ctx, id)
}

func (s *PMService) validateTaskRefs(ctx context.Context, projectID int64, milestoneID, requirementID, parentID *int64) error {
	if milestoneID != nil && *milestoneID > 0 {
		ok, err := s.repo.milestoneInProject(ctx, *milestoneID, projectID)
		if err != nil {
			return err
		}
		if !ok {
			return NewDevelopmentManagerError(ErrTaskCrossProject).WithDetail("里程碑不属于当前项目")
		}
	}
	if requirementID != nil && *requirementID > 0 {
		ok, err := s.repo.requirementInProject(ctx, *requirementID, projectID)
		if err != nil {
			return err
		}
		if !ok {
			return NewDevelopmentManagerError(ErrTaskCrossProject).WithDetail("需求不属于当前项目")
		}
	}
	if parentID != nil && *parentID > 0 {
		ok, err := s.repo.taskInProject(ctx, *parentID, projectID)
		if err != nil {
			return err
		}
		if !ok {
			return NewDevelopmentManagerError(ErrTaskCrossProject).WithDetail("父任务不属于当前项目")
		}
	}
	return nil
}

func normalizeID(id *int64) *int64 {
	if id == nil || *id <= 0 {
		return nil
	}
	return id
}

func (s *PMService) CreateTask(ctx context.Context, projectID int64, req *CreateTaskReq, actor string) (*Task, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, NewDevelopmentManagerError(ErrTaskTitleRequired)
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		var err error
		code, err = s.repo.NextTaskCode(ctx)
		if err != nil {
			return nil, err
		}
	}
	typ := req.Type
	if typ == "" {
		typ = TaskTypeFeature
	}
	if !ValidTaskTypes[typ] {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskType)
	}
	status := req.Status
	if status == "" {
		status = TaskStatusTodo
	}
	if !ValidTaskStatuses[status] {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskStatus)
	}
	priority := req.Priority
	if priority == "" {
		priority = PriorityP2
	}
	if !ValidRequirementPriorities[priority] {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskPriority)
	}
	estimate := 0.0
	if req.EstimateHours != nil {
		if *req.EstimateHours < 0 {
			return nil, NewDevelopmentManagerError(ErrInvalidTaskHours)
		}
		estimate = *req.EstimateHours
	}
	milestoneID := normalizeID(req.MilestoneID)
	requirementID := normalizeID(req.RequirementID)
	parentID := normalizeID(req.ParentID)
	if err := s.validateTaskRefs(ctx, projectID, milestoneID, requirementID, parentID); err != nil {
		return nil, err
	}
	start, err := parseDate(req.StartDate)
	if err != nil {
		return nil, err
	}
	due, err := parseDate(req.DueDate)
	if err != nil {
		return nil, err
	}
	orderNo := 0
	if req.OrderNo != nil {
		orderNo = *req.OrderNo
	}
	t := &Task{
		ProjectID: projectID, MilestoneID: milestoneID, IterationID: normalizeID(req.IterationID),
		RequirementID: requirementID, ParentID: parentID, Code: code, Title: title,
		Description: req.Description, Type: typ, Status: status, Priority: priority,
		EstimateHours: estimate, Owner: strings.TrimSpace(req.Owner), StartDate: start, DueDate: due,
		OrderNo: orderNo, CreatedBy: actor, UpdatedBy: actor,
	}
	return s.repo.CreateTask(ctx, t)
}

func (s *PMService) UpdateTask(ctx context.Context, id int64, req *UpdateTaskReq, actor string) (*Task, error) {
	existing, err := s.repo.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Deleted {
		return nil, NewDevelopmentManagerError(ErrTaskNotFound)
	}
	updates := map[string]any{"updated_by": actor}

	if req.Code != nil {
		code := strings.TrimSpace(*req.Code)
		if code == "" {
			return nil, NewDevelopmentManagerError(ErrTaskCodeRequired)
		}
		updates["code"] = code
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, NewDevelopmentManagerError(ErrTaskTitleRequired)
		}
		updates["title"] = title
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.Type != nil {
		if !ValidTaskTypes[*req.Type] {
			return nil, NewDevelopmentManagerError(ErrInvalidTaskType)
		}
		updates["type"] = *req.Type
	}
	if req.Priority != nil {
		if !ValidRequirementPriorities[*req.Priority] {
			return nil, NewDevelopmentManagerError(ErrInvalidTaskPriority)
		}
		updates["priority"] = *req.Priority
	}
	if req.EstimateHours != nil {
		if *req.EstimateHours < 0 {
			return nil, NewDevelopmentManagerError(ErrInvalidTaskHours)
		}
		updates["estimate_hours"] = *req.EstimateHours
	}
	if req.ActualHours != nil {
		if *req.ActualHours < 0 {
			return nil, NewDevelopmentManagerError(ErrInvalidTaskHours)
		}
		updates["actual_hours"] = *req.ActualHours
	}
	if req.Owner != nil {
		updates["owner"] = strings.TrimSpace(*req.Owner)
	}
	if req.OrderNo != nil {
		updates["order_no"] = *req.OrderNo
	}
	if req.MilestoneID != nil {
		updates["milestone_id"] = normalizeID(req.MilestoneID)
	}
	if req.IterationID != nil {
		updates["iteration_id"] = normalizeID(req.IterationID)
	}
	if req.RequirementID != nil {
		updates["requirement_id"] = normalizeID(req.RequirementID)
	}
	if req.ParentID != nil {
		updates["parent_id"] = normalizeID(req.ParentID)
	}
	if err := s.validateTaskRefs(ctx, existing.ProjectID,
		normalizeID(req.MilestoneID), normalizeID(req.RequirementID), normalizeID(req.ParentID)); err != nil {
		return nil, err
	}
	if req.StartDate != nil {
		t, err := parseDate(*req.StartDate)
		if err != nil {
			return nil, err
		}
		updates["start_date"] = t
	}
	if req.DueDate != nil {
		t, err := parseDate(*req.DueDate)
		if err != nil {
			return nil, err
		}
		updates["due_date"] = t
	}
	if err := s.repo.UpdateTask(ctx, id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetTask(ctx, id)
}

func (s *PMService) DeleteTask(ctx context.Context, id int64) error {
	return s.repo.DeleteTask(ctx, id)
}

func (s *PMService) ChangeTaskStatus(ctx context.Context, id int64, req *ChangeTaskStatusReq, actor string) (*Task, error) {
	existing, err := s.repo.GetTask(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Deleted {
		return nil, NewDevelopmentManagerError(ErrTaskNotFound)
	}
	if !ValidTaskStatuses[req.Status] {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskStatus)
	}
	if !CanTransitionTask(existing.Status, req.Status) {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskTransition).WithDetail(existing.Status + " -> " + req.Status)
	}
	hours := 0.0
	if req.Hours != nil {
		if *req.Hours < 0 {
			return nil, NewDevelopmentManagerError(ErrInvalidTaskHours)
		}
		hours = *req.Hours
	}
	if err := s.repo.ChangeTaskStatusTx(ctx, id, existing.Status, req.Status, hours, req.Note, actor); err != nil {
		return nil, err
	}
	return s.repo.GetTask(ctx, id)
}

func (s *PMService) ListTaskLogs(ctx context.Context, taskID int64, limit int) ([]TaskLog, error) {
	if _, err := s.repo.GetTask(ctx, taskID); err != nil {
		return nil, err
	}
	return s.repo.ListTaskLogs(ctx, taskID, limit)
}

func (s *PMService) AddTaskLog(ctx context.Context, taskID int64, req *AddTaskLogReq, actor string) (*TaskLog, error) {
	if _, err := s.repo.GetTask(ctx, taskID); err != nil {
		return nil, err
	}
	if req.FromStatus != "" && !ValidTaskStatuses[req.FromStatus] {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskStatus)
	}
	if req.ToStatus != "" && !ValidTaskStatuses[req.ToStatus] {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskStatus)
	}
	if req.Hours != nil && *req.Hours < 0 {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskHours)
	}
	return s.repo.AddTaskLog(ctx, taskID, req, actor)
}

func (s *PMService) ListDependencies(ctx context.Context, taskID int64) ([]TaskDependency, error) {
	if _, err := s.repo.GetTask(ctx, taskID); err != nil {
		return nil, err
	}
	return s.repo.ListDependencies(ctx, taskID)
}

func (s *PMService) AddDependency(ctx context.Context, taskID int64, req *AddTaskDependencyReq, actor string) (*TaskDependency, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if req.DependsOnTaskID == taskID {
		return nil, NewDevelopmentManagerError(ErrTaskSelfDependency)
	}
	dependsOn, err := s.repo.GetTask(ctx, req.DependsOnTaskID)
	if err != nil {
		return nil, err
	}
	if task.ProjectID != dependsOn.ProjectID {
		return nil, NewDevelopmentManagerError(ErrTaskCrossProject)
	}
	depType := strings.ToUpper(strings.TrimSpace(req.DepType))
	if depType == "" {
		depType = DependencyFS
	}
	if !ValidDependencyTypes[depType] {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskTransition).WithDetail("依赖类型应为 FS/SS/FF/SF")
	}
	cycle, err := s.repo.WouldCreateCycle(ctx, taskID, req.DependsOnTaskID)
	if err != nil {
		return nil, err
	}
	if cycle {
		return nil, NewDevelopmentManagerError(ErrTaskDependencyCycle)
	}
	req.DepType = depType
	return s.repo.AddDependency(ctx, taskID, req)
}

func (s *PMService) RemoveDependency(ctx context.Context, taskID, depID int64) error {
	if _, err := s.repo.GetTask(ctx, taskID); err != nil {
		return err
	}
	return s.repo.RemoveDependency(ctx, taskID, depID)
}

// ImportTasks 按任务编号幂等导入：已存在则更新内容，不改变状态。
func (s *PMService) ImportTasks(ctx context.Context, projectID int64, req *ImportTasksReq, actor string) (*ImportTasksResp, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	resp := &ImportTasksResp{}
	for _, item := range req.Tasks {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			resp.Skipped++
			continue
		}
		existing, getErr := s.repo.GetTaskByCode(ctx, projectID, code)
		if getErr != nil {
			if de, ok := getErr.(*DevelopmentManagerError); !ok || de.Code != ErrTaskNotFound {
				return nil, getErr
			}
			create := &CreateTaskReq{
				MilestoneID: item.MilestoneID, IterationID: item.IterationID, RequirementID: item.RequirementID,
				ParentID: item.ParentID, Code: code, Title: item.Title, Description: item.Description,
				Type: item.Type, Status: item.Status, Priority: item.Priority, EstimateHours: item.EstimateHours,
				Owner: item.Owner, StartDate: item.StartDate, DueDate: item.DueDate, OrderNo: item.OrderNo,
			}
			if _, err := s.CreateTask(ctx, projectID, create, actor); err != nil {
				return nil, err
			}
			resp.Created++
			continue
		}

		update := &UpdateTaskReq{
			MilestoneID: item.MilestoneID, IterationID: item.IterationID, RequirementID: item.RequirementID,
			ParentID: item.ParentID, Code: &code, Title: &item.Title, Description: &item.Description,
			EstimateHours: item.EstimateHours, ActualHours: item.ActualHours, Owner: &item.Owner,
			StartDate: &item.StartDate, DueDate: &item.DueDate, OrderNo: item.OrderNo,
		}
		if item.Type != "" {
			update.Type = &item.Type
		}
		if item.Priority != "" {
			update.Priority = &item.Priority
		}
		if _, err := s.UpdateTask(ctx, existing.ID, update, actor); err != nil {
			return nil, err
		}
		resp.Updated++
	}
	return resp, nil
}
