package developmentmanager

import (
	"context"
	"strings"
)

// ---------- Iteration ----------

func (s *PMService) ListIterations(ctx context.Context, projectID int64, req *ListProjectItemsReq) ([]Iteration, bool, int, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, false, 0, err
	}
	if req.Status != "" && !ValidIterationStatuses[req.Status] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidIterationStatus)
	}
	return s.repo.ListIterations(ctx, projectID, req)
}

func (s *PMService) CreateIteration(ctx context.Context, projectID int64, req *CreateIterationReq, actor string) (*Iteration, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, NewDevelopmentManagerError(ErrMilestoneNameRequired)
	}
	status := req.Status
	if status == "" {
		status = "planned"
	}
	if !ValidIterationStatuses[status] {
		return nil, NewDevelopmentManagerError(ErrInvalidIterationStatus)
	}
	if req.CapacityHours < 0 {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskHours)
	}
	start, err := parseDate(req.StartDate)
	if err != nil {
		return nil, err
	}
	end, err := parseDate(req.EndDate)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateIteration(ctx, &Iteration{
		ProjectID: projectID, MilestoneID: normalizeID(req.MilestoneID), Name: name, Goal: req.Goal,
		StartDate: start, EndDate: end, Status: status, CapacityHours: req.CapacityHours,
		CreatedBy: actor, UpdatedBy: actor,
	})
}

func (s *PMService) UpdateIteration(ctx context.Context, id int64, req *UpdateIterationReq, actor string) (*Iteration, error) {
	updates := map[string]any{"updated_by": actor}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			return nil, NewDevelopmentManagerError(ErrMilestoneNameRequired)
		}
		updates["name"] = name
	}
	if req.Goal != nil {
		updates["goal"] = *req.Goal
	}
	if req.Status != nil {
		if !ValidIterationStatuses[*req.Status] {
			return nil, NewDevelopmentManagerError(ErrInvalidIterationStatus)
		}
		updates["status"] = *req.Status
	}
	if req.CapacityHours != nil {
		if *req.CapacityHours < 0 {
			return nil, NewDevelopmentManagerError(ErrInvalidTaskHours)
		}
		updates["capacity_hours"] = *req.CapacityHours
	}
	if req.MilestoneID != nil {
		updates["milestone_id"] = normalizeID(req.MilestoneID)
	}
	if req.StartDate != nil {
		t, err := parseDate(*req.StartDate)
		if err != nil {
			return nil, err
		}
		updates["start_date"] = t
	}
	if req.EndDate != nil {
		t, err := parseDate(*req.EndDate)
		if err != nil {
			return nil, err
		}
		updates["end_date"] = t
	}
	if err := s.repo.UpdateIteration(ctx, id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetIteration(ctx, id)
}

func (s *PMService) GetIteration(ctx context.Context, id int64) (*Iteration, error) {
	return s.repo.GetIteration(ctx, id)
}

func (s *PMService) DeleteIteration(ctx context.Context, id int64) error {
	return s.repo.DeleteIteration(ctx, id)
}

// ---------- Risk ----------

func (s *PMService) ListRisks(ctx context.Context, projectID int64, req *ListProjectItemsReq) ([]Risk, bool, int, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, false, 0, err
	}
	if req.Status != "" && !ValidRiskStatuses[req.Status] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidRiskStatus)
	}
	return s.repo.ListRisks(ctx, projectID, req)
}

func (s *PMService) CreateRisk(ctx context.Context, projectID int64, req *CreateRiskReq, actor string) (*Risk, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, NewDevelopmentManagerError(ErrTaskTitleRequired)
	}
	probability := req.Probability
	if probability == "" {
		probability = "medium"
	}
	if !ValidRiskProbability[probability] {
		return nil, NewDevelopmentManagerError(ErrInvalidRiskProbability)
	}
	impact := req.Impact
	if impact == "" {
		impact = "medium"
	}
	if !ValidRiskImpact[impact] {
		return nil, NewDevelopmentManagerError(ErrInvalidRiskImpact)
	}
	level := req.Level
	if level == "" {
		level = "medium"
	}
	if !ValidRiskLevels[level] {
		return nil, NewDevelopmentManagerError(ErrInvalidRiskLevel)
	}
	status := req.Status
	if status == "" {
		status = "open"
	}
	if !ValidRiskStatuses[status] {
		return nil, NewDevelopmentManagerError(ErrInvalidRiskStatus)
	}
	due, err := parseDate(req.DueDate)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateRisk(ctx, &Risk{
		ProjectID: projectID, MilestoneID: normalizeID(req.MilestoneID), TaskID: normalizeID(req.TaskID),
		Title: title, Description: req.Description, Probability: probability, Impact: impact, Level: level,
		Mitigation: req.Mitigation, Owner: strings.TrimSpace(req.Owner), Status: status, DueDate: due,
		CreatedBy: actor, UpdatedBy: actor,
	})
}

func (s *PMService) UpdateRisk(ctx context.Context, id int64, req *UpdateRiskReq, actor string) (*Risk, error) {
	updates := map[string]any{"updated_by": actor}
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
	if req.Probability != nil {
		if !ValidRiskProbability[*req.Probability] {
			return nil, NewDevelopmentManagerError(ErrInvalidRiskProbability)
		}
		updates["probability"] = *req.Probability
	}
	if req.Impact != nil {
		if !ValidRiskImpact[*req.Impact] {
			return nil, NewDevelopmentManagerError(ErrInvalidRiskImpact)
		}
		updates["impact"] = *req.Impact
	}
	if req.Level != nil {
		if !ValidRiskLevels[*req.Level] {
			return nil, NewDevelopmentManagerError(ErrInvalidRiskLevel)
		}
		updates["level"] = *req.Level
	}
	if req.Mitigation != nil {
		updates["mitigation"] = *req.Mitigation
	}
	if req.Owner != nil {
		updates["owner"] = strings.TrimSpace(*req.Owner)
	}
	if req.Status != nil {
		if !ValidRiskStatuses[*req.Status] {
			return nil, NewDevelopmentManagerError(ErrInvalidRiskStatus)
		}
		updates["status"] = *req.Status
	}
	if req.MilestoneID != nil {
		updates["milestone_id"] = normalizeID(req.MilestoneID)
	}
	if req.TaskID != nil {
		updates["task_id"] = normalizeID(req.TaskID)
	}
	if req.DueDate != nil {
		due, err := parseDate(*req.DueDate)
		if err != nil {
			return nil, err
		}
		updates["due_date"] = due
	}
	if err := s.repo.UpdateRisk(ctx, id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetRisk(ctx, id)
}

func (s *PMService) GetRisk(ctx context.Context, id int64) (*Risk, error) {
	return s.repo.GetRisk(ctx, id)
}

func (s *PMService) DeleteRisk(ctx context.Context, id int64) error {
	return s.repo.DeleteRisk(ctx, id)
}

// ---------- Release ----------

func (s *PMService) ListReleases(ctx context.Context, projectID int64, req *ListProjectItemsReq) ([]Release, bool, int, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, false, 0, err
	}
	if req.Status != "" && !ValidReleaseStatuses[req.Status] {
		return nil, false, 0, NewDevelopmentManagerError(ErrInvalidReleaseStatus)
	}
	return s.repo.ListReleases(ctx, projectID, req)
}

func (s *PMService) CreateRelease(ctx context.Context, projectID int64, req *CreateReleaseReq, actor string) (*Release, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	version := strings.TrimSpace(req.Version)
	name := strings.TrimSpace(req.Name)
	if version == "" || name == "" {
		return nil, NewDevelopmentManagerError(ErrMilestoneNameRequired)
	}
	releaseType := req.ReleaseType
	if releaseType == "" {
		releaseType = "minor"
	}
	if !ValidReleaseTypes[releaseType] {
		return nil, NewDevelopmentManagerError(ErrInvalidReleaseType)
	}
	status := req.Status
	if status == "" {
		status = "planned"
	}
	if !ValidReleaseStatuses[status] {
		return nil, NewDevelopmentManagerError(ErrInvalidReleaseStatus)
	}
	releasedAt, err := parseDateTime(req.ReleasedAt)
	if err != nil {
		return nil, err
	}
	return s.repo.CreateRelease(ctx, &Release{
		ProjectID: projectID, MilestoneID: normalizeID(req.MilestoneID), Version: version, Name: name,
		ReleaseType: releaseType, Status: status, ReleasedAt: releasedAt, ChangeSummary: req.ChangeSummary,
		RollbackPlan: req.RollbackPlan, DeployedBy: strings.TrimSpace(req.DeployedBy),
		CreatedBy: actor, UpdatedBy: actor,
	})
}

func (s *PMService) UpdateRelease(ctx context.Context, id int64, req *UpdateReleaseReq, actor string) (*Release, error) {
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
	if req.ReleaseType != nil {
		if !ValidReleaseTypes[*req.ReleaseType] {
			return nil, NewDevelopmentManagerError(ErrInvalidReleaseType)
		}
		updates["release_type"] = *req.ReleaseType
	}
	if req.Status != nil {
		if !ValidReleaseStatuses[*req.Status] {
			return nil, NewDevelopmentManagerError(ErrInvalidReleaseStatus)
		}
		updates["status"] = *req.Status
	}
	if req.ChangeSummary != nil {
		updates["change_summary"] = *req.ChangeSummary
	}
	if req.RollbackPlan != nil {
		updates["rollback_plan"] = *req.RollbackPlan
	}
	if req.DeployedBy != nil {
		updates["deployed_by"] = strings.TrimSpace(*req.DeployedBy)
	}
	if req.MilestoneID != nil {
		updates["milestone_id"] = normalizeID(req.MilestoneID)
	}
	if req.ReleasedAt != nil {
		t, err := parseDateTime(*req.ReleasedAt)
		if err != nil {
			return nil, err
		}
		updates["released_at"] = t
	}
	if err := s.repo.UpdateRelease(ctx, id, updates); err != nil {
		return nil, err
	}
	return s.repo.GetRelease(ctx, id)
}

func (s *PMService) GetRelease(ctx context.Context, id int64) (*Release, error) {
	return s.repo.GetRelease(ctx, id)
}

func (s *PMService) DeleteRelease(ctx context.Context, id int64) error {
	return s.repo.DeleteRelease(ctx, id)
}

// ---------- Baseline ----------

func (s *PMService) ListBaselines(ctx context.Context, projectID int64) ([]Baseline, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListBaselines(ctx, projectID, 50)
}

func (s *PMService) CreateBaseline(ctx context.Context, projectID int64, req *CreateBaselineReq, actor string) (*Baseline, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, NewDevelopmentManagerError(ErrMilestoneNameRequired)
	}
	return s.repo.CreateBaseline(ctx, &Baseline{
		ProjectID: projectID, MilestoneID: normalizeID(req.MilestoneID), Name: name, Note: req.Note, CreatedBy: actor,
	})
}

func (s *PMService) GetBaseline(ctx context.Context, id int64) (*Baseline, error) {
	return s.repo.GetBaseline(ctx, id)
}

func (s *PMService) DeleteBaseline(ctx context.Context, id int64) error {
	return s.repo.DeleteBaseline(ctx, id)
}

// ---------- Comment / checklist ----------

func (s *PMService) ListTaskComments(ctx context.Context, taskID int64) ([]Comment, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return s.repo.ListComments(ctx, task.ProjectID, "task", taskID)
}

func (s *PMService) AddTaskComment(ctx context.Context, taskID int64, content, operator string) (*Comment, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, NewDevelopmentManagerError(ErrTaskTitleRequired).WithDetail("评论内容不能为空")
	}
	return s.repo.CreateComment(ctx, &Comment{ProjectID: task.ProjectID, TargetType: "task", TargetID: taskID, Content: content, Operator: operator})
}

func (s *PMService) DeleteComment(ctx context.Context, id int64) error {
	return s.repo.DeleteComment(ctx, id)
}

func (s *PMService) ListChecklist(ctx context.Context, taskID int64) ([]ChecklistItem, error) {
	if _, err := s.repo.GetTask(ctx, taskID); err != nil {
		return nil, err
	}
	return s.repo.ListChecklist(ctx, taskID)
}

func (s *PMService) CreateChecklistItem(ctx context.Context, taskID int64, req *CreateChecklistReq) (*ChecklistItem, error) {
	if _, err := s.repo.GetTask(ctx, taskID); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		return nil, NewDevelopmentManagerError(ErrTaskTitleRequired).WithDetail("检查项标题不能为空")
	}
	return s.repo.CreateChecklistItem(ctx, &ChecklistItem{TaskID: taskID, Title: title, Done: req.Done, OrderNo: req.OrderNo})
}

func (s *PMService) UpdateChecklistItem(ctx context.Context, id int64, req *UpdateChecklistReq) error {
	updates := map[string]any{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return NewDevelopmentManagerError(ErrTaskTitleRequired).WithDetail("检查项标题不能为空")
		}
		updates["title"] = title
	}
	if req.Done != nil {
		updates["done"] = *req.Done
	}
	if req.OrderNo != nil {
		updates["order_no"] = *req.OrderNo
	}
	return s.repo.UpdateChecklistItem(ctx, id, updates)
}

func (s *PMService) DeleteChecklistItem(ctx context.Context, id int64) error {
	return s.repo.DeleteChecklistItem(ctx, id)
}

// ---------- Label / task label / task asset ----------

func (s *PMService) ListLabels(ctx context.Context, projectID int64) ([]Label, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListLabels(ctx, projectID)
}

func (s *PMService) CreateLabel(ctx context.Context, projectID int64, req *CreateLabelReq) (*Label, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, NewDevelopmentManagerError(ErrMilestoneNameRequired).WithDetail("标签名不能为空")
	}
	color := strings.TrimSpace(req.Color)
	if color == "" {
		color = "#1890ff"
	}
	return s.repo.CreateLabel(ctx, &Label{ProjectID: projectID, Name: name, Color: color})
}

func (s *PMService) DeleteLabel(ctx context.Context, id int64) error {
	return s.repo.DeleteLabel(ctx, id)
}

func (s *PMService) ListTaskLabels(ctx context.Context, taskID int64) ([]Label, error) {
	if _, err := s.repo.GetTask(ctx, taskID); err != nil {
		return nil, err
	}
	return s.repo.ListTaskLabels(ctx, taskID)
}

func (s *PMService) AddTaskLabel(ctx context.Context, taskID int64, req *AddTaskLabelReq) (*TaskLabel, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.labelInProject(ctx, req.LabelID, task.ProjectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, NewDevelopmentManagerError(ErrLabelNotFound)
	}
	return s.repo.AddTaskLabel(ctx, taskID, req.LabelID)
}

func (s *PMService) RemoveTaskLabel(ctx context.Context, taskID, labelID int64) error {
	return s.repo.RemoveTaskLabel(ctx, taskID, labelID)
}

func (s *PMService) ListTaskAssets(ctx context.Context, taskID int64) ([]TaskAsset, error) {
	if _, err := s.repo.GetTask(ctx, taskID); err != nil {
		return nil, err
	}
	return s.repo.ListTaskAssets(ctx, taskID)
}

func (s *PMService) AddTaskAsset(ctx context.Context, taskID int64, req *AddTaskAssetReq) (*TaskAsset, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.assetInProject(ctx, req.AssetID, task.ProjectID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, NewDevelopmentManagerError(ErrAssetNotFound)
	}
	relation := req.RelationType
	if relation == "" {
		relation = "input"
	}
	if !ValidTaskAssetRelations[relation] {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskAssetRelation)
	}
	return s.repo.AddTaskAsset(ctx, &TaskAsset{TaskID: taskID, AssetID: req.AssetID, RelationType: relation})
}

func (s *PMService) RemoveTaskAsset(ctx context.Context, taskID, assetID int64) error {
	return s.repo.RemoveTaskAsset(ctx, taskID, assetID)
}
