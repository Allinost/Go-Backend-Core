package developmentmanager

import (
	"encoding/json"
	"testing"
)

func TestGenerationCollections(t *testing.T) {
	if len(ValidGenerationKinds) != 3 {
		t.Errorf("生成类型应为 3 种，实际 %d", len(ValidGenerationKinds))
	}
	if len(ValidGenerationEngines) != 3 {
		t.Errorf("生成引擎应为 3 种，实际 %d", len(ValidGenerationEngines))
	}
	for _, kind := range []string{GenerationKindPlan, GenerationKindCode, GenerationKindDoc} {
		if !ValidGenerationKinds[kind] {
			t.Errorf("生成类型 %s 应合法", kind)
		}
	}
}

func TestGenerationStatusForCode(t *testing.T) {
	cases := map[int]int{
		ErrGenerationNotFound:       404,
		ErrInvalidGenerationKind:    400,
		ErrInvalidGenerationEngine:  400,
		ErrAutoCommitKind:           400,
		ErrAutoCommitUnsafe:         409,
		ErrGenerationAlreadySuccess: 409,
	}
	for code, want := range cases {
		if got := statusForCode(code); got != want {
			t.Errorf("code %d 期望状态 %d，实际 %d", code, want, got)
		}
	}
}

func TestSnapshotInstructions(t *testing.T) {
	raw, _ := json.Marshal(map[string]any{"instructions": "补充：需要单元测试"})
	if got := snapshotInstructions(raw); got != "补充：需要单元测试" {
		t.Errorf("快照补充要求解析异常：%q", got)
	}
	if got := snapshotInstructions(nil); got != "" {
		t.Errorf("空快照应返回空字符串，实际 %q", got)
	}
	if got := snapshotInstructions(json.RawMessage(`{"kind":"plan"}`)); got != "" {
		t.Errorf("缺失字段应返回空字符串，实际 %q", got)
	}
}
