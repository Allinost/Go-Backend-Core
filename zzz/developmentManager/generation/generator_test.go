package generation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func sampleInput(kind string) Input {
	return Input{
		GenerationID: 7,
		Kind:         kind,
		Engine:       EngineTemplate,
		Requirement: RequirementInfo{
			Code:     "RQM-0001",
			Title:    "项目台账支持归档",
			Content:  "需要支持项目归档与恢复。",
			Type:     "feature",
			Priority: "P1",
			Owner:    "tom",
		},
		Project: ProjectInfo{Key: "go-backend-core", Name: "Go-Backend-Core", Type: "base", RepoPath: "."},
	}
}

func TestGenerateTemplatePlan(t *testing.T) {
	out := GenerateTemplate(sampleInput(KindPlan))
	want := "docs/projects/go-backend-core/plans/RQM-0001-plan-7.md"
	if len(out.Files) != 1 || out.Files[0].RelPath != want {
		t.Fatalf("方案产物路径异常：%+v", out.Files)
	}
	content := out.Files[0].Content
	for _, wantPart := range []string{"RQM-0001", "项目台账支持归档", "## 1. 背景与目标", "## 7. 验收标准", "## 8. 任务拆解"} {
		if !strings.Contains(content, wantPart) {
			t.Errorf("方案内容缺少 %q", wantPart)
		}
	}
	if out.EngineUsed != EngineTemplate || out.OutputRef != want {
		t.Errorf("Output 元信息异常：engine=%s ref=%s", out.EngineUsed, out.OutputRef)
	}
}

func TestGenerateTemplateDoc(t *testing.T) {
	out := GenerateTemplate(sampleInput(KindDoc))
	want := "docs/projects/go-backend-core/docs/RQM-0001-doc-7.md"
	if len(out.Files) != 1 || out.Files[0].RelPath != want {
		t.Fatalf("文档产物路径异常：%+v", out.Files)
	}
	if !strings.Contains(out.Files[0].Content, "## 4. 接口示例") {
		t.Error("文档内容缺少接口示例")
	}
}

func TestGenerateTemplateCode(t *testing.T) {
	out := GenerateTemplate(sampleInput(KindCode))
	if len(out.Files) != 3 {
		t.Fatalf("代码草案应为 3 个文件，实际 %d", len(out.Files))
	}
	dir := "docs/projects/go-backend-core/code-drafts/RQM-0001-7"
	if out.OutputRef != dir+"/" {
		t.Errorf("代码产物目录异常：%s", out.OutputRef)
	}
	for _, f := range out.Files {
		if !strings.HasPrefix(f.RelPath, dir+"/") {
			t.Errorf("文件 %s 不在产物目录下", f.RelPath)
		}
		if f.Content == "" {
			t.Errorf("文件 %s 内容为空", f.RelPath)
		}
	}
}

func TestGenerateTemplateFrontendCode(t *testing.T) {
	in := sampleInput(KindCode)
	in.Project = ProjectInfo{Key: "pure-frontend", Name: "Pure Frontend", Type: ProjectTypeFrontendModule, RepoPath: "../pure-frontend"}

	out := GenerateTemplate(in)
	if len(out.Files) != 4 {
		t.Fatalf("前端代码草案应为 4 个文件，实际 %d", len(out.Files))
	}
	dir := "docs/projects/pure-frontend/code-drafts/RQM-0001-7"
	if out.OutputRef != dir+"/" {
		t.Errorf("前端代码产物目录异常：%s", out.OutputRef)
	}
	var page string
	for _, f := range out.Files {
		if !strings.HasPrefix(f.RelPath, dir+"/") {
			t.Errorf("文件 %s 不在产物目录下", f.RelPath)
		}
		if strings.HasSuffix(f.RelPath, ".go") || strings.HasSuffix(f.RelPath, ".go.txt") {
			t.Errorf("前端项目不应生成 Go 文件：%s", f.RelPath)
		}
		if strings.HasSuffix(f.RelPath, "PageView.vue") {
			page = f.Content
		}
	}
	if !strings.Contains(page, `<script setup lang="ts">`) {
		t.Error("前端页面应使用 Vue 3 script setup")
	}
	if strings.Contains(page, "gin.") || strings.Contains(page, "go-backend-core") {
		t.Error("纯前端页面不应依赖后端模块")
	}
}

func TestFrontendPlanDoesNotRequireBackend(t *testing.T) {
	in := sampleInput(KindPlan)
	in.Project = ProjectInfo{Key: "pure-frontend", Name: "Pure Frontend", Type: ProjectTypeFrontendBase, RepoPath: "../pure-frontend"}

	out := GenerateTemplate(in)
	content := out.Files[0].Content
	for _, part := range []string{"纯前端", "本地 mock", "不要求绑定后端模块"} {
		if !strings.Contains(content, part) {
			t.Errorf("前端方案缺少 %q", part)
		}
	}
	if strings.Contains(content, "handler → service → repository") {
		t.Error("前端方案不应包含后端分层模板")
	}
}

func TestBuildAIPromptFrontendCode(t *testing.T) {
	in := sampleInput(KindCode)
	in.Project = ProjectInfo{Key: "pure-frontend", Name: "Pure Frontend", Type: ProjectTypeFrontendModule, RepoPath: "../pure-frontend"}
	system, _ := BuildAIPrompt(in)
	if !strings.Contains(system, "Vue 3") || !strings.Contains(system, "TypeScript") {
		t.Errorf("前端 AI prompt 应要求 Vue/TypeScript：%s", system)
	}
	if strings.Contains(system, "Go 代码") {
		t.Errorf("前端 AI prompt 不应要求 Go 代码：%s", system)
	}
}

func TestBuildAIPrompt(t *testing.T) {
	system, prompt := BuildAIPrompt(sampleInput(KindPlan))
	if !strings.Contains(system, "Markdown") {
		t.Error("system prompt 应要求 Markdown")
	}
	for _, part := range []string{"RQM-0001", "项目台账支持归档", "需要支持项目归档与恢复"} {
		if !strings.Contains(prompt, part) {
			t.Errorf("prompt 缺少 %q", part)
		}
	}
}

func TestWrapAIContent(t *testing.T) {
	out := WrapAIContent(sampleInput(KindPlan), "# AI 方案\n内容")
	if out.EngineUsed != EngineAI {
		t.Errorf("AI 产物引擎应为 ai，实际 %s", out.EngineUsed)
	}
	if !strings.Contains(out.Files[0].Content, "需人工复核") {
		t.Error("AI 产物应带人工复核提示")
	}
}

func TestWriteFiles(t *testing.T) {
	root := t.TempDir()
	written, err := WriteFiles(root, []File{
		{RelPath: "docs/projects/demo/plans/a.md", Content: "hello"},
		{RelPath: "docs/projects/demo/docs/b.md", Content: "world"},
	})
	if err != nil {
		t.Fatalf("写入失败：%v", err)
	}
	if len(written) != 2 {
		t.Fatalf("应写入 2 个文件，实际 %d", len(written))
	}
	data, err := os.ReadFile(filepath.Join(root, "docs", "projects", "demo", "plans", "a.md"))
	if err != nil || string(data) != "hello" {
		t.Errorf("文件内容异常：%q / %v", data, err)
	}
}

func TestWriteFilesRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	cases := []string{"../evil.md", "docs/../../evil.md", "/abs/evil.md"}
	for _, rel := range cases {
		if _, err := WriteFiles(root, []File{{RelPath: rel, Content: "x"}}); err == nil {
			t.Errorf("路径 %q 应被拒绝", rel)
		}
	}
}

func TestPackageAndIdent(t *testing.T) {
	if got := packageName("go-backend-core"); got != "gobackendcore" {
		t.Errorf("packageName 异常：%s", got)
	}
	if got := packageName("123-app"); got != "pkg123app" {
		t.Errorf("数字开头包名应加前缀：%s", got)
	}
	if got := exportedIdent("RQM-0001"); got != "RQM0001" {
		t.Errorf("exportedIdent 异常：%s", got)
	}
}
