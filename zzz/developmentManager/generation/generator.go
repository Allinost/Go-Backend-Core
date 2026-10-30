// Package generation 生成引擎：模板生成为主线，AI 生成为可选项（不可用时由调用方降级）。
// 本包只负责内容生成与产物落盘，任务状态与权限校验由 developmentManager 主包负责。
package generation

import (
	"context"
	"fmt"
	"strings"
)

// 生成类型
const (
	KindPlan = "plan" // 方案
	KindCode = "code" // 代码
	KindDoc  = "doc"  // 文档
)

// 引擎
const (
	EngineAI       = "ai"
	EngineTemplate = "template"
	EngineManual   = "manual"
)

// RequirementInfo 需求快照
type RequirementInfo struct {
	Code     string
	Title    string
	Content  string
	Type     string
	Priority string
	Source   string
	Owner    string
}

// ProjectInfo 项目快照
type ProjectInfo struct {
	Key      string
	Name     string
	Type     string
	RepoPath string
}

// Input 生成输入
type Input struct {
	GenerationID int64
	Kind         string
	Engine       string
	Instructions string
	Requirement  RequirementInfo
	Project      ProjectInfo
}

// File 生成的文件（RelPath 为相对仓库根的 slash 路径）
type File struct {
	RelPath string
	Content string
}

// Output 生成结果
type Output struct {
	Files      []File
	EngineUsed string
	OutputRef  string
}

// AIWriter AI 文本生成接口（由调用方注入具体 Provider，未注入时视为不可用）
type AIWriter interface {
	Write(ctx context.Context, system, prompt string) (string, error)
}

// BaseDir 项目产物根目录
func BaseDir(projectKey string) string {
	return "docs/projects/" + projectKey
}

// ARTIFACT 生成产物目录名（按类型）
func artifactDir(kind string) string {
	switch kind {
	case KindPlan:
		return "plans"
	case KindCode:
		return "code-drafts"
	default:
		return "docs"
	}
}

// GenerateTemplate 使用模板生成产物
func GenerateTemplate(in Input) Output {
	base := BaseDir(in.Project.Key)
	dir := base + "/" + artifactDir(in.Kind)
	switch in.Kind {
	case KindCode:
		dir = fmt.Sprintf("%s/%s-%d", dir, in.Requirement.Code, in.GenerationID)
		return Output{
			Files:      codeFiles(in, dir),
			EngineUsed: EngineTemplate,
			OutputRef:  dir + "/",
		}
	case KindDoc:
		path := fmt.Sprintf("%s/%s-doc-%d.md", dir, in.Requirement.Code, in.GenerationID)
		return Output{
			Files:      []File{{RelPath: path, Content: renderDoc(in)}},
			EngineUsed: EngineTemplate,
			OutputRef:  path,
		}
	default: // plan
		path := fmt.Sprintf("%s/%s-plan-%d.md", dir, in.Requirement.Code, in.GenerationID)
		return Output{
			Files:      []File{{RelPath: path, Content: renderPlan(in)}},
			EngineUsed: EngineTemplate,
			OutputRef:  path,
		}
	}
}

// BuildAIPrompt 构造 AI 生成提示词
func BuildAIPrompt(in Input) (system, prompt string) {
	system = "你是资深软件工程师，输出中文 Markdown，结构清晰、可直接落地，不要输出与任务无关的寒暄。"
	switch in.Kind {
	case KindCode:
		system += " 代码类任务请输出 Go 代码骨架（含 package、类型、方法签名与 TODO 注释），并用 Markdown 代码块包裹。"
	case KindDoc:
		system += " 文档类任务请输出面向使用者的说明文档。"
	default:
		system += " 方案类任务请输出需求方案，包含背景、范围、设计、接口、数据、风险、验收标准与任务拆解。"
	}
	prompt = fmt.Sprintf(
		"项目：%s（%s，仓库路径 %s）\n需求编号：%s\n需求标题：%s\n类型：%s\n优先级：%s\n负责人：%s\n\n需求正文：\n%s\n",
		in.Project.Name, in.Project.Type, in.Project.RepoPath,
		in.Requirement.Code, in.Requirement.Title, in.Requirement.Type,
		in.Requirement.Priority, in.Requirement.Owner, emptyAsPlaceholder(in.Requirement.Content))
	if strings.TrimSpace(in.Instructions) != "" {
		prompt += "\n补充要求：\n" + in.Instructions + "\n"
	}
	return system, prompt
}

// WrapAIContent 把 AI 返回内容包装成对应类型的产物
func WrapAIContent(in Input, content string) Output {
	content = strings.TrimSpace(content)
	base := BaseDir(in.Project.Key)
	dir := base + "/" + artifactDir(in.Kind)

	if in.Kind == KindCode {
		dir = fmt.Sprintf("%s/%s-%d", dir, in.Requirement.Code, in.GenerationID)
		files := codeFiles(in, dir)
		readme := fmt.Sprintf("# %s %s（AI 生成草案）\n\n> 引擎：ai ｜ 需求：%s ｜ 请人工审查后再合入。\n\n%s\n",
			in.Requirement.Code, in.Requirement.Title, in.Requirement.Code, content)
		files[0].Content = readme
		return Output{Files: files, EngineUsed: EngineAI, OutputRef: dir + "/"}
	}

	path := fmt.Sprintf("%s/%s-%s-%d.md", dir, in.Requirement.Code, in.Kind, in.GenerationID)
	header := fmt.Sprintf("> 引擎：ai ｜ 需求：%s ｜ 生成内容需人工复核\n\n", in.Requirement.Code)
	return Output{
		Files:      []File{{RelPath: path, Content: header + content + "\n"}},
		EngineUsed: EngineAI,
		OutputRef:  path,
	}
}

func emptyAsPlaceholder(s string) string {
	if strings.TrimSpace(s) == "" {
		return "（需求正文为空，待补充）"
	}
	return s
}

func renderPlan(in Input) string {
	return fmt.Sprintf(`# %s 需求方案：%s

> 项目：%s ｜ 类型：%s ｜ 优先级：%s ｜ 负责人：%s ｜ 引擎：template

## 1. 背景与目标

%s

## 2. 范围

- 本期范围：按上述需求完成核心功能与验收项
- 不在本期范围：需求未明确提及的扩展能力

## 3. 方案设计

- 分层：沿用项目既有分层（handler → service → repository）
- 关键流程：请求校验 → 业务编排 → 数据持久化 → 审计记录
- 失败处理：参数错误返回 4xx，内部错误记录日志并返回统一错误码

## 4. 接口草案

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | /api/v1/<module>/<resource> | 查询列表（待细化） |
| POST | /api/v1/<module>/<resource> | 新建（待细化） |
| PUT | /api/v1/<module>/<resource>/:id | 更新（待细化） |
| DELETE | /api/v1/<module>/<resource>/:id | 删除（待细化） |

## 5. 数据与兼容

- 表结构变更：按需新增/扩展，遵循项目表前缀规范
- 兼容性：新增字段可空或带默认值，避免破坏既有数据

## 6. 风险与应对

| 风险 | 应对 |
|---|---|
| 需求理解偏差 | 开发前与需求方核对验收标准 |
| 数据迁移风险 | 先备份、灰度验证、保留回滚脚本 |

## 7. 验收标准

- [ ] 核心流程可走通
- [ ] 参数校验与错误码符合约定
- [ ] 单元测试覆盖关键分支
- [ ] 文档与代码同提交

## 8. 任务拆解

1. 数据模型与迁移
2. 后端接口与业务逻辑
3. 单元测试
4. 前端联调
5. 文档与发布说明
`, in.Requirement.Code, in.Requirement.Title, in.Project.Name, in.Requirement.Type,
		in.Requirement.Priority, in.Requirement.Owner, emptyAsPlaceholder(in.Requirement.Content))
}

func renderDoc(in Input) string {
	return fmt.Sprintf(`# %s 使用说明：%s

> 项目：%s ｜ 类型：%s ｜ 优先级：%s ｜ 引擎：template

## 1. 概述

%s

## 2. 功能说明

- 功能入口：待补充
- 适用角色：待补充
- 前置条件：待补充

## 3. 使用流程

1. 进入功能页面/调用接口
2. 填写必要参数
3. 提交并查看结果

## 4. 接口示例

`+"```bash"+`
curl -X POST 'http://localhost:29092/api/v1/<module>/<resource>' \
  -H 'Authorization: Bearer <token>' \
  -H 'Content-Type: application/json' \
  -d '{}'
`+"```"+`

## 5. 注意事项

- 权限不足时返回 403，请确认角色已分配对应权限
- 参数错误返回 400，请按错误信息修正

## 6. 常见问题

| 问题 | 处理方式 |
|---|---|
| 接口 401 | 登录态失效，请重新登录 |
| 接口 403 | 联系管理员分配权限 |
| 数据为空 | 确认筛选条件与数据是否已创建 |
`, in.Requirement.Code, in.Requirement.Title, in.Project.Name, in.Requirement.Type,
		in.Requirement.Priority, emptyAsPlaceholder(in.Requirement.Content))
}

func codeFiles(in Input, dir string) []File {
	pkg := packageName(in.Project.Key)
	ident := exportedIdent(in.Requirement.Code)
	readme := fmt.Sprintf(`# %s %s（代码草案）

> 项目：%s ｜ 类型：%s ｜ 优先级：%s ｜ 引擎：template

## 说明

本目录由 developmentManager 生成，仅作为**代码草案**，不直接进入主干。

## 文件

- `+"`handler_skeleton.go.txt`"+`：HTTP 处理器骨架
- `+"`service_skeleton.go.txt`"+`：业务服务骨架

## 合入步骤

1. 复制骨架到目标模块（如 `+"`zzz/<module>`"+`），按项目分层补齐实现
2. 补充参数校验、错误码、权限与审计
3. 编写单元测试并运行 `+"`go test ./...`"+`
4. 提交到特性分支，经评审后合入

## 需求原文

%s
`, in.Requirement.Code, in.Requirement.Title, in.Project.Name, in.Requirement.Type,
		in.Requirement.Priority, emptyAsPlaceholder(in.Requirement.Content))

	handler := fmt.Sprintf(`package %[1]s

import "github.com/gin-gonic/gin"

// %[2]sHandler 需求 %[3]s 的 HTTP 处理器骨架（自动生成，待实现）
type %[2]sHandler struct {
	svc *%[2]sService
}

// New%[2]sHandler 创建处理器
func New%[2]sHandler(svc *%[2]sService) *%[2]sHandler {
	return &%[2]sHandler{svc: svc}
}

// Handle 处理需求 %[3]s 对应的接口（待补充路由与参数绑定）
func (h *%[2]sHandler) Handle(c *gin.Context) {
	// TODO: 绑定参数、调用 service、统一响应
	c.JSON(200, gin.H{"code": 0, "message": "ok"})
}
`, pkg, ident, in.Requirement.Title)

	service := fmt.Sprintf(`package %[1]s

import "context"

// %[2]sService 需求 %[3]s 的业务服务骨架（自动生成，待实现）
type %[2]sService struct {
	// TODO: 注入 repository / 依赖
}

// New%[2]sService 创建服务
func New%[2]sService() *%[2]sService {
	return &%[2]sService{}
}

// Execute 实现需求 %[3]s 的核心业务（待实现）
func (s *%[2]sService) Execute(ctx context.Context, input string) (string, error) {
	// TODO: 参数校验 -> 业务编排 -> 持久化 -> 审计
	return "", nil
}
`, pkg, ident, in.Requirement.Title)

	return []File{
		{RelPath: dir + "/README.md", Content: readme},
		{RelPath: dir + "/handler_skeleton.go.txt", Content: handler},
		{RelPath: dir + "/service_skeleton.go.txt", Content: service},
	}
}

// packageName 由项目 key 推导合法 Go 包名
func packageName(projectKey string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(projectKey) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if name == "" {
		return "generated"
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "pkg" + name
	}
	return name
}

// exportedIdent 由需求编号推导导出标识符
func exportedIdent(code string) string {
	var b strings.Builder
	for _, r := range code {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	name := b.String()
	if name == "" {
		return "Generated"
	}
	if name[0] >= '0' && name[0] <= '9' {
		return "Gen" + name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}
