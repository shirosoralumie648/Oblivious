package scheduler

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	defaultMaxConcurrent = 10
)

var (
	ErrInvalidCronExpression = errors.New("cron expression is required")
	ErrInvalidTaskID         = errors.New("task id is required")
	ErrSchedulerNotRunning   = errors.New("scheduler is not running")
	ErrTaskAlreadyScheduled  = errors.New("task is already scheduled")
	ErrMaxConcurrentReached  = errors.New("max concurrent tasks reached")
)

var standardCronParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

type ScheduledEntry struct {
	ID             string         `json:"id"`
	TaskID         string         `json:"taskId"`
	CronExpression string         `json:"cronExpression"`
	Status         TaskStatus     `json:"status"`
	NextRunAt      *time.Time     `json:"nextRunAt,omitempty"`
	LastRunAt      *time.Time     `json:"lastRunAt,omitempty"`
	RunCount       int            `json:"runCount"`
	FailCount      int            `json:"failCount"`
	LastError      string         `json:"lastError,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type TaskHandler func(ctx context.Context, entry ScheduledEntry) error

type CronSchedulerConfig struct {
	MaxConcurrent int
	Now           func() time.Time
	OnError       func(error)
	OnTaskStart   func(entry ScheduledEntry)
	OnTaskEnd     func(entry ScheduledEntry, err error)
}

type CronScheduler struct {
	mu            sync.Mutex
	entries       map[string]*ScheduledEntry
	handlers      map[string]TaskHandler
	cronEngine    *cron.Cron
	maxConcurrent int
	running       int
	now           func() time.Time
	onError       func(error)
	onTaskStart   func(entry ScheduledEntry)
	onTaskEnd     func(entry ScheduledEntry, err error)
	started       bool
	stopCh        chan struct{}
}

func NewCronScheduler(config CronSchedulerConfig) *CronScheduler {
	maxConcurrent := config.MaxConcurrent
	if maxConcurrent <= 0 {
		maxConcurrent = defaultMaxConcurrent
	}
	now := config.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &CronScheduler{
		entries:       make(map[string]*ScheduledEntry),
		handlers:      make(map[string]TaskHandler),
		cronEngine:    cron.New(cron.WithParser(standardCronParser), cron.WithLocation(time.UTC)),
		maxConcurrent: maxConcurrent,
		now:           now,
		onError:       config.OnError,
		onTaskStart:   config.OnTaskStart,
		onTaskEnd:     config.OnTaskEnd,
		stopCh:        make(chan struct{}),
	}
}

func (s *CronScheduler) ParseCron(expression string) (time.Time, error) {
	normalized := strings.TrimSpace(expression)
	if normalized == "" {
		return time.Time{}, ErrInvalidCronExpression
	}

	schedule, err := standardCronParser.Parse(normalized)
	if err != nil {
		return time.Time{}, err
	}

	now := s.now()
	return schedule.Next(now), nil
}

func (s *CronScheduler) Add(taskID string, expression string, handler TaskHandler) (ScheduledEntry, error) {
	normalizedID := strings.TrimSpace(taskID)
	if normalizedID == "" {
		return ScheduledEntry{}, ErrInvalidTaskID
	}

	normalizedExpr := strings.TrimSpace(expression)
	if normalizedExpr == "" {
		return ScheduledEntry{}, ErrInvalidCronExpression
	}

	if handler == nil {
		return ScheduledEntry{}, errors.New("task handler is required")
	}

	if _, err := standardCronParser.Parse(normalizedExpr); err != nil {
		return ScheduledEntry{}, err
	}

	now := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[normalizedID]; exists {
		return ScheduledEntry{}, ErrTaskAlreadyScheduled
	}

	nextRunAt, err := s.parseNextRun(normalizedExpr, now)
	if err != nil {
		return ScheduledEntry{}, err
	}

	entry := &ScheduledEntry{
		ID:             normalizedID,
		TaskID:         normalizedID,
		CronExpression: normalizedExpr,
		Status:         TaskStatusPending,
		NextRunAt:      nextRunAt,
		Metadata:       make(map[string]any),
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	s.entries[normalizedID] = entry
	s.handlers[normalizedID] = handler

	if s.started {
		s.addCronEntry(normalizedID, normalizedExpr, handler)
	}

	return cloneScheduledEntry(entry), nil
}

func (s *CronScheduler) Remove(taskID string) bool {
	normalizedID := strings.TrimSpace(taskID)
	if normalizedID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[normalizedID]; !exists {
		return false
	}

	delete(s.entries, normalizedID)
	delete(s.handlers, normalizedID)

	return true
}

func (s *CronScheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.started {
		return nil
	}

	for id, entry := range s.entries {
		handler := s.handlers[id]
		if handler != nil {
			s.addCronEntry(id, entry.CronExpression, handler)
		}
	}

	s.cronEngine.Start()
	s.started = true
	return nil
}

func (s *CronScheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.started {
		return
	}

	s.cronEngine.Stop()
	s.started = false
	close(s.stopCh)
	s.stopCh = make(chan struct{})
}

func (s *CronScheduler) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.started
}

func (s *CronScheduler) GetEntry(taskID string) (ScheduledEntry, bool) {
	normalizedID := strings.TrimSpace(taskID)
	if normalizedID == "" {
		return ScheduledEntry{}, false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[normalizedID]
	if !exists {
		return ScheduledEntry{}, false
	}

	return cloneScheduledEntry(entry), true
}

func (s *CronScheduler) ListEntries() []ScheduledEntry {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries := make([]ScheduledEntry, 0, len(s.entries))
	for _, entry := range s.entries {
		entries = append(entries, cloneScheduledEntry(entry))
	}

	return entries
}

func (s *CronScheduler) addCronEntry(taskID string, expression string, handler TaskHandler) {
	normalizedID := taskID
	normalizedHandler := handler

	s.cronEngine.AddFunc(expression, func() {
		s.executeTask(normalizedID, normalizedHandler)
	})
}

func (s *CronScheduler) executeTask(taskID string, handler TaskHandler) {
	s.mu.Lock()
	entry, exists := s.entries[taskID]
	if !exists {
		s.mu.Unlock()
		return
	}

	if s.running >= s.maxConcurrent {
		s.mu.Unlock()
		if s.onError != nil {
			s.onError(ErrMaxConcurrentReached)
		}
		return
	}

	entryClone := cloneScheduledEntry(entry)
	entry.Status = TaskStatusRunning
	entry.UpdatedAt = s.now()
	s.running++
	s.mu.Unlock()

	if s.onTaskStart != nil {
		s.onTaskStart(entryClone)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	startTime := s.now()
	err := handler(ctx, entryClone)
	endTime := s.now()

	s.mu.Lock()
	defer s.mu.Unlock()

	s.running--
	entry.UpdatedAt = endTime
	entry.LastRunAt = &endTime
	entry.RunCount++

	if err != nil {
		entry.Status = TaskStatusFailed
		entry.FailCount++
		entry.LastError = err.Error()
	} else {
		entry.Status = TaskStatusCompleted
		entry.LastError = ""
	}

	nextRunAt, nextErr := s.parseNextRun(entry.CronExpression, endTime)
	if nextErr == nil {
		entry.NextRunAt = nextRunAt
	}

	resultEntry := cloneScheduledEntry(entry)

	if s.onTaskEnd != nil {
		s.onTaskEnd(resultEntry, err)
	}

	_ = startTime
}

func (s *CronScheduler) parseNextRun(expression string, after time.Time) (*time.Time, error) {
	schedule, err := standardCronParser.Parse(expression)
	if err != nil {
		return nil, err
	}

	next := schedule.Next(after)
	return &next, nil
}

func cloneScheduledEntry(entry *ScheduledEntry) ScheduledEntry {
	if entry == nil {
		return ScheduledEntry{}
	}

	cloned := *entry
	if entry.Metadata != nil {
		cloned.Metadata = make(map[string]any, len(entry.Metadata))
		for k, v := range entry.Metadata {
			cloned.Metadata[k] = v
		}
	}

	if entry.NextRunAt != nil {
		value := *entry.NextRunAt
		cloned.NextRunAt = &value
	}
	if entry.LastRunAt != nil {
		value := *entry.LastRunAt
		cloned.LastRunAt = &value
	}

	return cloned
}
