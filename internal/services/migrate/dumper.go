package migrate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

func Dump(ctx context.Context, adapter DBAdapter, opts DumpOptions) (*DumpResult, error) {
	tables := opts.Tables
	if len(tables) == 0 {
		var err error
		tables, err = adapter.GetTables(ctx)
		if err != nil {
			return nil, err
		}
	}

	if opts.Format == "" {
		opts.Format = FormatJSON
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000
	}

	var totalRows int64
	dumped := make(map[string]bool)

	switch opts.Format {
	case FormatJSON:
		return dumpJSON(ctx, adapter, tables, opts, &totalRows, dumped)
	case FormatCSV:
		return dumpCSV(ctx, adapter, tables, opts, &totalRows, dumped)
	case FormatSQL:
		return dumpSQL(ctx, adapter, tables, opts, &totalRows, dumped)
	default:
		return nil, fmt.Errorf("migrate: 不支持的导出格式: %s", opts.Format)
	}
}

func dumpJSON(ctx context.Context, adapter DBAdapter, tables []string, opts DumpOptions, totalRows *int64, dumped map[string]bool) (*DumpResult, error) {
	result := make(map[string][]map[string]any)
	for _, table := range tables {
		if dumped[table] {
			continue
		}
		dumped[table] = true
		if opts.SchemaOnly || opts.NoData {
			schema, err := adapter.GetTableSchema(ctx, table)
			if err != nil {
				return nil, fmt.Errorf("migrate: 获取表 %s 结构失败: %w", table, err)
			}
			result[table] = []map[string]any{{"_schema": schema}}
			continue
		}

		where := fmtMapKey(opts.Where, table)
		var tableRows []map[string]any
		if err := adapter.SelectAll(ctx, table, nil, where, opts.BatchSize, func(batch []map[string]any) error {
			tableRows = append(tableRows, batch...)
			return nil
		}); err != nil {
			return nil, fmt.Errorf("migrate: 导出表 %s 失败: %w", table, err)
		}
		result[table] = tableRows
		*totalRows += int64(len(tableRows))
	}

	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("migrate: JSON 序列化失败: %w", err)
	}

	var tableList []string
	for _, t := range tables {
		if !containsString(tableList, t) {
			tableList = append(tableList, t)
		}
	}
	return &DumpResult{
		Data:       data,
		Format:     FormatJSON,
		TableCount: len(tableList),
		RowCount:   *totalRows,
		Tables:     tableList,
	}, nil
}

func dumpCSV(ctx context.Context, adapter DBAdapter, tables []string, opts DumpOptions, totalRows *int64, dumped map[string]bool) (*DumpResult, error) {
	var buf bytes.Buffer
	for _, table := range tables {
		if dumped[table] {
			continue
		}
		dumped[table] = true

		cols, err := adapter.GetColumns(ctx, table)
		if err != nil {
			return nil, err
		}
		colNames := make([]string, len(cols))
		for i, c := range cols {
			colNames[i] = c.Name
		}

		buf.WriteString(fmt.Sprintf("-- TABLE: %s\n", table))
		buf.WriteString(strings.Join(colNames, ",") + "\n")

		if opts.SchemaOnly || opts.NoData {
			continue
		}

		where := fmtMapKey(opts.Where, table)
		var count int64
		if err := adapter.SelectAll(ctx, table, nil, where, opts.BatchSize, func(batch []map[string]any) error {
			for _, row := range batch {
				var vals []string
				for _, col := range colNames {
					vals = append(vals, formatCSVValue(row[col]))
				}
				buf.WriteString(strings.Join(vals, ",") + "\n")
			}
			count += int64(len(batch))
			return nil
		}); err != nil {
			return nil, fmt.Errorf("migrate: 导出表 %s 失败: %w", table, err)
		}
		*totalRows += count
	}

	var tableList []string
	for _, t := range tables {
		if !containsString(tableList, t) {
			tableList = append(tableList, t)
		}
	}
	return &DumpResult{
		Data:       buf.Bytes(),
		Format:     FormatCSV,
		TableCount: len(tableList),
		RowCount:   *totalRows,
		Tables:     tableList,
	}, nil
}

func dumpSQL(ctx context.Context, adapter DBAdapter, tables []string, opts DumpOptions, totalRows *int64, dumped map[string]bool) (*DumpResult, error) {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("-- Dump created at %s\n", "now"))
	buf.WriteString(fmt.Sprintf("-- Database: %s\n\n", adapter.Type()))

	for _, table := range tables {
		if dumped[table] {
			continue
		}
		dumped[table] = true

		schema, err := adapter.GetTableSchema(ctx, table)
		if err != nil {
			return nil, err
		}

		var defs []string
		for _, col := range schema.Columns {
			def := adapter.Quote(col.Name) + " " + col.Type
			if !col.Nullable {
				def += " NOT NULL"
			}
			defs = append(defs, def)
		}

		var engine string
		if adapter.Type() == DBTypeMySQL {
			engine = " ENGINE=InnoDB DEFAULT CHARSET=utf8mb4"
		}
		buf.WriteString(fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (\n  %s\n)%s;\n\n", adapter.Quote(table), strings.Join(defs, ",\n  "), engine))

		if opts.SchemaOnly || opts.NoData {
			continue
		}

		cols, _ := adapter.GetColumns(ctx, table)
		colNames := make([]string, len(cols))
		for i, c := range cols {
			colNames[i] = adapter.Quote(c.Name)
		}

		where := fmtMapKey(opts.Where, table)
		var count int64
		if err := adapter.SelectAll(ctx, table, nil, where, opts.BatchSize, func(batch []map[string]any) error {
			for _, row := range batch {
				var vals []string
				for _, col := range cols {
					vals = append(vals, formatSQLValue(row[col.Name]))
				}
				buf.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s);\n", adapter.Quote(table), strings.Join(colNames, ", "), strings.Join(vals, ", ")))
			}
			count += int64(len(batch))
			return nil
		}); err != nil {
			return nil, fmt.Errorf("migrate: 导出表 %s 失败: %w", table, err)
		}
		*totalRows += count
		buf.WriteString("\n")
	}

	var tableList []string
	for _, t := range tables {
		if !containsString(tableList, t) {
			tableList = append(tableList, t)
		}
	}
	return &DumpResult{
		Data:       buf.Bytes(),
		Format:     FormatSQL,
		TableCount: len(tableList),
		RowCount:   *totalRows,
		Tables:     tableList,
	}, nil
}

func formatCSVValue(v any) string {
	if v == nil {
		return ""
	}
	s := fmt.Sprintf("%v", v)
	s = strings.ReplaceAll(s, "\"", "\"\"")
	if strings.ContainsAny(s, ",\"\n") {
		return "\"" + s + "\""
	}
	return s
}

func formatSQLValue(v any) string {
	if v == nil {
		return "NULL"
	}
	switch val := v.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return fmt.Sprintf("%d", val)
	case float32, float64:
		return fmt.Sprintf("%v", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	case []byte:
		if val == nil {
			return "NULL"
		}
		return "'" + strings.ReplaceAll(string(val), "'", "''") + "'"
	default:
		return "'" + strings.ReplaceAll(fmt.Sprintf("%v", val), "'", "''") + "'"
	}
}
