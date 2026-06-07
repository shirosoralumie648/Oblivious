package version

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrVersionNotFound     = errors.New("workflow version not found")
	ErrVersionExists       = errors.New("workflow version already exists")
	ErrCannotDeleteCurrent = errors.New("cannot delete the current published version")
	ErrNoPublishedVersion  = errors.New("workflow has no published version")
)

// Manager manages workflow versions, isolation, rollback, and branching.
type Manager struct {
	mu       sync.RWMutex
	versions map[string][]*VersionEntry
}

// VersionEntry represents a single version of a workflow.
type VersionEntry struct {
	WorkflowID  string         `json:"workflowId"`
	Version     int            `json:"version"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Status      string         `json:"status"`
	BranchName  string         `json:"branchName,omitempty"`
	Definition  map[string]any `json:"definition,omitempty"`
	Variables   map[string]any `json:"variables,omitempty"`
	Changelog   string         `json:"changelog,omitempty"`
	IsPublished bool           `json:"isPublished"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

// VersionIsolation binds an execution to a specific version snapshot.
type VersionIsolation struct {
	ExecutionID  string         `json:"executionId"`
	WorkflowID   string         `json:"workflowId"`
	Version      int            `json:"version"`
	Snapshot     map[string]any `json:"snapshot"`
	BoundAt      time.Time      `json:"boundAt"`
}

// NewManager creates a new version manager.
func NewManager() *Manager {
	return &Manager{
		versions: make(map[string][]*VersionEntry),
	}
}

// CreateVersion creates a new version for a workflow.
func (m *Manager) CreateVersion(workflowID, name, description string, definition, variables map[string]any) (*VersionEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	workflowID = strings.TrimSpace(workflowID)
	if workflowID == "" {
		return nil, fmt.Errorf("workflow ID is required")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("version name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.versions[workflowID]
	nextVersion := 1
	if len(versions) > 0 {
		nextVersion = versions[len(versions)-1].Version + 1
	}

	now := time.Now().UTC()
	entry := &VersionEntry{
		WorkflowID:  workflowID,
		Version:     nextVersion,
		Name:        name,
		Description: strings.TrimSpace(description),
		Status:      "draft",
		Definition:  cloneMap(definition),
		Variables:   cloneMap(variables),
		IsPublished: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.versions[workflowID] = append(m.versions[workflowID], entry)

	return cloneVersionEntry(entry), nil
}

// GetVersion returns a specific version of a workflow.
func (m *Manager) GetVersion(workflowID string, version int) (*VersionEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry := m.findVersion(workflowID, version)
	if entry == nil {
		return nil, fmt.Errorf("%w: workflow %s version %d", ErrVersionNotFound, workflowID, version)
	}
	return cloneVersionEntry(entry), nil
}

// ListVersions returns all versions for a workflow.
func (m *Manager) ListVersions(workflowID string) ([]*VersionEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[strings.TrimSpace(workflowID)]
	result := make([]*VersionEntry, len(versions))
	for i, v := range versions {
		result[i] = cloneVersionEntry(v)
	}
	return result, nil
}

// GetLatestVersion returns the latest version for a workflow.
func (m *Manager) GetLatestVersion(workflowID string) (*VersionEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[strings.TrimSpace(workflowID)]
	if len(versions) == 0 {
		return nil, fmt.Errorf("%w: workflow %s", ErrVersionNotFound, workflowID)
	}
	return cloneVersionEntry(versions[len(versions)-1]), nil
}

// GetPublishedVersion returns the latest published version for a workflow.
func (m *Manager) GetPublishedVersion(workflowID string) (*VersionEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[strings.TrimSpace(workflowID)]
	var published *VersionEntry
	for _, v := range versions {
		if v.IsPublished {
			published = v
		}
	}
	if published == nil {
		return nil, fmt.Errorf("%w: workflow %s", ErrNoPublishedVersion, workflowID)
	}
	return cloneVersionEntry(published), nil
}

// PublishVersion marks a specific version as published and un-publishes all others.
func (m *Manager) PublishVersion(workflowID string, version int) (*VersionEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.findVersion(workflowID, version)
	if entry == nil {
		return nil, fmt.Errorf("%w: workflow %s version %d", ErrVersionNotFound, workflowID, version)
	}

	now := time.Now().UTC()
	versions := m.versions[strings.TrimSpace(workflowID)]
	for _, v := range versions {
		if v.Version != version && v.IsPublished {
			v.IsPublished = false
			v.Status = "draft"
			v.UpdatedAt = now
		}
	}

	entry.IsPublished = true
	entry.Status = "published"
	entry.UpdatedAt = now

	return cloneVersionEntry(entry), nil
}

// Rollback reverts a workflow to a specific version by creating a new version from the target version's definition.
func (m *Manager) Rollback(workflowID string, targetVersion int) (*VersionEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	target := m.findVersion(workflowID, targetVersion)
	if target == nil {
		return nil, fmt.Errorf("%w: workflow %s version %d", ErrVersionNotFound, workflowID, targetVersion)
	}

	versions := m.versions[strings.TrimSpace(workflowID)]
	nextVersion := 1
	if len(versions) > 0 {
		nextVersion = versions[len(versions)-1].Version + 1
	}

	now := time.Now().UTC()
	entry := &VersionEntry{
		WorkflowID:  strings.TrimSpace(workflowID),
		Version:     nextVersion,
		Name:        target.Name,
		Description: target.Description,
		Status:      "draft",
		BranchName:  "",
		Definition:  cloneMap(target.Definition),
		Variables:   cloneMap(target.Variables),
		Changelog:   fmt.Sprintf("Rolled back from version %d", targetVersion),
		IsPublished: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.versions[strings.TrimSpace(workflowID)] = append(versions, entry)

	return cloneVersionEntry(entry), nil
}

// CreateBranch creates a branch from a specific version.
func (m *Manager) CreateBranch(workflowID string, sourceVersion int, branchName, description string) (*VersionEntry, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	branchName = strings.TrimSpace(branchName)
	if branchName == "" {
		return nil, fmt.Errorf("branch name is required")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	source := m.findVersion(workflowID, sourceVersion)
	if source == nil {
		return nil, fmt.Errorf("%w: workflow %s version %d", ErrVersionNotFound, workflowID, sourceVersion)
	}

	versions := m.versions[strings.TrimSpace(workflowID)]
	nextVersion := 1
	if len(versions) > 0 {
		nextVersion = versions[len(versions)-1].Version + 1
	}

	now := time.Now().UTC()
	desc := strings.TrimSpace(description)
	if desc == "" {
		desc = fmt.Sprintf("Branch '%s' from version %d", branchName, sourceVersion)
	}

	entry := &VersionEntry{
		WorkflowID:  strings.TrimSpace(workflowID),
		Version:     nextVersion,
		Name:        fmt.Sprintf("%s (%s)", source.Name, branchName),
		Description: desc,
		Status:      "draft",
		BranchName:  branchName,
		Definition:  cloneMap(source.Definition),
		Variables:   cloneMap(source.Variables),
		Changelog:   fmt.Sprintf("Branched from version %d as '%s'", sourceVersion, branchName),
		IsPublished: false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	m.versions[strings.TrimSpace(workflowID)] = append(versions, entry)

	return cloneVersionEntry(entry), nil
}

// DeleteVersion deletes a draft version. Published versions cannot be deleted.
func (m *Manager) DeleteVersion(workflowID string, version int) error {
	if m == nil {
		return fmt.Errorf("version manager is nil")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := m.findVersion(workflowID, version)
	if entry == nil {
		return fmt.Errorf("%w: workflow %s version %d", ErrVersionNotFound, workflowID, version)
	}
	if entry.IsPublished {
		return ErrCannotDeleteCurrent
	}

	versions := m.versions[strings.TrimSpace(workflowID)]
	for i, v := range versions {
		if v.Version == version {
			m.versions[strings.TrimSpace(workflowID)] = append(versions[:i], versions[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("%w: workflow %s version %d", ErrVersionNotFound, workflowID, version)
}

// BindExecutionToVersion creates a version isolation binding for an execution.
func (m *Manager) BindExecutionToVersion(executionID, workflowID string, version int) (*VersionIsolation, error) {
	if m == nil {
		return nil, fmt.Errorf("version manager is nil")
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	entry := m.findVersion(workflowID, version)
	if entry == nil {
		return nil, fmt.Errorf("%w: workflow %s version %d", ErrVersionNotFound, workflowID, version)
	}

	return &VersionIsolation{
		ExecutionID: executionID,
		WorkflowID:  strings.TrimSpace(workflowID),
		Version:     version,
		Snapshot:    cloneMap(entry.Definition),
		BoundAt:     time.Now().UTC(),
	}, nil
}

// NextVersionNumber returns the next version number for a workflow.
func (m *Manager) NextVersionNumber(workflowID string) int {
	if m == nil {
		return 1
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions := m.versions[strings.TrimSpace(workflowID)]
	if len(versions) == 0 {
		return 1
	}
	return versions[len(versions)-1].Version + 1
}

func (m *Manager) findVersion(workflowID string, version int) *VersionEntry {
	workflowID = strings.TrimSpace(workflowID)
	for _, v := range m.versions[workflowID] {
		if v.Version == version {
			return v
		}
	}
	return nil
}

func cloneVersionEntry(entry *VersionEntry) *VersionEntry {
	if entry == nil {
		return nil
	}
	cloned := *entry
	cloned.Definition = cloneMap(entry.Definition)
	cloned.Variables = cloneMap(entry.Variables)
	return &cloned
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	result := make(map[string]any, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
