package ops

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Allinost/go-backend-core/internal/pkg/dbutil"
	"github.com/jackc/pgx/v5/pgconn"
)

const tableMaintenance = "zzz_ops_maintenance"

type MaintenanceRepository struct {
	db *sql.DB
}

func NewMaintenanceRepository(db *sql.DB) *MaintenanceRepository {
	return &MaintenanceRepository{db: db}
}

func (r *MaintenanceRepository) queryRow(ctx context.Context, query string, args ...any) *sql.Row {
	return r.db.QueryRowContext(ctx, dbutil.Rebind(query), args...)
}

func (r *MaintenanceRepository) query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return r.db.QueryContext(ctx, dbutil.Rebind(query), args...)
}

func (r *MaintenanceRepository) exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, dbutil.Rebind(query), args...)
}

func scanMaintenance(row interface{ Scan(dest ...any) error }) (*Maintenance, error) {
	var m Maintenance
	var endedAt sql.NullTime
	if err := row.Scan(&m.ID, &m.Mode, &m.Status, &m.SourceNode, &m.TargetNode,
		&m.SwitchedToL2, &m.SyncConfirmed, &m.SyncBackConfirmed, &m.Note,
		&m.StartedBy, &m.StartedAt, &m.EndedBy, &endedAt, &m.EndNote,
		&m.CreatedAt, &m.UpdatedAt); err != nil {
		return nil, err
	}
	if endedAt.Valid {
		t := endedAt.Time
		m.EndedAt = &t
	}
	return &m, nil
}

const maintenanceColumns = `id, mode, status, source_node, target_node, switched_to_l2,
	sync_confirmed, sync_back_confirmed, note, started_by, started_at, ended_by, ended_at, end_note,
	created_at, updated_at`

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func (r *MaintenanceRepository) Get(ctx context.Context, id int64) (*Maintenance, error) {
	m, err := scanMaintenance(r.queryRow(ctx, `SELECT `+maintenanceColumns+` FROM `+tableMaintenance+` WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrMaintenanceNotFound
	}
	return m, err
}

func (r *MaintenanceRepository) Active(ctx context.Context) (*Maintenance, error) {
	m, err := scanMaintenance(r.queryRow(ctx, `SELECT `+maintenanceColumns+` FROM `+tableMaintenance+`
		WHERE status = ? ORDER BY id DESC LIMIT 1`, MaintenanceStatusInProgress))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return m, err
}

func (r *MaintenanceRepository) List(ctx context.Context, limit int) ([]Maintenance, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.query(ctx, `SELECT `+maintenanceColumns+` FROM `+tableMaintenance+` ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Maintenance
	for rows.Next() {
		m, err := scanMaintenance(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *m)
	}
	return items, rows.Err()
}

func (r *MaintenanceRepository) Start(ctx context.Context, m *Maintenance) (*Maintenance, error) {
	var id int64
	err := r.queryRow(ctx, `INSERT INTO `+tableMaintenance+`
		(mode, status, source_node, target_node, switched_to_l2, sync_confirmed, sync_back_confirmed, note, started_by)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		m.Mode, m.Status, m.SourceNode, m.TargetNode, m.SwitchedToL2, m.SyncConfirmed, m.SyncBackConfirmed, m.Note, m.StartedBy).Scan(&id)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrMaintenanceActive
		}
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *MaintenanceRepository) End(ctx context.Context, id int64, switchedToL2, syncBackConfirmed bool, targetNode, operator, note string) (*Maintenance, error) {
	now := time.Now()
	res, err := r.exec(ctx, `UPDATE `+tableMaintenance+`
		SET status = ?, switched_to_l2 = ?, sync_back_confirmed = ?, target_node = ?,
		    ended_by = ?, ended_at = ?, end_note = ?, updated_at = ?
		WHERE id = ? AND status = ?`,
		MaintenanceStatusCompleted, switchedToL2, syncBackConfirmed, targetNode,
		operator, now, note, now, id, MaintenanceStatusInProgress)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, ErrMaintenanceNotFound
	}
	return r.Get(ctx, id)
}
