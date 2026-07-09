package migrate

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
)

func Restore(ctx context.Context, adapter DBAdapter, data []byte, opts RestoreOptions) (*RestoreResult, error) {
	if opts.Format == "" {
		opts.Format = detectFormat(data)
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 500
	}

	result := &RestoreResult{}

	switch opts.Format {
	case FormatJSON:
		return restoreJSON(ctx, adapter, data, opts, result)
	case FormatCSV:
		return restoreCSV(ctx, adapter, data, opts, result)
	case FormatSQL:
		return restoreSQL(ctx, adapter, data, opts, result)
	default:
		return nil, fmt.Errorf("migrate: 不支持的导入格式: %s", opts.Format)
	}
}

func restoreJSON(ctx context.Context, adapter DBAdapter, data []byte, opts RestoreOptions, result *RestoreResult) (*RestoreResult, error) {
	var tablesData map[string][]map[string]any
	if err := json.Unmarshal(data, &tablesData); err != nil {
		return nil, fmt.Errorf("migrate: JSON 解析失败: %w", err)
	}

	for table, rows := range tablesData {
		if len(opts.Tables) > 0 && !containsString(opts.Tables, table) {
			continue
		}

		if len(rows) == 0 {
			continue
		}

		if len(rows) == 1 {
			if _, hasSchema := rows[0]["_schema"]; hasSchema {
				continue
			}
		}

		if opts.Truncate {
			if err := adapter.Truncate(ctx, table); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("清空表 %s 失败: %v", table, err))
				continue
			}
		}

		var columns []string
		for k := range rows[0] {
			columns = append(columns, k)
		}

		tableCols, err := adapter.GetColumns(ctx, table)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("获取表 %s 列信息失败: %v", table, err))
			continue
		}
		colMap := make(map[string]bool)
		for _, c := range tableCols {
			colMap[c.Name] = true
		}

		var validCols []string
		for _, col := range columns {
			if colMap[col] {
				validCols = append(validCols, col)
			}
		}

		if opts.SkipExisting {
			var existing int64
			_ = adapter.SelectAll(ctx, table, []string{"COUNT(*) AS cnt"}, "", 1, func(batch []map[string]any) error {
				if len(batch) > 0 {
					existing = 1
				}
				return nil
			})
			if existing > 0 {
				result.TableCount++
				continue
			}
		}

		for i := 0; i < len(rows); i += opts.BatchSize {
			end := i + opts.BatchSize
			if end > len(rows) {
				end = len(rows)
			}
			batch := rows[i:end]
			if err := adapter.InsertBatch(ctx, table, validCols, batch); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("导入表 %s 第 %d 行失败: %v", table, i, err))
				continue
			}
			result.RowCount += int64(len(batch))
		}
		result.TableCount++
	}

	return result, nil
}

func restoreCSV(ctx context.Context, adapter DBAdapter, data []byte, opts RestoreOptions, result *RestoreResult) (*RestoreResult, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	allRecords, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("migrate: CSV 解析失败: %w", err)
	}
	if len(allRecords) < 2 {
		return nil, fmt.Errorf("migrate: CSV 数据不足（需至少表头+1行）")
	}

	headers := allRecords[0]
	var tableName string
	for _, h := range headers {
		if strings.HasPrefix(h, "TABLE: ") {
			tableName = strings.TrimPrefix(h, "TABLE: ")
			headers = allRecords[1]
			allRecords = allRecords[1:]
			break
		}
	}

	if tableName == "" {
		tableName = "data"
	}

	if len(opts.Tables) > 0 && !containsString(opts.Tables, tableName) {
		return result, nil
	}

	var rows []map[string]any
	for _, record := range allRecords[1:] {
		if len(record) == 0 {
			continue
		}
		if len(record) != len(headers) {
			continue
		}
		row := make(map[string]any)
		for i, header := range headers {
			if strings.HasPrefix(header, "--") {
				continue
			}
			row[header] = record[i]
		}
		if len(row) > 0 {
			rows = append(rows, row)
		}
	}

	if opts.Truncate {
		_ = adapter.Truncate(ctx, tableName)
	}

	for i := 0; i < len(rows); i += opts.BatchSize {
		end := i + opts.BatchSize
		if end > len(rows) {
			end = len(rows)
		}
		if err := adapter.InsertBatch(ctx, tableName, headers, rows[i:end]); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("导入 CSV 第 %d 行失败: %v", i, err))
			continue
		}
		result.RowCount += int64(len(rows[i:end]))
	}
	result.TableCount++

	return result, nil
}

func restoreSQL(ctx context.Context, adapter DBAdapter, data []byte, opts RestoreOptions, result *RestoreResult) (*RestoreResult, error) {
	statements := splitSQL(string(data))
	var filtered []string
	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt)
		if trimmed == "" {
			continue
		}
		if len(opts.Tables) > 0 {
			upper := strings.ToUpper(trimmed)
			matched := false
			for _, table := range opts.Tables {
				if strings.Contains(upper, strings.ToUpper(table)) {
					matched = true
					break
				}
			}
			if !matched {
				continue
			}
		}
		filtered = append(filtered, trimmed)
	}

	for _, stmt := range filtered {
		upper := strings.ToUpper(stmt)
		if opts.SkipExisting && strings.HasPrefix(upper, "INSERT") {
			tableName := extractTableFromInsert(stmt)
			if tableName != "" {
				var dummy int64
				_ = adapter.SelectAll(ctx, tableName, []string{"COUNT(*) AS cnt"}, "", 1, func(batch []map[string]any) error {
					if len(batch) > 0 {
						dummy = 1
					}
					return nil
				})
				if dummy > 0 {
					continue
				}
			}
		}
		if err := adapter.Exec(ctx, stmt); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("执行 SQL 失败: %s: %v", truncateStr(stmt, 80), err))
			continue
		}
		result.RowCount++
	}
	result.TableCount = len(filtered)

	return result, nil
}

func extractTableFromInsert(stmt string) string {
	upper := strings.ToUpper(stmt)
	idx := strings.Index(upper, "INTO")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimSpace(stmt[idx+4:])
	end := strings.IndexAny(rest, " (")
	if end < 0 {
		end = len(rest)
	}
	return strings.Trim(strings.Trim(rest[:end], "`\"'"), " ")
}
