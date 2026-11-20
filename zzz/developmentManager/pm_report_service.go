package developmentmanager

import (
	"context"
	"time"
)

func (s *PMService) Dashboard(ctx context.Context, projectID int64) (*ProjectDashboard, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.Dashboard(ctx, projectID)
}

func (s *PMService) Board(ctx context.Context, projectID int64) ([]BoardColumn, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.Board(ctx, projectID)
}

func (s *PMService) Roadmap(ctx context.Context, projectID int64) ([]RoadmapItem, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.Roadmap(ctx, projectID)
}

func (s *PMService) WeeklyReport(ctx context.Context, projectID int64, from, to time.Time) (*WeeklyReport, error) {
	if err := s.ensureProject(ctx, projectID); err != nil {
		return nil, err
	}
	if to.Before(from) {
		return nil, NewDevelopmentManagerError(ErrInvalidTaskTransition).WithDetail("to 必须晚于 from")
	}
	return s.repo.WeeklyReport(ctx, projectID, from, to)
}
