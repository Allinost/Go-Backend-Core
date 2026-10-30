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
