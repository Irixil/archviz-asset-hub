package memory

import (
	"context"
	"sync"

	"damask/server/internal/repository"
)

// StorageStatsMemoryRepo is an in-memory implementation of StorageStatsRepository.
// Rows are seeded directly (there's no underlying asset/version model to derive
// aggregates from in-memory) and scoped by workspace at seed time.
type StorageStatsMemoryRepo struct {
	mu          sync.RWMutex
	projectType map[string][]repository.StorageProjectTypeRow
	folderCount map[string][]repository.StorageFolderCountRow
	limitBytes  map[string]*int64
	byFolder    map[string][]repository.StorageFolderRow // key: workspaceID + "|" + projectID
}

// NewStorageStatsRepo creates a new in-memory StorageStatsRepository.
func NewStorageStatsRepo() *StorageStatsMemoryRepo {
	return &StorageStatsMemoryRepo{
		projectType: map[string][]repository.StorageProjectTypeRow{},
		folderCount: map[string][]repository.StorageFolderCountRow{},
		limitBytes:  map[string]*int64{},
		byFolder:    map[string][]repository.StorageFolderRow{},
	}
}

func (r *StorageStatsMemoryRepo) SeedProjectType(workspaceID string, rows ...repository.StorageProjectTypeRow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.projectType[workspaceID] = append(r.projectType[workspaceID], rows...)
}

func (r *StorageStatsMemoryRepo) SeedFolderCount(workspaceID string, rows ...repository.StorageFolderCountRow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.folderCount[workspaceID] = append(r.folderCount[workspaceID], rows...)
}

func (r *StorageStatsMemoryRepo) SeedLimitBytes(workspaceID string, limit *int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.limitBytes[workspaceID] = limit
}

func (r *StorageStatsMemoryRepo) SeedByFolder(workspaceID, projectID string, rows ...repository.StorageFolderRow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byFolder[workspaceID+"|"+projectID] = append(r.byFolder[workspaceID+"|"+projectID], rows...)
}

func (r *StorageStatsMemoryRepo) GetByProjectAndType(
	_ context.Context,
	workspaceID string,
) ([]repository.StorageProjectTypeRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.projectType[workspaceID], nil
}

func (r *StorageStatsMemoryRepo) GetFolderCountsByProject(
	_ context.Context,
	workspaceID string,
) ([]repository.StorageFolderCountRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.folderCount[workspaceID], nil
}

func (r *StorageStatsMemoryRepo) GetStorageLimitBytes(_ context.Context, workspaceID string) (*int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.limitBytes[workspaceID], nil
}

func (r *StorageStatsMemoryRepo) GetByFolder(
	_ context.Context,
	workspaceID, projectID string,
) ([]repository.StorageFolderRow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.byFolder[workspaceID+"|"+projectID], nil
}
