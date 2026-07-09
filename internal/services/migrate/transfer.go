package migrate

import (
	"context"
	"fmt"
)

func Transfer(ctx context.Context, src, dst DBAdapter, opts TransferOptions) (*TransferResult, error) {
	tables := opts.Tables
	if len(tables) == 0 {
		var err error
		tables, err = src.GetTables(ctx)
		if err != nil {
			return nil, fmt.Errorf("migrate: 获取源数据库表列表失败: %w", err)
		}
	}

	if opts.BatchSize <= 0 {
		opts.BatchSize = 500
	}

	result := &TransferResult{}

	for _, table := range tables {
		if err := transferTable(ctx, src, dst, table, opts, result); err != nil {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	return result, nil
}

func transferTable(ctx context.Context, src, dst DBAdapter, table string, opts TransferOptions, result *TransferResult) error {
	schema, err := src.GetTableSchema(ctx, table)
	if err != nil {
		return fmt.Errorf("获取表 %s 结构失败: %w", table, err)
	}

	if opts.CreateTable || opts.DropTarget {
		if opts.DropTarget {
			_ = dst.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", dst.Quote(table)))
		}
		if err := dst.CreateTableIfNotExists(ctx, schema); err != nil {
			return fmt.Errorf("创建表 %s 失败: %w", table, err)
		}
	}

	colNames := make([]string, len(schema.Columns))
	for i, c := range schema.Columns {
		colNames[i] = c.Name
	}

	var rowCount int64
	if err := src.SelectAll(ctx, table, nil, "", opts.BatchSize, func(batch []map[string]any) error {
		if err := dst.InsertBatch(ctx, table, colNames, batch); err != nil {
			return fmt.Errorf("写入表 %s 失败: %w", table, err)
		}
		rowCount += int64(len(batch))
		return nil
	}); err != nil {
		return fmt.Errorf("迁移表 %s 失败: %w", table, err)
	}

	result.TableCount++
	result.RowCount += rowCount
	return nil
}
