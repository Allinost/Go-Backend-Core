package developmentmanager

import (
	"testing"
	"time"
)

func validReq() *CreateProjectReq {
	return &CreateProjectReq{
		ProjectKey: "demo-project",
		Name:       "演示项目",
		Type:       ProjectTypeBase,
	}
}

func TestValidateProjectReq(t *testing.T) {
	t.Run("合法请求", func(t *testing.T) {
		p, err := validateProjectReq(validReq(), "tester", time.Now())
		if err != nil {
			t.Fatalf("期望成功，实际 %v", err)
		}
		if p.Status != ProjectStatusActive {
			t.Errorf("默认状态应为 active，实际 %s", p.Status)
		}
		if p.DefaultBranch != "main" {
			t.Errorf("默认分支应为 main，实际 %s", p.DefaultBranch)
		}
		if p.CreatedBy != "tester" {
			t.Errorf("创建人应为 tester，实际 %s", p.CreatedBy)
		}
	})

	t.Run("唯一键格式非法", func(t *testing.T) {
		for _, key := range []string{"", "UPPER", "含中文", "-lead", "a", "a b"} {
			req := validReq()
			req.ProjectKey = key
			if _, err := validateProjectReq(req, "", time.Now()); err == nil {
				t.Errorf("键 %q 应校验失败", key)
			}
		}
	})

	t.Run("名称过短", func(t *testing.T) {
		req := validReq()
		req.Name = "短"
		if _, err := validateProjectReq(req, "", time.Now()); err == nil {
			t.Error("单字符名称应校验失败")
		}
	})

	t.Run("类型非法", func(t *testing.T) {
		req := validReq()
		req.Type = "mobile"
		_, err := validateProjectReq(req, "", time.Now())
		if err == nil {
			t.Fatal("非法类型应校验失败")
		}
		dmErr, ok := err.(*DevelopmentManagerError)
		if !ok || dmErr.Code != ErrInvalidProjectType {
			t.Errorf("期望 ErrInvalidProjectType，实际 %v", err)
		}
	})

	t.Run("状态缺省与非法", func(t *testing.T) {
		req := validReq()
		req.Status = ""
		p, err := validateProjectReq(req, "", time.Now())
		if err != nil || p.Status != ProjectStatusActive {
			t.Errorf("空状态应默认 active，实际 %s / %v", p.Status, err)
		}
		req.Status = "unknown"
		if _, err := validateProjectReq(req, "", time.Now()); err == nil {
			t.Error("非法状态应校验失败")
		}
	})
}

func TestStatusForCode(t *testing.T) {
	cases := map[int]int{
		ErrProjectNotFound:             404,
		ErrTargetProjectMissing:        404,
		ErrDuplicateProjectKey:         409,
		ErrProjectHasDependents:        409,
		ErrInvalidProjectKey:           400,
		ErrInvalidRelationType:         400,
		ErrProjectHasDependents + 9999: 500, // 未知码兜底
	}
	for code, want := range cases {
		if got := statusForCode(code); got != want {
			t.Errorf("code %d 期望状态 %d，实际 %d", code, want, got)
		}
	}
}

func TestValidCollections(t *testing.T) {
	if len(ValidProjectTypes) != 6 {
		t.Errorf("项目类型应为 6 种（含 frontend_base / container_project），实际 %d", len(ValidProjectTypes))
	}
	if len(ValidRelationTypes) != 4 {
		t.Errorf("关联类型应为 4 种，实际 %d", len(ValidRelationTypes))
	}
	if len(ValidAssetCategories) != 4 {
		t.Errorf("资产类别应为 4 种，实际 %d", len(ValidAssetCategories))
	}
}
