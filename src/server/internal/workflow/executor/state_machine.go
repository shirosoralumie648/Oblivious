package executor

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidStateTransition = errors.New("invalid workflow state transition")
	ErrStateMachineLocked     = errors.New("workflow state machine is locked")
)

// StateMachine manages execution status transitions with event-driven rules.
type StateMachine struct {
	mu            sync.RWMutex
	currentStatus string
	transitions   map[stateEventPair]string
	history       []TransitionRecord
	locked        bool
}

// stateEventPair is the key for the transition lookup table.
type stateEventPair struct {
	From  string
	Event string
}

// TransitionRecord records a single state transition.
type TransitionRecord struct {
	From      string    `json:"from"`
	To        string    `json:"to"`
	Event     string    `json:"event"`
	Timestamp time.Time `json:"timestamp"`
}

// NewStateMachine creates a state machine initialized to "draft" with standard transitions.
func NewStateMachine() *StateMachine {
	sm := &StateMachine{
		currentStatus: "draft",
		transitions:   defaultTransitions(),
		history:       []TransitionRecord{},
	}
	return sm
}

// NewStateMachineWithStatus creates a state machine initialized to the given status.
func NewStateMachineWithStatus(status string) *StateMachine {
	sm := NewStateMachine()
	sm.currentStatus = normalizeStatus(status)
	return sm
}

// CurrentStatus returns the current state.
func (sm *StateMachine) CurrentStatus() string {
	if sm == nil {
		return ""
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.currentStatus
}

// History returns the full transition history.
func (sm *StateMachine) History() []TransitionRecord {
	if sm == nil {
		return nil
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	copied := make([]TransitionRecord, len(sm.history))
	copy(copied, sm.history)
	return copied
}

// CanTransition checks if a given event can be applied in the current state.
func (sm *StateMachine) CanTransition(event string) bool {
	if sm == nil {
		return false
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	if sm.locked {
		return false
	}
	_, ok := sm.transitions[stateEventPair{
		From:  sm.currentStatus,
		Event: normalizeEvent(event),
	}]
	return ok
}

// Transition applies an event and returns the new status, or an error if invalid.
func (sm *StateMachine) Transition(event string) (string, error) {
	if sm == nil {
		return "", fmt.Errorf("%w: state machine is nil", ErrInvalidStateTransition)
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.locked {
		return sm.currentStatus, fmt.Errorf("%w: state machine is locked at %s", ErrStateMachineLocked, sm.currentStatus)
	}

	normalizedEvent := normalizeEvent(event)
	key := stateEventPair{
		From:  sm.currentStatus,
		Event: normalizedEvent,
	}
	newStatus, ok := sm.transitions[key]
	if !ok {
		return sm.currentStatus, fmt.Errorf("%w: cannot apply event %q in state %q", ErrInvalidStateTransition, normalizedEvent, sm.currentStatus)
	}

	record := TransitionRecord{
		From:      sm.currentStatus,
		To:        newStatus,
		Event:     normalizedEvent,
		Timestamp: time.Now().UTC(),
	}
	sm.history = append(sm.history, record)
	sm.currentStatus = newStatus

	return newStatus, nil
}

// IsTerminal returns true if the current status is a terminal state.
func (sm *StateMachine) IsTerminal() bool {
	if sm == nil {
		return false
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return isTerminalStatus(sm.currentStatus)
}

// AvailableEvents returns the list of events that can be applied in the current state.
func (sm *StateMachine) AvailableEvents() []string {
	if sm == nil {
		return nil
	}
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	events := []string{}
	for pair := range sm.transitions {
		if pair.From == sm.currentStatus {
			events = append(events, pair.Event)
		}
	}
	return events
}

// Lock prevents further transitions. Used when execution enters a terminal state.
func (sm *StateMachine) Lock() {
	if sm == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.locked = true
}

// Unlock re-enables transitions.
func (sm *StateMachine) Unlock() {
	if sm == nil {
		return
	}
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.locked = false
}

// defaultTransitions returns the standard workflow state machine transitions.
func defaultTransitions() map[stateEventPair]string {
	return map[stateEventPair]string{
		// draft -> running
		{From: "draft", Event: "start"}: "running",

		// running -> paused
		{From: "running", Event: "pause"}: "paused",

		// running -> completed/succeeded
		{From: "running", Event: "complete"}: "succeeded",

		// running -> failed
		{From: "running", Event: "fail"}: "failed",

		// running -> timeout
		{From: "running", Event: "timeout"}: "timeout",

		// running -> cancelled
		{From: "running", Event: "cancel"}: "cancelled",

		// paused -> running (resume)
		{From: "paused", Event: "resume"}: "running",

		// paused -> cancelled
		{From: "paused", Event: "cancel"}: "cancelled",

		// queued -> running
		{From: "queued", Event: "start"}: "running",

		// queued -> cancelled
		{From: "queued", Event: "cancel"}: "cancelled",

		// partial_success -> running (re-run remaining)
		{From: "partial_success", Event: "start"}: "running",

		// partial_success -> completed
		{From: "partial_success", Event: "complete"}: "succeeded",

		// partial_success -> cancelled
		{From: "partial_success", Event: "cancel"}: "cancelled",
	}
}

func isTerminalStatus(status string) bool {
	switch normalizeStatus(status) {
	case "succeeded", "completed", "failed", "cancelled", "timeout", "max_iterations":
		return true
	default:
		return false
	}
}

func normalizeStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func normalizeEvent(event string) string {
	return strings.ToLower(strings.TrimSpace(event))
}
