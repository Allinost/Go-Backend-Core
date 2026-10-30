package developmentmanager

import "testing"

func TestIdentifierRe(t *testing.T) {
	valid := []string{"zzz_development_manager_project", "users", "T_01", "a"}
	for _, name := range valid {
		if !identifierRe.MatchString(name) {
			t.Errorf("%q 应合法", name)
		}
	}
	invalid := []string{"", "has-dash", "has space", "中文表", "a.b", "a;drop"}
	for _, name := range invalid {
		if identifierRe.MatchString(name) {
			t.Errorf("%q 应非法", name)
		}
	}
}

func TestNormalizeColumnReq(t *testing.T) {
	t.Run("缺省可空且 trim", func(t *testing.T) {
		c, err := normalizeColumnReq(&CreateColumnReq{
			ColumnName: " project_id ",
			DataType:   " bigint ",
			Meaning:    " 所属项目 ",
		})
		if err != nil {
			t.Fatalf("期望成功，实际 %v", err)
		}
		if c.ColumnName != "project_id" || c.DataType != "bigint" || c.Meaning != "所属项目" {
			t.Errorf("归一化异常：%+v", c)
		}
		if !c.Nullable {
			t.Error("nullable 缺省应为 true")
		}
	})

	t.Run("显式不可空", func(t *testing.T) {
		no := false
		c, err := normalizeColumnReq(&CreateColumnReq{ColumnName: "id", DataType: "bigint", Nullable: &no})
		if err != nil {
			t.Fatalf("期望成功，实际 %v", err)
		}
		if c.Nullable {
			t.Error("显式 nullable=false 应生效")
		}
	})

	t.Run("字段名非法", func(t *testing.T) {
		for _, name := range []string{"", "a-b", "中文"} {
			if _, err := normalizeColumnReq(&CreateColumnReq{ColumnName: name, DataType: "int"}); err == nil {
				t.Errorf("字段名 %q 应校验失败", name)
			}
		}
	})

	t.Run("类型为空", func(t *testing.T) {
		if _, err := normalizeColumnReq(&CreateColumnReq{ColumnName: "id", DataType: " "}); err == nil {
			t.Error("空类型应校验失败")
		}
	})
}

func TestDictionaryCollections(t *testing.T) {
	if len(ValidDBTypes) != 2 {
		t.Errorf("数据库类型应为 2 种，实际 %d", len(ValidDBTypes))
	}
	if len(ValidTableStatuses) != 3 {
		t.Errorf("表状态应为 3 种，实际 %d", len(ValidTableStatuses))
	}
}

func TestDictionaryStatusForCode(t *testing.T) {
	cases := map[int]int{
		ErrTableNotFound:      404,
		ErrColumnNotFound:     404,
		ErrDuplicateTable:     409,
		ErrDuplicateColumn:    409,
		ErrInvalidDBType:      400,
		ErrInvalidTableName:   400,
		ErrInvalidTableStatus: 400,
		ErrInvalidColumnName:  400,
		ErrScanUnsupported:    400,
	}
	for code, want := range cases {
		if got := statusForCode(code); got != want {
			t.Errorf("code %d 期望状态 %d，实际 %d", code, want, got)
		}
	}
}
