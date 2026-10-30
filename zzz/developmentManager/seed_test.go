package developmentmanager

import "testing"

func TestSeedAssetsAndRequirements(t *testing.T) {
	projectKeys := make(map[string]bool, len(seedProjects))
	for _, p := range seedProjects {
		projectKeys[p.Key] = true
	}

	assetURIs := make(map[string]bool)
	for _, a := range seedAssets {
		if !projectKeys[a.Key] {
			t.Errorf("资产归属项目未登记：%s (%s)", a.Key, a.Title)
		}
		if !ValidAssetCategories[a.Category] {
			t.Errorf("资产类别非法：%s / %s", a.Title, a.Category)
		}
		if a.URI == "" || a.Title == "" {
			t.Errorf("资产标题/URI 不能为空：%+v", a)
		}
		key := a.Key + "|" + a.URI
		if assetURIs[key] {
			t.Errorf("资产重复登记：%s / %s", a.Key, a.URI)
		}
		assetURIs[key] = true
	}

	requirementTitles := make(map[string]bool)
	for _, r := range seedRequirements {
		if !projectKeys[r.Key] {
			t.Errorf("需求归属项目未登记：%s (%s)", r.Key, r.Title)
		}
		if !ValidRequirementTypes[r.Type] {
			t.Errorf("需求类型非法：%s / %s", r.Title, r.Type)
		}
		if !ValidRequirementPriorities[r.Priority] {
			t.Errorf("需求优先级非法：%s / %s", r.Title, r.Priority)
		}
		if r.Status != "" && !ValidRequirementStatuses[r.Status] {
			t.Errorf("需求状态非法：%s / %s", r.Title, r.Status)
		}
		if r.Title == "" || r.Content == "" {
			t.Errorf("需求标题/内容不能为空：%+v", r)
		}
		key := r.Key + "|" + r.Title
		if requirementTitles[key] {
			t.Errorf("同项目需求标题重复：%s / %s", r.Key, r.Title)
		}
		requirementTitles[key] = true
	}

	if len(seedAssets) < 20 {
		t.Errorf("历史资产索引过少：%d", len(seedAssets))
	}
	if len(seedRequirements) < 15 {
		t.Errorf("历史需求过少：%d", len(seedRequirements))
	}
}
