package developmentmanager

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/logger"
	"github.com/Allinost/go-backend-core/zzz/developmentManager/generation"
)

// CreateGeneration 触发生成任务：创建任务记录并同步执行（模板/AI/手工）
func (s *Service) CreateGeneration(ctx context.Context, requirementID int64, req *CreateGenerationReq, actor string) (*Generation, error) {
	requirement, err := s.repo.GetRequirement(ctx, requirementID)
	if err != nil {
		return nil, err
	}
	if requirement.Deleted {
		return nil, NewDevelopmentManagerError(ErrRequirementDeleted)
	}
	project, err := s.requireProject(ctx, requirement.ProjectID)
	if err != nil {
		return nil, err
	}

	kind := strings.TrimSpace(req.Kind)
	if !ValidGenerationKinds[kind] {
		return nil, NewDevelopmentManagerError(ErrInvalidGenerationKind)
	}
	engine := strings.TrimSpace(req.Engine)
	if engine == "" {
		engine = GenerationEngineTemplate
	}
	if !ValidGenerationEngines[engine] {
		return nil, NewDevelopmentManagerError(ErrInvalidGenerationEngine)
	}
	if req.AutoCommit && kind != GenerationKindCode {
		return nil, NewDevelopmentManagerError(ErrAutoCommitKind)
	}

	snapshot, err := buildGenerationSnapshot(requirement, project, kind, engine, req)
	if err != nil {
		return nil, err
	}
	g := &Generation{
		RequirementID: requirement.ID,
		ProjectID:     project.ID,
		Kind:          kind,
		Engine:        engine,
		AutoCommit:    req.AutoCommit,
		Status:        GenerationStatusPending,
		InputSnapshot: snapshot,
		Operator:      actor,
	}
	g, err = s.repo.CreateGeneration(ctx, g)
	if err != nil {
		return nil, err
	}
	return s.runGeneration(ctx, g, requirement, project)
}

// RetryGeneration 重试失败的生成任务（沿用原请求参数）
func (s *Service) RetryGeneration(ctx context.Context, id int64, actor string) (*Generation, error) {
	g, err := s.repo.GetGeneration(ctx, id)
	if err != nil {
		return nil, err
	}
	if g.Status == GenerationStatusSuccess {
		return nil, NewDevelopmentManagerError(ErrGenerationAlreadySuccess)
	}
	requirement, err := s.repo.GetRequirement(ctx, g.RequirementID)
	if err != nil {
		return nil, err
	}
	if requirement.Deleted {
		return nil, NewDevelopmentManagerError(ErrRequirementDeleted)
	}
	project, err := s.requireProject(ctx, requirement.ProjectID)
	if err != nil {
		return nil, err
	}

	g.Status = GenerationStatusPending
	g.EngineUsed = ""
	g.OutputRef = ""
	g.Files = nil
	g.Error = ""
	g.Committed = false
	g.Branch = ""
	g.CommitHash = ""
	g.StartedAt = nil
	g.FinishedAt = nil
	g.Operator = actor
	if err := s.repo.UpdateGeneration(ctx, g); err != nil {
		return nil, err
	}
	return s.runGeneration(ctx, g, requirement, project)
}

// GetGeneration 查询生成任务
func (s *Service) GetGeneration(ctx context.Context, id int64) (*Generation, error) {
	return s.repo.GetGeneration(ctx, id)
}

// ListGenerations 查询需求下的生成任务
func (s *Service) ListGenerations(ctx context.Context, requirementID int64) ([]Generation, error) {
	if _, err := s.repo.GetRequirement(ctx, requirementID); err != nil {
		return nil, err
	}
	return s.repo.ListGenerationsByRequirement(ctx, requirementID)
}

// buildGenerationSnapshot 构造输入快照（用于审计与重试）
func buildGenerationSnapshot(requirement *Requirement, project *Project, kind, engine string, req *CreateGenerationReq) (json.RawMessage, error) {
	snapshot := map[string]any{
		"requirement": map[string]any{
			"id":       requirement.ID,
			"code":     requirement.Code,
			"title":    requirement.Title,
			"content":  requirement.Content,
			"type":     requirement.Type,
			"priority": requirement.Priority,
			"source":   requirement.Source,
			"owner":    requirement.Owner,
			"status":   requirement.Status,
		},
		"project": map[string]any{
			"id":        project.ID,
			"key":       project.ProjectKey,
			"name":      project.Name,
			"type":      project.Type,
			"repo_path": project.RepoPath,
		},
		"kind":             kind,
		"requested_engine": engine,
		"auto_commit":      req.AutoCommit,
		"instructions":     req.Instructions,
	}
	b, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// runGeneration 执行生成并回写任务状态
func (s *Service) runGeneration(ctx context.Context, g *Generation, requirement *Requirement, project *Project) (*Generation, error) {
	started := time.Now()
	g.Status = GenerationStatusRunning
	g.StartedAt = &started
	g.Error = ""
	if err := s.repo.UpdateGeneration(ctx, g); err != nil {
		return nil, err
	}

	in := generation.Input{
		GenerationID: g.ID,
		Kind:         g.Kind,
		Engine:       g.Engine,
		Instructions: snapshotInstructions(g.InputSnapshot),
		Requirement: generation.RequirementInfo{
			Code:     requirement.Code,
			Title:    requirement.Title,
			Content:  requirement.Content,
			Type:     requirement.Type,
			Priority: requirement.Priority,
			Source:   requirement.Source,
			Owner:    requirement.Owner,
		},
		Project: generation.ProjectInfo{
			Key:      project.ProjectKey,
			Name:     project.Name,
			Type:     project.Type,
			RepoPath: project.RepoPath,
		},
	}

	out, genErr := s.generate(ctx, in)
	if genErr != nil {
		return s.finishFailed(ctx, g, genErr)
	}

	written, werr := generation.WriteFiles("", out.Files)
	if werr != nil {
		return s.finishFailed(ctx, g, NewDevelopmentManagerError(ErrGenerationOutputWrite).WithDetail(werr.Error()))
	}

	g.EngineUsed = out.EngineUsed
	g.OutputRef = out.OutputRef
	g.Files = written
	g.Status = GenerationStatusSuccess

	if g.AutoCommit && g.Kind == GenerationKindCode {
		msg := fmt.Sprintf("chore(dm): generate %s %s (generation %d)", requirement.Code, g.Kind, g.ID)
		branch, hash, cerr := generation.CommitFiles("", msg, written)
		if cerr != nil {
			// 产物已生成，提交失败仅记录，不判定任务失败
			g.Error = "自动提交失败: " + cerr.Error()
			logger.Warn().Err(cerr).Int64("generation_id", g.ID).Msg("developmentmanager: 自动提交失败")
		} else {
			g.Committed = true
			g.Branch = branch
			g.CommitHash = hash
		}
	}

	finished := time.Now()
	g.FinishedAt = &finished
	if err := s.repo.UpdateGeneration(ctx, g); err != nil {
		return nil, err
	}
	return g, nil
}

// generate 按引擎生成产物：AI 不可用或失败时自动降级模板
func (s *Service) generate(ctx context.Context, in generation.Input) (generation.Output, error) {
	switch in.Engine {
	case GenerationEngineAI:
		if s.aiWriter != nil {
			system, prompt := generation.BuildAIPrompt(in)
			content, err := s.aiWriter.Write(ctx, system, prompt)
			if err == nil && strings.TrimSpace(content) != "" {
				return generation.WrapAIContent(in, content), nil
			}
			logger.Warn().Err(err).Msg("developmentmanager: AI 生成失败，降级为模板引擎")
		} else {
			logger.Warn().Msg("developmentmanager: 未配置 AI 生成器，降级为模板引擎")
		}
		return generation.GenerateTemplate(in), nil
	case GenerationEngineManual:
		out := generation.GenerateTemplate(in)
		out.EngineUsed = GenerationEngineManual
		return out, nil
	default:
		return generation.GenerateTemplate(in), nil
	}
}

// finishFailed 标记任务失败
func (s *Service) finishFailed(ctx context.Context, g *Generation, cause error) (*Generation, error) {
	finished := time.Now()
	g.Status = GenerationStatusFailed
	g.FinishedAt = &finished
	g.Error = cause.Error()
	if err := s.repo.UpdateGeneration(ctx, g); err != nil {
		return nil, err
	}
	logger.Warn().Err(cause).Int64("generation_id", g.ID).Msg("developmentmanager: 生成任务失败")
	return g, nil
}

// snapshotInstructions 从输入快照读取补充要求（重试时复用）
func snapshotInstructions(raw json.RawMessage) string {
	var m struct {
		Instructions string `json:"instructions"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m.Instructions
}
