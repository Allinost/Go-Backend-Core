package developmentmanager

import "testing"

func TestValidateRequirementReq(t *testing.T) {
	t.Run("缺省值", func(t *testing.T) {
		r, err := validateRequirementReq(&CreateRequirementReq{Title: " 新增需求 "})
		if err != nil {
			t.Fatalf("期望成功，实际 %v", err)
		}
		if r.Title != "新增需求" {
			t.Errorf("标题应 trim，实际 %q", r.Title)
		}
		if r.Type != RequirementTypeFeature || r.Priority != PriorityP2 || r.Source != RequirementSourceManual {
			t.Errorf("缺省值异常：type=%s priority=%s source=%s", r.Type, r.Priority, r.Source)
		}
	})

	t.Run("标题过短", func(t *testing.T) {
		if _, err := validateRequirementReq(&CreateRequirementReq{Title: "短"}); err == nil {
			t.Error("单字标题应校验失败")
		}
	})

	t.Run("枚举非法", func(t *testing.T) {
		cases := []*CreateRequirementReq{
			{Title: "合法标题", Type: "story"},
			{Title: "合法标题", Priority: "P9"},
			{Title: "合法标题", Source: "unknown"},
		}
		for i, req := range cases {
			if _, err := validateRequirementReq(req); err == nil {
				t.Errorf("用例 %d 应校验失败", i)
			}
		}
	})
}

func TestCanTransition(t *testing.T) {
	allowed := [][2]string{
		{RequirementStatusDraft, RequirementStatusReviewing},
		{RequirementStatusReviewing, RequirementStatusApproved},
		{RequirementStatusReviewing, RequirementStatusDraft},
		{RequirementStatusApproved, RequirementStatusPlanned},
		{RequirementStatusPlanned, RequirementStatusDeveloping},
		{RequirementStatusDeveloping, RequirementStatusDone},
		{RequirementStatusDone, RequirementStatusDeveloping},
		{RequirementStatusDraft, RequirementStatusClosed},
	}
	for _, pair := range allowed {
		if !CanTransition(pair[0], pair[1]) {
			t.Errorf("%s -> %s 应允许", pair[0], pair[1])
		}
	}

	denied := [][2]string{
		{RequirementStatusDraft, RequirementStatusDone},
		{RequirementStatusDraft, RequirementStatusApproved},
		{RequirementStatusClosed, RequirementStatusDraft},
		{RequirementStatusApproved, RequirementStatusDeveloping},
		{RequirementStatusDraft, RequirementStatusDraft},
		{"unknown", RequirementStatusDraft},
	}
	for _, pair := range denied {
		if CanTransition(pair[0], pair[1]) {
			t.Errorf("%s -> %s 应拒绝", pair[0], pair[1])
		}
	}
}

func TestFormatRequirementCode(t *testing.T) {
	cases := map[int]string{
		1:     "RQM-0001",
		42:    "RQM-0042",
		9999:  "RQM-9999",
		12345: "RQM-12345",
	}
	for n, want := range cases {
		if got := formatRequirementCode(n); got != want {
			t.Errorf("formatRequirementCode(%d)=%s，期望 %s", n, got, want)
		}
	}
}

func TestRequirementStatusForCode(t *testing.T) {
	cases := map[int]int{
		ErrRequirementNotFound:          404,
		ErrRequirementVersionNotFound:   404,
		ErrInvalidRequirementTransition: 409,
		ErrRequirementDeleted:           409,
		ErrInvalidRequirementType:       400,
		ErrInvalidRequirementTitle:      400,
	}
	for code, want := range cases {
		if got := statusForCode(code); got != want {
			t.Errorf("code %d 期望状态 %d，实际 %d", code, want, got)
		}
	}
}
