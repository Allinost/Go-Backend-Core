package developmentmanager_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Allinost/go-backend-core/internal/testutil"
	dm "github.com/Allinost/go-backend-core/zzz/developmentManager"
)

func TestPMRepositoryCreateProjectAndTask(t *testing.T) {
	db := testutil.OpenPG(t)
	require.NoError(t, testutil.ApplyMigrations(t, db))

	repo := dm.NewRepository(db)
	ctx := context.Background()

	project, err := repo.CreateProject(ctx, &dm.Project{
		ProjectKey: "itest-pm",
		Name:       "Integration PM",
		Type:       dm.ProjectTypeBase,
		Status:     dm.ProjectStatusActive,
		CreatedBy:  "test",
		UpdatedBy:  "test",
	})
	require.NoError(t, err)
	require.NotZero(t, project.ID)

	pmRepo := dm.NewPMRepository(db)
	task, err := pmRepo.CreateTask(ctx, &dm.Task{
		ProjectID: project.ID,
		Code:      "ITEST-1",
		Title:     "Integration task",
		Type:      dm.TaskTypeFeature,
		Status:    dm.TaskStatusTodo,
		Priority:  "P2",
		CreatedBy: "test",
		UpdatedBy: "test",
	})
	require.NoError(t, err)
	require.NotZero(t, task.ID)

	got, err := pmRepo.GetTask(ctx, task.ID)
	require.NoError(t, err)
	require.Equal(t, "ITEST-1", got.Code)
	require.Equal(t, int64(project.ID), got.ProjectID)
}
