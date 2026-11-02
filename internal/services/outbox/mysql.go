package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TableName 外盒事件表名
const TableName = "zzz_integration_outbox"

// MySQLStore 基于 MySQL 的外盒存储
type MySQLStore struct {
	db *sql.DB
}

// NewMySQLStore 创建 MySQL 存储
func NewMySQLStore(db *sql.DB) *MySQLStore {
	return &MySQLStore{db: db}
}

// Append 写入事件
func (s *MySQLStore) Append(ctx context.Context, event Event) error {
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage("null")
	}
	status := event.Status
	if status == "" {
		status = StatusPending
	}
	createdAt := event.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := event.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO `+TableName+`
		(id, topic, source, aggregate_type, aggregate_id, payload, status, attempts, last_error, created_at, updated_at, delivered_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.Topic, event.Source, event.AggregateType, event.AggregateID,
		string(payload), string(status), event.Attempts, event.LastError,
		createdAt, updatedAt, event.DeliveredAt,
	)
	return err
}

// Get 读取单条事件
func (s *MySQLStore) Get(ctx context.Context, id string) (Event, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, topic, source, aggregate_type, aggregate_id,
		payload, status, attempts, last_error, created_at, updated_at, delivered_at
		FROM `+TableName+` WHERE id = ?`, id)
	event, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Event{}, ErrNotFound
	}
	return event, err
}

// List 按条件查询事件
func (s *MySQLStore) List(ctx context.Context, filter Filter) ([]Event, error) {
	var conditions []string
	var args []any
	if filter.Topic != "" {
		conditions = append(conditions, "topic = ?")
		args = append(args, filter.Topic)
	}
	if filter.Source != "" {
		conditions = append(conditions, "source = ?")
		args = append(args, filter.Source)
	}
	if filter.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filter.Status)
	}
	if filter.AggregateType != "" {
		conditions = append(conditions, "aggregate_type = ?")
		args = append(args, filter.AggregateType)
	}
	if filter.AggregateID != "" {
		conditions = append(conditions, "aggregate_id = ?")
		args = append(args, filter.AggregateID)
	}
	if !filter.Since.IsZero() {
		conditions = append(conditions, "created_at >= ?")
		args = append(args, filter.Since)
	}

	query := `SELECT id, topic, source, aggregate_type, aggregate_id,
		payload, status, attempts, last_error, created_at, updated_at, delivered_at
		FROM ` + TableName
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at ASC, id ASC"
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}
	query += " LIMIT ? OFFSET ?"
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	args = append(args, limit, offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make([]Event, 0)
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// MarkDelivered 标记为已投递
func (s *MySQLStore) MarkDelivered(ctx context.Context, id string, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE `+TableName+`
		SET status = ?, last_error = '', updated_at = ?, delivered_at = ?
		WHERE id = ?`, string(StatusDelivered), at, at, id)
	if err != nil {
		return err
	}
	return ensureAffected(result)
}

// MarkFailed 标记失败并累加尝试次数
func (s *MySQLStore) MarkFailed(ctx context.Context, id, reason string, at time.Time) error {
	result, err := s.db.ExecContext(ctx, `UPDATE `+TableName+`
		SET status = ?, attempts = attempts + 1, last_error = ?, updated_at = ?
		WHERE id = ?`, string(StatusFailed), strings.TrimSpace(reason), at, id)
	if err != nil {
		return err
	}
	return ensureAffected(result)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEvent(scanner rowScanner) (Event, error) {
	var (
		event       Event
		payload     []byte
		status      string
		deliveredAt sql.NullTime
	)
	if err := scanner.Scan(
		&event.ID, &event.Topic, &event.Source, &event.AggregateType, &event.AggregateID,
		&payload, &status, &event.Attempts, &event.LastError,
		&event.CreatedAt, &event.UpdatedAt, &deliveredAt,
	); err != nil {
		return Event{}, err
	}
	event.Payload = json.RawMessage(payload)
	event.Status = Status(status)
	if deliveredAt.Valid {
		at := deliveredAt.Time
		event.DeliveredAt = &at
	}
	return event, nil
}

func ensureAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("outbox: 读取影响行数失败: %w", err)
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}
