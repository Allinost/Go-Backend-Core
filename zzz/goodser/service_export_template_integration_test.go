package goodser

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExportTemplateCRUD(t *testing.T) {
	ctx := context.Background()
	repo := newPGRepository(t)
	svc := NewService(repo, nil)

	// 名称为空
	_, err := svc.CreateExportTemplate(ctx, &ExportTemplateRequest{Name: "  ", Config: json.RawMessage(`{}`)})
	require.Error(t, err)

	// config 必须是 JSON 对象
	_, err = svc.CreateExportTemplate(ctx, &ExportTemplateRequest{Name: "bad", Config: json.RawMessage(`[1,2]`)})
	require.Error(t, err)
	_, err = svc.CreateExportTemplate(ctx, &ExportTemplateRequest{Name: "bad", Config: json.RawMessage(`not-json`)})
	require.Error(t, err)

	first, err := svc.CreateExportTemplate(ctx, &ExportTemplateRequest{
		InventoryID: "inv-1",
		Name:        "默认样式",
		Config:      json.RawMessage(`{"columns":3,"background":"black"}`),
		IsDefault:   true,
	})
	require.NoError(t, err)
	require.True(t, first.IsDefault)
	require.NotNil(t, first.InventoryID)

	// 同目录新默认应清除旧默认
	second, err := svc.CreateExportTemplate(ctx, &ExportTemplateRequest{
		InventoryID: "inv-1",
		Name:        "第二套",
		Config:      json.RawMessage(`{"columns":5}`),
		IsDefault:   true,
	})
	require.NoError(t, err)
	reloaded, err := repo.GetExportTemplate(ctx, first.ID)
	require.NoError(t, err)
	require.False(t, reloaded.IsDefault)

	list, err := svc.ListExportTemplates(ctx, "inv-1")
	require.NoError(t, err)
	require.Len(t, list, 2)
	require.True(t, list[0].IsDefault) // 默认模板排在最前

	// 更新
	updated, err := svc.UpdateExportTemplate(ctx, second.ID, &ExportTemplateRequest{
		InventoryID: "inv-1",
		Name:        "第二套-改",
		Config:      json.RawMessage(`{"columns":4}`),
		IsDefault:   true,
	})
	require.NoError(t, err)
	require.Equal(t, "第二套-改", updated.Name)
	require.JSONEq(t, `{"columns":4}`, string(updated.Config))

	// 全局模板（inventory_id 为空）
	global, err := svc.CreateExportTemplate(ctx, &ExportTemplateRequest{
		Name:   "全局模板",
		Config: json.RawMessage(`{"format":"jpeg"}`),
	})
	require.NoError(t, err)
	require.Nil(t, global.InventoryID)

	list, err = svc.ListExportTemplates(ctx, "inv-1")
	require.NoError(t, err)
	require.Len(t, list, 3) // inv-1 两张 + 全局一张

	// 删除
	require.NoError(t, svc.DeleteExportTemplate(ctx, second.ID))
	_, err = repo.GetExportTemplate(ctx, second.ID)
	require.Error(t, err)
	require.Error(t, svc.DeleteExportTemplate(ctx, second.ID))
}
