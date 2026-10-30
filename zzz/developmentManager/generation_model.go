package developmentmanager

import (
	"encoding/json"
	"time"

	"github.com/Allinost/go-backend-core/zzz/developmentManager/generation"
)

// 生成类型（与 generation 包常量保持一致）
const (
	GenerationKindPlan = generation.KindPlan
	GenerationKindCode = generation.KindCode
	GenerationKindDoc  = generation.KindDoc
)

// 生成引擎
const (
	GenerationEngineAI       = generation.EngineAI
	GenerationEngineTemplate = generation.EngineTemplate
	GenerationEngineManual   = generation.EngineManual
)

// 生成任务状态
const (
	GenerationStatusPending = "pending"
	GenerationStatusRunning = "running"
	GenerationStatusSuccess = "success"
	GenerationStatusFailed  = "failed"
)

// ValidGenerationKinds 生成类型合法集合
var ValidGenerationKinds = map[string]bool{
	GenerationKindPlan: true, GenerationKindCode: true, GenerationKindDoc: true,
}

// ValidGenerationEngines 生成引擎合法集合
var ValidGenerationEngines = map[string]bool{
	GenerationEngineAI: true, GenerationEngineTemplate: true, GenerationEngineManual: true,
}

// Generation 生成任务实体
type Generation struct {
	ID            int64           `json:"id"`
	RequirementID int64           `json:"requirement_id"`
	ProjectID     int64           `json:"project_id"`
	Kind          string          `json:"kind"`
	Engine        string          `json:"engine"`
	EngineUsed    string          `json:"engine_used"`
	AutoCommit    bool            `json:"auto_commit"`
	Committed     bool            `json:"committed"`
	Branch        string          `json:"branch"`
	CommitHash    string          `json:"commit_hash"`
	Status        string          `json:"status"`
	InputSnapshot json.RawMessage `json:"input_snapshot,omitempty"`
	OutputRef     string          `json:"output_ref"`
	Files         []string        `json:"files,omitempty"`
	Error         string          `json:"error"`
	Operator      string          `json:"operator"`
	StartedAt     *time.Time      `json:"started_at,omitempty"`
	FinishedAt    *time.Time      `json:"finished_at,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
}

// CreateGenerationReq 触发生成请求
type CreateGenerationReq struct {
	Kind         string `json:"kind" binding:"required"`
	Engine       string `json:"engine"`
	AutoCommit   bool   `json:"auto_commit"`
	Instructions string `json:"instructions"`
}
