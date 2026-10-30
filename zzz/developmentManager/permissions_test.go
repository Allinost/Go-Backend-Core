package developmentmanager

import (
	"strings"
	"testing"
)

func TestPermissionSpecsNaming(t *testing.T) {
	seen := make(map[string]bool, len(PermissionSpecs))
	for _, spec := range PermissionSpecs {
		if spec.Name != spec.Resource+"."+spec.Action {
			t.Errorf("权限名称应为 resource.action，实际 %s", spec.Name)
		}
		if !strings.HasPrefix(spec.Resource, "dm.") {
			t.Errorf("资源应以 dm. 前缀，实际 %s", spec.Resource)
		}
		if spec.Desc == "" {
			t.Errorf("权限 %s 缺少中文说明", spec.Name)
		}
		if seen[spec.Name] {
			t.Errorf("权限名称重复：%s", spec.Name)
		}
		seen[spec.Name] = true
	}
	if len(PermissionSpecs) != 28 {
		t.Errorf("权限点应为 28 个（规划 §4.1 矩阵），实际 %d", len(PermissionSpecs))
	}
}

func TestSeedRelationsReferenceExistingProjects(t *testing.T) {
	keys := make(map[string]bool, len(seedProjects))
	for _, p := range seedProjects {
		keys[p.Key] = true
	}
	if !keys["go-backend-core"] || !keys["nkui"] || !keys["developmentmanager"] {
		t.Fatal("seed 项目缺少核心条目")
	}
	for _, rel := range seedRelations {
		if !keys[rel.From] {
			t.Errorf("关联发起方未登记：%s", rel.From)
		}
		if !keys[rel.To] {
			t.Errorf("关联接收方未登记：%s", rel.To)
		}
		if !ValidRelationTypes[rel.Type] {
			t.Errorf("关联类型非法：%s", rel.Type)
		}
	}
	for _, a := range seedAssets {
		if !keys[a.Key] {
			t.Errorf("资产归属项目未登记：%s", a.Key)
		}
		if !ValidAssetCategories[a.Category] {
			t.Errorf("资产类别非法：%s", a.Category)
		}
	}
}
